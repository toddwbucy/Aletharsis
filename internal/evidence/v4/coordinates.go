// Package v4 implements the Office evidence contract. Validation establishes
// internal consistency, never source authenticity or permission to modify it.
package v4

import (
	"errors"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

type Span struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

func (s Span) Within(size int64) bool { return s.Start >= 0 && s.End >= s.Start && s.End <= size }

type Scalar struct {
	Index          int    `json:"scalar"`
	CodePoint      int    `json:"code_point"`
	Source         Span   `json:"source"`
	UTF8           Span   `json:"utf8"`
	Transformation string `json:"transformation"`
}
type Segment struct {
	Token       int      `json:"token"`
	Element     *int     `json:"element"`
	TokenSpan   Span     `json:"token_span"`
	ContentSpan Span     `json:"content_span"`
	CDATA       bool     `json:"cdata"`
	Text        string   `json:"text"`
	SHA256      string   `json:"sha256"`
	Scalars     []Scalar `json:"scalars"`
}

// Origin is decoded only after the closed wire discriminator has been checked.
// Pointers distinguish absent stored indices from zero and generated controls.
type Origin struct {
	Kind           string `json:"kind"`
	XMLRef         string `json:"xml_ref"`
	TextIndex      *int   `json:"text_index,omitempty"`
	Segment        *int   `json:"segment,omitempty"`
	Scalar         *int   `json:"scalar,omitempty"`
	Control        *int   `json:"control,omitempty"`
	Element        *int   `json:"element,omitempty"`
	Repetition     *int   `json:"repetition,omitempty"`
	Source         Span   `json:"source"`
	UTF8           Span   `json:"utf8"`
	Transformation string `json:"transformation"`
}

var ErrCoordinates = errors.New("Office coordinate linkage is invalid")

// ValidateSegment verifies the complete scalar map without opening any source.
// Lexical correspondence is not asserted: source bytes must be checked separately.
func ValidateSegment(s Segment, partBytes int64) error {
	if !utf8.ValidString(s.Text) || identity.ExactBytes([]byte(s.Text)) != s.SHA256 ||
		!s.TokenSpan.Within(partBytes) || !s.ContentSpan.Within(partBytes) ||
		s.ContentSpan.Start < s.TokenSpan.Start || s.ContentSpan.End > s.TokenSpan.End ||
		s.Scalars == nil || len(s.Scalars) != utf8.RuneCountInString(s.Text) {
		return ErrCoordinates
	}
	i := 0
	for offset, r := range s.Text {
		m := s.Scalars[i]
		if m.Index != i || m.CodePoint != int(r) || m.UTF8 != (Span{int64(offset), int64(offset + utf8.RuneLen(r))}) ||
			!m.Source.Within(partBytes) || m.Source.Start < s.ContentSpan.Start ||
			m.Source.End > s.ContentSpan.End || m.Source.Start == m.Source.End || m.Transformation == "" {
			return ErrCoordinates
		}
		if i > 0 && m.Source.Start < s.Scalars[i-1].Source.End {
			return ErrCoordinates
		}
		i++
	}
	return nil
}

// ValidateStoredOrigins proves every scope scalar points to the declared XML
// scalar. XML maps must already have passed ValidateSegment. Generated controls
// require their own element/control verification and are not accepted here.
// TextIndex is a producer-local ordinal, not an XML segment index. The record
// validator must bind it to its own producer inventory where reconstructible.
func ValidateStoredOrigins(text, digest string, origins []Origin, segments map[string][]Segment) error {
	if !utf8.ValidString(text) || identity.ExactBytes([]byte(text)) != digest ||
		origins == nil || len(origins) != utf8.RuneCountInString(text) {
		return ErrCoordinates
	}
	i := 0
	for offset, r := range text {
		o := origins[i]
		if o.Kind != "stored" || o.TextIndex == nil || *o.TextIndex < 0 ||
			o.Segment == nil || o.Scalar == nil || o.Control != nil || o.Element != nil || o.Repetition != nil {
			return ErrCoordinates
		}
		ss, ok := segments[o.XMLRef]
		if !ok || *o.Segment < 0 || *o.Segment >= len(ss) {
			return ErrCoordinates
		}
		scalars := ss[*o.Segment].Scalars
		if *o.Scalar < 0 || *o.Scalar >= len(scalars) {
			return ErrCoordinates
		}
		m := scalars[*o.Scalar]
		if m.CodePoint != int(r) || o.Source != m.Source || o.Transformation != m.Transformation ||
			o.UTF8 != (Span{int64(offset), int64(offset + utf8.RuneLen(r))}) {
			return ErrCoordinates
		}
		i++
	}
	return nil
}

// Control describes an admitted ODF control, not literal character data.
type Control struct {
	Index   int    `json:"index"`
	Element int    `json:"element"`
	Kind    string `json:"kind"`
	Count   int    `json:"count"`
	Source  Span   `json:"source"`
}

// ValidateControlOrigin proves a generated scalar's declared derivation. The
// caller separately binds the control's element and region to verified XML.
func ValidateControlOrigin(o Origin, r rune, controls map[string][]Control) error {
	if o.Kind != "control_expansion" || o.Control == nil || o.Element == nil || o.Repetition == nil ||
		o.TextIndex != nil || o.Segment != nil || o.Scalar != nil {
		return ErrCoordinates
	}
	cs, ok := controls[o.XMLRef]
	if !ok || *o.Control < 0 || *o.Control >= len(cs) {
		return ErrCoordinates
	}
	c := cs[*o.Control]
	if c.Index != *o.Control || c.Element != *o.Element || *o.Repetition < 0 ||
		*o.Repetition >= c.Count || c.Source != o.Source || c.Kind != o.Transformation {
		return ErrCoordinates
	}
	expected, ok := map[string]rune{"space": ' ', "tab": '\t', "line_break": '\n'}[c.Kind]
	if !ok || r != expected {
		return ErrCoordinates
	}
	return nil
}
