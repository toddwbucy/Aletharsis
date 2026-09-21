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

// InspectRootCounted also returns actual bytes read, including discarded partial
// or invalid snapshots. fileLimit is the configured per-file ceiling; limit
// may be smaller when the corpus acquisition allowance is nearly exhausted.
// Rejections before Read consume zero acquisition bytes.
func InspectRootCounted(root *os.Root, relative string, limit, fileLimit int) (Outcome, []byte, int) {
	consumed := 0
	outcome, raw := inspectSnapshotWithReader(relative, limit, func(_ string, limit int) ([]byte, error) {
		return readRootSnapshotCounted(root, relative, limit, fileLimit, &consumed)
	})
	return outcome, raw, consumed
}

// RunV2RootCounted retains consumption even if later report construction fails.
func RunV2RootCounted(ctx context.Context, root *os.Root, relative string, options V2Options, fileLimit int) (*V2Output, error, int) {
	consumed := 0
	result, err := runV2WithReader(ctx, relative, options, func(_ string, limit int) ([]byte, error) {
		return readRootSnapshotCounted(root, relative, limit, fileLimit, &consumed)
	}, nil)
	return result, err, consumed
}
