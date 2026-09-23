package officeplan

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestCollectInventoriesCrossLinksAndMappingGaps(t *testing.T) {
	for _, mappingGap := range []bool{false, true} {
		raw := analysisFixture(t, func(parts map[string]string) {
			parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/properties/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`, 1)
			parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="properties/core.xml"/></Relationships>`, 1)
			parts["properties/core.xml"] = `<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Example</dc:creator></cp:coreProperties>`
			padding := ""
			if mappingGap {
				padding = strings.Repeat(" ", xmlparts.MaxScalarMappings+1)
			}
			parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="embedded" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/object.bin"/>` + padding + `</Relationships>`
			parts["word/embeddings/object.bin"] = "%PDF-1.7 payload"
		})
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
		xml, err := CollectXML(context.Background(), p, a, base)
		if err != nil {
			t.Fatal(err)
		}
		inventory, err := CollectInventories(context.Background(), p, a, base, xml, 4)
		if err != nil {
			t.Fatal(err)
		}
		again, err := CollectInventories(context.Background(), p, a, base, xml, 4)
		if err != nil || !reflect.DeepEqual(inventory, again) {
			t.Fatal("nondeterministic inventory", err)
		}
		if len(inventory.Metadata) != 1 || inventory.Metadata[0].LexicalValue != "Example" || len(inventory.Objects) != 1 {
			t.Fatal("usable evidence lost")
		}
		object := inventory.Objects[0]
		if mappingGap {
			if len(inventory.Gaps) != 2 || len(object.RelationshipRefs) != 0 || object.Inspection.State != "partial" || len(object.Inspection.Codes) != 1 || object.SHA256 == nil {
				t.Fatal("missing relation not disclosed", inventory)
			}
			if len(a.Objects.Objects[0].Relationships) != 1 {
				t.Fatal("producer evidence mutated")
			}
		} else if len(inventory.Gaps) != 0 || len(object.RelationshipRefs) != 1 || object.Inspection.State != "completed" {
			t.Fatal("valid backlink lost")
		}
		combined := base.Evidence
		combined.XML = xml.Documents
		combined.Metadata = inventory.Metadata
		combined.Relationships = inventory.Relationships
		combined.Objects = inventory.Objects
		if _, err := v4.IndexEvidence(combined, evidence.Hash(raw), int64(len(raw))); err != nil {
			t.Fatal("cross-record linkage invalid", err)
		}
	}
}
