package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/publication"
	"github.com/toddwbucy/Aletharsis/internal/reveal"
)

func TestRevealEndToEnd(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("verified no-atime acquisition is Linux-only")
	}
	samples := map[string][]byte{"bidi": []byte("invoice\u202egnp.txt"), "clean": []byte("plain\r\n"), "zero": []byte("a\u200bb"), "binary": []byte(strings.Repeat("\u200b\u200c", 32)), "multilingual": []byte("日本語 café e\u0301 👩\u200d💻\r\n"), "utf16": {0xff, 0xfe, 'a', 0, 0x0b, 0x20}, "utf32": {0, 0, 0xfe, 0xff, 0, 0, 0, 97, 0, 0, 0x20, 0x0b}}
	for name, data := range samples {
		for _, schema := range []string{"1.0", "2.0"} {
			t.Run(name+schema, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "source.txt")
				if err := os.WriteFile(source, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(source, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(source)
				if err != nil {
					t.Fatal(err)
				}
				var original bytes.Buffer
				baseCode := Run([]string{"audit", source, "--schema-version", schema, "--json"}, &original, io.Discard)
				var firstManifest []byte
				for _, bundle := range []string{"review-a", "review-b"} {
					output := filepath.Join(dir, bundle)
					var stdout, stderr bytes.Buffer
					code := Run([]string{"audit", source, "--schema-version", schema, "--json", "--reveal-out", output}, &stdout, &stderr)
					if code != baseCode || code == 4 || !bytes.Equal(stdout.Bytes(), original.Bytes()) || stderr.Len() != 0 {
						t.Fatalf("report changed: code=%d baseline=%d stderr=%q", code, baseCode, stderr.Bytes())
					}
					raw, err := os.ReadFile(filepath.Join(output, "manifest.json"))
					if err != nil {
						t.Fatal(err)
					}
					if firstManifest != nil && !bytes.Equal(firstManifest, raw) {
						t.Fatal("bundle nondeterministic")
					}
					firstManifest = raw
					assertASCIIJSON(t, raw)
					var m publication.Manifest
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatal(err)
					}
					if m.SourceSHA256 != evidence.Hash(data) || m.ReportSHA256 != evidence.Hash(stdout.Bytes()) || len(m.Artifacts) != 5 {
						t.Fatal("broken source/report chain")
					}
					for _, entry := range m.Artifacts {
						raw, err := os.ReadFile(filepath.Join(output, entry.Name))
						if err != nil {
							t.Fatal(err)
						}
						if evidence.Hash(raw) != entry.SHA256 || len(raw) != entry.Size {
							t.Fatal("artifact digest mismatch")
						}
					}
					raw, err = os.ReadFile(filepath.Join(output, "comparison.json"))
					if err != nil {
						t.Fatal(err)
					}
					assertASCIIJSON(t, raw)
					var comparison reveal.Comparison
					if err := json.Unmarshal(raw, &comparison); err != nil {
						t.Fatal(err)
					}
					if name == "bidi" && comparison.DecodedText != string(data) {
						t.Fatal("JSON escaping changed decoded evidence")
					}
					if comparison.Reveal.SourceSHA256 != m.SourceSHA256 {
						t.Fatal("comparison has different source")
					}
					if name == "zero" && comparison.Reveal.Text != "a⟦U+200B ZERO WIDTH SPACE⟧b" {
						t.Fatal("incorrect revealed text")
					}
					if name == "clean" && (comparison.Reveal.Text != string(data) || comparison.Faithful.Text != "") {
						t.Fatal("clean text changed")
					}
				}
				after, err := os.Stat(source)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before.Sys(), after.Sys()) {
					t.Fatal("source timestamps or identity changed")
				}
				unchanged, err := os.ReadFile(source)
				if err != nil || !bytes.Equal(data, unchanged) {
					t.Fatal("source bytes changed", err)
				}
			})
		}
	}
}
func TestRevealRejectsDestinations(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux reader")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	data := []byte("a\u200bb")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "existing")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(dir, "hard")
	if err := os.Link(source, hard); err != nil {
		t.Fatal(err)
	}
	sym := filepath.Join(dir, "sym")
	if err := os.Symlink(existing, sym); err != nil {
		t.Fatal(err)
	}
	for _, dest := range []string{source, hard, sym, existing, filepath.Join(dir, "missing", "new")} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"audit", source, "--reveal-out", dest}, &stdout, &stderr); code != 4 || !strings.Contains(stderr.String(), "output.publish_failed") {
			t.Fatalf("accepted destination %q: %d %q", dest, code, stderr.Bytes())
		}
	}
	entries, err := os.ReadDir(existing)
	if err != nil || len(entries) != 0 {
		t.Fatal("wrote through link", err)
	}
	intact, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(intact, data) {
		t.Fatal("overwrote source", err)
	}
}
func TestRevealUsageAndFailedAudit(t *testing.T) {
	for _, args := range [][]string{{"audit", "unused", "--reveal-out"}, {"audit", "unused", "--reveal-out="}, {"audit", "unused", "--reveal-out", "-h"}, {"unicode", "unused", "--reveal-out", "new"}, {"audit", "unused", "--output", "x", "--reveal-out", "new"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 4 || out.Len() != 0 {
			t.Fatal("invalid invocation accepted")
		}
	}
	for _, schema := range []string{"1.0", "2.0"} {
		dir := t.TempDir()
		source := filepath.Join(dir, "bad.txt")
		output := filepath.Join(dir, "new")
		if err := os.WriteFile(source, []byte{0xff}, 0600); err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		if Run([]string{"audit", source, "--schema-version", schema, "--json", "--reveal-out", output}, &out, &stderr) != 4 || !json.Valid(out.Bytes()) {
			t.Fatal("failed audit lost report")
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatal("failed audit published derivative")
		}
	}
}

func TestRevealStdoutFailureRetainsCompleteBundle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux reader")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	output := filepath.Join(dir, "review")
	if err := os.WriteFile(source, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := Run([]string{"audit", source, "--reveal-out=" + output}, failingWriter{}, &stderr); code != 4 || !strings.Contains(stderr.String(), "bundle was published") {
		t.Fatal("ambiguous delivery failure")
	}
	if _, err := os.Stat(filepath.Join(output, "manifest.json")); err != nil {
		t.Fatal("complete bundle removed", err)
	}
}

func TestRevealAcceptsResolvedParentAlias(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux acquisition")
	}
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(parent, "source.txt")
	if err := os.WriteFile(source, []byte("plain"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []bool{false, true} {
		input := source
		name := "single"
		if directory {
			input = filepath.Join(parent, "corpus")
			name = "corpus"
			if err := os.Mkdir(input, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(input, "a.txt"), []byte("plain"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		var out, stderr bytes.Buffer
		if code := Run([]string{"audit", input, "--reveal-out", filepath.Join(alias, name)}, &out, &stderr); code != 0 {
			t.Fatal(code, stderr.String())
		}
		if _, err := os.Stat(filepath.Join(outside, name, "manifest.json")); err != nil {
			t.Fatal(err)
		}
	}
}

func assertASCIIJSON(t *testing.T, raw []byte) {
	t.Helper()
	if !json.Valid(raw) {
		t.Fatal("invalid JSON artifact")
	}
	for _, b := range raw {
		if b > 127 {
			t.Fatal("raw Unicode in presentation JSON artifact")
		}
	}
}
