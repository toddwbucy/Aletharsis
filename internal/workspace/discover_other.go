//go:build !linux

package workspace

import "os"

func available() bool                                { return false }
func openDirectory(*os.Root) (*os.File, error)       { return nil, ErrUnavailable }
func endOfDirectory(error) bool                      { return false }
func directoryChanged(os.FileInfo, os.FileInfo) bool { return true }
