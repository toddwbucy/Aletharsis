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

	"github.com/toddwbucy/Aletharsis/internal/corpus"
)

func corpusFixture(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux no-atime reader")
	}
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"clean.txt": []byte("clean"), "zero.txt": []byte("a\u200bb"), "bad.txt": {0xff}, "pdf.txt": []byte("%PDF-1.7\n"), "nested/code.rs": []byte("fn main() {}"), "nested/\x1b[31m.txt": []byte("clean")}
	for relative, data := range files {
		if err := os.WriteFile(filepath.Join(dir, relative), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	return dir, files
}
func TestDirectoryCLIFormatsAndIntegrity(t *testing.T) {
	dir, files := corpusFixture(t)
	paths := []string{dir, filepath.Join(dir, "nested")}
	for relative := range files {
		paths = append(paths, filepath.Join(dir, relative))
	}
	before := map[string]os.FileInfo{}
	for _, path := range paths {
		if err := os.Chtimes(path, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = info
	}
	for _, schema := range []string{"1.0", "2.0"} {
		var document bytes.Buffer
		var stderr bytes.Buffer
		args := []string{"audit", dir, "--recursive", "--schema-version", schema, "--json"}
		if code := Run(args, &document, &stderr); code != 4 || stderr.Len() != 0 {
			t.Fatal("mixed corpus exit", code, stderr.String())
		}
		var batch struct {
			Header  map[string]any `json:"header"`
			Entries []corpus.Entry `json:"entries"`
			Summary corpus.Summary `json:"summary"`
		}
		if err := json.Unmarshal(document.Bytes(), &batch); err != nil {
			t.Fatal(err)
		}
		if batch.Header["report_schema"] != schema || len(batch.Entries) != 7 || batch.Summary.Counts["failed"] != 1 || batch.Summary.Counts["unsupported"] != 1 || batch.Summary.Counts["skipped"] != 1 || batch.Summary.Counts["requires_review"] != 1 || batch.Summary.Counts["no_reported_findings"] != 3 {
			t.Fatalf("missing outcome: %+v", batch.Summary)
		}
		var lines bytes.Buffer
		if Run([]string{"audit", dir, "--recursive", "--schema-version", schema, "--jsonl"}, &lines, &stderr) != 4 {
			t.Fatal("JSONL exit")
		}
		decoder := json.NewDecoder(&lines)
		for i := 0; i < 9; i++ {
			var record json.RawMessage
			if err := decoder.Decode(&record); err != nil {
				t.Fatal(err)
			}
			if i > 0 && i < 8 {
				var entry corpus.Entry
				if err := json.Unmarshal(record, &entry); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(entry, batch.Entries[i-1]) {
					t.Fatal("JSON and JSONL differ")
				}
			}
		}
		if decoder.Decode(new(any)) != io.EOF {
			t.Fatal("extra records")
		}
		var repeated bytes.Buffer
		if Run(args, &repeated, &stderr) != 4 || !bytes.Equal(document.Bytes(), repeated.Bytes()) {
			t.Fatal("nondeterministic JSON")
		}
	}
	for path, info := range before {
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info.Sys(), after.Sys()) {
			t.Fatalf("source timestamps changed: %q", path)
		}
	}
	for relative, data := range files {
		raw, err := os.ReadFile(filepath.Join(dir, relative))
		if err != nil || !bytes.Equal(raw, data) {
			t.Fatal("source bytes changed", err)
		}
	}
}
func TestDirectoryConsoleAndRecursionPolicy(t *testing.T) {
	dir, _ := corpusFixture(t)
	var out, stderr bytes.Buffer
	if Run([]string{"audit", dir, "--verbose"}, &out, &stderr) != 4 {
		t.Fatal("mixed corpus exit")
	}
	if !strings.Contains(out.String(), "[skipped] nested (recursion_disabled)") || strings.Contains(out.String(), "nested/code.rs") {
		t.Fatal("implicit recursion")
	}
	if !strings.Contains(stderr.String(), "corpus_completed") {
		t.Fatal("missing diagnostic")
	}
	out.Reset()
	stderr.Reset()
	if Run([]string{"audit", dir, "--recursive"}, &out, &stderr) != 4 {
		t.Fatal("mixed corpus exit")
	}
	if strings.ContainsRune(out.String(), '\x1b') || !strings.Contains(out.String(), "discovery_complete=true") {
		t.Fatal("unsafe or incomplete console")
	}
}
func TestDirectoryReportSafety(t *testing.T) {
	dir, _ := corpusFixture(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "batch.json")
	var out, stderr bytes.Buffer
	if Run([]string{"audit", dir, "--recursive", "--output", target}, &out, &stderr) != 4 || out.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("could not retain valid partial report")
	}
	raw, err := os.ReadFile(target)
	if err != nil || !json.Valid(raw) {
		t.Fatal("invalid saved JSON", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("exposed saved report", err)
	}
	for _, path := range []string{target, filepath.Join(dir, "new.json"), filepath.Join(dir, "nested", "new.json"), filepath.Join(dir, "clean.txt")} {
		stderr.Reset()
		if Run([]string{"audit", dir, "--output", path}, &out, &stderr) != 4 || !strings.Contains(stderr.String(), "output.create_failed") {
			t.Fatal("unsafe output accepted", path)
		}
	}
	link := filepath.Join(outside, "alias")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if Run([]string{"audit", dir, "--output", filepath.Join(link, "new.json")}, &out, &stderr) != 4 {
		t.Fatal("alias output accepted")
	}
	if _, err := os.Lstat(filepath.Join(dir, "new.json")); !os.IsNotExist(err) {
		t.Fatal("modified input tree")
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("existing report overwritten")
	}
}
func TestDirectoryUsageAndOutputFailure(t *testing.T) {
	dir, _ := corpusFixture(t)
	for _, args := range [][]string{{"audit", dir, "--json", "--jsonl"}, {"unicode", dir}, {"audit", filepath.Join(dir, "clean.txt"), "--recursive"}, {"audit", dir, "--reveal-out", "new", "--output", "report.json"}} {
		var out, stderr bytes.Buffer
		if Run(args, &out, &stderr) != 4 || out.Len() != 0 {
			t.Fatal("invalid directory options accepted")
		}
	}
	for _, flag := range []string{"--json", "--jsonl", "--verbose"} {
		var stderr bytes.Buffer
		if Run([]string{"audit", dir, flag}, failingWriter{}, &stderr) != 4 || !strings.Contains(stderr.String(), "output.write_failed") {
			t.Fatal("writer failure ignored")
		}
	}
	var out, stderr bytes.Buffer
	if Run([]string{"audit", t.TempDir(), "--json"}, &out, &stderr) != 0 || !json.Valid(out.Bytes()) {
		t.Fatal("empty workspace")
	}
	out.Reset()
	if Run([]string{"audit", filepath.Join(dir, "missing"), "--jsonl"}, &out, &stderr) != 4 || !strings.Contains(out.String(), `"state":"failed"`) {
		t.Fatal("missing workspace reported clean")
	}
}

func TestFormattedCorpusOutputBudget(t *testing.T) {
	var out bytes.Buffer
	bounded := &boundedCorpusOutput{out: &out, remaining: 3}
	if n, err := bounded.Write([]byte("abc")); n != 3 || err != nil {
		t.Fatal("exact budget rejected")
	}
	if n, err := bounded.Write([]byte("d")); n != 0 || err != corpus.ErrOutputLimit || out.String() != "abc" {
		t.Fatal("formatted output exceeded budget")
	}
}
