package officeplan

import (
	"context"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestXMLRecordPreservesEveryCoordinate(t *testing.T) {
	for _, source := range []string{"<r/>", "\ufeff \r\n<r>A&#x200B;&amp;<![CDATA[😀\r\n]]><x/>z</r>\n", "<r xmlns='urn:test'><x>é</x><!--review data--><?p data?></r>"} {
		b := []byte(source)
		digest := evidence.Hash(b)
		size := int64(len(b))
		mapped, err := xmlparts.ParseWithTextMaps(context.Background(), b, digest, xmlparts.DefaultLimits(), xmlparts.MaxScalarMappings)
		if err != nil {
			t.Fatal(err)
		}
		part := v4.Part{PartRef: "office-part/0", SHA256: &digest, ByteLength: &size}
		got, err := BuildXMLRecord(mapped, part, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Tokens) != len(mapped.Document.Tokens) || len(got.Elements) != len(mapped.Document.Elements) || len(got.Segments) != len(mapped.Segments) {
			t.Fatal("incomplete XML records")
		}
		for i, token := range got.Tokens {
			want := mapped.Document.Tokens[i]
			if token.Kind != want.Kind || token.Span != wireSpan(want.Span) || (token.Element == nil) != (want.Element < 0) {
				t.Fatal("token changed")
			}
		}
		for i, element := range got.Elements {
			want := mapped.Document.Elements[i]
			if element.Span != wireSpan(want.Full) || element.Namespace != want.Name.Namespace || element.LocalName != want.Name.Local || (element.Parent == nil) != (want.Parent < 0) {
				t.Fatal("element changed")
			}
		}
		for i, segment := range got.Segments {
			want := mapped.Segments[i]
			if segment.Text != want.Text || segment.SHA256 != want.DecodedSHA256 || segment.CDATA != want.CDATA || len(segment.Scalars) != len(want.Scalars) {
				t.Fatal("text changed")
			}
			for j, scalar := range segment.Scalars {
				origin := want.Scalars[j]
				if scalar.Source != wireSpan(origin.Source) || scalar.UTF8 != wireSpan(origin.UTF8) || scalar.Transformation != origin.Transformation || scalar.CodePoint != int(origin.CodePoint) {
					t.Fatal("scalar map changed")
				}
			}
		}
		again, err := BuildXMLRecord(mapped, part, 0)
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatal("nondeterministic XML records")
		}
		wrong := evidence.Hash(nil)
		part.SHA256 = &wrong
		if _, err := BuildXMLRecord(mapped, part, 0); err == nil {
			t.Fatal("foreign part accepted")
		}
	}
}
