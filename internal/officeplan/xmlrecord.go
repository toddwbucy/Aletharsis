package officeplan

import (
	"fmt"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

// BuildXMLRecord copies parser coordinates without reconstructing or normalizing
// source text. Format-specific issues and admitted ODF controls are attached by
// the inventory/scope coordinator before final graph validation.
func BuildXMLRecord(mapped *xmlparts.MappedDocument, part v4.Part, ordinal int) (v4.XML, error) {
	r, err := buildXMLRecord(mapped, part, ordinal)
	if err != nil {
		return v4.XML{}, err
	}
	if err := v4.ValidateXML(r, part); err != nil {
		return v4.XML{}, err
	}
	return r, nil
}

func buildXMLRecord(mapped *xmlparts.MappedDocument, part v4.Part, ordinal int) (v4.XML, error) {
	if mapped == nil || mapped.Document == nil || ordinal < 0 || part.SHA256 == nil || mapped.Document.PartSHA256 != *part.SHA256 {
		return v4.XML{}, xmlparts.ErrIdentity
	}
	d := mapped.Document
	r := v4.XML{XMLRef: fmt.Sprintf("office-xml/%d", ordinal), PartRef: part.PartRef, ParserVersion: d.Parser, MapperVersion: mapped.Mapper, Tokens: []v4.Token{}, Elements: []v4.Element{}, Segments: []v4.Segment{}, Controls: []v4.Control{}, Issues: []v4.Issue{}}
	for i, t := range d.Tokens {
		token := v4.Token{Index: int64(i), Kind: t.Kind, Span: wireSpan(t.Span)}
		if t.Element >= 0 {
			index := int64(t.Element)
			token.Element = &index
		}
		r.Tokens = append(r.Tokens, token)
	}
	for i, e := range d.Elements {
		element := v4.Element{Index: int64(i), Namespace: e.Name.Namespace, LocalName: e.Name.Local, Span: wireSpan(e.Full)}
		if e.Parent >= 0 {
			index := int64(e.Parent)
			element.Parent = &index
		}
		r.Elements = append(r.Elements, element)
	}
	for _, s := range mapped.Segments {
		segment := v4.Segment{Token: s.Token, TokenSpan: wireSpan(s.TokenSpan), ContentSpan: wireSpan(s.ContentSpan), CDATA: s.CDATA, Text: s.Text, SHA256: s.DecodedSHA256, Scalars: []v4.Scalar{}}
		if s.Element >= 0 {
			index := s.Element
			segment.Element = &index
		}
		for _, m := range s.Scalars {
			segment.Scalars = append(segment.Scalars, v4.Scalar{Index: m.Scalar, CodePoint: int(m.CodePoint), Source: wireSpan(m.Source), UTF8: wireSpan(m.UTF8), Transformation: m.Transformation})
		}
		r.Segments = append(r.Segments, segment)
	}
	return r, nil
}
func wireSpan(s xmlparts.Span) v4.Span { return v4.Span{Start: s.Start, End: s.End} }
