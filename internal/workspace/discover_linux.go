//go:build linux

package workspace

import (
	"errors"
	"io"
	"os"
	"syscall"
)

func available() bool { return true }
func openDirectory(root *os.Root) (*os.File, error) {
	return root.OpenFile(".", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NOATIME|syscall.O_NONBLOCK, 0)
}
func endOfDirectory(err error) bool { return errors.Is(err, io.EOF) }
func directoryChanged(a, b os.FileInfo) bool {
	x, y := a.Sys().(*syscall.Stat_t), b.Sys().(*syscall.Stat_t)
	return !os.SameFile(a, b) || a.Size() != b.Size() || x.Mtim != y.Mtim || x.Ctim != y.Ctim
}
