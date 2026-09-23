package v4

import (
	"context"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func number(n int) *int { return &n }
func TestSegmentCoordinatesFromXMLMapper(t *testing.T) {
	// Exercise the mapper's carrier classes, including non-BMP UTF-8, entities,
	// newline normalization, CDATA, invisible scalars, and ordinary combining text.
	for _, input := range []string{"<r>A😀é</r>", "<r>&#x200B;&amp;&#65;</r>",
		"<r>A\r\nB\rC</r>", "<r><![CDATA[A😀\r\nB]]></r>", "<r>A\u200bB</r>"} {
		t.Run(input, func(t *testing.T) {
			raw := []byte(input)
			mapped, err := xmlparts.ParseWithTextMaps(context.Background(), raw, identity.ExactBytes(raw), xmlparts.DefaultLimits(), 100)
			if err != nil {
				t.Fatal(err)
			}
			for _, parsed := range mapped.Segments {
				s := Segment{Token: parsed.Token, Element: number(parsed.Element),
					TokenSpan:   Span{parsed.TokenSpan.Start, parsed.TokenSpan.End},
					ContentSpan: Span{parsed.ContentSpan.Start, parsed.ContentSpan.End},
					CDATA:       parsed.CDATA, Text: parsed.Text, SHA256: parsed.DecodedSHA256, Scalars: []Scalar{}}
				origins := []Origin{}
				for _, m := range parsed.Scalars {
					s.Scalars = append(s.Scalars, Scalar{m.Scalar, int(m.CodePoint),
						Span{m.Source.Start, m.Source.End}, Span{m.UTF8.Start, m.UTF8.End}, m.Transformation})
					origins = append(origins, Origin{Kind: "stored", XMLRef: "office-xml/0", TextIndex: number(0),
						Segment: number(0), Scalar: number(m.Scalar), Source: Span{m.Source.Start, m.Source.End},
						UTF8: Span{m.UTF8.Start, m.UTF8.End}, Transformation: m.Transformation})
				}
				if err := ValidateSegment(s, int64(len(raw))); err != nil {
					t.Fatal(err)
				}
				segments := map[string][]Segment{"office-xml/0": {s}}
				if err := ValidateStoredOrigins(s.Text, s.SHA256, origins, segments); err != nil {
					t.Fatal(err)
				}
				if len(origins) == 0 {
					continue
				}
				for _, mutate := range []func(*Origin){
					func(o *Origin) { o.Source.End++ }, func(o *Origin) { o.UTF8.Start++ },
					func(o *Origin) { o.XMLRef = "office-xml/999" }, func(o *Origin) { o.Segment = number(-1) },
					func(o *Origin) { o.Scalar = number(999) }, func(o *Origin) { o.Control = number(0) },
					func(o *Origin) { o.Transformation = "invented" }, func(o *Origin) { o.Kind = "control_expansion" },
				} {
					changed := append([]Origin{}, origins...)
					mutate(&changed[0])
					if ValidateStoredOrigins(s.Text, s.SHA256, changed, segments) == nil {
						t.Fatal("accepted damaged origin")
					}
				}
				if ValidateStoredOrigins(s.Text, s.SHA256, origins[:len(origins)-1], segments) == nil {
					t.Fatal("accepted truncated origins")
				}
				changed := s
				changed.Scalars = append([]Scalar{}, s.Scalars...)
				changed.Scalars[0].UTF8.End++
				if ValidateSegment(changed, int64(len(raw))) == nil {
					t.Fatal("accepted incorrect scalar extent")
				}
				changed = s
				changed.SHA256 = identity.ExactBytes([]byte("other"))
				if ValidateSegment(changed, int64(len(raw))) == nil {
					t.Fatal("accepted hash mismatch")
				}
			}
		})
	}
}

func TestGeneratedControlOrigins(t *testing.T) {
	for kind, r := range map[string]rune{"space": ' ', "tab": '\t', "line_break": '\n'} {
		t.Run(kind, func(t *testing.T) {
			control := Control{Index: 0, Element: 2, Kind: kind, Count: 3, Source: Span{20, 30}}
			controls := map[string][]Control{"office-xml/0": {control}}
			origin := Origin{Kind: "control_expansion", XMLRef: "office-xml/0",
				Control: number(0), Element: number(2), Repetition: number(2), Source: control.Source,
				UTF8: Span{0, 1}, Transformation: kind}
			if err := ValidateControlOrigin(origin, r, controls); err != nil {
				t.Fatal(err)
			}
			for _, mutate := range []func(*Origin){
				func(o *Origin) { o.Repetition = number(3) }, func(o *Origin) { o.Repetition = number(-1) },
				func(o *Origin) { o.Element = number(3) }, func(o *Origin) { o.Source.Start++ },
				func(o *Origin) { o.Scalar = number(0) }, func(o *Origin) { o.XMLRef = "office-xml/1" },
			} {
				bad := origin
				mutate(&bad)
				if ValidateControlOrigin(bad, r, controls) == nil {
					t.Fatal("accepted invented control linkage")
				}
			}
			if ValidateControlOrigin(origin, 'X', controls) == nil {
				t.Fatal("accepted wrong generated character")
			}
		})
	}
}
