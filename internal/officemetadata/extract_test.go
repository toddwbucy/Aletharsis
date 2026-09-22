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
