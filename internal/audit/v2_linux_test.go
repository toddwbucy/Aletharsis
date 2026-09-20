//go:build linux

package audit_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/audit"
)

func TestV2OriginalBytesAndTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	data := []byte("é😀\u200b text\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := audit.RunV2(context.Background(), path, audit.DefaultV2Options())
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	if out.Report.Status != "completed" || a.Atim != b.Atim || a.Mtim != b.Mtim || a.Ctim != b.Ctim || a.Size != b.Size {
		t.Fatal("v2 changed source integrity")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, got) {
		t.Fatal("source bytes changed")
	}
}
