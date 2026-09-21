package publication

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

var limits = Limits{Files: 10, Bytes: 1 << 20}
var digest = strings.Repeat("a", 64)

func root(t *testing.T) (*os.Root, string) {
	t.Helper()
	path := t.TempDir()
	r, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	return r, path
}
func TestPublishDeterministic(t *testing.T) {
	items := []Artifact{{"revealed.txt", []byte("Hello⟦U+200B ZERO WIDTH SPACE⟧")}, {"comparison.diff", []byte("diff\r\n")}}
	var previous []byte
	for i := 0; i < 2; i++ {
		r, path := root(t)
		got, err := Publish(r, digest, digest, items, limits)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(path, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && !bytes.Equal(raw, previous) {
			t.Fatal("nondeterministic manifest")
		}
		previous = raw
		if got.ManifestSHA256 != evidence.Hash(raw) {
			t.Fatal("manifest identity")
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(m, got.Manifest) || m.SourceSHA256 != digest || m.ReportSHA256 != digest {
			t.Fatal("manifest lineage")
		}
		for _, entry := range m.Artifacts {
			data, err := os.ReadFile(filepath.Join(path, entry.Name))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Hash(data) != entry.SHA256 || len(data) != entry.Size {
				t.Fatal("artifact identity")
			}
			info, err := r.Stat(entry.Name)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
				t.Fatal("exposed artifact")
			}
		}
	}
	if items[0].Name != "revealed.txt" {
		t.Fatal("caller order mutated")
	}
}
func TestCollisionRollback(t *testing.T) {
	for _, kind := range []string{"file", "directory", "hardlink", "symlink", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			r, path := root(t)
			source := filepath.Join(t.TempDir(), "source")
			original := []byte("original evidence")
			if err := os.WriteFile(source, original, 0600); err != nil {
				t.Fatal(err)
			}
			name := "z.txt"
			if kind == "manifest" {
				name = "manifest.json"
			}
			target := filepath.Join(path, name)
			var err error
			switch kind {
			case "directory":
				err = os.Mkdir(target, 0700)
			case "hardlink":
				err = os.Link(source, target)
			case "symlink":
				err = os.Symlink(source, target)
			default:
				err = os.WriteFile(target, original, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := Publish(r, digest, digest, []Artifact{{"a.txt", []byte("first")}, {"z.txt", []byte("second")}}, limits)
			if err == nil || got != nil {
				t.Fatal("collision succeeded")
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != name {
				t.Fatal("rollback touched preexisting entry or left partial output", entries)
			}
			intact, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(intact, original) {
				t.Fatal("source altered", err)
			}
			if kind != "directory" {
				intact, err = os.ReadFile(target)
				if err != nil || !bytes.Equal(intact, original) {
					t.Fatal("collision target altered", err)
				}
			}
		})
	}
}
func TestPreflightRejectsWithoutWrites(t *testing.T) {
	for _, name := range []string{"../escape.txt", "/absolute.txt", "sub/a.txt", "manifest.json", "A.txt", "x\x1b.txt"} {
		r, path := root(t)
		if got, err := Publish(r, digest, digest, []Artifact{{name, nil}}, limits); !errors.Is(err, ErrInvalid) || got != nil {
			t.Fatal("unsafe name accepted", name, err)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			t.Fatal("preflight wrote files", err)
		}
	}
	r, path := root(t)
	for _, tc := range []struct {
		items []Artifact
		limit Limits
		hash  string
	}{
		{[]Artifact{{"a.txt", nil}, {"a.txt", nil}}, limits, digest},
		{[]Artifact{{"a.txt", []byte("large")}}, Limits{1, 1}, digest},
		{[]Artifact{{"a.txt", nil}}, Limits{1, 100}, digest}, // manifest also consumes bytes
		{nil, limits, digest}, {[]Artifact{{"a.txt", nil}}, limits, "bad"},
	} {
		if got, err := Publish(r, tc.hash, digest, tc.items, tc.limit); err == nil || got != nil {
			t.Fatal("invalid request accepted")
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatal("preflight wrote files", err)
	}
}

type brokenDestination struct {
	rootDirectory
	mode    string
	removed []string
}

func (d *brokenDestination) create(name string) (io.WriteCloser, error) {
	f, err := d.rootDirectory.create(name)
	if err != nil {
		return nil, err
	}
	if name == "z.txt" {
		return brokenWriter{f, d.mode}, nil
	}
	return f, nil
}
func (d *brokenDestination) remove(name string) error {
	d.removed = append(d.removed, name)
	err := d.rootDirectory.remove(name)
	if d.mode == "cleanup" {
		return errors.Join(err, errInjected)
	}
	return err
}

var errInjected = errors.New("injected publication failure")

type brokenWriter struct {
	io.WriteCloser
	mode string
}

func (w brokenWriter) Write(p []byte) (int, error) {
	switch w.mode {
	case "short":
		return 0, nil
	case "write", "cleanup":
		return 0, errInjected
	}
	return w.WriteCloser.Write(p)
}
func (w brokenWriter) Close() error {
	err := w.WriteCloser.Close()
	if w.mode == "close" {
		return errors.Join(err, errInjected)
	}
	return err
}
func TestWriteCloseAndCleanupFailures(t *testing.T) {
	for _, mode := range []string{"write", "short", "close", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			r, path := root(t)
			d := &brokenDestination{rootDirectory: rootDirectory{r}, mode: mode}
			got, err := publish(d, digest, digest, []Artifact{{"a.txt", []byte("a")}, {"z.txt", []byte("z")}}, limits)
			if got != nil || err == nil {
				t.Fatal("failure returned successful receipt")
			}
			if mode == "short" && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal(err)
			}
			if mode != "short" && !errors.Is(err, errInjected) {
				t.Fatal(err)
			}
			if mode == "cleanup" && !strings.Contains(err.Error(), "cleanup a.txt") {
				t.Fatal("cleanup failure hidden")
			}
			if !reflect.DeepEqual(d.removed, []string{"z.txt", "a.txt"}) {
				t.Fatal("incorrect rollback", d.removed)
			}
			entries, err := os.ReadDir(path)
			if err != nil || len(entries) != 0 {
				t.Fatal("partial output retained", err)
			}
		})
	}
}
