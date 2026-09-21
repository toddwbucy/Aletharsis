package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

var testOptions = Options{Recursive: true, MaxEntries: 1000, MaxDepth: 16, PathBytes: 1 << 20}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
}
func linux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("no-atime discovery requires Linux")
	}
}
func TestDiscoveryOrderPoliciesAndIntegrity(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"z.rs", "A.txt", "a.txt", ".hidden", "unknown.bin"} {
		write(t, filepath.Join(dir, name))
	}
	write(t, filepath.Join(nested, "doc.md"))
	outside := t.TempDir()
	write(t, filepath.Join(outside, "not-scanned.txt"))
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(nested, "cycle")); err != nil {
		t.Fatal(err)
	}
	before := map[string]os.FileInfo{}
	for _, path := range []string{dir, nested, outside, filepath.Join(nested, "doc.md")} {
		if err := os.Chtimes(path, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = info
	}
	got, err := Discover(context.Background(), dir, testOptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{".hidden", "file", "candidate", ""}, {"A.txt", "file", "candidate", ""}, {"a.txt", "file", "candidate", ""}, {"link", "symlink", "skipped", "symlink_not_followed"}, {"nested/cycle", "symlink", "skipped", "symlink_not_followed"}, {"nested/doc.md", "file", "candidate", ""}, {"unknown.bin", "file", "candidate", ""}, {"z.rs", "file", "candidate", ""}}
	if !reflect.DeepEqual(got.Entries, want) || !got.Complete || got.Directories != 2 {
		t.Fatalf("unexpected discovery: %+v", got)
	}
	again, err := Discover(context.Background(), dir, testOptions)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("nondeterministic discovery", err)
	}
	for path, info := range before {
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info.Sys(), after.Sys()) {
			t.Fatalf("source metadata changed: %q", path)
		}
	}
	options := testOptions
	options.Recursive = false
	got, err = Discover(context.Background(), dir, options)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range got.Entries {
		if entry.RelativePath == "nested" {
			found = true
			if entry.State != "skipped" || entry.Reason != "recursion_disabled" {
				t.Fatal("implicit recursion")
			}
		}
	}
	if !found || got.Directories != 1 {
		t.Fatal("directory omission")
	}
}
func TestLimitsCancelAndInvalidRoot(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.txt"))
	write(t, filepath.Join(dir, "b.txt"))
	if err := os.Mkdir(filepath.Join(dir, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, options := range []Options{{true, 1, 16, 1000}, {true, 100, 0, 1000}, {true, 100, 16, 1}} {
		got, err := Discover(context.Background(), dir, options)
		if !errors.Is(err, ErrLimit) || got != nil {
			t.Fatal("limit returned usable partial set", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := Discover(ctx, dir, testOptions); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("canceled discovery returned candidates")
	}
	for _, options := range []Options{{}, {true, 1, -1, 1}} {
		if _, err := Discover(context.Background(), dir, options); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid bounds accepted")
		}
	}
	if _, err := Discover(context.Background(), filepath.Join(dir, "a.txt"), testOptions); !errors.Is(err, ErrInvalid) {
		t.Fatal("file treated as directory")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(context.Background(), link, testOptions); !errors.Is(err, ErrInvalid) {
		t.Fatal("symlink root followed")
	}
}
func TestEmptyAndUnavailable(t *testing.T) {
	got, err := Discover(context.Background(), t.TempDir(), testOptions)
	if runtime.GOOS != "linux" {
		if !errors.Is(err, ErrUnavailable) || got != nil {
			t.Fatal("unverified acquisition enabled")
		}
		return
	}
	if err != nil || got == nil || !got.Complete || len(got.Entries) != 0 || got.Directories != 1 {
		t.Fatal("empty directory ambiguous", err)
	}
}

func TestBatchedEnumerationBoundary(t *testing.T) {
	linux(t)
	dir := t.TempDir()
	for i := 0; i < 130; i++ {
		write(t, filepath.Join(dir, fmt.Sprintf("%03d.txt", i)))
	}
	options := testOptions
	options.MaxEntries = 130
	options.PathBytes = 130 * len("000.txt")
	got, err := Discover(context.Background(), dir, options)
	if err != nil || len(got.Entries) != 130 || got.Entries[0].RelativePath != "000.txt" || got.Entries[129].RelativePath != "129.txt" {
		t.Fatal("multi-batch discovery failed", err)
	}
	options.MaxEntries = 129
	if got, err := Discover(context.Background(), dir, options); !errors.Is(err, ErrLimit) || got != nil {
		t.Fatal("entry boundary returned partial set")
	}
	options.MaxEntries = 130
	options.PathBytes--
	if got, err := Discover(context.Background(), dir, options); !errors.Is(err, ErrLimit) || got != nil {
		t.Fatal("path-byte boundary returned partial set")
	}
}
