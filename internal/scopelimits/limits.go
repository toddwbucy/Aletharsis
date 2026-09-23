// Package scopelimits describes whole-scope admission for Office analysis.
package scopelimits

import (
	"errors"

	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

type Limits struct{ TextUTF8Bytes, ScalarOrigins int }

func (l Limits) Validate() error {
	if l.TextUTF8Bytes <= 0 || l.ScalarOrigins <= 0 {
		return errors.New("positive scope limits required")
	}
	return nil
}

// Dimension uses deterministic precedence when both dimensions are exceeded.
func (l Limits) Dimension(textBytes, origins int) string {
	if textBytes > l.TextUTF8Bytes {
		return "max_scope_text_utf8_bytes"
	}
	if origins > l.ScalarOrigins {
		return "max_scope_scalar_origins"
	}
	return ""
}

// Omission locates an excluded candidate in original XML part bytes. It carries
// counts and a structural bounding span, never retained text, origins or findings.
// The bounding span can include intervening markup and is not a deletion range.
type Omission struct {
	LocalID, Dimension           string
	Code                         failure.Code
	Source                       xmlparts.Span
	TextUTF8Bytes, ScalarOrigins int
}
