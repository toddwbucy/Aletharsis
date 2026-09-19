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
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	r := audit.Run(p)
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	if r.Status != "completed" || a.Atim != b.Atim || a.Mtim != b.Mtim || a.Ctim != b.Ctim || a.Size != b.Size {
		t.Fatal("source integrity changed or audit failed")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, got) {
		t.Fatal("source bytes changed")
	}
}
func TestReadFailuresAndLimits(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(p, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, link, filepath.Join(dir, "missing")} {
		if audit.Run(path).Summary["exit_code"] != 4 {
			t.Errorf("accepted invalid source %s", path)
		}
	}
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	limited := audit.WithLimit(p, 4)
	if limited.Summary["exit_code"] != 4 || limited.File.SHA256 != nil || limited.File.Size != nil || len(limited.Evidence.Texts) != 0 {
		t.Fatal("size cap ignored")
	}
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	if a.Atim != b.Atim || a.Mtim != b.Mtim || a.Ctim != b.Ctim || a.Size != b.Size {
		t.Fatal("oversized rejection changed source metadata")
	}
	if audit.WithLimit(p, 5).Status != "completed" {
		t.Fatal("exact cap rejected")
	}
}
