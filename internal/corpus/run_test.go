package corpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func linux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux no-atime acquisition")
	}
}
func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func records(t *testing.T, data []byte) []json.RawMessage {
	t.Helper()
	var result []json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, raw)
	}
	return result
}
func TestCorpusStatesHashesAndDeterminism(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a.txt"), []byte("clean"))
	put(t, filepath.Join(dir, "b.md"), []byte("a\u200bb"))
	put(t, filepath.Join(dir, "c.txt"), []byte("%PDF-1.7\n"))
	put(t, filepath.Join(dir, "d.txt"), []byte{0xff})
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "nested", "other.rs"), []byte("fn main() {}"))
	if err := os.Symlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"1.0", "2.0"} {
		t.Run(schema, func(t *testing.T) {
			options := DefaultOptions()
			options.Schema = schema
			options.Discovery.Recursive = true
			var out bytes.Buffer
			s, err := Run(context.Background(), dir, options, &out)
			if err != nil || s.State != "partial" || s.ExitCode != 4 || s.Entries != 6 || s.Counts["no_reported_findings"] != 2 || s.Counts["requires_review"] != 1 || s.Counts["unsupported"] != 1 || s.Counts["failed"] != 1 || s.Counts["skipped"] != 1 {
				t.Fatalf("incorrect outcomes: %+v %v", s, err)
			}
			lines := records(t, out.Bytes())
			if len(lines) != 8 {
				t.Fatal("missing header/summary/entry")
			}
			for _, raw := range lines[1 : len(lines)-1] {
				var entry Entry
				if err := json.Unmarshal(raw, &entry); err != nil {
					t.Fatal(err)
				}
				if entry.State == "skipped" {
					if len(entry.Report) != 0 {
						t.Fatal("invented skipped report")
					}
					continue
				}
				canonical, err := identity.Canonicalize(entry.Report, audit.DefaultV2Options().ReportLimits)
				if err != nil {
					t.Fatal(err)
				}
				if evidence.Hash(canonical) != entry.ReportCanonicalSHA256 {
					t.Fatal("canonical report hash mismatch")
				}
			}
			var again bytes.Buffer
			s2, err := Run(context.Background(), dir, options, &again)
			if err != nil || !reflect.DeepEqual(s, s2) || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal("nondeterministic corpus")
			}
		})
	}
}

type reactingWriter struct {
	bytes.Buffer
	act   func()
	acted bool
}

func (w *reactingWriter) Write(p []byte) (int, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(p, &header); err != nil {
		return 0, err
	}
	if header.Type == "entry" && !w.acted {
		w.acted = true
		w.act()
	}
	return w.Buffer.Write(p)
}
func TestDiscoveredParentReplacementCannotEscape(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a.txt"), []byte("first"))
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(nested, "data.txt"), []byte("original"))
	outside := t.TempDir()
	put(t, filepath.Join(outside, "data.txt"), []byte("outside"))
	writer := &reactingWriter{act: func() {
		if err := os.Rename(nested, filepath.Join(dir, "moved")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, nested); err != nil {
			t.Fatal(err)
		}
	}}
	options := DefaultOptions()
	options.Discovery.Recursive = true
	summary, err := Run(context.Background(), dir, options, writer)
	if err != nil || summary.Counts["failed"] != 1 || summary.ExitCode != 4 {
		t.Fatal("replacement accepted", summary, err)
	}
	lines := records(t, writer.Bytes())
	var entry Entry
	if err := json.Unmarshal(lines[2], &entry); err != nil {
		t.Fatal(err)
	}
	if entry.RelativePath != "nested/data.txt" || entry.State != "failed" {
		t.Fatal("followed parent link or lost source path")
	}
	var report struct {
		File struct {
			Hash *string `json:"sha256"`
		}
	}
	if err := json.Unmarshal(entry.Report, &report); err != nil {
		t.Fatal(err)
	}
	if report.File.Hash != nil {
		t.Fatal("acquired outside source")
	}
}
func TestCancelAfterEntryAccountsForRemainingCandidates(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		put(t, filepath.Join(dir, name), []byte("clean"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &reactingWriter{act: cancel}
	summary, err := Run(ctx, dir, DefaultOptions(), writer)
	if err != nil || summary.State != "canceled" || summary.Entries != 3 || summary.Counts["canceled"] != 2 || summary.ExitCode != 4 {
		t.Fatal("cancellation lost scope", summary, err)
	}
	if len(records(t, writer.Bytes())) != 5 {
		t.Fatal("missing completion marker")
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestOutputAndDiscoveryFailures(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("clean"))
	if _, err := Run(context.Background(), dir, DefaultOptions(), shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short output accepted")
	}
	options := DefaultOptions()
	options.OutputBytes = 1
	if _, err := Run(context.Background(), dir, options, io.Discard); !errors.Is(err, ErrOutputLimit) {
		t.Fatal("output bound ignored")
	}
	options = DefaultOptions()
	options.Discovery.PathBytes = 1
	put(t, filepath.Join(dir, "long"), []byte("clean"))
	var out bytes.Buffer
	s, err := Run(context.Background(), dir, options, &out)
	if err != nil || s.State != "failed" || s.Reason != "execution.resource_limit" || s.Entries != 0 || len(records(t, out.Bytes())) != 2 {
		t.Fatal("discovery failure omitted", s, err)
	}
}

func TestAggregateInputBudgetAccountsWithoutInventedReports(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("abc"))
	put(t, filepath.Join(dir, "b"), []byte("def"))
	for _, schema := range []string{"1.0", "2.0"} {
		options := DefaultOptions()
		options.Schema = schema
		options.AcquisitionBytes = 4
		var out bytes.Buffer
		summary, err := Run(context.Background(), dir, options, &out)
		if err != nil || summary.Entries != 2 || summary.Counts["no_reported_findings"] != 1 || summary.Counts["failed"] != 1 || summary.ExitCode != 4 {
			t.Fatal("budget lost accounting", summary, err)
		}
		lines := records(t, out.Bytes())
		var last Entry
		if err := json.Unmarshal(lines[2], &last); err != nil {
			t.Fatal(err)
		}
		if last.Reason != "execution.resource_limit" || len(last.Report) != 0 {
			t.Fatal("invented report for unacquired file")
		}
	}
}
func TestOutputLimitLeavesNoCompletionMarker(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("clean"))
	options := DefaultOptions()
	options.OutputBytes = 500
	var out bytes.Buffer
	if _, err := Run(context.Background(), dir, options, &out); !errors.Is(err, ErrOutputLimit) {
		t.Fatal("expected bounded output failure", err)
	}
	for _, raw := range records(t, out.Bytes()) {
		var record struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if record.Type == "summary" {
			t.Fatal("failed delivery claimed completion")
		}
	}
}

func TestLegacyCorpusReportBudgetIsIndependentOfSourceBudget(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "large.txt")
	put(t, source, []byte(strings.Repeat("a ", 1<<20)))
	inspected, _ := audit.InspectSnapshot(source, audit.MaxBytes)
	if inspected.Report.Status != "completed" {
		t.Fatal("valid standalone source failed")
	}
	raw, err := json.Marshal(inspected.Report)
	if err != nil {
		t.Fatal(err)
	}
	limits := audit.DefaultV2Options().ReportLimits
	if len(raw) <= limits.InputBytes {
		t.Fatal("fixture does not cross corpus report budget")
	}
	var out bytes.Buffer
	summary, err := Run(context.Background(), dir, DefaultOptions(), &out)
	if err != nil || summary.State != "partial" || summary.ExitCode != 4 || summary.Counts["failed"] != 1 {
		t.Fatal("corpus report budget not explicit", summary, err)
	}
	decoder := json.NewDecoder(&out)
	var header map[string]any
	var entry Entry
	if err := decoder.Decode(&header); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&entry); err != nil {
		t.Fatal(err)
	}
	if entry.State != "failed" || entry.Reason != "execution.resource_limit" || len(entry.Report) != 0 {
		t.Fatal("partial/oversized report escaped", entry.State, entry.Reason)
	}
}

func TestPreReadFailuresDoNotExhaustAcquisitionBudget(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	for i := 0; i < 40; i++ {
		put(t, filepath.Join(dir, fmt.Sprintf("a%02d.txt", i)), []byte("too large"))
	}
	put(t, filepath.Join(dir, "z.txt"), []byte("ok"))
	for _, schema := range []string{"1.0", "2.0"} {
		options := DefaultOptions()
		options.Schema = schema
		options.InputBytes = 4
		options.AcquisitionBytes = 6
		var out bytes.Buffer
		summary, err := Run(context.Background(), dir, options, &out)
		if err != nil || summary.Counts["failed"] != 40 || summary.Counts["no_reported_findings"] != 1 {
			t.Fatalf("%s: %+v %v", schema, summary, err)
		}
		rows := records(t, out.Bytes())
		for _, row := range rows[1:41] {
			var entry Entry
			if err := json.Unmarshal(row, &entry); err != nil {
				t.Fatal(err)
			}
			if entry.Reason != "file.too_large" {
				t.Fatalf("lost acquisition failure: %+v", entry)
			}
		}
	}
}

func TestCorpusEscapesUnicodeAndPreservesCanonicalHash(t *testing.T) {
	linux(t)
	dir := filepath.Join(t.TempDir(), "workspace\u202e")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	name := "source\u2066\U0001f600.txt"
	put(t, filepath.Join(dir, name), []byte("a\u200bb\u202e"))
	for _, schema := range []string{"1.0", "2.0"} {
		options := DefaultOptions()
		options.Schema = schema
		var out bytes.Buffer
		if _, err := Run(context.Background(), dir, options, &out); err != nil {
			t.Fatal(err)
		}
		for _, b := range out.Bytes() {
			if b > 127 {
				t.Fatal("raw non-ASCII in stream")
			}
		}
		var entry Entry
		if err := json.Unmarshal(records(t, out.Bytes())[1], &entry); err != nil {
			t.Fatal(err)
		}
		canonical, err := identity.Canonicalize(entry.Report, audit.DefaultV2Options().ReportLimits)
		if err != nil || entry.RelativePath != name || evidence.Hash(canonical) != entry.ReportCanonicalSHA256 {
			t.Fatal("escaping changed identity", err)
		}
		tight := options
		tight.OutputBytes = out.Len() - 100
		if _, err := Run(context.Background(), dir, tight, io.Discard); !errors.Is(err, ErrOutputLimit) {
			t.Fatal("escaped byte budget not enforced", err)
		}
	}
}
