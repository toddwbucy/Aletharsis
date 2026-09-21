package xmlparts

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

const TextMapVersion = "xml-text-map/1"
const MaxScalarMappings = 200000

var ErrMapping = errors.New("XML text mapping disagrees with parsed evidence")

// ScalarMapping joins one decoded Unicode scalar to its complete lexical source
// region. Entity references and line-ending normalization are transformations,
// not identical byte spans or authority to edit the original document.
type ScalarMapping struct {
	Scalar         int
	CodePoint      rune
	Source, UTF8   Span
	Transformation string
}
type TextSegment struct {
	Token, Element         int
	TokenSpan, ContentSpan Span
	CDATA                  bool
	Text, DecodedSHA256    string
	Scalars                []ScalarMapping
}
type MappedDocument struct {
	Mapper   string
	Document *Document
	Segments []TextSegment
}

// ParseWithTextMaps performs one XML parse, then maps every character-data token.
// It includes outside-root whitespace but not attributes/comments/instructions.
// Source regions index original part bytes; UTF8 regions index each segment's
// decoded text. All errors return no partially mapped document.
func ParseWithTextMaps(ctx context.Context, source []byte, expectedSHA256 string, limits Limits, maxMappings int) (*MappedDocument, error) {
	if maxMappings < 1 || maxMappings > MaxScalarMappings {
		return nil, ErrLimit
	}
	doc, err := Parse(ctx, source, expectedSHA256, limits)
	if err != nil {
		return nil, err
	}
	result := &MappedDocument{Mapper: TextMapVersion, Document: doc, Segments: []TextSegment{}}
	remaining := maxMappings
	for i, token := range doc.Tokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if token.Kind != "text" {
			continue
		}
		segment, err := mapText(ctx, source, token, remaining)
		if err != nil {
			return nil, err
		}
		segment.Token = i
		remaining -= len(segment.Scalars)
		result.Segments = append(result.Segments, segment)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func mapText(ctx context.Context, source []byte, token Token, remaining int) (TextSegment, error) {
	empty := TextSegment{}
	if token.Span.Start < 0 || token.Span.End < token.Span.Start || token.Span.End > int64(len(source)) {
		return empty, ErrMapping
	}
	span := token.Span
	raw := source[span.Start:span.End]
	segment := TextSegment{Element: token.Element, TokenSpan: span, ContentSpan: span, Text: token.Value, Scalars: []ScalarMapping{}}
	if bytes.HasPrefix(raw, []byte("<![CDATA[")) {
		if len(raw) < 12 || !bytes.HasSuffix(raw, []byte("]]>")) {
			return empty, ErrMapping
		}
		segment.CDATA = true
		segment.ContentSpan.Start += 9
		segment.ContentSpan.End -= 3
	}
	body := source[segment.ContentSpan.Start:segment.ContentSpan.End]
	var decoded strings.Builder
	for pos := 0; pos < len(body); {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if len(segment.Scalars) >= remaining {
			return empty, ErrLimit
		}
		start := pos
		kind := "literal"
		var r rune
		switch {
		case body[pos] == '&' && !segment.CDATA:
			semi := bytes.IndexByte(body[pos:], ';')
			if semi < 0 {
				return empty, ErrMapping
			}
			reference := string(body[pos+1 : pos+semi])
			var err error
			r, kind, err = referenceRune(reference)
			if err != nil {
				return empty, err
			}
			pos += semi + 1
		case body[pos] == '\r':
			r = '\n'
			kind = "line_ending"
			pos++
			if pos < len(body) && body[pos] == '\n' {
				pos++
			}
		default:
			var size int
			r, size = utf8.DecodeRune(body[pos:])
			if size == 0 || r == utf8.RuneError && size == 1 {
				return empty, ErrMapping
			}
			pos += size
		}
		before := decoded.Len()
		decoded.WriteRune(r)
		segment.Scalars = append(segment.Scalars, ScalarMapping{Scalar: len(segment.Scalars), CodePoint: r, Source: Span{segment.ContentSpan.Start + int64(start), segment.ContentSpan.Start + int64(pos)}, UTF8: Span{int64(before), int64(decoded.Len())}, Transformation: kind})
	}
	if decoded.String() != token.Value {
		return empty, ErrMapping
	}
	segment.DecodedSHA256 = evidence.Hash([]byte(token.Value))
	return segment, nil
}
func referenceRune(reference string) (rune, string, error) {
	switch reference {
	case "amp":
		return '&', "entity_reference", nil
	case "lt":
		return '<', "entity_reference", nil
	case "gt":
		return '>', "entity_reference", nil
	case "apos":
		return '\'', "entity_reference", nil
	case "quot":
		return '"', "entity_reference", nil
	}
	if !strings.HasPrefix(reference, "#") {
		return 0, "", ErrMapping
	}
	digits := reference[1:]
	base := 10
	if strings.HasPrefix(digits, "x") {
		base = 16
		digits = digits[1:]
	}
	value, err := strconv.ParseUint(digits, base, 32)
	if err != nil || !utf8.ValidRune(rune(value)) {
		return 0, "", ErrMapping
	}
	return rune(value), "character_reference", nil
}
