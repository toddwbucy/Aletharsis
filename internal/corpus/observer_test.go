package corpus

import (
	"bytes"
	"context"
	"errors"
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
