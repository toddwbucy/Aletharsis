//go:build !linux

package audit

import "fmt"

func readSnapshot(path string, limit int) ([]byte, error) {
	return nil, fmt.Errorf("This platform lacks the verified no-atime reader; strict timestamp-preserving reads are unavailable")
}
