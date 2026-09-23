package officeplan

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestAssembleRetainedOfficeSources(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		for _, limited := range []bool{false, true} {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
			if err != nil {
				t.Fatal(err)
			}
			limits := DefaultLimits()
			if limited {
				limits.ScopeScalarOrigins = 1
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Assemble(context.Background(), raw, p, a, 1, 4)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Assemble(context.Background(), raw, p, a, 1, 4)
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("nondeterministic assembly", err)
			}
			if len(result.Evidence.Packages) != 1 || len(result.Evidence.XML) == 0 || len(result.XMLFailures) != 0 || len(result.InventoryGaps) != 0 {
				t.Fatal("unexpected assembly loss")
			}
			if limited {
				omitted := 0
				for _, part := range a.Parts {
					if part.Word != nil {
						omitted += len(part.Word.Omitted)
					}
					if part.ODT != nil {
						omitted += len(part.ODT.Omitted)
					}
				}
				if len(result.Evidence.Scopes) != 0 || len(result.Findings) != 0 || omitted != 1 {
					t.Fatal("omission authority lost")
				}
			} else if len(result.Evidence.Scopes) != 1 || len(result.Findings) != 1 || len(result.Findings[0].Findings) == 0 {
				t.Fatal("missing findings")
			}
			if name == "office-partial-crc.docx" && result.Evidence.Packages[0].State != "partial" {
				t.Fatal("CRC gap lost")
			}
		}
	}
}

func TestAssembleCompleteInventoryWithMappingGap(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`, 1)
		parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="core.xml"/></Relationships>`, 1)
		parts["core.xml"] = `<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Example</dc:creator></cp:coreProperties>`
		parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="object" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/a.bin"/>` + strings.Repeat(" ", xmlparts.MaxScalarMappings+1) + `</Relationships>`
		parts["word/embeddings/a.bin"] = "%PDF-1.7 payload"
	})
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Assemble(context.Background(), raw, p, a, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence.Metadata) != 1 || len(result.Evidence.Objects) != 1 || len(result.Evidence.Relationships) != 2 || len(result.Evidence.Scopes) != 1 || len(result.XMLFailures) != 1 || len(result.InventoryGaps) != 2 {
		t.Fatal("complete evidence/gap set not retained")
	}
}
