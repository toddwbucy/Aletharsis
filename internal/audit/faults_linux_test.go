//go:build linux

package audit

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Synthetic stat snapshots make same-size timestamp races deterministic even
// on filesystems whose timestamp resolution cannot distinguish adjacent writes.
type snapshotInfo struct {
	size     int64
	mode     os.FileMode
	modified time.Time
	changed  syscall.Timespec
}

func (s snapshotInfo) Name() string       { return "source.txt" }
func (s snapshotInfo) Size() int64        { return s.size }
func (s snapshotInfo) Mode() os.FileMode  { return s.mode }
func (s snapshotInfo) ModTime() time.Time { return s.modified }
func (s snapshotInfo) IsDir() bool        { return s.mode.IsDir() }
func (s snapshotInfo) Sys() any           { return &syscall.Stat_t{Ctim: s.changed} }

type snapshotStub struct {
	reader               io.Reader
	before, after        snapshotInfo
	statFailure          int
	stats, reads, closes int
}

func (s *snapshotStub) Read(b []byte) (int, error) { s.reads++; return s.reader.Read(b) }
func (s *snapshotStub) Stat() (os.FileInfo, error) {
	s.stats++
	if s.stats == s.statFailure {
		return nil, syscall.EIO
	}
	if s.stats == 1 {
		return s.before, nil
	}
	return s.after, nil
}
func (s *snapshotStub) Close() error { s.closes++; return nil }

type partialReadError struct{}

func (partialReadError) Read(p []byte) (int, error) { return copy(p, "partial"), syscall.EIO }

func TestAcquisitionOpenFailureNeverRetries(t *testing.T) {
	for _, err := range []error{syscall.EACCES, syscall.EPERM, syscall.EOPNOTSUPP, syscall.ELOOP} {
		t.Run(err.Error(), func(t *testing.T) {
			calls := 0
			data, got := readSnapshotWithOpen("source.txt", 32, func(path string, flags int) (snapshotFile, error) {
				calls++
				if path != "source.txt" || flags != syscall.O_RDONLY|syscall.O_NOATIME|syscall.O_NOFOLLOW|syscall.O_NONBLOCK {
					t.Fatalf("unsafe acquisition flags: %x", flags)
				}
				return nil, &os.PathError{Op: "open", Path: path, Err: err}
			})
			var typed *failure.Error
			wantCode := failure.IOFailed
			if errors.Is(err, os.ErrPermission) {
				wantCode = failure.PermissionDenied
			}
			if !errors.As(got, &typed) || typed.Code() != wantCode {
				t.Fatalf("wrong typed open failure: %v", got)
			}
			if calls != 1 || data != nil || !errors.Is(got, err) {
				t.Fatalf("retried or swallowed error: calls=%d data=%q err=%v", calls, data, got)
			}
		})
	}
}

func TestAcquisitionFaultsDiscardPartialEvidence(t *testing.T) {
	base := snapshotInfo{size: 4, modified: time.Unix(1, 0), changed: syscall.Timespec{Sec: 1}}
	for _, name := range []string{"first_stat", "read", "second_stat", "size_changed", "mtime_changed", "ctime_changed", "grew_past_limit", "oversized", "nonregular"} {
		t.Run(name, func(t *testing.T) {
			f := &snapshotStub{reader: strings.NewReader("text"), before: base, after: base}
			want := "Source changed during snapshot acquisition"
			switch name {
			case "first_stat":
				f.statFailure = 1
				want = "input/output error"
			case "read":
				f.reader = partialReadError{}
				want = "input/output error"
			case "second_stat":
				f.statFailure = 2
				want = "input/output error"
			case "size_changed":
				f.after.size++
			case "mtime_changed":
				f.after.modified = time.Unix(2, 0)
			case "ctime_changed":
				f.after.changed.Nsec = 1
			case "grew_past_limit":
				f.reader = strings.NewReader("longer")
				want = "analysis limit"
			case "oversized":
				f.before.size = 5
				want = "analysis limit"
			case "nonregular":
				f.before.mode = os.ModeNamedPipe
				want = "Only regular files"
			}
			data, err := readSnapshotWithOpen("source.txt", 4, func(string, int) (snapshotFile, error) { return f, nil })
			wantCode := failure.ChangedDuringRead
			switch name {
			case "first_stat", "read", "second_stat":
				wantCode = failure.IOFailed
			case "oversized", "grew_past_limit":
				wantCode = failure.TooLarge
			case "nonregular":
				wantCode = failure.NotRegular
			}
			var typed *failure.Error
			if !errors.As(err, &typed) || typed.Code() != wantCode {
				t.Fatalf("wrong typed snapshot failure: %v", err)
			}
			if err == nil || !strings.Contains(err.Error(), want) || data != nil {
				t.Fatalf("partial acquisition accepted: %q %v", data, err)
			}
			if f.closes != 1 {
				t.Fatalf("descriptor closed %d times", f.closes)
			}
			if (name == "first_stat" || name == "oversized" || name == "nonregular") && f.reads != 0 {
				t.Fatal("read occurred after failed preflight")
			}
		})
	}
}

func TestBoundedSnapshotReads(t *testing.T) {
	for _, n := range []int{0, 3, 4, 5} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			base := snapshotInfo{size: int64(n)}
			f := &snapshotStub{reader: bytes.NewReader(bytes.Repeat([]byte{'a'}, n)), before: base, after: base}
			got, err := readSnapshotWithOpen("source.txt", 4, func(string, int) (snapshotFile, error) { return f, nil })
			if n > 4 {
				if err == nil || got != nil || f.reads != 0 {
					t.Fatal("oversized snapshot read")
				}
			} else if err != nil || !bytes.Equal(got, bytes.Repeat([]byte{'a'}, n)) {
				t.Fatalf("boundary %d: %q %v", n, got, err)
			}
			if f.closes != 1 {
				t.Fatal("descriptor leak")
			}
		})
	}
	// An understated pre-read stat cannot cause unbounded reading of a growing file.
	reader := bytes.NewReader(bytes.Repeat([]byte{'a'}, 100))
	f := &snapshotStub{reader: reader, before: snapshotInfo{}, after: snapshotInfo{}}
	if got, err := readSnapshotWithOpen("source.txt", 4, func(string, int) (snapshotFile, error) { return f, nil }); err == nil || got != nil {
		t.Fatal("growth accepted")
	}
	if reader.Len() != 95 {
		t.Fatalf("read exceeded limit+1: %d bytes left", reader.Len())
	}
}

func TestFailedAcquisitionReportHasNoSnapshotIdentity(t *testing.T) {
	for _, err := range []error{syscall.EIO, &os.PathError{Op: "open", Path: "source.txt", Err: syscall.EPERM}} {
		r := withReader("source.txt", 4, func(path string, limit int) ([]byte, error) {
			if path != "source.txt" || limit != 4 {
				t.Fatal("reader arguments changed")
			}
			return []byte("partial"), err
		})
		if r.Status != "failed" || r.Summary["exit_code"] != 4 || r.File.SHA256 != nil || r.File.Size != nil || r.File.Parser != nil || len(r.Evidence.Texts) != 0 {
			t.Fatalf("failed acquisition published identity/evidence: %+v", r)
		}
		if len(r.Findings) != 1 || r.Findings[0].ID != "parser.failure" {
			t.Fatal(r.Findings)
		}
	}
}

type readHook struct {
	*os.File
	once func()
}

func (f *readHook) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	if f.once != nil {
		hook := f.once
		f.once = nil
		hook()
	}
	return n, err
}

func TestSourceChangeInsideReadIsRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(p, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	data, err := readSnapshotWithOpen(p, 32, func(path string, flags int) (snapshotFile, error) {
		f, err := openSnapshot(path, flags)
		if err != nil {
			return nil, err
		}
		return &readHook{File: f.(*os.File), once: func() {
			// Deliberately emulate an external same-size edit between the two stats.
			if err := os.WriteFile(p, []byte("after!"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(p, time.Unix(1, 0), time.Unix(2, 0)); err != nil {
				t.Fatal(err)
			}
		}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "Source changed") || data != nil {
		t.Fatalf("changed source accepted: %q %v", data, err)
	}
}

func TestCountedSnapshotAccountsDiscardedBytes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		reader      io.Reader
		size        int64
		afterSize   int64
		statFailure int
		want        int
	}{
		{"pre_read_oversize", strings.NewReader("oversize"), 40, 40, 0, 0},
		{"partial_error", partialReadError{}, 7, 7, 0, 7},
		{"post_read_stat_error", strings.NewReader("abc"), 3, 3, 2, 3},
		{"changed", strings.NewReader("abc"), 3, 4, 0, 3},
		{"grew_over_limit", strings.NewReader(strings.Repeat("x", 40)), 3, 40, 0, 33},
	} {
		t.Run(tc.name, func(t *testing.T) {
			consumed := 0
			f := &snapshotStub{reader: tc.reader, before: snapshotInfo{size: tc.size}, after: snapshotInfo{size: tc.afterSize}, statFailure: tc.statFailure}
			data, err := readSnapshotWithOpen("source.txt", 32, func(string, int) (snapshotFile, error) {
				return countedSnapshot{snapshotFile: f, consumed: &consumed}, nil
			})
			if err == nil || data != nil || consumed != tc.want {
				t.Fatalf("data=%q consumed=%d err=%v", data, consumed, err)
			}
		})
	}
}
