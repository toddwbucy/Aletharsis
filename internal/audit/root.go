package audit

import (
	"context"
	"os"
)

// InspectRoot uses exactly one snapshot acquired relative to a pinned directory.
// The report path is root-relative; the corpus envelope identifies its root.
func InspectRoot(root *os.Root, relative string, limit int) (Outcome, []byte) {
	return inspectSnapshotWithReader(relative, limit, func(_ string, limit int) ([]byte, error) { return readRootSnapshot(root, relative, limit) })
}
func RunV2Root(ctx context.Context, root *os.Root, relative string, options V2Options) (*V2Output, error) {
	return runV2WithReader(ctx, relative, options, func(_ string, limit int) ([]byte, error) { return readRootSnapshot(root, relative, limit) }, nil)
}
