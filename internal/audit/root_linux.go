//go:build linux

package audit

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
)

// Each path component is opened relative to an already-open directory with
// O_NOFOLLOW. No candidate pathname is joined to an ambient absolute root.
func readRootSnapshot(root *os.Root, relative string, limit int) ([]byte, error) {
	var consumed int
	return readRootSnapshotCounted(root, relative, limit, limit, &consumed)
}

func readRootSnapshotCounted(root *os.Root, relative string, limit, fileLimit int, consumed *int) ([]byte, error) {
	if root == nil || !fs.ValidPath(relative) || relative == "." || len(relative) > 4096 || strings.Count(relative, "/") > 64 || limit <= 0 || limit > fileLimit || fileLimit > MaxBytes {
		return nil, errors.New("invalid rooted acquisition")
	}
	flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOATIME | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	dir, err := root.OpenFile(".", flags, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	components := strings.Split(relative, "/")
	for _, component := range components[:len(components)-1] {
		fd, err := syscall.Openat(int(dir.Fd()), component, flags, 0)
		if err != nil {
			return nil, err
		}
		next := os.NewFile(uintptr(fd), component)
		if err := dir.Close(); err != nil {
			_ = next.Close()
			return nil, err
		}
		dir = next
	}
	return readSnapshotWithOpenLimits(relative, limit, fileLimit, func(_ string, flags int) (snapshotFile, error) {
		fd, err := syscall.Openat(int(dir.Fd()), components[len(components)-1], flags|syscall.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		return countedSnapshot{snapshotFile: os.NewFile(uintptr(fd), relative), consumed: consumed}, nil
	})
}

// Count only bytes returned by Read, even when acquisition later rejects them.
// Partial bytes remain private to acquisition and never become evidence.
type countedSnapshot struct {
	snapshotFile
	consumed *int
}

func (f countedSnapshot) Read(p []byte) (int, error) {
	n, err := f.snapshotFile.Read(p)
	*f.consumed += n
	return n, err
}
