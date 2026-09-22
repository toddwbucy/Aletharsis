//go:build linux

package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSpecialFilesAreNotOpened(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(context.Background(), dir, testOptions)
	if err != nil || len(got.Entries) != 1 || got.Entries[0].Reason != "not_regular" {
		t.Fatal("special file not skipped", err)
	}
}
func TestInvalidUTF8NameFailsWithoutLossyCoordinates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "ordinary.txt"))
	write(t, filepath.Join(dir, string([]byte{'b', 0xff})))
	if got, err := Discover(context.Background(), dir, testOptions); !errors.Is(err, ErrInvalid) || got != nil {
		t.Fatal("invalid path normalized silently")
	}
}
func TestDirectoryChangeDetection(t *testing.T) {
	dir := t.TempDir()
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "new"))
	after, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !directoryChanged(before, after) {
		t.Fatal("directory mutation missed")
	}
	if directoryChanged(after, after) {
		t.Fatal("unchanged directory rejected")
	}
}
func TestUnreadableChildIsExplicit(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission denial")
	}
	dir := t.TempDir()
	child := filepath.Join(dir, "denied")
	if err := os.Mkdir(child, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(child, 0700); err != nil {
			t.Error(err)
		}
	})
	got, err := Discover(context.Background(), dir, testOptions)
	if err != nil || got.Complete || len(got.Entries) != 1 || got.Entries[0].State != "failed" || got.Entries[0].Reason != "directory_unavailable" {
		t.Fatal("permission failure disappeared", got, err)
	}
}
