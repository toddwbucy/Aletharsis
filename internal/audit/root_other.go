//go:build !linux

package audit

import (
	"errors"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"os"
)

func readRootSnapshot(*os.Root, string, int) ([]byte, error) {
	return nil, failure.Wrap(failure.Acquisition, failure.NoAtimeUnavailable, errors.New("rooted no-atime acquisition unavailable"))
}
