package v4

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
	"testing"
)

func wide(n int) *int64 { v := int64(n); return &v }
func TestXMLRecordChecksAgainstParser(t *testing.T) {
	raw := []byte("<r>A&amp;<b><![CDATA[😀]]></b>Z</r>")
	hash := identity.ExactBytes(raw)
	size := int64(len(raw))
	part := Part{PartRef: "office-part/0", SHA256: &hash, ByteLength: &size}
	parsed, err := xmlparts.ParseWithTextMaps(context.Background(), raw, hash, xmlparts.DefaultLimits(), 100)
	if err != nil {
		t.Fatal(err)
	}
	x := XML{XMLRef: "office-xml/0", PartRef: part.PartRef, Tokens: []Token{}, Elements: []Element{}, Segments: []Segment{}, Controls: []Control{}}
	for i, e := range parsed.Document.Elements {
		var parent *int64
		if e.Parent >= 0 {
			parent = wide(e.Parent)
		}
		x.Elements = append(x.Elements, Element{Index: int64(i), Parent: parent, Namespace: e.Name.Namespace, LocalName: e.Name.Local, Span: Span{e.Full.Start, e.Full.End}})
	}
	for i, v := range parsed.Document.Tokens {
		var element *int64
		if v.Element >= 0 {
			element = wide(v.Element)
		}
		x.Tokens = append(x.Tokens, Token{Index: int64(i), Kind: v.Kind, Span: Span{v.Span.Start, v.Span.End}, Element: element})
	}
	for _, v := range parsed.Segments {
		s := Segment{Token: v.Token, Element: number(v.Element), TokenSpan: Span{v.TokenSpan.Start, v.TokenSpan.End}, ContentSpan: Span{v.ContentSpan.Start, v.ContentSpan.End}, Text: v.Text, SHA256: v.DecodedSHA256, CDATA: v.CDATA, Scalars: []Scalar{}}
		for _, m := range v.Scalars {
			s.Scalars = append(s.Scalars, Scalar{m.Scalar, int(m.CodePoint), Span{m.Source.Start, m.Source.End}, Span{m.UTF8.Start, m.UTF8.End}, m.Transformation})
		}
		x.Segments = append(x.Segments, s)
	}
	if err := ValidateXML(x, part); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*XML){
		func(v *XML) { v.PartRef = "office-part/1" }, func(v *XML) { v.Elements[1].Parent = wide(1) },
		func(v *XML) { v.Tokens[0].Element = wide(99) }, func(v *XML) { v.Tokens[0].Span.End = size + 1 },
		func(v *XML) { v.Segments = v.Segments[1:] }, func(v *XML) { v.Segments = append(v.Segments, v.Segments[0]) },
		func(v *XML) { v.Segments[0].Token = 0 }, func(v *XML) { v.Segments[0].Element = number(99) },
	} {
		bad := x
		bad.Elements = append([]Element{}, x.Elements...)
		bad.Tokens = append([]Token{}, x.Tokens...)
		bad.Segments = append([]Segment{}, x.Segments...)
		mutate(&bad)
		if ValidateXML(bad, part) == nil {
			t.Fatal("accepted damaged XML graph")
		}
	}
}
