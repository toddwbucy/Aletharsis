package reveal

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

var ErrDiffLimit = errors.New("reveal diff resource limit")

// DiffLimits bounds both input representation pairs, total unified output, line
// indexing and display escape maps. Context is the exact requested hunk context.
type DiffLimits struct{ TextBytes, OutputBytes, Lines, Escapes, Context int }

// Escape maps one display transformation. InputBytes indexes the named UTF-8
// representation, not the original UTF-16/32 source. InputScalar can be joined to
// native text evidence; after-reveal mappings join through Result.Occurrences.
type Escape struct {
	InputScalar int             `json:"input_scalar"`
	InputBytes  identity.Region `json:"input_bytes"`
	OutputBytes identity.Region `json:"output_bytes"`
	CodePoint   string          `json:"code_point"`
}

type EscapedText struct {
	InputSHA256 string   `json:"input_sha256"`
	SHA256      string   `json:"sha256"`
	Text        string   `json:"text"`
	Escapes     []Escape `json:"escapes"`
}

type Unified struct {
	Representation string `json:"representation"`
	BeforeSHA256   string `json:"before_sha256"`
	AfterSHA256    string `json:"after_sha256"`
	SHA256         string `json:"sha256"`
	Text           string `json:"text"`
}

type Presentation struct {
	Before EscapedText `json:"before"`
	After  EscapedText `json:"after"`
	Diff   Unified     `json:"diff"`
}

type Comparison struct {
	SourceEncoding string       `json:"source_encoding"`
	DecodedText    string       `json:"decoded_text"`
	Reveal         *Result      `json:"reveal"`
	Faithful       Unified      `json:"faithful"`
	Presentation   Presentation `json:"presentation"`
}

// Compare builds a verified reveal and two labeled diffs. Faithful compares exact
// decoded UTF-8 text with the reveal; it is not terminal-safe or a patch for a
// UTF-16/32 original. Presentation contains ASCII-only escaped text (plus LF),
// with explicit mapping and hashes. Neither diff carries edit authorization.
// The function performs no file I/O and never accepts a caller-forged reveal.
func Compare(source []byte, expectedSHA256 string, text *evidence.Text, findings []evidence.Finding, renderLimits Limits, limits DiffLimits) (*Comparison, error) {
	if limits.TextBytes <= 0 || limits.OutputBytes <= 0 || limits.Lines <= 0 || limits.Escapes <= 0 || limits.Context < 0 || limits.Context > 1000 {
		return nil, ErrDiffLimit
	}
	limits.TextBytes = min(limits.TextBytes, 32<<20)
	limits.OutputBytes = min(limits.OutputBytes, 64<<20)
	limits.Lines = min(limits.Lines, 200000)
	limits.Escapes = min(limits.Escapes, 100000)
	if text != nil && len(text.Text) > limits.TextBytes {
		return nil, ErrDiffLimit
	}
	revealed, err := Render(source, expectedSHA256, text, findings, renderLimits)
	if err != nil {
		return nil, err
	}
	if len(revealed.Text) > limits.TextBytes || len(text.Text) > limits.TextBytes-len(revealed.Text) {
		return nil, ErrDiffLimit
	}
	remaining := limits.OutputBytes
	faithful, err := unified(text.Text, revealed.Text, "decoded_utf8", limits, &remaining)
	if err != nil {
		return nil, err
	}
	before, err := escapeDisplay(text.Text, limits.TextBytes, limits.Escapes)
	if err != nil {
		return nil, err
	}
	after, err := escapeDisplay(revealed.Text, limits.TextBytes-len(before.Text), limits.Escapes-len(before.Escapes))
	if err != nil {
		return nil, err
	}
	display, err := unified(before.Text, after.Text, "ascii_escaped", limits, &remaining)
	if err != nil {
		return nil, err
	}
	return &Comparison{SourceEncoding: text.Encoding, DecodedText: text.Text, Reveal: revealed, Faithful: faithful, Presentation: Presentation{before, after, display}}, nil
}

// display escaping is transport/presentation, not another Unicode detector.
// Backslashes are doubled, so literal escape-looking input stays distinguishable.
func escapeDisplay(input string, maxBytes, maxEscapes int) (EscapedText, error) {
	var b strings.Builder
	escapes := []Escape{}
	scalar := 0
	for offset, r := range input {
		value := string(r)
		switch {
		case r == '\\':
			value = "\\\\"
		case r == '\r':
			value = "\\r"
		case r == '\t':
			value = "\\t"
		case r != '\n' && (r < 0x20 || r > 0x7e):
			value = fmt.Sprintf("\\u{%04X}", r)
		}
		if len(value) > maxBytes-b.Len() {
			return EscapedText{}, ErrDiffLimit
		}
		start := b.Len()
		b.WriteString(value)
		if value != string(r) {
			if len(escapes) >= maxEscapes {
				return EscapedText{}, ErrDiffLimit
			}
			escapes = append(escapes, Escape{InputScalar: scalar, InputBytes: identity.Region{Start: uint64(offset), End: uint64(offset + utf8.RuneLen(r))}, OutputBytes: identity.Region{Start: uint64(start), End: uint64(b.Len())}, CodePoint: fmt.Sprintf("U+%04X", r)})
		}
		scalar++
	}
	return EscapedText{InputSHA256: evidence.Hash([]byte(input)), SHA256: evidence.Hash([]byte(b.String())), Text: b.String(), Escapes: escapes}, nil
}

type diffWriter struct {
	text      strings.Builder
	remaining *int
}

func (w *diffWriter) append(s string) error {
	if len(s) > *w.remaining {
		return ErrDiffLimit
	}
	*w.remaining -= len(s)
	w.text.WriteString(s)
	return nil
}
func (w *diffWriter) line(prefix, line string) error {
	if err := w.append(prefix); err != nil {
		return err
	}
	if err := w.append(line); err != nil {
		return err
	}
	if !strings.HasSuffix(line, "\n") {
		return w.append("\n\\ No newline at end of file\n")
	}
	return nil
}
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Reveal normally preserves LF boundaries. Equal-length inputs therefore use
// positional line alignment and merged context windows, in linear time/space.
// Different line counts use one whole-range replacement hunk. Minimal edit
// distance is intentionally not promised; no quadratic LCS allocation is used.
func unified(before, after, representation string, limits DiffLimits, remaining *int) (Unified, error) {
	result := Unified{Representation: representation, BeforeSHA256: evidence.Hash([]byte(before)), AfterSHA256: evidence.Hash([]byte(after))}
	oldCount, newCount := countLines(before), countLines(after)
	if newCount > limits.Lines || oldCount > limits.Lines-newCount {
		return Unified{}, ErrDiffLimit
	}
	w := diffWriter{remaining: remaining}
	if before == after {
		result.SHA256 = evidence.Hash(nil)
		return result, nil
	}
	labelBefore, labelAfter := "decoded.txt", "revealed.txt"
	if representation == "ascii_escaped" {
		labelBefore, labelAfter = "escaped-decoded.txt", "escaped-revealed.txt"
	}
	if err := w.append("--- a/" + labelBefore + "\n+++ b/" + labelAfter + "\n"); err != nil {
		return Unified{}, err
	}
	old, new := splitLines(before), splitLines(after)
	if oldCount != newCount {
		oldStart, newStart := 1, 1
		if oldCount == 0 {
			oldStart = 0
		}
		if newCount == 0 {
			newStart = 0
		}
		if err := w.append(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)); err != nil {
			return Unified{}, err
		}
		for _, line := range old {
			if err := w.line("-", line); err != nil {
				return Unified{}, err
			}
		}
		for _, line := range new {
			if err := w.line("+", line); err != nil {
				return Unified{}, err
			}
		}
	} else {
		changes := []int{}
		for i := range old {
			if old[i] != new[i] {
				changes = append(changes, i)
			}
		}
		for index := 0; index < len(changes); {
			start := max(0, changes[index]-limits.Context)
			end := min(oldCount, changes[index]+limits.Context+1)
			index++
			for index < len(changes) && changes[index]-limits.Context <= end {
				end = min(oldCount, changes[index]+limits.Context+1)
				index++
			}
			if err := w.append(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", start+1, end-start, start+1, end-start)); err != nil {
				return Unified{}, err
			}
			for i := start; i < end; {
				if old[i] == new[i] {
					if err := w.line(" ", old[i]); err != nil {
						return Unified{}, err
					}
					i++
					continue
				}
				stop := i + 1
				for stop < end && old[stop] != new[stop] {
					stop++
				}
				for _, line := range old[i:stop] {
					if err := w.line("-", line); err != nil {
						return Unified{}, err
					}
				}
				for _, line := range new[i:stop] {
					if err := w.line("+", line); err != nil {
						return Unified{}, err
					}
				}
				i = stop
			}
		}
	}
	result.Text = w.text.String()
	result.SHA256 = evidence.Hash([]byte(result.Text))
	return result, nil
}
