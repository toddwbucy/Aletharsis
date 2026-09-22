package wordtext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func story(ns, root, body string) string {
	return `<w:` + root + ` xmlns:w="` + ns + `">` + body + `</w:` + root + `>`
}
func document(body string) string {
	return story(docxidentify.TransitionalWord, "document", "<w:body>"+body+"</w:body>")
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
func TestDirectVisibilityAndRevisionContext(t *testing.T) {
	source := document(`<w:p><w:pPr><w:rPr><w:vanish/></w:rPr></w:pPr><w:r><w:t>inherit</w:t></w:r><w:del w:author="reviewer"><w:r><w:rPr><w:vanish/><w:webHidden w:val="0"/></w:rPr><w:delText xml:space="preserve"> deleted </w:delText><w:delInstrText>instruction</w:delInstrText></w:r></w:del><w:ins><w:r><w:rPr><w:vanish w:val="false"/><w:rPrChange><w:rPr><w:vanish/></w:rPr></w:rPrChange></w:rPr><w:instrText>DO NOT EXECUTE</w:instrText></w:r></w:ins></w:p>`)
	r := extract(t, source)
	if r.State != "completed" || len(r.Texts) != 4 {
		t.Fatalf("result: %+v", r)
	}
	if r.Texts[0].DirectVanish.State != "unspecified" {
		t.Fatal("paragraph property interpreted as direct run visibility")
	}
	deleted := r.Texts[1]
	if deleted.Role != "deleted_text" || deleted.DirectVanish.State != "on" || deleted.DirectWebHidden.State != "off" || !deleted.SpaceDeclared || deleted.SpaceValue != "preserve" || len(deleted.Revisions) != 1 {
		t.Fatal(deleted)
	}
	if r.XML.Document.Elements[deleted.Revisions[0]].Name.Local != "del" || r.XML.Segments[deleted.Segment].Text != " deleted " {
		t.Fatal("lost deleted content or context")
	}
	if r.Texts[2].Role != "deleted_field_instruction" || r.Texts[3].Role != "field_instruction" || r.Texts[3].DirectVanish.State != "off" {
		t.Fatal("instruction/revision flags")
	}
	r.Texts[1].DirectVanish.Elements[0] = -1
	if r.Texts[2].DirectVanish.Elements[0] < 0 {
		t.Fatal("mutable context aliases across observations")
	}
}
func TestToggleAdmission(t *testing.T) {
	for _, tc := range []struct{ property, state string }{
		{`<w:vanish/>`, "on"}, {`<w:vanish w:val="1"/>`, "on"}, {`<w:vanish w:val="off"/>`, "off"},
		{`<w:vanish w:val="false"/>`, "off"}, {`<w:vanish val="false"/>`, "unknown"},
		{`<w:vanish w:val="maybe"/>`, "unknown"}, {`<w:vanish/><w:vanish/>`, "unknown"},
		{`<w:vanish><w:x/></w:vanish>`, "unknown"}, {`<w:vanish>unexpected</w:vanish>`, "unknown"},
	} {
		t.Run(tc.property, func(t *testing.T) {
			r := extract(t, document(`<w:p><w:r><w:rPr>`+tc.property+`</w:rPr><w:t>x</w:t></w:r></w:p>`))
			if len(r.Texts) != 1 || r.Texts[0].DirectVanish.State != tc.state {
				t.Fatal(r.Texts)
			}
			if tc.state == "unknown" && r.State != "partial" {
				t.Fatal("unknown context reported complete")
			}
		})
	}
	r := extract(t, document(`<w:p><w:r><w:rPr/><w:rPr/><w:t>x</w:t></w:r></w:p>`))
	if r.Texts[0].DirectVanish.State != "unknown" {
		t.Fatal("duplicate properties accepted")
	}
	for _, value := range []string{"true", "on"} {
		r := extract(t, story(docxidentify.StrictWord, "hdr", `<w:p><w:r><w:rPr><w:vanish w:val="`+value+`"/></w:rPr><w:t>x</w:t></w:r></w:p>`))
		want := "on"
		if value == "on" {
			want = "unknown"
		}
		if r.Texts[0].DirectVanish.State != want {
			t.Fatal("strict toggle admission")
		}
	}
}
func TestStoriesAndControls(t *testing.T) {
	for _, root := range []string{"hdr", "ftr", "comments", "footnotes", "endnotes"} {
		r := extract(t, story(docxidentify.TransitionalWord, root, `<w:p><w:pPr><w:tabs><w:tab w:pos="100"/></w:tabs></w:pPr><w:r><w:t>A</w:t><w:tab/><w:br w:type="page"/><w:cr/><w:t>B</w:t></w:r></w:p>`))
		if r.State != "completed" || len(r.Controls) != 3 || len(r.Texts) != 2 {
			t.Fatalf("%s: %+v", root, r)
		}
		for _, c := range r.Controls {
			if c.Span != r.XML.Document.Elements[c.Element].Full {
				t.Fatal("control source span")
			}
		}
		if r.XML.Segments[r.Texts[0].Segment].Text != "A" || r.XML.Segments[r.Texts[1].Segment].Text != "B" {
			t.Fatal("invented control characters")
		}
	}
}
func TestForeignWrappersAndUnselectedContent(t *testing.T) {
	r := extract(t, document(`<w:p><w:r><w:drawing><x:wrapper xmlns:x="urn:foreign"><w:txbxContent><w:p><w:r><w:t>inside</w:t></w:r></w:p></w:txbxContent></x:wrapper></w:drawing></w:r><w:r><w:t xmlns:w="urn:spoof">not Word text</w:t></w:r><w:unknown>retained</w:unknown><w:r><w:t xml:space="other">space</w:t></w:r></w:p>`))
	if r.State != "partial" || len(r.Texts) != 2 || !r.Texts[0].TextBox || len(r.Texts[0].UnresolvedAncestors) != 1 || len(r.Unselected) != 2 {
		t.Fatalf("%+v", r)
	}
	for _, u := range r.Unselected {
		if u.Reason != "unclassified_character_data" {
			t.Fatal(u)
		}
	}
	r = extract(t, document(`<w:p><w:t>no run</w:t><w:r><w:t>a<w:x/>b</w:t></w:r><w:br/></w:p>`))
	if len(r.Texts) != 0 || r.State != "partial" || len(r.Unselected) != 3 {
		t.Fatal("unsupported structures accepted")
	}
}
func TestExactPartMappingsAndIntegrity(t *testing.T) {
	source := []byte(document(`<w:p><w:r><w:t>A&#x200B;😀` + "\r\n" + `<![CDATA[B]]></w:t></w:r></w:p>`))
	before := bytes.Clone(source)
	first := extract(t, string(source))
	second := extract(t, string(source))
	if !reflect.DeepEqual(first, second) || !bytes.Equal(before, source) {
		t.Fatal("determinism or integrity")
	}
	segment := first.XML.Segments[first.Texts[0].Segment]
	span := segment.Scalars[1].Source
	if segment.Scalars[1].CodePoint != '\u200b' || string(source[span.Start:span.End]) != "&#x200B;" {
		t.Fatal("source coordinate mismatch")
	}
	if len(first.Texts) != 2 || first.Texts[0].Element != first.Texts[1].Element {
		t.Fatal("CDATA segment linkage")
	}
	source[0] = 'x'
	if first.XML.Document.PartSHA256 != evidence.Hash(before) || segment.Text != "A\u200b😀\n" {
		t.Fatal("output aliases caller source")
	}
}
func TestCommittedHiddenPackage(t *testing.T) {
	source, err := os.ReadFile("../packageparts/testdata/hidden.docx")
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(source)
	pkg, err := packageparts.Read(context.Background(), source, evidence.Hash(source), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, part := range pkg.Parts {
		if part.Name != "word/document.xml" {
			continue
		}
		r, err := Extract(context.Background(), part.Bytes, part.SHA256)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range r.Texts {
			if item.DirectVanish.State != "on" {
				continue
			}
			s := r.XML.Segments[item.Segment]
			if len(s.Scalars) != 64 || s.Text != strings.Repeat("\u200b\u200c", 32) {
				t.Fatal("lost hidden payload")
			}
			for _, m := range s.Scalars {
				if string(part.Bytes[m.Source.Start:m.Source.End]) != string(m.CodePoint) {
					t.Fatal("part mapping mismatch")
				}
			}
			found = true
		}
	}
	if !found || !bytes.Equal(source, before) {
		t.Fatal("hidden run absent or archive mutated")
	}
}
func TestFailureBoundaries(t *testing.T) {
	for _, source := range []string{`<document/>`, story(docxidentify.TransitionalWord, "document", ""), story(docxidentify.TransitionalWord, "document", "<w:body/><w:body/>"), story(docxidentify.TransitionalWord, "styles", "")} {
		b := []byte(source)
		r, err := Extract(context.Background(), b, evidence.Hash(b))
		if r != nil || !errors.Is(err, ErrStructure) {
			t.Fatal("root admission", err)
		}
	}
	b := []byte(document(""))
	if r, err := Extract(context.Background(), b, strings.Repeat("0", 64)); r != nil || !errors.Is(err, xmlparts.ErrIdentity) {
		t.Fatal("identity", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Extract(ctx, b, evidence.Hash(b)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	source := document(`<w:p><w:r><w:t>` + strings.Repeat("a", xmlparts.MaxScalarMappings+1) + `</w:t></w:r></w:p>`)
	if r, err := Extract(context.Background(), []byte(source), evidence.Hash([]byte(source))); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
		t.Fatal("mapping budget", err)
	}
	// Many segments share deeply nested revision ancestors: charge every retained link.
	source = document(`<w:p>` + strings.Repeat(`<w:ins>`, 40) + `<w:r>` + strings.Repeat(`<w:t>x</w:t>`, 5001) + `</w:r>` + strings.Repeat(`</w:ins>`, 40) + `</w:p>`)
	if r, err := Extract(context.Background(), []byte(source), evidence.Hash([]byte(source))); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
		t.Fatal("context budget", err)
	}
}
func FuzzExtract(f *testing.F) {
	f.Add(document(`<w:p><w:r><w:rPr><w:vanish/></w:rPr><w:t>A&#x200B;</w:t></w:r></w:p>`))
	f.Add(document(""))
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 65536 {
			t.Skip()
		}
		b := []byte(source)
		before := bytes.Clone(b)
		r, err := Extract(context.Background(), b, evidence.Hash(b))
		if !bytes.Equal(b, before) {
			t.Fatal("source mutation")
		}
		if err != nil {
			if r != nil {
				t.Fatal("partial result on failure")
			}
			return
		}
		if len(r.Texts)+len(r.Unselected) != len(r.XML.Segments) {
			t.Fatal("segment coverage lost")
		}
		for _, item := range r.Texts {
			s := r.XML.Segments[item.Segment]
			if s.Element != item.Element {
				t.Fatal("broken segment link")
			}
		}
	})
}

func TestFormattingControlsRemainLocatedXMLOnly(t *testing.T) {
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, property := range []string{"rPr", "pPr", "sectPr", "tblPr", "tblPrEx", "trPr", "tcPr"} {
			for _, control := range []string{"tab", "br", "cr"} {
				t.Run(ns+"/"+property+"/"+control, func(t *testing.T) {
					body := `<w:p><w:r><w:` + property + `><w:r><w:` + control + `/></w:r></w:` + property + `><w:t>x</w:t><w:` + control + `/></w:r></w:p>`
					r := extract(t, strings.ReplaceAll(document(body), docxidentify.TransitionalWord, ns))
					if r.State != "partial" || len(r.Controls) != 1 || len(r.Issues) != 1 || r.Issues[0].Code != "word.control_structure_unsupported" {
						t.Fatal("formatting control admitted", r)
					}
					e := r.XML.Document.Elements[r.Issues[0].Element]
					if e.Name.Local != control || e.Full.End <= e.Full.Start || r.Issues[0].Element == r.Controls[0].Element {
						t.Fatal("lost control location")
					}
				})
			}
		}
	}
}
