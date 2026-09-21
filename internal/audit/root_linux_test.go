//go:build linux

package audit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestRootedAcquisitionAndRenamedRoot(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "source")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(path, "file.txt")
	data := []byte("original\u200b")
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
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	moved := filepath.Join(parent, "moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	out, raw := InspectRoot(root, "file.txt", MaxBytes)
	if out.Failure != nil || string(raw) != string(data) || *out.Report.File.SHA256 != evidence.Hash(data) {
		t.Fatal("escaped pinned root", out.Failure)
	}
	options := DefaultV2Options()
	options.RetainSnapshot = true
	second, err := RunV2Root(context.Background(), root, "file.txt", options)
	if err != nil || second.Failure != nil || string(second.Source) != string(data) {
		t.Fatal("v2 escaped pinned root", err)
	}
	after, err := os.Stat(filepath.Join(moved, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Sys(), after.Sys()) {
		t.Fatal("source metadata changed")
	}
}
func TestRootedParentLinksAndTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	file := filepath.Join(outside, "private.txt")
	if err := os.WriteFile(file, []byte("must not acquire"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "parent")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(dir, "leaf")); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, path := range []string{"parent/private.txt", "leaf", "../private.txt", "/private.txt", "a/../private.txt", ".", ""} {
		out, raw := InspectRoot(root, path, MaxBytes)
		if out.Failure == nil || raw != nil || out.Report.File.SHA256 != nil {
			t.Fatalf("unsafe acquisition %q", path)
		}
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Sys(), after.Sys()) {
		t.Fatal("outside file accessed")
	}
}
