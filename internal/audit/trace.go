package audit

import (
	"context"
	"errors"
	"strconv"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// nativeTrace is private to one native v2 run. No imported descriptor can enter
// the dispatcher. The legacy path passes nil and preserves its existing behavior.
type nativeTrace struct {
	ctx          context.Context
	source       []byte
	completed    map[string][]evidence.Finding
	canceled     string
	stopReason   failure.Code
	reportLimits identity.Limits
	budgetErr    error
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

// Check only a provable lower bound of the retained text JSON. Full report
// validation still applies later. This cannot reject a report that would fit.
func (t *nativeTrace) textFits(document evidence.Document) bool {
	if t == nil || t.reportLimits.InputBytes == 0 {
		return true
	}
	byteLimit := min(t.reportLimits.InputBytes, t.reportLimits.OutputBytes)
	size, nodes := 0, 0
	for _, text := range document.Texts {
		if len(text.Text) > byteLimit-size {
			t.budgetErr = identity.ErrLimit
			return false
		}
		size += len(text.Text)
		for i, offset := range text.ByteOffsets {
			n := len(strconv.Itoa(offset))
			if i > 0 {
				n++
			}
			if n > byteLimit-size || nodes >= t.reportLimits.Nodes {
				t.budgetErr = identity.ErrLimit
				return false
			}
			size += n
			nodes++
		}
	}
	return true
}
