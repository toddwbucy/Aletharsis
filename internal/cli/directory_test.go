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
	for _, args := range [][]string{{"audit", dir, "--json", "--jsonl"}, {"audit", filepath.Join(dir, "clean.txt"), "--recursive"}, {"audit", dir, "--reveal-out", "new", "--output", "report.json"}} {
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

func TestDirectorySubviewsRetainFailureReports(t *testing.T) {
	dir := t.TempDir()
	for _, schema := range []string{"1.0", "2.0"} {
		for _, view := range []string{"unicode", "metadata", "structure"} {
			var out, stderr bytes.Buffer
			if Run([]string{view, dir, "--json", "--schema-version=" + schema}, &out, &stderr) != 4 || stderr.Len() != 0 {
				t.Fatal(view, schema, stderr.String())
			}
			var report struct {
				Schema  string         `json:"schema_version"`
				Summary map[string]int `json:"summary"`
			}
			if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Schema != schema || report.Summary["exit_code"] != 4 {
				t.Fatal("missing failure report", err, out.String())
			}
		}
	}
}

func TestDirectoryOutputUnpinnableSourceFailsBeforeCreation(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "report.json")
	var out, stderr bytes.Buffer
	if Run([]string{"audit", filepath.Join(parent, "missing"), "--recursive", "--output", target}, &out, &stderr) != 4 || out.Len() != 0 || !strings.Contains(stderr.String(), "input.open_failed") {
		t.Fatal("unsafe preflight", stderr.String())
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("created report without source pin", err)
	}
}

func TestUnsupportedHostCanSaveCorpusFailure(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("unsupported-host contract")
	}
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"1.0", "2.0"} {
		var out, stderr bytes.Buffer
		target := filepath.Join(parent, schema+".json")
		if Run([]string{"audit", source, "--output", target, "--schema-version=" + schema}, &out, &stderr) != 4 || out.Len() != 0 || stderr.Len() != 0 {
			t.Fatal(stderr.String())
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Summary corpus.Summary `json:"summary"`
		}
		if err := json.Unmarshal(raw, &report); err != nil || report.Summary.Reason != "integrity.no_atime_unavailable" || report.Summary.State != "failed" {
			t.Fatal("missing failure envelope", err, string(raw))
		}
	}
}

func TestCorpusOutputParentResolvesAliasesAndRejectsContainment(t *testing.T) {
	parent := t.TempDir()
	sourcePath := filepath.Join(parent, "source")
	outside := filepath.Join(parent, "outside")
	for _, p := range []string{sourcePath, outside} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.OpenRoot(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(outside, alias); err != nil {
		t.Skipf("symlink setup unavailable: %v", err)
	}
	for _, output := range []string{sourcePath, filepath.Join(sourcePath, "new"), parent, filepath.Dir(parent)} {
		root, _, err := corpusOutputParent(source, sourcePath, output)
		if root != nil {
			root.Close()
		}
		if err == nil {
			t.Fatal("overlap admitted before creation", output)
		}
	}
	root, name, err := corpusOutputParent(source, sourcePath, filepath.Join(alias, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// Retargeting the lexical alias cannot redirect the pinned output parent.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sourcePath, alias); err != nil {
		t.Fatal(err)
	}
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, name)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(sourcePath, name)); !os.IsNotExist(err) {
		t.Fatal("write redirected into source", err)
	}
	if p, _, err := corpusOutputParent(source, sourcePath, filepath.Join(alias, "new")); err == nil {
		p.Close()
		t.Fatal("source alias accepted")
	}
}

func TestMissingInputFlagSelectsDocumentedEnvelope(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux acquisition diagnostic")
	}
	for _, schema := range []string{"1.0", "2.0"} {
		for _, flag := range []string{"--json", "--jsonl", "--recursive"} {
			var out, stderr bytes.Buffer
			args := []string{"audit", filepath.Join(t.TempDir(), "missing"), "--schema-version=" + schema, flag}
			if flag == "--recursive" {
				args = append(args, "--json")
			}
			if code := Run(args, &out, &stderr); code != 4 || stderr.Len() != 0 {
				t.Fatal("missing input did not report failure", code, stderr.String())
			}
			if flag == "--json" {
				var r struct {
					Schema string `json:"schema_version"`
				}
				if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Schema != schema {
					t.Fatal("file envelope changed", err)
				}
			} else if !strings.Contains(out.String(), `"contract":"aletharsis.corpus/1"`) || !strings.Contains(out.String(), `"reason":"file.not_found"`) {
				t.Fatal("missing corpus failure", out.String())
			}
		}
	}
}

func TestInvalidUTF8WorkspaceIsInputFailureBeforeOutput(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("arbitrary byte filenames are tested on Linux")
	}
	dir := filepath.Join(t.TempDir(), "d\xffir")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"1.0", "2.0"} {
		for _, mode := range []string{"--jsonl", "--json", "--output", "--reveal-out"} {
			var out, stderr bytes.Buffer
			args := []string{"audit", dir, "--schema-version", schema, mode}
			destination := filepath.Join(t.TempDir(), "new-output")
			if mode == "--output" || mode == "--reveal-out" {
				args = append(args, destination)
			}
			if code := Run(args, &out, &stderr); code != 4 || out.Len() != 0 || !strings.Contains(stderr.String(), "input.open_failed") || strings.Contains(stderr.String(), "output.write_failed") {
				t.Fatalf("%s %s: incorrect diagnostic: %d %q %q", schema, mode, code, out.Bytes(), stderr.Bytes())
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("created output: %v", err)
			}
		}
	}
}
