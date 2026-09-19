package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHelpVersionAndUsage(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--help"}, {"audit", "--help"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 0 || out.Len() == 0 {
			t.Fatal("missing help/version")
		}
	}
	var out, err bytes.Buffer
	if Run([]string{"audit"}, &out, &err) != 4 {
		t.Fatal("usage exit")
	}
}
func TestViewsOutputsAndNoOverwrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only reader")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.py")
	original := []byte("# 😀\u200b\x1b\n\nmore ordinary prose\n")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "hardlink")
	if err := os.Link(source, link); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{source, link, symlink} {
		var out, err bytes.Buffer
		if Run([]string{"audit", source, "--output", output}, &out, &err) != 4 {
			t.Fatal("overwrote existing destination")
		}
	}
	for _, command := range []string{"audit", "unicode", "metadata", "structure"} {
		var out, err bytes.Buffer
		code := Run([]string{command, source, "--json", "--verbose"}, &out, &err)
		var report map[string]any
		if json.Unmarshal(out.Bytes(), &report) != nil {
			t.Fatal("invalid JSON stdout")
		}
		if command == "audit" && code != 2 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(err.String(), "audit_completed") {
			t.Fatal("missing structured log")
		}
	}
	var out, err bytes.Buffer
	Run([]string{"audit", source, "--verbose"}, &out, &err)
	if strings.ContainsAny(out.String(), "\x1b\x00") {
		t.Fatal("unsafe console output")
	}
	got, _ := os.ReadFile(source)
	if !bytes.Equal(got, original) {
		t.Fatal("source bytes changed")
	}
}

func TestMissingOutputArgument(t *testing.T) {
	for _, args := range [][]string{{"audit", "unused.txt", "--output", "--json"}, {"audit", "unused.txt", "--output="}, {"audit", "unused.txt", "--output", "-h"}, {"audit", "unused.txt", "--output", "-"}, {"audit", "unused.txt", "--output", ""}, {"audit", "unused.txt", "--output"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 4 || !strings.Contains(err.String(), "requires a path") {
			t.Fatal("accepted missing output path")
		}
	}
}

func TestReportFilePermissions(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only reader")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("Ordinary text.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "report.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"audit", source, "--output", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("report grants group/other permissions: %04o", info.Mode().Perm())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("report is not valid JSON")
	}
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestHelpVersionWriteFailure(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--help"}, {"-h"}, {"audit", "--help"}, {"audit", "-h"}} {
		if code := Run(args, failingWriter{}, io.Discard); code != 4 {
			t.Fatalf("%v: got exit %d, want 4", args, code)
		}
	}
}

func TestExplicitDashOutputName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only reader")
	}
	t.Chdir(t.TempDir())
	if err := os.WriteFile("source.txt", []byte("Ordinary text.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"audit", "source.txt", "--output=-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile("-h")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("report is not valid JSON")
	}
}
