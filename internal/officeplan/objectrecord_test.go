package officeplan

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func TestObjectRecordsPreserveDistinctCandidates(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="one" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/object.bin"/><Relationship Id="two" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="embeddings/object.bin"/></Relationships>`
		parts["word/embeddings/object.bin"] = "%PDF-1.7 more bytes"
		parts["word/embeddings/orphan.bin"] = "%PDF-1.7 more bytes"
		parts["word/embeddings/empty.bin"] = ""
		parts["word/embeddings/oversize.bin"] = string(make([]byte, 2048))
	})
	limits := DefaultLimits()
	limits.Package.PartBytes = 1024
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := AnalyzePrepared(context.Background(), p)
	if err != nil || analysis.Objects == nil {
		t.Fatal(err)
	}
	base, err := BuildPackageRecords(context.Background(), raw, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]v4.Part{}
	for _, part := range base.Evidence.Packages[0].Parts {
		parts[part.Name] = part
	}
	refs := map[int]string{}
	for i := range p.DOCX.OPC.Relationships {
		refs[i] = "office-relationship/" + strconv.Itoa(i)
	}
	records := map[string]v4.Object{}
	for _, native := range analysis.Objects.Objects {
		r, err := ObjectRecord(native, parts[native.Anchor.Part], refs, 4)
		if err != nil {
			t.Fatal(err)
		}
		if r.ObjectRef != native.ID || !reflect.DeepEqual(r.InclusionEvidence, native.Reasons) || r.Inspection.ExecutionRef != "exec/4" {
			t.Fatal("identity or inclusion changed")
		}
		records[native.Anchor.Part] = r
		if len(native.Relationships) > 0 {
			if _, err := ObjectRecord(native, parts[native.Anchor.Part], nil, 4); err == nil {
				t.Fatal("missing relationship silently dropped")
			}
		}
	}
	linked := records["word/embeddings/object.bin"]
	orphan := records["word/embeddings/orphan.bin"]
	if len(linked.RelationshipRefs) != 2 || len(orphan.RelationshipRefs) != 0 || linked.ObjectRef == orphan.ObjectRef || *linked.SHA256 != *orphan.SHA256 {
		t.Fatal("candidate identity confused with content identity")
	}
	if linked.ObservedSignature == nil || len(linked.Inspection.Assessed) != 1 || linked.Inspection.Assessed[0].End != 8 {
		t.Fatal("bounded signature scope lost")
	}
	empty := records["word/embeddings/empty.bin"]
	if empty.SHA256 == nil || *empty.SHA256 != evidence.Hash(nil) || empty.Inspection.State != "completed" {
		t.Fatal("empty candidate lost")
	}
	unavailable := records["word/embeddings/oversize.bin"]
	if unavailable.SHA256 != nil || unavailable.ObservedSignature != nil || unavailable.Inspection.State != "not_run" || len(unavailable.Inspection.Assessed) != 0 || len(unavailable.Inspection.Codes) != 1 {
		t.Fatal("unavailable candidate gained evidence")
	}
}
