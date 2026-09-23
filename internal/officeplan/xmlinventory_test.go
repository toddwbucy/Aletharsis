package officeplan

import (
	"context"
	"errors"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func TestCollectXMLFromPreparedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{{"office-minimal.docx", 3}, {"office-partial-crc.docx", 3}, {"office-odt-minimal.odt", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			base, err := BuildPackageRecords(context.Background(), raw, p, 1)
			if err != nil {
				t.Fatal(err)
			}
			x, err := CollectXML(context.Background(), p, a, base)
			if err != nil {
				t.Fatal(err)
			}
			if len(x.Documents) != tc.count || len(x.ByPart) != tc.count || len(x.Failures) != 0 {
				t.Fatal("XML inventory incomplete", x)
			}
			again, err := CollectXML(context.Background(), p, a, base)
			if err != nil || !reflect.DeepEqual(x, again) {
				t.Fatal("nondeterministic inventory", err)
			}
			base.Evidence.XML = x.Documents
			if _, err := v4.IndexEvidence(base.Evidence, evidence.Hash(raw), int64(len(raw))); err != nil {
				t.Fatal("invalid assembled XML evidence", err)
			}
			for _, part := range a.Parts {
				if part.Word != nil || part.ODT != nil {
					i, ok := x.ByPart[part.Part]
					if !ok || len(x.Documents[i].Segments) == 0 {
						t.Fatal("text maps absent")
					}
				}
			}
		})
	}
}

func TestCollectXMLRetainsSiblingsWhenMappingIsUnavailable(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", strings.Repeat(" ", xmlparts.MaxScalarMappings+1)+"</Types>", 1)
	})
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if p.DOCX.Format != "docx" {
		t.Fatal("fixture lost identity")
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	base, err := BuildPackageRecords(context.Background(), raw, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	x, err := CollectXML(context.Background(), p, a, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Failures) != 1 || x.Failures[0].Part != "[Content_Types].xml" || !errors.Is(x.Failures[0].Error, xmlparts.ErrLimit) || len(x.Documents) != 2 {
		t.Fatal("mapping failure erased siblings", x)
	}
	if _, ok := x.ByPart["[Content_Types].xml"]; ok {
		t.Fatal("partial map retained")
	}
	if _, ok := x.ByPart["word/document.xml"]; !ok {
		t.Fatal("usable text XML lost")
	}
}
