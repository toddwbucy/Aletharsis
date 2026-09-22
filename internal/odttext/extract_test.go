package odttext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func document(body string) string {
	return `<office:document-content xmlns:office="` + odtidentify.OfficeNS + `" xmlns:text="` + TextNS + `" office:version="1.3"><office:body><office:text>` + body + `</office:text></office:body></office:document-content>`
}
func extract(t testing.TB, source string) *Result {
	t.Helper()
	b := []byte(source)
	r, err := Extract(context.Background(), b, evidence.Hash(b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestTextControlsAndExactMappings(t *testing.T) {
	source := document(`<text:p>A&#x200B;😀<text:s text:c="3"/><text:span text:style-name="test">é</text:span><text:tab text:tab-ref="2"/><text:line-break/><![CDATA[B]]></text:p>`)
	r := extract(t, source)
	if r.State != "completed" || len(r.Texts) != 3 || len(r.Controls) != 3 {
		t.Fatalf("%+v", r)
	}
	if r.Controls[0].Kind != "spaces" || r.Controls[0].Count != 3 || r.Controls[1].Kind != "tab" || r.Controls[2].Kind != "line_break" {
		t.Fatal(r.Controls)
	}
	first := r.XML.Segments[r.Texts[0].Segment]
	m := first.Scalars[1]
	if first.Text != "A\u200b😀" || m.CodePoint != '\u200b' || source[m.Source.Start:m.Source.End] != "&#x200B;" {
		t.Fatal("entity source mapping")
	}
	for _, c := range r.Controls {
		if c.Span != r.XML.Document.Elements[c.Element].Full || c.CountState != "known" || c.Paragraph != r.Texts[0].Paragraph {
			t.Fatal("control linkage")
		}
	}
	if r.XML.Segments[r.Texts[2].Segment].Text != "B" {
		t.Fatal("controls expanded into text")
	}
}
func TestHiddenAndConditionalDeclarations(t *testing.T) {
	r := extract(t, document(`<text:p>before<text:span><text:hidden-paragraph text:condition="untrusted()" text:is-hidden="true"/></text:span><text:hidden-text text:is-hidden="false" text:condition="do-not-evaluate()">secret</text:hidden-text><text:conditional-text text:string-value-if-true="yes" text:string-value-if-false="no">stored</text:conditional-text></text:p>`))
	if r.State != "completed" || len(r.Declarations) != 3 || len(r.Texts) != 3 {
		t.Fatal("declaration coverage", r)
	}
	if len(r.Texts[0].HiddenDeclarations) != 1 || len(r.Texts[1].HiddenDeclarations) != 2 || len(r.Texts[2].HiddenDeclarations) != 2 {
		t.Fatal("paragraph declaration not applied across paragraph")
	}
	hidden := r.XML.Document.Elements[r.Texts[1].HiddenDeclarations[0]]
	if value, _ := attr(hidden, TextNS, "is-hidden"); value != "false" {
		t.Fatal("rewrote visibility claim")
	}
	if value, _ := attr(hidden, TextNS, "condition"); value != "do-not-evaluate()" {
		t.Fatal("condition lost")
	}
	r.Texts[0].HiddenDeclarations[0] = -1
	if r.Texts[1].HiddenDeclarations[1] < 0 {
		t.Fatal("shared mutable reference slices")
	}
}
func TestContextAndCoverage(t *testing.T) {
	r := extract(t, document(`<text:tracked-changes><text:changed-region text:id="c1"><text:deletion><text:p>deleted</text:p></text:deletion></text:changed-region></text:tracked-changes><text:h>heading<office:annotation><text:p>comment</text:p></office:annotation><text:note><text:note-citation>1</text:note-citation><text:note-body><text:p>footnote</text:p></text:note-body></text:note></text:h><text:p><x:unknown xmlns:x="urn:foreign">retained</x:unknown><text:script>inert script</text:script></text:p><text:unknown>outside paragraph</text:unknown>`))
	if r.State != "partial" || len(r.Texts) != 7 || len(r.Unselected) != 1 {
		t.Fatalf("%+v", r)
	}
	first := r.Texts[0]
	found := false
	for _, i := range first.Context {
		if named(r.XML.Document.Elements[i], TextNS, "deletion") {
			found = true
		}
	}
	if !found {
		t.Fatal("deletion context lost")
	}
	if r.Unselected[0].Reason != "unclassified_character_data" {
		t.Fatal("lost unsupported data")
	}
	if len(r.Texts)+len(r.Unselected) != len(r.XML.Segments) {
		t.Fatal("segment coverage")
	}
}
func TestSpaceCountsAndMalformedControls(t *testing.T) {
	for _, tc := range []struct {
		value string
		known bool
		count uint64
	}{
		{"1", true, 1}, {"+0003", true, 3}, {" 2 ", true, 2}, {"18446744073709551615", true, ^uint64(0)},
		{"18446744073709551616", false, 0}, {"0", true, 0}, {"+000", true, 0}, {"-1", false, 0}, {"1e2", false, 0}, {"", false, 0}, {"1 2", false, 0}, {"١", false, 0},
	} {
		t.Run(tc.value, func(t *testing.T) {
			r := extract(t, document(`<text:p><text:s text:c="`+tc.value+`"/></text:p>`))
			c := r.Controls[0]
			if (c.CountState == "known") != tc.known || c.Count != tc.count {
				t.Fatal(c)
			}
			if !tc.known && r.State != "partial" {
				t.Fatal("unresolved count reported complete")
			}
		})
	}
	for _, body := range []string{`<text:p><text:s c="3"/></text:p>`, `<text:p><text:s>bad</text:s></text:p>`, `<text:p><text:tab><text:span/></text:tab></text:p>`, `<text:line-break/>`} {
		r := extract(t, document(body))
		if len(r.Controls) != 1 || r.Controls[0].CountState != "unknown" || r.State != "partial" {
			t.Fatal("invalid control", body, r)
		}
	}
	r := extract(t, document(`<text:p><text:s/></text:p>`))
	if r.Controls[0].Count != 1 {
		t.Fatal("default count")
	}
}
func TestRootAdmissionAndSpoofing(t *testing.T) {
	for _, source := range []string{`<document-content/>`, strings.Replace(document(""), "<office:text></office:text>", "<office:spreadsheet/>", 1), strings.Replace(document(""), "</office:body>", "<office:text/></office:body>", 1)} {
		b := []byte(source)
		if r, err := Extract(context.Background(), b, evidence.Hash(b)); r != nil || !errors.Is(err, ErrStructure) {
			t.Fatal("root", err)
		}
	}
	r := extract(t, document(`<text:p><text:s xmlns:text="urn:spoof">ordinary foreign data</text:s></text:p>`))
	if len(r.Controls) != 0 || len(r.Texts) != 1 || r.State != "partial" {
		t.Fatal("namespace spoof recognized")
	}
	r = extract(t, strings.Replace(document(`<text:p>1.2</text:p>`), `office:version="1.3"`, `office:version="1.2"`, 1))
	if r.DocumentVersion != "1.2" {
		t.Fatal("version")
	}
}
func TestCommittedODTHiddenText(t *testing.T) {
	source, err := os.ReadFile("../packageparts/testdata/hidden.odt")
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(source)
	identified, err := odtidentify.Inspect(context.Background(), source, evidence.Hash(source))
	if err != nil || identified.Format != "odt" {
		t.Fatal("ODT identity", err)
	}
	found := false
	for _, part := range identified.Package.Parts {
		if part.Name != "content.xml" {
			continue
		}
		r, err := Extract(context.Background(), part.Bytes, part.SHA256)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range r.Texts {
			if len(item.HiddenDeclarations) == 0 {
				continue
			}
			s := r.XML.Segments[item.Segment]
			if s.Text != strings.Repeat("\u200b\u200c", 32) || len(s.Scalars) != 64 {
				t.Fatal("hidden payload")
			}
			for _, m := range s.Scalars {
				if string(part.Bytes[m.Source.Start:m.Source.End]) != string(m.CodePoint) {
					t.Fatal("part coordinate")
				}
			}
			found = true
		}
	}
	if !found || !bytes.Equal(before, source) {
		t.Fatal("missing hidden text or mutation")
	}
}
func TestDeterminismOwnershipAndLimits(t *testing.T) {
	source := []byte(document(`<text:p>A&#x200B;` + "\r\n" + `B</text:p>`))
	before := bytes.Clone(source)
	a := extract(t, string(source))
	b := extract(t, string(source))
	if !reflect.DeepEqual(a, b) || !bytes.Equal(source, before) {
		t.Fatal("determinism/integrity")
	}
	source[0] = 'x'
	if a.XML.Document.PartSHA256 != evidence.Hash(before) {
		t.Fatal("source alias")
	}
	if r, err := Extract(context.Background(), before, strings.Repeat("0", 64)); r != nil || !errors.Is(err, xmlparts.ErrIdentity) {
		t.Fatal("identity", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Extract(ctx, before, evidence.Hash(before)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancel", err)
	}
	for _, body := range []string{`<text:p>` + strings.Repeat("a", xmlparts.MaxScalarMappings+1) + `</text:p>`, `<text:p>` + strings.Repeat(`<text:span>`, 40) + strings.Repeat(`<text:span>x</text:span>`, 5001) + strings.Repeat(`</text:span>`, 40) + `</text:p>`} {
		s := []byte(document(body))
		if r, err := Extract(context.Background(), s, evidence.Hash(s)); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
			t.Fatal("budget", err)
		}
	}
}
func FuzzExtract(f *testing.F) {
	f.Add(document(`<text:p>A&#x200B;<text:s text:c="2"/><text:hidden-text>hidden</text:hidden-text></text:p>`))
	f.Add(document(""))
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 65536 {
			t.Skip()
		}
		b := []byte(source)
		before := bytes.Clone(b)
		r, err := Extract(context.Background(), b, evidence.Hash(b))
		if !bytes.Equal(before, b) {
			t.Fatal("source mutation")
		}
		if err != nil {
			if r != nil {
				t.Fatal("partial result on error")
			}
			return
		}
		if len(r.Texts)+len(r.Unselected) != len(r.XML.Segments) {
			t.Fatal("coverage")
		}
		for _, item := range r.Texts {
			if r.XML.Segments[item.Segment].Element != item.Element {
				t.Fatal("segment link")
			}
		}
	})
}

func TestUnknownVersionsRetainObservedText(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.4"} {
		source := strings.Replace(document(`<text:p>A&#x200B;</text:p>`), `office:version="1.3"`, `office:version="`+version+`"`, 1)
		r := extract(t, source)
		if r.State != "partial" || len(r.Texts) != 1 || r.DocumentVersion != version || r.Issues[0].Code != "odt.content_version_unsupported" {
			t.Fatal("lost version caveat or text", r)
		}
	}
}
