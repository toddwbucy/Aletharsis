package audit

import (
	"context"
	"errors"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

// nativeTrace is private to one native v2 run. No imported descriptor can enter
// the dispatcher. The legacy path passes nil and preserves its existing behavior.
type nativeTrace struct {
	ctx        context.Context
	source     []byte
	completed  map[string][]evidence.Finding
	canceled   string
	stopReason failure.Code
}

func (t *nativeTrace) before(id string) bool {
	if t == nil {
		return true
	}
	if err := t.ctx.Err(); err != nil {
		t.stopReason = failure.Canceled
		if errors.Is(err, context.DeadlineExceeded) {
			t.stopReason = failure.Timeout
		}
		t.canceled = id
		return false
	}
	return true
}
func (t *nativeTrace) acquired(source []byte) {
	if t == nil {
		return
	}
	// readSnapshot transfers ownership of a fresh buffer; no public API exposes it.
	t.source = source
	t.complete(capability.AcquireID, nil)
}
func (t *nativeTrace) complete(id string, findings []evidence.Finding) {
	if t != nil {
		t.completed[id] = findings
	}
}
