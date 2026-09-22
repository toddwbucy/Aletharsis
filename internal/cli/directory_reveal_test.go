package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/corpus"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/publication"
	"github.com/toddwbucy/Aletharsis/internal/reveal"
	"github.com/toddwbucy/Aletharsis/internal/workspace"
)

func TestDirectoryRevealCompleteWorkflow(t *testing.T) {
	dir, files := corpusFixture(t)
	// Portable artifact paths are required; path-control handling is tested as an
	// explicit rejected export separately. Audit-only mode still observes it.
	unsafe := "nested/\x1b[31m.txt"
	if err := os.Remove(filepath.Join(dir, unsafe)); err != nil {
		t.Fatal(err)
	}
	delete(files, unsafe)
	extras := map[string][]byte{"invoice\u202egnp.txt": []byte("invoice\u202e.txt"), "nested/binary.md": []byte(strings.Repeat("\u200b\u200c", 32)), "nested/日本語.md": []byte("Café 日本語 e\u0301 👩\u200d💻\r\n"), "empty.txt": {}, "utf16.txt": {0xff, 0xfe, 'a', 0, 0x0b, 0x20}, "utf32.txt": {0, 0, 0xfe, 0xff, 0, 0, 0, 97, 0, 0, 0x20, 0x0b}}
	for name, data := range extras {
		files[name] = data
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{dir, filepath.Join(dir, "nested")}
	for name := range files {
		paths = append(paths, filepath.Join(dir, name))
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
		t.Run(schema, func(t *testing.T) {
			var previous []byte
			for pass := 0; pass < 2; pass++ {
				output := filepath.Join(t.TempDir(), "review")
				var out, stderr bytes.Buffer
				code := Run([]string{"audit", dir, "--recursive", "--schema-version", schema, "--jsonl", "--reveal-out", output}, &out, &stderr)
				if code != 4 || stderr.Len() != 0 {
					t.Fatalf("mixed corpus failed to publish: %d %q", code, stderr.Bytes())
				}
				raw, err := os.ReadFile(filepath.Join(output, "manifest.json"))
				if err != nil {
					t.Fatal(err)
				}
				if previous != nil && !bytes.Equal(previous, raw) {
					t.Fatal("nondeterministic tree")
				}
				previous = raw
				assertASCIIJSON(t, raw)
				var manifest publication.TreeManifest
				if err := json.Unmarshal(raw, &manifest); err != nil {
					t.Fatal(err)
				}
				if len(manifest.Sources) != len(files)+1 {
					t.Fatal("missing source ledger record")
				}
				corpusBytes, err := os.ReadFile(filepath.Join(output, manifest.Corpus.Name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(corpusBytes, out.Bytes()) || len(corpusBytes) != manifest.Corpus.Size || evidence.Hash(corpusBytes) != manifest.Corpus.SHA256 {
					t.Fatal("corpus identity mismatch")
				}
				for _, b := range corpusBytes {
					if b > 127 {
						t.Fatal("raw Unicode in corpus transport")
					}
				}
				var bidiEntry corpus.Entry
				for _, line := range bytes.Split(bytes.TrimSpace(corpusBytes), []byte("\n")) {
					var entry corpus.Entry
					if err := json.Unmarshal(line, &entry); err != nil {
						t.Fatal(err)
					}
					if entry.RelativePath == "invoice\u202egnp.txt" {
						bidiEntry = entry
					}
				}
				canonical, err := identity.Canonicalize(bidiEntry.Report, audit.DefaultV2Options().ReportLimits)
				if err != nil {
					t.Fatal(err)
				}
				savedBidi, err := os.ReadFile(filepath.Join(output, "reports", "invoice\u202egnp.txt.json"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(savedBidi, []byte("\u202e")) || !bytes.Equal(savedBidi, canonical) || evidence.Hash(savedBidi) != bidiEntry.ReportCanonicalSHA256 {
					t.Fatal("canonical raw evidence/hash binding changed")
				}
				for _, source := range manifest.Sources {
					if source.RelativePath == "link" {
						if source.State != "skipped" || len(source.Artifacts) != 0 {
							t.Fatal("link acquired")
						}
						continue
					}
					if source.SourceSHA256 != evidence.Hash(files[source.RelativePath]) {
						t.Fatal("source identity mismatch", source.RelativePath)
					}
					for _, artifact := range source.Artifacts {
						data, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(artifact.Name)))
						if err != nil {
							t.Fatal(err)
						}
						if len(data) != artifact.Size || evidence.Hash(data) != artifact.SHA256 {
							t.Fatal("artifact hash mismatch")
						}
					}
					if source.RelativePath == "bad.txt" || source.RelativePath == "pdf.txt" {
						if len(source.Artifacts) != 1 {
							t.Fatal("invented derivative for failed input")
						}
						continue
					}
					if source.State != "revealed" || len(source.Artifacts) != 5 {
						t.Fatal("missing derivatives", source.RelativePath)
					}
					data, err := os.ReadFile(filepath.Join(output, "mappings", filepath.FromSlash(source.RelativePath)+".json"))
					if err != nil {
						t.Fatal(err)
					}
					assertASCIIJSON(t, data)
					var comparison reveal.Comparison
					if err := json.Unmarshal(data, &comparison); err != nil {
						t.Fatal(err)
					}
					if comparison.Reveal.SourceSHA256 != source.SourceSHA256 || comparison.Faithful.BeforeSHA256 != evidence.Hash([]byte(comparison.DecodedText)) {
						t.Fatal("mapping lineage mismatch")
					}
					reportBytes, err := os.ReadFile(filepath.Join(output, "reports", filepath.FromSlash(source.RelativePath)+".json"))
					if err != nil {
						t.Fatal(err)
					}
					var saved struct {
						Findings []evidence.Finding `json:"findings"`
					}
					if err := json.Unmarshal(reportBytes, &saved); err != nil {
						t.Fatal(err)
					}
					for _, occurrence := range comparison.Reveal.Occurrences {
						for _, ref := range occurrence.Findings {
							if ref.Index < 0 || ref.Index >= len(saved.Findings) {
								t.Fatal("unresolved finding index", ref)
							}
							finding := saved.Findings[ref.Index]
							if ref.ID != finding.ID || ref.Category != finding.Category || ref.Severity != finding.Severity || ref.Classification != finding.Classification {
								t.Fatal("occurrence reference disagrees with saved report", ref, finding)
							}
						}
					}
					if source.RelativePath == "zero.txt" && comparison.Reveal.Text != "a⟦U+200B ZERO WIDTH SPACE⟧b" {
						t.Fatal("incorrect revealed content")
					}
					if source.RelativePath == "nested/binary.md" && len(comparison.Reveal.Occurrences) != 64 {
						t.Fatal("lost pattern occurrences")
					}
					if source.RelativePath == "clean.txt" && comparison.Faithful.Text != "" {
						t.Fatal("clean text changed")
					}
				}
			}
		})
	}
	for path, info := range before {
		after, err := os.Stat(path)
		if err != nil || !reflect.DeepEqual(info.Sys(), after.Sys()) {
			t.Fatal("source metadata changed", path, err)
		}
	}
	for name, data := range files {
		after, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(data, after) {
			t.Fatal("source bytes changed", err)
		}
	}
}
func TestDirectoryRevealRejectionAndRollback(t *testing.T) {
	dir, files := corpusFixture(t)
	for _, dest := range []string{dir, filepath.Join(dir, "new"), filepath.Join(dir, "nested", "new")} {
		var out, stderr bytes.Buffer
		if Run([]string{"audit", dir, "--reveal-out", dest}, &out, &stderr) != 4 || out.Len() != 0 {
			t.Fatal("unsafe destination accepted")
		}
	}
	output := filepath.Join(t.TempDir(), "new")
	var out, stderr bytes.Buffer
	// Recursive export reaches the control-containing filename and must reject
	// the plan before acquisition/publication rather than rename it silently.
	if Run([]string{"audit", dir, "--recursive", "--reveal-out", output}, &out, &stderr) != 4 || out.Len() != 0 || stderr.Len() == 0 {
		t.Fatal("unsafe export accepted")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("failed plan retained directory")
	}
	for name, data := range files {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(raw, data) {
			t.Fatal("source changed")
		}
	}
}
func TestDirectoryRevealSuffixCollision(t *testing.T) {
	dir, _ := corpusFixture(t)
	// report for x would be reports/x.json, colliding with the directory needed
	// for the report of x.json/y.
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "x.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.json", "y"), []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "nested", "\x1b[31m.txt")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "review")
	var out, stderr bytes.Buffer
	if Run([]string{"audit", dir, "--recursive", "--reveal-out", output}, &out, &stderr) != 4 || out.Len() != 0 {
		t.Fatal("prefix collision accepted")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("collision left output")
	}
}
func TestDirectoryRevealEmptyAndDeliveryFailure(t *testing.T) {
	dir, _ := corpusFixture(t)
	empty := t.TempDir()
	output := filepath.Join(t.TempDir(), "review")
	var out, stderr bytes.Buffer
	if Run([]string{"audit", empty, "--reveal-out", output, "--json"}, &out, &stderr) != 0 || !json.Valid(out.Bytes()) {
		t.Fatal("empty workspace did not publish")
	}
	output = filepath.Join(t.TempDir(), "review")
	if Run([]string{"audit", dir, "--reveal-out", output}, failingWriter{}, &stderr) != 4 || !strings.Contains(stderr.String(), "published reveal tree remains") {
		t.Fatal("delivery failure ambiguous")
	}
	if _, err := os.Stat(filepath.Join(output, "manifest.json")); err != nil {
		t.Fatal("completed tree was removed")
	}
}
func TestDirectoryRevealSnapshotMismatch(t *testing.T) {
	// Visitor failures are fatal and never result in a complete stream. This path
	// deliberately supplies no native snapshot for a supposedly completed audit.
	r, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	observer := &treeObserver{root: r}
	if err := observer.Prepare([]workspace.Entry{{RelativePath: "a", State: "candidate", Kind: "file"}}); err != nil {
		t.Fatal(err)
	}
	if err := observer.Visit(corpus.Entry{RelativePath: "a", State: "no_reported_findings"}, corpus.Snapshot{}); err == nil {
		t.Fatal("unverified snapshot accepted")
	}
	if err := observer.tree.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryRevealPerSourceLimitPreservesOtherArtifacts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux acquisition")
	}
	for _, schema := range []string{"1.0", "2.0"} {
		for name, payload := range map[string]string{"render": strings.Repeat("\u200b", 100001), "diff": strings.Repeat("日", 60000)} {
			t.Run(schema+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				files := map[string]string{"a.txt": "clean before", "b.txt": payload, "c.txt": "clean after"}
				before := map[string]os.FileInfo{}
				for name, text := range files {
					path := filepath.Join(dir, name)
					if err := os.WriteFile(path, []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					before[name] = info
				}
				output := filepath.Join(t.TempDir(), "review")
				var out, stderr bytes.Buffer
				code := Run([]string{"audit", dir, "--schema-version", schema, "--json", "--reveal-out", output}, &out, &stderr)
				if code != 4 || stderr.Len() != 0 {
					t.Fatalf("limit aborted corpus: %d %s", code, stderr.String())
				}
				var report struct {
					Entries []corpus.Entry
					Summary corpus.Summary
				}
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if len(report.Entries) != 3 || report.Summary.State != "partial" || report.Summary.Counts["failed"] != 1 || report.Summary.Counts["no_reported_findings"] != 2 {
					t.Fatal("incorrect aggregate", report.Summary)
				}
				failed := report.Entries[1]
				if failed.State != "failed" || failed.Reason != "execution.resource_limit" || len(failed.Report) == 0 {
					t.Fatal("missing per-source failure", failed.State, failed.Reason)
				}
				var native struct{ Status string }
				if err := json.Unmarshal(failed.Report, &native); err != nil || native.Status != "completed" {
					t.Fatal("audit outcome rewritten", err)
				}
				raw, err := os.ReadFile(filepath.Join(output, "manifest.json"))
				if err != nil {
					t.Fatal(err)
				}
				var manifest publication.TreeManifest
				if err = json.Unmarshal(raw, &manifest); err != nil {
					t.Fatal(err)
				}
				for _, item := range manifest.Sources {
					if item.RelativePath == "b.txt" {
						if item.State != "failed" || item.Reason != "execution.resource_limit" || len(item.Artifacts) != 1 {
							t.Fatal("bad source ledger", item)
						}
					} else if item.State != "revealed" || len(item.Artifacts) != 5 {
						t.Fatal("unrelated artifacts lost", item)
					}
				}
				if len(manifest.Sources) != 3 {
					t.Fatal("incomplete ledger")
				}
				for name, text := range files {
					path := filepath.Join(dir, name)
					info, err := os.Stat(path)
					if err != nil || !reflect.DeepEqual(before[name], info) {
						t.Fatal("source stat changed", err)
					}
					raw, err := os.ReadFile(path)
					if err != nil || string(raw) != text {
						t.Fatal("source bytes changed", err)
					}
				}
			})
		}
	}
}

func TestPortableNamePreflightDiagnostic(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux source names and no-atime acquisition")
	}
	for _, name := range []string{"Chapter 1: Intro.md", "aux.md", "draft."} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			raw := []byte("clean source")
			source := filepath.Join(dir, name)
			if err := os.WriteFile(source, raw, 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "review")
			var out, stderr bytes.Buffer
			code := Run([]string{"audit", dir, "--reveal-out", output}, &out, &stderr)
			want := "aletharsis: execution.unsupported_input: a source name cannot be exported under the portable-name policy\n"
			if code != 4 || stderr.String() != want || out.Len() != 0 {
				t.Fatal(code, stderr.String(), out.String())
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatal("preflight left artifacts", err)
			}
			after, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(raw, after) {
				t.Fatal("source changed", err)
			}
		})
	}
}
