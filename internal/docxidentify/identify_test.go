package docxidentify

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

func archive(t testing.TB, parts map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(parts))
	for n := range parts {
		names = append(names, n)
	}
	sort.Strings(names)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(parts[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func types(s string) string { return `<Types xmlns="` + TypesNamespace + `">` + s + `</Types>` }
func override(name, typ string) string {
	return `<Override PartName="/` + name + `" ContentType="` + typ + `"/>`
}
func base(main string) map[string]string {
	return map[string]string{
		"[Content_Types].xml": types(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` + override(main, MainContentType)),
		"_rels/.rels":         `<Relationships xmlns="` + opcrels.Namespace + `"><Relationship Id="main" Type="` + TransitionalRelationship + `" Target="` + main + `"/></Relationships>`,
		main:                  `<w:document xmlns:w="` + TransitionalWord + `"><w:body><w:p><w:r><w:t>A&#x200B;😀</w:t></w:r></w:p></w:body></w:document>`,
	}
}
func inspect(t testing.TB, parts map[string]string) *Result {
	t.Helper()
	b := archive(t, parts)
	r, err := Inspect(context.Background(), b, evidence.Hash(b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func hasIssue(r *Result, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
func TestEvidenceSelectedMainPart(t *testing.T) {
	for _, strict := range []bool{false, true} {
		parts := base("custom/main.XML")
		parts["word/document.xml"] = "<unrelated/>"
		if strict {
			parts["_rels/.rels"] = strings.ReplaceAll(parts["_rels/.rels"], TransitionalRelationship, StrictRelationship)
			parts["custom/main.XML"] = strings.ReplaceAll(parts["custom/main.XML"], TransitionalWord, StrictWord)
		}
		r := inspect(t, parts)
		if r.Format != "docx" || r.State != "completed" || r.MainPart != "custom/main.XML" || r.MainSHA256 != evidence.Hash([]byte(parts[r.MainPart])) || r.MainXML.PartSHA256 != r.MainSHA256 {
			t.Fatal("main identity", r)
		}
		variant := "transitional"
		if strict {
			variant = "strict"
		}
		if r.Variant != variant {
			t.Fatal("variant")
		}
		assignment := r.Assignments[r.MainAssignment]
		decl := r.Declarations[assignment.Declaration]
		if decl.Kind != "Override" || decl.ContentType != MainContentType || r.OPC.Relationships[r.MainRelationship].ResolvedPart != r.MainPart {
			t.Fatal("selection linkage")
		}
		span := decl.Anchor.Span
		if parts[r.TypesPart][span.Start:span.End] != override("custom/main.XML", MainContentType) {
			t.Fatal("declaration anchor")
		}
	}
}
func TestDefaultOverrideAndCaseEquivalence(t *testing.T) {
	parts := base("word/document.xml")
	parts["[Content_Types].xml"] = strings.ReplaceAll(parts["[Content_Types].xml"], `Extension="xml"`, `Extension="XML"`)
	parts["[Content_Types].xml"] = strings.ReplaceAll(parts["[Content_Types].xml"], `PartName="/word/document.xml"`, `PartName="/WORD/DOCUMENT.XML"`)
	r := inspect(t, parts)
	if r.Format != "docx" || r.State != "completed" {
		t.Fatal("case equivalence", r.Issues)
	}
	parts["[Content_Types].xml"] = types(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="` + MainContentType + `"/>`)
	r = inspect(t, parts)
	if r.Format != "docx" || r.Declarations[r.Assignments[r.MainAssignment].Declaration].Kind != "Default" {
		t.Fatal("default selection")
	}
}
func TestAmbiguousAndUnsupportedTypes(t *testing.T) {
	cases := []struct{ extra, code string }{
		{override("WORD/document.xml", MainContentType), "opc.content_type_ambiguous"},
		{`<Default Extension="XML" ContentType="application/xml"/>`, "opc.content_type_ambiguous"},
		{`<Override PartName="/word/%64ocument.xml" ContentType="application/xml"/>`, "opc.override_name_unsupported"},
		{`<Default Extension="tar.gz" ContentType="application/xml"/>`, "opc.extension_unsupported"},
		{`<Default Extension="bin" ContentType="application/octet-stream; x=y"/>`, "opc.media_type_unsupported"},
		{`<Default Extension="bin"/>`, "opc.content_type_required_attribute"},
		{`<Other/>`, "opc.content_types_structure_unknown"},
		{`<Default Extension="bin" ContentType="application/octet-stream"><child/></Default>`, "opc.content_types_structure_unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.code+tc.extra, func(t *testing.T) {
			parts := base("word/document.xml")
			parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", tc.extra+"</Types>", 1)
			r := inspect(t, parts)
			want := "docx"
			if tc.extra == override("WORD/document.xml", MainContentType) {
				want = ""
			}
			if r.Format != want || r.State != "partial" || !hasIssue(r, tc.code) || r.TypesXML == nil {
				t.Fatal("type conflict hidden", r.Issues)
			}
		})
	}
}
func TestMissingAliasesAndMalformedParts(t *testing.T) {
	cases := []struct {
		name, code string
		change     func(map[string]string)
	}{
		{"manifest_missing", "opc.content_types_missing", func(p map[string]string) { delete(p, "[Content_Types].xml") }},
		{"manifest_alias", "opc.content_types_ambiguous", func(p map[string]string) { p["[content_types].xml"] = p["[Content_Types].xml"] }},
		{"manifest_spoof", "opc.content_types_root_invalid", func(p map[string]string) {
			p["[Content_Types].xml"] = strings.ReplaceAll(p["[Content_Types].xml"], TypesNamespace, "urn:spoof")
		}},
		{"manifest_dtd", "xml.unsupported", func(p map[string]string) { p["[Content_Types].xml"] = "<!DOCTYPE Types>" + p["[Content_Types].xml"] }},
		{"root_type", "docx.root_relationship_type_unknown", func(p map[string]string) {
			p["[Content_Types].xml"] = strings.ReplaceAll(p["[Content_Types].xml"], "application/vnd.openxmlformats-package.relationships+xml", "image/png")
		}},
		{"main_missing", "docx.root_relationships_incomplete", func(p map[string]string) { delete(p, "word/document.xml") }},
		{"main_alias", "docx.root_relationships_incomplete", func(p map[string]string) { p["WORD/document.xml"] = p["word/document.xml"] }},
		{"main_type", "docx.main_type_unsupported", func(p map[string]string) {
			p["[Content_Types].xml"] = strings.ReplaceAll(p["[Content_Types].xml"], MainContentType, "application/vnd.ms-word.document.macroEnabled.main+xml")
		}},
		{"mixed_variant", "docx.main_root_mismatch", func(p map[string]string) {
			p["_rels/.rels"] = strings.ReplaceAll(p["_rels/.rels"], TransitionalRelationship, StrictRelationship)
		}},
		{"main_ns", "docx.main_root_mismatch", func(p map[string]string) {
			p["word/document.xml"] = strings.ReplaceAll(p["word/document.xml"], TransitionalWord, "urn:spoof")
		}},
		{"body_missing", "docx.main_body_ambiguous", func(p map[string]string) { p["word/document.xml"] = `<w:document xmlns:w="` + TransitionalWord + `"/>` }},
		{"body_duplicate", "docx.main_body_ambiguous", func(p map[string]string) {
			p["word/document.xml"] = `<w:document xmlns:w="` + TransitionalWord + `"><w:body/><w:body/></w:document>`
		}},
		{"broken_main", "xml.invalid", func(p map[string]string) { p["word/document.xml"] = "<broken>" }},
		{"external_main", "docx.main_relationship_unresolved", func(p map[string]string) {
			p["_rels/.rels"] = strings.ReplaceAll(p["_rels/.rels"], ` Target="word/document.xml"`, ` Target="https://example.invalid/main" TargetMode="External"`)
		}},
		{"multiple_main", "docx.main_relationship_ambiguous", func(p map[string]string) {
			p["_rels/.rels"] = strings.ReplaceAll(p["_rels/.rels"], `</Relationships>`, `<Relationship Id="second" Type="`+TransitionalRelationship+`" Target="word/document.xml"/></Relationships>`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base("word/document.xml")
			tc.change(p)
			r := inspect(t, p)
			if r.Format != "" || r.State != "partial" || !hasIssue(r, tc.code) {
				t.Fatal(tc.name, r.Format, r.Issues)
			}
		})
	}
}
func TestFormatIdentityDoesNotEraseOtherGaps(t *testing.T) {
	parts := base("word/document.xml")
	parts["custom/payload.bin"] = "untyped"
	parts["_rels/missing.xml.rels"] = `<Relationships xmlns="` + opcrels.Namespace + `"/>`
	r := inspect(t, parts)
	if r.Format != "docx" || r.State != "partial" || !hasIssue(r, "opc.part_type_missing") || !hasIssue(r, "opc.relationship_inventory_partial") {
		t.Fatal("format identity erased incomplete coverage", r.Issues)
	}
	if len(r.OPC.Package.Parts) != 5 {
		t.Fatal("unknown parts lost")
	}
}
func TestRetainedFixturesAndUnchangedSource(t *testing.T) {
	for _, name := range []string{"minimal.docx", "hidden.docx", "minimal.odt", "hidden.odt"} {
		b, err := os.ReadFile(filepath.Join("..", "packageparts", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(b)
		r, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil {
			t.Fatal(err)
		}
		again, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil || !reflect.DeepEqual(r, again) || !bytes.Equal(before, b) {
			t.Fatal("nondeterministic or source mutation")
		}
		if strings.HasSuffix(name, ".docx") {
			if r.Format != "docx" || r.State != "completed" {
				t.Fatal(name, r.Issues)
			}
		} else if r.Format != "" || r.State != "not_applicable" {
			t.Fatal("ODT guessed as DOCX")
		}
	}
}
func TestBoundsIdentityAndCancellation(t *testing.T) {
	p := base("word/document.xml")
	var extra strings.Builder
	for i := 0; i < maxDeclarations+1; i++ {
		fmt.Fprintf(&extra, `<Default Extension="x%d" ContentType="application/octet-stream"/>`, i)
	}
	p["[Content_Types].xml"] = types(extra.String())
	r := inspect(t, p)
	if r.State != "partial" || len(r.Declarations) != maxDeclarations || !hasIssue(r, "opc.content_types_limit") {
		t.Fatal("declaration budget")
	}
	b := archive(t, base("word/document.xml"))
	if r, err := Inspect(context.Background(), b, strings.Repeat("0", 64)); r != nil || err == nil {
		t.Fatal("identity")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Inspect(ctx, b, evidence.Hash(b)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
}
func FuzzMainXML(f *testing.F) {
	f.Add(base("word/document.xml")["word/document.xml"])
	f.Add(`<broken>`)
	f.Fuzz(func(t *testing.T, xml string) {
		if len(xml) > 1<<16 {
			t.Skip()
		}
		p := base("word/document.xml")
		p["word/document.xml"] = xml
		r := inspect(t, p)
		if r.Format == "docx" {
			if r.MainPart != "word/document.xml" || r.MainSHA256 != evidence.Hash([]byte(xml)) || r.MainXML == nil || r.MainXML.Elements[0].Name.Namespace != TransitionalWord {
				t.Fatal("invented main identity")
			}
		}
	})
}

func TestDefectiveDeclarationsCannotSupplyMainIdentity(t *testing.T) {
	for _, extra := range []string{` foreign="x"`, ` xmlns:x="urn:foreign" x:attr="x"`} {
		p := base("word/document.xml")
		p["[Content_Types].xml"] = strings.Replace(p["[Content_Types].xml"], `<Override PartName=`, `<Override`+extra+` PartName=`, 1)
		r := inspect(t, p)
		if r.Format != "" || !hasIssue(r, "docx.main_type_unknown") {
			t.Fatal("unsupported declaration promoted", r.Issues)
		}
	}
	p := base("word/document.xml")
	p["[Content_Types].xml"] = strings.Replace(p["[Content_Types].xml"], `</Types>`, `<Default xmlns:x="urn:foreign" x:attr="x" Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
	r := inspect(t, p)
	if r.Format != "docx" || r.MainXML == nil || r.State != "partial" {
		t.Fatal("unrelated declaration blocked extraction", r.Issues)
	}
}
func TestMediaTypeAndKeyDefectsBothRetained(t *testing.T) {
	for _, tc := range []struct{ declaration, code string }{
		{`<Default Extension="a b!" ContentType="not a media type"/>`, "opc.extension_unsupported"},
		{`<Override PartName="bad" ContentType="not a media type"/>`, "opc.override_name_unsupported"},
	} {
		p := base("word/document.xml")
		p["[Content_Types].xml"] = strings.Replace(p["[Content_Types].xml"], `</Types>`, tc.declaration+`</Types>`, 1)
		r := inspect(t, p)
		if !hasIssue(r, "opc.media_type_unsupported") || !hasIssue(r, tc.code) || r.Declarations[len(r.Declarations)-1].Code != "opc.media_type_unsupported" {
			t.Fatal("overwritten issue", r.Issues)
		}
	}
}
