package corpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/workspace"
)

type testObserver struct {
	t                      *testing.T
	prepared               bool
	visited                int
	failPrepare, failVisit bool
}

var errObserver = errors.New("observer failure")

func (o *testObserver) Prepare(entries []workspace.Entry) error {
	o.prepared = true
	if len(entries) != 2 {
		o.t.Fatal("incomplete discovery plan")
	}
	if o.failPrepare {
		return errObserver
	}
	return nil
}
func (o *testObserver) Visit(entry Entry, snapshot Snapshot) error {
	if !o.prepared {
		o.t.Fatal("visit before plan")
	}
	o.visited++
	if entry.State == "no_reported_findings" {
		if snapshot.Native == nil || evidence.Hash(snapshot.Source) != *snapshot.Native.File.SHA256 || snapshot.Native.Evidence.Texts[0].Text != "clean" {
			o.t.Fatal("unverified or missing native snapshot")
		}
	} else if snapshot.Native != nil || snapshot.Source != nil {
		o.t.Fatal("failed audit exposed a success snapshot")
	}
	if o.failVisit {
		return errObserver
	}
	return nil
}
func TestObserverSnapshotAndFailures(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("clean"))
	put(t, filepath.Join(dir, "b"), []byte{0xff})
	for _, schema := range []string{"1.0", "2.0"} {
		for _, mode := range []string{"complete", "plan", "visit"} {
			observer := &testObserver{t: t, failPrepare: mode == "plan", failVisit: mode == "visit"}
			options := DefaultOptions()
			options.Schema = schema
			options.Observer = observer
			var out bytes.Buffer
			_, err := Run(context.Background(), dir, options, &out)
			if mode == "complete" {
				if err != nil || observer.visited != 2 {
					t.Fatal("observer lost outcomes", err)
				}
			} else {
				if !errors.Is(err, errObserver) {
					t.Fatal("observer failure swallowed", err)
				}
				if bytes.Contains(out.Bytes(), []byte(`"type":"summary"`)) {
					t.Fatal("failed observer completed stream")
				}
			}
		}
	}
}

type limitObserver struct {
	outcome error
	visits  int
}

func (o *limitObserver) Prepare([]workspace.Entry) error { return nil }
func (o *limitObserver) Visit(Entry, Snapshot) error     { o.visits++; return o.outcome }
func TestSourceLimitMustBeAnUnambiguousOutcome(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("clean"))
	put(t, filepath.Join(dir, "b"), []byte("clean"))
	for _, outcome := range []error{ErrSourceLimit, fmt.Errorf("context: %w", ErrSourceLimit), errors.Join(ErrSourceLimit, errObserver)} {
		observer := &limitObserver{outcome: outcome}
		options := DefaultOptions()
		options.Observer = observer
		var out bytes.Buffer
		summary, err := Run(context.Background(), dir, options, &out)
		if outcome == ErrSourceLimit {
			if err != nil || observer.visits != 2 || summary.ExitCode != 4 || !bytes.Contains(out.Bytes(), []byte(`"type":"summary"`)) {
				t.Fatal("recoverable outcome lost", err)
			}
		} else if err != outcome || observer.visits != 1 || bytes.Contains(out.Bytes(), []byte(`"type":"summary"`)) {
			t.Fatal("ambiguous observer error swallowed", err)
		}
	}
}

func TestDiscoveryCapsDoNotStartObservers(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a"), []byte("clean"))
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "nested", "b"), []byte("clean"))
	for _, mode := range []string{"entries", "depth"} {
		options := DefaultOptions()
		options.Discovery.Recursive = true
		if mode == "entries" {
			options.Discovery.MaxEntries = 1
		} else {
			options.Discovery.MaxDepth = 0
		}
		observer := &testObserver{t: t}
		options.Observer = observer
		var out bytes.Buffer
		summary, err := Run(context.Background(), dir, options, &out)
		if err != nil || summary.State != "failed" || summary.Reason != "execution.resource_limit" || summary.Entries != 0 || observer.prepared || observer.visited != 0 {
			t.Fatal("limit admitted a partial candidate plan", summary, err)
		}
		if !bytes.Contains(out.Bytes(), []byte(`"discovery_complete":false`)) {
			t.Fatal("missing incomplete coverage disclosure")
		}
	}
}
