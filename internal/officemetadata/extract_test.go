package officemetadata

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func core(body string) []byte {
	return []byte(`<cp:coreProperties xmlns:cp="` + CoreNamespace + `" xmlns:dc="` + dcNamespace + `" xmlns:dcterms="` + termsNamespace + `">` + body + `</cp:coreProperties>`)
}
func extract(t *testing.T, raw []byte) *Result {
	t.Helper()
	r, err := Extract(context.Background(), raw, evidence.Hash(raw))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestValuesDuplicatesAndLexicalMaps(t *testing.T) {
	raw := core(`<dc:creator>A&#x200B;<![CDATA[😀]]></dc:creator><dc:creator>B</dc:creator><dcterms:created>not a date</dcterms:created><cp:revision>007</cp:revision>`)
	before := append([]byte{}, raw...)
	r := extract(t, raw)
	if r.State != "completed" || len(r.Properties) != 4 {
		t.Fatalf("%+v", r)
	}
	if r.Properties[0].Value != "A\u200b😀" || r.Properties[1].Occurrence != 1 || r.Properties[2].Value != "not a date" || r.Properties[3].Value != "007" {
		t.Fatalf("%+v", r.Properties)
	}
	p := r.Properties[0]
	if len(p.Segments) != 2 {
		t.Fatalf("segments: %v", p.Segments)
	}
	scalar := r.XML.Segments[p.Segments[0]].Scalars[1]
	if string(raw[scalar.Source.Start:scalar.Source.End]) != "&#x200B;" {
		t.Fatalf("%+v", scalar)
	}
	emoji := r.XML.Segments[p.Segments[1]].Scalars[0]
	if string(raw[emoji.Source.Start:emoji.Source.End]) != "😀" {
		t.Fatalf("%+v", emoji)
	}
	if !bytes.Equal(before, raw) || !reflect.DeepEqual(r, extract(t, raw)) {
		t.Fatal("mutation or nondeterminism")
	}
}
func TestUnsupportedContextDoesNotHideNeighbors(t *testing.T) {
	raw := core(`<dc:creator>before</dc:creator><cp:unknown><dc:creator>hidden</dc:creator></cp:unknown><dc:creator><dc:creator>nested</dc:creator></dc:creator><dc:creator xmlns:dc="urn:wrong">spoof</dc:creator><dc:creator>after</dc:creator>`)
	r := extract(t, raw)
	if r.State != "partial" || len(r.Issues) != 3 || len(r.Properties) != 2 {
		t.Fatalf("%+v", r)
	}
	if r.Properties[0].Value != "before" || r.Properties[1].Value != "after" || r.Properties[1].Occurrence != 2 {
		t.Fatalf("%+v", r.Properties)
	}
	if r.Issues[1].Code != "metadata.structured_value_unassessed" {
		t.Fatal(r.Issues)
	}
	for _, issue := range r.Issues {
		if issue.Element <= 0 || issue.Element >= len(r.XML.Document.Elements) {
			t.Fatal(issue)
		}
	}
}
func TestApplicationAndEmptyProperty(t *testing.T) {
	raw := []byte(`<Properties xmlns="` + AppNamespace + `"><Application>Writer</Application><AppVersion>1</AppVersion><Company/><Template>x</Template></Properties>`)
	r := extract(t, raw)
	if r.Kind != "app" || r.State != "completed" || len(r.Properties) != 4 || r.Properties[2].Value != "" || r.Properties[2].Segments == nil {
		t.Fatalf("%+v", r)
	}
	keys := []string{}
	for _, p := range r.Properties {
		keys = append(keys, p.Key)
	}
	if !reflect.DeepEqual(keys, []string{"application", "application_version", "company", "template"}) {
		t.Fatal(keys)
	}
}
func TestRootTextAndNamespace(t *testing.T) {
	r := extract(t, core(`stray<dc:creator>x</dc:creator>`))
	if r.State != "partial" || r.Issues[0].Code != "metadata.root_text_unassessed" || len(r.Properties) != 1 {
		t.Fatalf("%+v", r)
	}
	raw := []byte(`<coreProperties><creator>x</creator></coreProperties>`)
	if r, err := Extract(context.Background(), raw, evidence.Hash(raw)); r != nil || !errors.Is(err, ErrStructure) {
		t.Fatalf("%v %v", r, err)
	}
}
func TestFailedPartDoesNotPoisonNextCall(t *testing.T) {
	for _, raw := range [][]byte{core(`<dc:creator>`), []byte(`<!DOCTYPE x SYSTEM "https://example.invalid"><x/>`)} {
		if r, err := Extract(context.Background(), raw, evidence.Hash(raw)); r != nil || err == nil {
			t.Fatalf("%v %v", r, err)
		}
		if r := extract(t, core(`<dc:creator>good</dc:creator>`)); r.State != "completed" {
			t.Fatal(r)
		}
	}
	raw := core("")
	if r, err := Extract(context.Background(), raw, strings.Repeat("0", 64)); r != nil || !errors.Is(err, xmlparts.ErrIdentity) {
		t.Fatalf("%v %v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Extract(ctx, raw, evidence.Hash(raw)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("%v %v", r, err)
	}
}
func TestScalarLimit(t *testing.T) {
	raw := core(`<dc:creator>` + strings.Repeat("a", xmlparts.MaxScalarMappings+1) + `</dc:creator>`)
	if r, err := Extract(context.Background(), raw, evidence.Hash(raw)); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
		t.Fatalf("%v %v", r, err)
	}
}

func TestOnlyXMLWhitespaceIsIgnorableAtRoot(t *testing.T) {
	for _, body := range []string{"\u00a0", "\u200b"} {
		r := extract(t, core(body))
		if r.State != "partial" || len(r.Issues) != 1 {
			t.Fatalf("body %q: %+v", body, r)
		}
	}
	for _, body := range []string{"", " \t\r\n"} {
		r := extract(t, core(body))
		if r.State != "completed" || len(r.Properties) != 0 {
			t.Fatalf("body %q: %+v", body, r)
		}
	}
}

func TestRootWhitespaceLexicalFormAndIssueLocations(t *testing.T) {
	for _, body := range []string{"&#x20;", "&#xD;", "&#32;", "<![CDATA[   ]]>", "<![CDATA[]]>"} {
		raw := core(body + `<dc:creator>good</dc:creator>`)
		r := extract(t, raw)
		if r.State != "partial" || len(r.Issues) != 1 || len(r.Properties) != 1 {
			t.Fatalf("%q: %+v", body, r)
		}
		issue := r.Issues[0]
		if issue.Segment < 0 || issue.Attribute != -1 || string(raw[issue.Span.Start:issue.Span.End]) != body {
			t.Fatalf("%q: %+v", body, issue)
		}
	}
	raw := core(`x<cp:unknown/><dc:creator>a</dc:creator>y`)
	r := extract(t, raw)
	if len(r.Issues) != 3 || r.Issues[0].Code != "metadata.root_text_unassessed" || r.Issues[1].Code != "metadata.property_unassessed" || r.Issues[2].Code != "metadata.root_text_unassessed" {
		t.Fatalf("%+v", r.Issues)
	}
	if r.Issues[0].Segment == r.Issues[2].Segment || r.Issues[0].Span == r.Issues[2].Span {
		t.Fatal("root observations collapsed")
	}
	for i := 1; i < len(r.Issues); i++ {
		if r.Issues[i-1].Span.Start > r.Issues[i].Span.Start {
			t.Fatal("not document order")
		}
	}
}

func TestRecognizedUnselectedPropertiesStayDistinctFromUnknown(t *testing.T) {
	for _, raw := range [][]byte{
		core(`<dc:title>T</dc:title><dc:subject>S</dc:subject><dc:description>D</dc:description><cp:keywords>K</cp:keywords><cp:category>C</cp:category><cp:contentStatus>X</cp:contentStatus><cp:alien/><dc:creator>A</dc:creator>`),
		[]byte(`<Properties xmlns="` + AppNamespace + `"><TotalTime>3</TotalTime><Pages>1</Pages><Words>10</Words><DocSecurity>0</DocSecurity><HeadingPairs><x/></HeadingPairs><alien/><Application>A</Application></Properties>`),
	} {
		r := extract(t, raw)
		if r.State != "partial" || len(r.Properties) != 1 || len(r.Issues) < 2 {
			t.Fatalf("%+v", r)
		}
		for i, issue := range r.Issues {
			want := "metadata.standard_property_unassessed"
			if i == len(r.Issues)-1 {
				want = "metadata.property_unassessed"
			}
			if issue.Code != want {
				t.Fatalf("%+v", r.Issues)
			}
		}
	}
	r := extract(t, core(`<dc:title xmlns:dc="urn:spoof">T</dc:title>`))
	if r.Issues[0].Code != "metadata.property_unassessed" {
		t.Fatal(r.Issues)
	}
}

func TestAttributesRetainedWithoutClaimingTheirSemantics(t *testing.T) {
	raw := core(`<dc:creator xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/><dcterms:created xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="dcterms:W3CDTF">not a date</dcterms:created><dc:creator>good</dc:creator>`)
	r := extract(t, raw)
	if r.State != "partial" || len(r.Issues) != 2 || len(r.Properties) != 3 {
		t.Fatalf("%+v", r)
	}
	for i := 0; i < 2; i++ {
		p := r.Properties[i]
		issue := r.Issues[i]
		if len(p.Attributes) != 1 || issue.Attribute != p.Attributes[0] || issue.Element != p.Element || issue.Segment != -1 || issue.Code != []string{"metadata.attribute_semantics_unassessed", "metadata.standard_attribute_unassessed"}[i] {
			t.Fatalf("%+v %+v", p, issue)
		}
		attr := r.XML.Document.Elements[p.Element].Attributes[p.Attributes[0]]
		if attr.NamespaceDeclaration || attr.Name.Namespace != "http://www.w3.org/2001/XMLSchema-instance" {
			t.Fatal(attr)
		}
	}
	if r.Properties[0].Value != "" || r.Properties[1].Value != "not a date" || r.Properties[2].Value != "good" || len(r.Properties[2].Attributes) != 0 {
		t.Fatal(r.Properties)
	}
	if len(r.Limitations) == 0 {
		t.Fatal("limitations missing")
	}
}

func TestUnsupportedMetadataRootsHaveTypedReasons(t *testing.T) {
	for _, tc := range []struct{ namespace, root, code string }{
		{"http://purl.oclc.org/ooxml/officeDocument/extendedProperties", "Properties", "metadata.strict_app_unsupported"},
		{"http://schemas.openxmlformats.org/officeDocument/2006/custom-properties", "Properties", "metadata.custom_properties_unsupported"},
		{"http://purl.oclc.org/ooxml/officeDocument/customProperties", "Properties", "metadata.custom_properties_unsupported"},
		{"urn:oasis:names:tc:opendocument:xmlns:office:1.0", "document-meta", "metadata.odf_unsupported"},
	} {
		raw := []byte(`<` + tc.root + ` xmlns="` + tc.namespace + `"/>`)
		r, err := Extract(context.Background(), raw, evidence.Hash(raw))
		var reason *UnsupportedRootError
		if r != nil || !errors.As(err, &reason) || reason.Code != tc.code || !errors.Is(err, ErrStructure) || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), ErrStructure.Error()) {
			t.Fatalf("%s: %v %v", tc.code, r, err)
		}
	}
	raw := []byte(`<notMetadata/>`)
	_, err := Extract(context.Background(), raw, evidence.Hash(raw))
	var reason *UnsupportedRootError
	if !errors.Is(err, ErrStructure) || errors.As(err, &reason) {
		t.Fatalf("%v", err)
	}
}

func TestBaselineCoverageSeparated(t *testing.T) {
	for _, raw := range [][]byte{
		core(`<dc:title>Title</dc:title><dc:creator>A</dc:creator><dcterms:created xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="dcterms:W3CDTF">2026-01-01</dcterms:created>`),
		[]byte(`<Properties xmlns="` + AppNamespace + `"><Pages>1</Pages><Words>2</Words><Application>Word</Application></Properties>`),
	} {
		r := extract(t, raw)
		if r.State != "partial" || r.Coverage.StandardProjectionGaps != 2 || r.Coverage.OtherGaps != 0 {
			t.Fatalf("%+v", r)
		}
		pos := bytes.LastIndex(raw, []byte("</"))
		altered := append(append(append([]byte{}, raw[:pos]...), []byte(`<alien xmlns="urn:other"/>`)...), raw[pos:]...)
		changed := extract(t, altered)
		if changed.Coverage.StandardProjectionGaps != 2 || changed.Coverage.OtherGaps != 1 || !reflect.DeepEqual(r.Properties, changed.Properties) {
			t.Fatalf("%+v", changed)
		}
	}
}

func TestDateTypeQNameResolution(t *testing.T) {
	for _, tc := range []struct {
		declarations, value string
		standard            bool
	}{
		{`xmlns:t="` + termsNamespace + `"`, "t:W3CDTF", true},
		{`xmlns:dcterms="urn:spoof"`, "dcterms:W3CDTF", false},
		{``, "missing:W3CDTF", false},
		{``, "dcterms:other", false},
		{`xmlns="` + termsNamespace + `"`, "W3CDTF", true},
		{``, "W3CDTF", false},
	} {
		raw := core(`<x:created xmlns:x="` + termsNamespace + `" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` + tc.declarations + ` xsi:type="` + tc.value + `">value</x:created>`)
		r := extract(t, raw)
		if len(r.Issues) != 1 || r.Issues[0].StandardProjectionGap != tc.standard || r.Properties[0].Value != "value" {
			t.Fatalf("%+v: %+v", tc, r)
		}
	}
}

func TestRootAttributesAndMarkupRetainLocations(t *testing.T) {
	for _, tc := range []struct{ root, namespace, property string }{{"coreProperties", CoreNamespace, "revision"}, {"Properties", AppNamespace, "Company"}} {
		raw := []byte(`<` + tc.root + ` xmlns="` + tc.namespace + `" flag="x"><!--root--><?root data?><` + tc.property + `>a<!--value--><?value data?>b</` + tc.property + `></` + tc.root + `>`)
		r := extract(t, raw)
		if r.State != "partial" || r.Coverage.OtherGaps != 5 || r.Coverage.StandardProjectionGaps != 0 || len(r.RootAttributes) != 1 || len(r.Properties) != 1 || r.Properties[0].Value != "ab" {
			t.Fatalf("%+v", r)
		}
		attr := r.XML.Document.Elements[0].Attributes[r.RootAttributes[0]]
		if attr.Name.Local != "flag" || attr.Value != "x" {
			t.Fatal(attr)
		}
		for i, want := range []string{"<!--root-->", "<?root data?>", "<!--value-->", "<?value data?>"} {
			issue := r.Issues[i+1]
			if issue.Code != "metadata.markup_unassessed" || issue.Token < 0 || issue.Segment != -1 || issue.Attribute != -1 || string(raw[issue.Span.Start:issue.Span.End]) != want || r.XML.Document.Tokens[issue.Token].Span != issue.Span {
				t.Fatalf("%+v", issue)
			}
		}
		clean := []byte(`<` + tc.root + ` xmlns="` + tc.namespace + `"><` + tc.property + `>ab</` + tc.property + `></` + tc.root + `>`)
		if result := extract(t, clean); result.State != "completed" || len(result.RootAttributes) != 0 {
			t.Fatalf("%+v", result)
		}
	}
}
