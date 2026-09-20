//go:build !linux

package audit

import (
	"fmt"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func readSnapshot(path string, limit int) ([]byte, error) {
	return nil, failure.Wrap(failure.Acquisition, failure.NoAtimeUnavailable, fmt.Errorf("This platform lacks the verified no-atime reader; strict timestamp-preserving reads are unavailable"))
}
