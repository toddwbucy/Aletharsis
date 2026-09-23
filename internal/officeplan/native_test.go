package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func TestNativeResolverRetainedOfficeSources(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			resolver, err := NewNativeResolver(reader)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Admit(context.Background(), reader, resolver)
			if err != nil {
				t.Fatal(err)
			}
			before := reader.View()
			docx, odt, err := resolver.Identify(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(name, ".odt") {
				if odt.Format != "odt" || docx.Format != "" || !reflect.DeepEqual(plan.Phases[1].Parts, []string{"content.xml"}) {
					t.Fatal("ODT dispatch")
				}
			} else if docx.Format != "docx" || odt.Format != "" || !reflect.DeepEqual(plan.Phases[1].Parts, []string{"word/document.xml"}) {
				t.Fatal("DOCX dispatch")
			}
			if !reflect.DeepEqual(before, reader.View()) {
				t.Fatal("final identity changed admission")
			}
			if name == "office-partial-crc.docx" && plan.Outcomes.State != "partial" {
				t.Fatal("lost CRC coverage gap")
			}
		})
	}
}
func TestNativeDeclaredTargetsBeatUnrelatedXMLBudget(t *testing.T) {
	archive, err := zip.OpenReader("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	parts := map[string]string{}
	for _, f := range archive.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		ce := r.Close()
		if err != nil || ce != nil {
			t.Fatal(err, ce)
		}
		parts[f.Name] = string(raw)
	}
	parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/properties/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/word/header.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`, 1)
	parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="properties/core.xml"/></Relationships>`, 1)
	parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="header" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header.xml"/><Relationship Id="object" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/a.bin"/></Relationships>`
	parts["properties/core.xml"] = "<core/>"
	parts["word/header.xml"] = "<header/>"
	parts["word/embeddings/a.bin"] = "object"
	budget := 0
	for _, v := range parts {
		budget += len(v)
	}
	parts["aaa.xml"] = strings.Repeat("x", budget)
	parts["customXml/_rels/a.xml.rels"] = "<Relationships/>"
	names := []string{}
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
	limits := packageparts.DefaultLimits()
	limits.TotalBytes = budget
	reader, err := packageparts.OpenOutcomes(context.Background(), b.Bytes(), evidence.Hash(b.Bytes()), limits)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewNativeResolver(reader)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Admit(context.Background(), reader, resolver)
	if err != nil {
		t.Fatal(err)
	}
	s := states(plan.Outcomes)
	for _, n := range []string{"properties/core.xml", "word/header.xml", "word/embeddings/a.bin"} {
		if s[n] != "completed" {
			t.Fatal("declared target starved", n, plan.Phases)
		}
	}
	for _, n := range []string{"aaa.xml", "customXml/_rels/a.xml.rels"} {
		if s[n] != "not_run" {
			t.Fatal("ordinary content consumed priority budget", n)
		}
	}
	if !reflect.DeepEqual(plan.Phases[2].Parts, []string{"properties/core.xml"}) || !reflect.DeepEqual(plan.Phases[4].Parts, []string{"word/header.xml"}) {
		t.Fatal("wrong declaration priority", plan.Phases)
	}
}
