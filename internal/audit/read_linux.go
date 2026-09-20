//go:build linux

package audit

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/toddwbucy/Aletharsis/internal/failure"
)

type snapshotFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

// readSnapshot never falls back to a read that could alter source access time.
func readSnapshot(path string, limit int) ([]byte, error) {
	return readSnapshotWithOpen(path, limit, openSnapshot)
}

func openSnapshot(path string, flags int) (snapshotFile, error) {
	fd, err := syscall.Open(path, flags, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// Inject dependencies per call, never through mutable package-level hooks.
func readSnapshotWithOpen(path string, limit int, open func(string, int) (snapshotFile, error)) ([]byte, error) {
	f, err := open(path, syscall.O_RDONLY|syscall.O_NOATIME|syscall.O_NOFOLLOW|syscall.O_NONBLOCK)
	if err != nil {
		return nil, failure.AcquisitionError(err)
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return nil, failure.AcquisitionError(err)
	}
	if !before.Mode().IsRegular() {
		return nil, failure.Wrap(failure.Acquisition, failure.NotRegular, fmt.Errorf("Only regular files are supported; directory auditing is planned for M4"))
	}
	if before.Size() > int64(limit) {
		return nil, failure.Wrap(failure.Acquisition, failure.TooLarge, fmt.Errorf("File exceeds the %d-byte analysis limit", limit))
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, failure.AcquisitionError(err)
	}
	after, err := f.Stat()
	if err != nil {
		return nil, failure.AcquisitionError(err)
	}
	if len(data) > limit {
		return nil, failure.Wrap(failure.Acquisition, failure.TooLarge, fmt.Errorf("File exceeds the %d-byte analysis limit", limit))
	}
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || a.Ctim != b.Ctim {
		return nil, failure.Wrap(failure.Acquisition, failure.ChangedDuringRead, fmt.Errorf("Source changed during snapshot acquisition; evidence may be inconsistent"))
	}
	return data, nil
}
