// Package reveal produces presentation data from verified native text evidence.
// It does not acquire files, detect artifacts, authorize edits, or write output.
package reveal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type Limits struct {
	SourceBytes int
	OutputBytes int
	Occurrences int
}

// FindingRef retains the index in the supplied, unfiltered finding collection.
// A stable finding ID alone cannot distinguish multiple instances of a finding.
type FindingRef struct {
	Index          int    `json:"index"`
	ID             string `json:"id"`
	Category       string `json:"category"`
	Severity       string `json:"severity"`
	Classification string `json:"classification"`
}

type Occurrence struct {
	Scalar      identity.Region `json:"scalar"`
	SourceBytes identity.Region `json:"source_bytes"`
	// RenderedBytes indexes UTF-8 bytes in Result.Text, never source bytes.
	RenderedBytes identity.Region `json:"rendered_bytes"`
	CodePoint     string          `json:"code_point"`
	Name          string          `json:"name"`
	Reason        string          `json:"reason"`
	Findings      []FindingRef    `json:"findings"`
}

type Result struct {
	Source         string       `json:"source"`
	SourceSHA256   string       `json:"source_sha256"`
	DecodedSHA256  string       `json:"decoded_sha256"`
	RenderedSHA256 string       `json:"rendered_sha256"`
	Text           string       `json:"text"`
	Occurrences    []Occurrence `json:"occurrences"`
}

// Render accepts native, in-memory evidence, not arbitrary imported JSON maps.
// The caller must not mutate its inputs concurrently. The complete original byte
// snapshot and decoded map must agree before any presentation is returned.
// Returned text is inert UTF-8 data: consumers must use textContent, not HTML,
// and escape line controls before displaying it directly in a terminal.
func Render(source []byte, expectedSHA256 string, text *evidence.Text, findings []evidence.Finding, limits Limits) (*Result, error) {
	if limits.SourceBytes <= 0 || limits.OutputBytes <= 0 || limits.Occurrences <= 0 {
		return nil, errors.New("reveal requires positive limits")
	}
	if len(source) > limits.SourceBytes {
		return nil, errors.New("reveal source limit exceeded")
	}
	if evidence.Hash(source) != expectedSHA256 {
		return nil, errors.New("reveal source hash mismatch")
	}
	if _, err := identity.VerifyText(source, text, limits.SourceBytes); err != nil {
		return nil, err
	}
	runes := []rune(text.Text)
	refs := make(map[int][]FindingRef)
	marked := make(map[int]bool)
	links := 0
	for index, f := range findings {
		if f.Location["source"] != text.Source {
			continue
		}
		raw, exists := f.Location["character_offsets"]
		if !exists {
			continue
		} // Unlocated findings do not invent spans.
		positions, ok := raw.([]int)
		bytes, bytesOK := f.Location["byte_offsets"].([]int)
		if !ok || !bytesOK || len(positions) != len(bytes) {
			return nil, errors.New("invalid reveal finding coordinates")
		}
		unicodeFinding := strings.HasPrefix(f.ID, "unicode.")
		code, codeOK := f.Evidence["code_point"].(string)
		if unicodeFinding && !codeOK {
			return nil, errors.New("invalid reveal finding code point")
		}
		previous := -1
		for i, p := range positions {
			if p <= previous || p < 0 || p >= len(runes) || bytes[i] != text.ByteOffsets[p] {
				return nil, errors.New("invalid reveal finding coordinates")
			}
			previous = p
			if unicodeFinding {
				if code != fmt.Sprintf("U+%04X", runes[p]) {
					return nil, errors.New("reveal finding code point mismatch")
				}
				marked[p] = true
			}
			links++
			if links > limits.Occurrences {
				return nil, errors.New("reveal finding-link limit exceeded")
			}
			refs[p] = append(refs[p], FindingRef{index, f.ID, f.Category, f.Severity, f.Classification})
		}
	}
	result := &Result{Source: text.Source, SourceSHA256: expectedSHA256, DecodedSHA256: evidence.Hash([]byte(text.Text)), Occurrences: []Occurrence{}}
	var b strings.Builder
	for p, r := range runes {
		piece, reason := string(r), ""
		name := u.Name(r, "UNNAMED CONTROL")
		code := fmt.Sprintf("U+%04X", r)
		// Findings select evidence markers. Control escaping is presentation safety,
		// not an additional detector or a claim that the character is suspicious.
		if marked[p] {
			piece, reason = "⟦"+code+" "+name+"⟧", "finding"
		} else if r == '⟦' {
			piece, reason = "⟦⟦", "literal_marker_escape"
		} else if (u.Category(r) == "Cc" || u.Category(r) == "Cf") && !strings.ContainsRune("\r\n\t", r) {
			piece, reason = "⟦"+code+" "+name+"⟧", "display_escape"
		}
		if len(piece) > limits.OutputBytes-b.Len() {
			return nil, errors.New("reveal output limit exceeded")
		}
		start := b.Len()
		b.WriteString(piece)
		if reason != "" {
			if len(result.Occurrences) >= limits.Occurrences {
				return nil, errors.New("reveal occurrence limit exceeded")
			}
			links := append([]FindingRef{}, refs[p]...)
			result.Occurrences = append(result.Occurrences, Occurrence{
				Scalar:        identity.Region{Start: uint64(p), End: uint64(p + 1)},
				SourceBytes:   identity.Region{Start: uint64(text.ByteOffsets[p]), End: uint64(text.ByteOffsets[p+1])},
				RenderedBytes: identity.Region{Start: uint64(start), End: uint64(b.Len())},
				CodePoint:     code, Name: name, Reason: reason, Findings: links,
			})
		}
	}
	result.Text = b.String()
	result.RenderedSHA256 = evidence.Hash([]byte(result.Text))
	return result, nil
}
