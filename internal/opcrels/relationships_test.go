package opcrels

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
)

func archive(t testing.TB, parts map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(parts[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func rels(body string) string {
	return `<Relationships xmlns="` + Namespace + `">` + body + `</Relationships>`
}
func relationship(id, target, extra string) string {
	return `<Relationship Id="` + id + `" Type="urn:test" Target="` + target + `"` + extra + `/>`
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
func TestInternalExternalAndExactAnchors(t *testing.T) {
	xml := rels(relationship("body", "WORD/document.xml", "") + relationship("external", "https://example.invalid/tracker", ` TargetMode="External"`))
	parts := map[string]string{"_rels/.rels": xml, "word/document.xml": "<body/>", "word/_rels/document.xml.rels": rels(relationship("up", "../custom/item.xml", "") + relationship("absolute", "/custom/item.xml", "")), "custom/item.xml": "<item/>"}
	r := inspect(t, parts)
	if r.State != "completed" || len(r.Parts) != 2 || len(r.Relationships) != 4 {
		t.Fatal("inventory", r)
	}
	for i, rel := range r.Relationships {
		if i == 1 {
			if rel.State != "external" || rel.ResolvedPart != "" || rel.TargetSHA256 != "" {
				t.Fatal("external target resolved")
			}
			continue
		}
		if rel.State != "resolved" || rel.TargetSHA256 != evidence.Hash([]byte(parts[rel.ResolvedPart])) {
			t.Fatal("resolution identity", rel)
		}
		anchor := rel.Anchor
		raw := parts[anchor.Part]
		if !strings.HasPrefix(raw[anchor.Span.Start:anchor.Span.End], "<Relationship ") || anchor.PartSHA256 != evidence.Hash([]byte(raw)) {
			t.Fatal("anchor")
		}
	}
	if r.Relationships[0].ResolvedPart != "word/document.xml" || r.Relationships[2].SourcePart != "word/document.xml" {
		t.Fatal("case/source identity")
	}
}
func TestResolutionStates(t *testing.T) {
	cases := []struct{ target, mode, state, code string }{
		{"missing.xml", "", "missing", "opc.target_missing"}, {"../escape.xml", "", "unresolved", "opc.target_outside_package"}, {"//host/path", "", "unresolved", "opc.target_syntax_unsupported"}, {"word/a%20b.xml", "", "unresolved", "opc.target_syntax_unsupported"}, {"word/a.xml#fragment", "", "unresolved", "opc.target_syntax_unsupported"}, {"word/a.xml?q=x", "", "unresolved", "opc.target_syntax_unsupported"}, {"word/文.xml", "", "unresolved", "opc.target_syntax_unsupported"}, {"word/a.xml", ` TargetMode="external"`, "invalid", "opc.target_mode_invalid"}, {"word/a.xml", ` TargetMode=""`, "invalid", "opc.target_mode_invalid"}, {"javascript:alert(1)", ` TargetMode="External"`, "external", ""},
	}
	for _, tc := range cases {
		t.Run(tc.target+tc.mode, func(t *testing.T) {
			r := inspect(t, map[string]string{"_rels/.rels": rels(relationship("r", tc.target, tc.mode)), "word/a.xml": "<a/>"})
			rel := r.Relationships[0]
			if rel.State != tc.state || rel.Code != tc.code || rel.ResolvedPart != "" {
				t.Fatal("state", rel)
			}
			if tc.state != "external" && r.State != "partial" {
				t.Fatal("unresolved link became complete")
			}
		})
	}
}
func TestAmbiguousNamesIDsAndSources(t *testing.T) {
	r := inspect(t, map[string]string{"_rels/.rels": rels(relationship("r", "a.xml", "")), "a.xml": "one", "A.xml": "two"})
	if r.Relationships[0].State != "ambiguous" {
		t.Fatal("case alias selected")
	}
	r = inspect(t, map[string]string{"_rels/.rels": rels(relationship("same", "a.xml", "") + relationship("same", "b.xml", "")), "a.xml": "one", "b.xml": "two"})
	for _, rel := range r.Relationships {
		if rel.Code != "opc.relationship_id_ambiguous" || rel.ResolvedPart != "" {
			t.Fatal("duplicate ID resolved")
		}
	}
	r = inspect(t, map[string]string{"word/_rels/missing.xml.rels": rels(relationship("r", "../a.xml", "")), "a.xml": "one"})
	if r.Relationships[0].Code != "opc.source_missing" || r.Parts[0].Code != "opc.source_missing" {
		t.Fatal("missing source")
	}
	r = inspect(t, map[string]string{"word/_rels/a.xml.rels": rels(relationship("r", "../b.xml", "")), "word/a.xml": "one", "word/A.xml": "two", "b.xml": "target"})
	if r.Relationships[0].Code != "opc.source_ambiguous" {
		t.Fatal("source case alias selected")
	}
	r = inspect(t, map[string]string{"_rels/.rels": rels(""), "_RELS/.RELS": rels("")})
	if len(r.Parts) != 2 {
		t.Fatal("lost ambiguous parts")
	}
	for _, p := range r.Parts {
		if p.Code != "opc.relationship_part_ambiguous" {
			t.Fatal("ambiguous relationship part accepted")
		}
	}
}
func TestUnknownStructureAndFailedPartsRetained(t *testing.T) {
	for _, xml := range []string{`<Relationships xmlns="` + Namespace + `" xml:base="https://example.invalid/">` + relationship("r", "a.xml", "") + `</Relationships>`, rels(relationship("r", "a.xml", ` Other="value"`)), rels(`<Relationship Id="r" Type="urn:test" Target="a.xml"><child/></Relationship>`)} {
		r := inspect(t, map[string]string{"_rels/.rels": xml, "a.xml": "a"})
		if r.State != "partial" || r.Relationships[0].Code != "opc.relationship_structure_unknown" || r.Relationships[0].ResolvedPart != "" {
			t.Fatal("unknown semantics resolved", r.Relationships)
		}
	}
	for _, tc := range []struct{ name, xml, code string }{{"_rels/.rels", "<broken>", "xml.invalid"}, {"_rels/.rels", `<!DOCTYPE Relationships><Relationships/>`, "xml.unsupported"}, {"_rels/.rels", `<Relationships xmlns="urn:spoof"/>`, "opc.relationship_root_invalid"}, {"misplaced.rels", rels(""), "opc.relationship_name_unsupported"}} {
		r := inspect(t, map[string]string{tc.name: tc.xml})
		if r.State != "partial" || len(r.Package.Parts) != 1 || r.Parts[0].Code != tc.code {
			t.Fatal("failure lost", r)
		}
	}
	r := inspect(t, map[string]string{"_rels/.rels": rels(`<unknown/>` + relationship("r", "a.xml", "")), "a.xml": "a"})
	if r.State != "partial" || r.Parts[0].XML == nil || len(r.Relationships) != 1 {
		t.Fatal("unknown root child lost")
	}
}
func TestDeterminismIdentityAndCancellation(t *testing.T) {
	b := archive(t, map[string]string{"_rels/.rels": rels(relationship("r", "a.xml", "")), "a.xml": "a"})
	before := bytes.Clone(b)
	r, err := Inspect(context.Background(), b, evidence.Hash(b))
	if err != nil {
		t.Fatal(err)
	}
	again, err := Inspect(context.Background(), b, evidence.Hash(b))
	if err != nil || !reflect.DeepEqual(r, again) || !bytes.Equal(b, before) {
		t.Fatal("nondeterministic or mutated source", err)
	}
	if r, err := Inspect(context.Background(), b, strings.Repeat("0", 64)); r != nil || err == nil {
		t.Fatal("wrong identity")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Inspect(ctx, b, evidence.Hash(b)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	if r := inspect(t, map[string]string{"content.xml": "<a/>"}); r.State != "not_applicable" || len(r.Relationships) != 0 {
		t.Fatal("invented OPC coverage")
	}
}
func TestAggregateBudgets(t *testing.T) {
	parts := map[string]string{}
	for i := 0; i < maxParts+1; i++ {
		owner := fmt.Sprintf("p%03d.xml", i)
		parts[owner] = "<a/>"
		parts["_rels/"+owner+".rels"] = rels("")
	}
	r := inspect(t, parts)
	if len(r.Parts) != maxParts+1 || r.Parts[maxParts].Code != "opc.resource_limit" || r.Parts[maxParts].State != "not_run" {
		t.Fatal("part budget bypass")
	}
	var b strings.Builder
	for i := 0; i < maxRelationships+1; i++ {
		b.WriteString(relationship(fmt.Sprintf("r%d", i), "a.xml", ""))
	}
	r = inspect(t, map[string]string{"_rels/.rels": rels(b.String()), "a.xml": "a"})
	if len(r.Relationships) != maxRelationships || r.Parts[0].Code != "opc.resource_limit" || r.State != "partial" {
		t.Fatal("relationship budget bypass")
	}
}
func TestRetainedDOCXAndODT(t *testing.T) {
	for _, name := range []string{"minimal.docx", "hidden.docx", "minimal.odt", "hidden.odt"} {
		b, err := os.ReadFile(filepath.Join("..", "packageparts", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		r, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".docx") {
			if r.State != "completed" || len(r.Relationships) != 1 || r.Relationships[0].ResolvedPart != "word/document.xml" {
				t.Fatal("retained document", name)
			}
		} else if r.State != "not_applicable" {
			t.Fatal("ODT classified as OPC")
		}
	}
}
func TestResolverBoundary(t *testing.T) {
	for _, tc := range []struct{ source, target, want string }{{"word/document.xml", "./media/image.png", "word/media/image.png"}, {"word/document.xml", "../custom.xml", "custom.xml"}, {"word/document.xml", "/root.xml", "root.xml"}, {"", "a/b/../c.xml", "a/c.xml"}} {
		got, code := resolve(tc.source, tc.target)
		if got != tc.want || code != "" {
			t.Fatal(tc, got, code)
		}
	}
	for _, target := range []string{".", "..", "/", "a//b", "a/.", "a/..", "a/../../b", "a<b", "a\\b"} {
		if _, code := resolve("", target); code == "" {
			t.Fatal("unsafe URI accepted", target)
		}
	}
}
func FuzzRelationshipXML(f *testing.F) {
	f.Add(rels(relationship("r", "a.xml", "")))
	f.Add(`<broken>`)
	f.Fuzz(func(t *testing.T, xml string) {
		if len(xml) > 1<<16 {
			t.Skip()
		}
		r := inspect(t, map[string]string{"_rels/.rels": xml, "a.xml": "a"})
		for _, rel := range r.Relationships {
			if rel.State == "resolved" {
				target := ""
				switch rel.ResolvedPart {
				case "a.xml":
					target = "a"
				case "_rels/.rels":
					target = xml
				default:
					t.Fatal("invented target")
				}
				if rel.TargetSHA256 != evidence.Hash([]byte(target)) {
					t.Fatal("invented target identity")
				}
			}
			if rel.Anchor.Span.Start < 0 || rel.Anchor.Span.End > int64(len(xml)) || rel.Anchor.Span.End <= rel.Anchor.Span.Start {
				t.Fatal("invalid anchor")
			}
		}
	})
}

func TestUnsupportedSourceURI(t *testing.T) {
	r := inspect(t, map[string]string{"word/a%20b.xml": "source", "word/_rels/a%20b.xml.rels": rels(relationship("r", "target.xml", "")), "word/target.xml": "target"})
	if r.Relationships[0].Code != "opc.source_syntax_unsupported" || r.Relationships[0].ResolvedPart != "" {
		t.Fatal("unsupported source URI resolved")
	}
}

func TestXMLAggregateAndFailureAccounting(t *testing.T) {
	body := rels("<!--" + strings.Repeat("x", 3<<20) + "-->")
	r := inspect(t, map[string]string{"_rels/a.xml.rels": body, "_rels/b.xml.rels": body, "_rels/c.xml.rels": body, "a.xml": "a", "b.xml": "b", "c.xml": "c"})
	if r.Parts[0].State != "completed" || r.Parts[1].State != "completed" || r.Parts[2].State != "not_run" || r.Parts[2].Code != "opc.resource_limit" {
		t.Fatal("aggregate XML byte limit", r.Parts)
	}
	r = inspect(t, map[string]string{"_rels/a.xml.rels": "<broken>", "_rels/b.xml.rels": rels(""), "a.xml": "a", "b.xml": "b"})
	if r.Parts[0].Code != "xml.invalid" || r.Parts[1].Code != "opc.resource_limit" || r.Parts[1].XML != nil {
		t.Fatal("failed parse allowance reused")
	}
}
