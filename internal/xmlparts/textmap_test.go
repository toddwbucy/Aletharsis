package xmlparts

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

func mapped(t testing.TB, s string, limit int) *MappedDocument {
	t.Helper()
	b := []byte(s)
	d, err := ParseWithTextMaps(context.Background(), b, evidence.Hash(b), DefaultLimits(), limit)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func TestManualTextMappings(t *testing.T) {
	source := "<r>A&#x200B;&amp;\r\n<![CDATA[😀\r\n]]></r>"
	d := mapped(t, source, 6)
	if d.Mapper != TextMapVersion || len(d.Segments) != 2 {
		t.Fatal("segments")
	}
	first, second := d.Segments[0], d.Segments[1]
	want := []ScalarMapping{
		{0, 'A', Span{3, 4}, Span{0, 1}, "literal"},
		{1, '\u200b', Span{4, 12}, Span{1, 4}, "character_reference"},
		{2, '&', Span{12, 17}, Span{4, 5}, "entity_reference"},
		{3, '\n', Span{17, 19}, Span{5, 6}, "line_ending"},
	}
	if !reflect.DeepEqual(first.Scalars, want) || first.Text != "A\u200b&\n" || first.TokenSpan != (Span{3, 19}) || first.ContentSpan != first.TokenSpan || first.CDATA {
		t.Fatal("manual entity mappings", first)
	}
	want = []ScalarMapping{{0, '😀', Span{28, 32}, Span{0, 4}, "literal"}, {1, '\n', Span{32, 34}, Span{4, 5}, "line_ending"}}
	if !reflect.DeepEqual(second.Scalars, want) || second.TokenSpan != (Span{19, 37}) || second.ContentSpan != (Span{28, 34}) || !second.CDATA || second.Text != "😀\n" {
		t.Fatal("manual CDATA mappings", second)
	}
	for _, s := range d.Segments {
		if s.DecodedSHA256 != evidence.Hash([]byte(s.Text)) || d.Document.Tokens[s.Token].Element != s.Element {
			t.Fatal("identity/linkage")
		}
	}
}
func TestReferenceWhitespaceRemainsDistinct(t *testing.T) {
	d := mapped(t, "<r>&#13;\n&#x9;&lt;&gt;&apos;&quot;</r>", 7)
	s := d.Segments[0]
	if s.Text != "\r\n\t<>'\"" || s.Scalars[0].Source != (Span{3, 8}) || s.Scalars[0].Transformation != "character_reference" || s.Scalars[1].Source != (Span{8, 9}) || s.Scalars[1].Transformation != "literal" {
		t.Fatal("reference normalization confused", s)
	}
	c := mapped(t, "<r><![CDATA[&amp;]]></r>", 5).Segments[0]
	if c.Text != "&amp;" || len(c.Scalars) != 5 {
		t.Fatal("decoded CDATA entity")
	}
	for _, m := range c.Scalars {
		if m.Transformation != "literal" {
			t.Fatal("CDATA literal misclassified")
		}
	}
}
func TestBOMCombiningAndTokenScope(t *testing.T) {
	d := mapped(t, "\ufeff<r a='ignored'><!-- ignored --><?agent ignored?>e\u0301\u200b</r>\n", 4)
	if len(d.Segments) != 2 || d.Segments[0].Text != "e\u0301\u200b" || len(d.Segments[0].Scalars) != 3 || d.Segments[1].Element != -1 {
		t.Fatal("mapped non-character data or lost outside whitespace")
	}
	for _, s := range d.Segments {
		for _, m := range s.Scalars {
			if m.Source.Start < 3 {
				t.Fatal("BOM offsets not applied")
			}
		}
	}
	d = mapped(t, "<r><![CDATA[]]></r>", 1)
	if len(d.Segments) != 1 || len(d.Segments[0].Scalars) != 0 || d.Segments[0].ContentSpan.Start != d.Segments[0].ContentSpan.End {
		t.Fatal("empty CDATA")
	}
}
func TestMappingBoundsIdentityAndFailure(t *testing.T) {
	source := []byte("<r>ab<s/>cd</r>")
	for _, limit := range []int{0, 3, MaxScalarMappings + 1} {
		d, err := ParseWithTextMaps(context.Background(), source, evidence.Hash(source), DefaultLimits(), limit)
		if d != nil || !errors.Is(err, ErrLimit) {
			t.Fatal("mapping limit", limit, err)
		}
	}
	d := mapped(t, string(source), 4)
	if len(d.Segments) != 2 {
		t.Fatal("budget charged per document incorrectly")
	}
	if d, err := ParseWithTextMaps(context.Background(), source, strings.Repeat("0", 64), DefaultLimits(), 4); d != nil || !errors.Is(err, ErrIdentity) {
		t.Fatal("identity", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if d, err := ParseWithTextMaps(ctx, source, evidence.Hash(source), DefaultLimits(), 4); d != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	for _, s := range []string{"<r>&unknown;</r>", "<r>&#0;</r>", "<r>&#xD800;</r>"} {
		b := []byte(s)
		if d, err := ParseWithTextMaps(context.Background(), b, evidence.Hash(b), DefaultLimits(), 100); d != nil || err == nil {
			t.Fatal("invalid XML accepted")
		}
	}
	if _, err := mapText(context.Background(), []byte("x"), Token{Span: Span{0, 1}, Value: "y"}, 1); !errors.Is(err, ErrMapping) {
		t.Fatal("decoder/map mismatch accepted")
	}
}
func TestMappedOutputDeterminismAndOwnership(t *testing.T) {
	source := []byte("<r>A&#x200B;</r>")
	before := bytes.Clone(source)
	d, err := ParseWithTextMaps(context.Background(), source, evidence.Hash(source), DefaultLimits(), 10)
	if err != nil {
		t.Fatal(err)
	}
	again := mapped(t, string(source), 10)
	if !reflect.DeepEqual(d, again) || !bytes.Equal(source, before) {
		t.Fatal("nondeterministic or changed source")
	}
	d.Segments[0].Scalars[0].Source.Start = 0
	if again.Segments[0].Scalars[0].Source.Start != 3 || !bytes.Equal(source, before) {
		t.Fatal("shared mutable maps")
	}
}
func mapInvariants(t testing.TB, source []byte, d *MappedDocument) {
	t.Helper()
	for _, s := range d.Segments {
		position, decoded := s.ContentSpan.Start, int64(0)
		var text strings.Builder
		for i, m := range s.Scalars {
			if m.Scalar != i || m.Source.Start != position || m.Source.End <= m.Source.Start || m.Source.End > s.ContentSpan.End || m.UTF8.Start != decoded || m.UTF8.End-m.UTF8.Start != int64(utf8.RuneLen(m.CodePoint)) {
				t.Fatal("invalid scalar partition", m)
			}
			position = m.Source.End
			decoded = m.UTF8.End
			text.WriteRune(m.CodePoint)
			if m.Transformation == "literal" && string(source[m.Source.Start:m.Source.End]) != string(m.CodePoint) {
				t.Fatal("literal bytes differ")
			}
		}
		if position != s.ContentSpan.End || decoded != int64(len(s.Text)) || text.String() != s.Text || s.DecodedSHA256 != evidence.Hash([]byte(s.Text)) {
			t.Fatal("mapping reconstruction")
		}
	}
}
func FuzzXMLTextMaps(f *testing.F) {
	for _, s := range []string{"<r>A&#x200B;&amp;\r\n<![CDATA[😀]]></r>", "<r>&#13;\r\n</r>", "<r/>"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			t.Skip()
		}
		d, err := ParseWithTextMaps(context.Background(), b, evidence.Hash(b), DefaultLimits(), MaxScalarMappings)
		if err != nil {
			if errors.Is(err, ErrMapping) {
				t.Fatal("valid XML mapping disagreement", err)
			}
			if d != nil {
				t.Fatal("partial mapping")
			}
			return
		}
		mapInvariants(t, b, d)
	})
}

func TestUnicodeFindingJoinsThroughDecodedScalar(t *testing.T) {
	source := "<r>A&#x200B;B</r>"
	mapped := mapped(t, source, 3)
	segment := mapped.Segments[0]
	decoded, err := (parsers.TextParser{}).Parse([]byte(segment.Text))
	if err != nil {
		t.Fatal(err)
	}
	findings := (analyzers.Unicode{}).Analyze(&decoded)
	found := false
	for _, finding := range findings {
		if finding.ID != "unicode.zero_width" {
			continue
		}
		positions, ok := finding.Location["character_offsets"].([]int)
		if !ok || len(positions) != 1 || positions[0] != 1 {
			t.Fatal("unexpected analyzer coordinate", finding.Location)
		}
		span := segment.Scalars[positions[0]].Source
		if span != (Span{4, 12}) || source[span.Start:span.End] != "&#x200B;" || segment.Scalars[1].Transformation != "character_reference" {
			t.Fatal("finding did not resolve to original lexical evidence")
		}
		found = true
	}
	if !found {
		t.Fatal("native Unicode finding absent")
	}
}

func TestMapParsedMatchesSinglePass(t *testing.T) {
	for _, source := range []string{"<r/>", "\ufeff \r\n<r>A&#x200B;&amp;<![CDATA[😀\r\n]]><x/>z</r>\n", "<r xmlns='urn:test'>é</r>"} {
		b := []byte(source)
		doc, err := Parse(context.Background(), b, evidence.Hash(b), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		got, err := MapParsed(context.Background(), b, doc, MaxScalarMappings)
		if err != nil {
			t.Fatal(err)
		}
		want := mapped(t, source, MaxScalarMappings)
		if got.Document != doc || !reflect.DeepEqual(got, want) {
			t.Fatal("mapped parse diverged")
		}
		if _, err := MapParsed(context.Background(), append(append([]byte{}, b...), byte(' ')), doc, MaxScalarMappings); !errors.Is(err, ErrIdentity) {
			t.Fatal("different bytes accepted", err)
		}
	}
	b := []byte("<r>ab</r>")
	doc, err := Parse(context.Background(), b, evidence.Hash(b), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := MapParsed(context.Background(), b, doc, 1); got != nil || !errors.Is(err, ErrLimit) {
		t.Fatal("partial mapping survived", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := MapParsed(ctx, b, doc, 10); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled mapping survived", err)
	}
	if got, err := MapParsed(context.Background(), b, nil, 10); got != nil || !errors.Is(err, ErrIdentity) {
		t.Fatal("nil document accepted", err)
	}
}
