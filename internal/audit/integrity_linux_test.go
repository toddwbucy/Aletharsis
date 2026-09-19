//go:build linux

package audit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/audit"
)

func TestOriginalBytesAndTimestamps(t *testing.T) {
	p := filepath.Join(t.TempDir(), "source.txt")
	data := []byte("é😀\u200b text\n")
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(p)
	r := audit.Run(p)
	after, _ := os.Stat(p)
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	if r.Status != "completed" || a.Atim != b.Atim || a.Mtim != b.Mtim || a.Ctim != b.Ctim || a.Size != b.Size {
		t.Fatal("source integrity changed or audit failed")
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(data, got) {
		t.Fatal("source bytes changed")
	}
}
func TestReadFailuresAndLimits(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	os.WriteFile(p, []byte("12345"), 0600)
	link := filepath.Join(dir, "link")
	os.Symlink(p, link)
	for _, path := range []string{dir, link, filepath.Join(dir, "missing")} {
		if audit.Run(path).Summary["exit_code"] != 4 {
			t.Errorf("accepted invalid source %s", path)
		}
	}
	if audit.WithLimit(p, 4).Summary["exit_code"] != 4 {
		t.Fatal("size cap ignored")
	}
	if audit.WithLimit(p, 5).Status != "completed" {
		t.Fatal("exact cap rejected")
	}
}
