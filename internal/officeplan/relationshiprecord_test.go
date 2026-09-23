package officeplan

import (
	"context"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestRelationshipRecordsPreserveResolutionEvidence(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="ok" Type="test" Target="target.bin"/>
<Relationship Id="external" Type="test" TargetMode="External" Target="https://example.invalid/never-fetch"/>
<Relationship Id="missing" Type="test" Target="missing.bin"/>
<Relationship Id="ambiguous" Type="test" Target="clash.bin"/>
<Relationship Id="invalid" Type="test" TargetMode="Other" Target="target.bin"/>
<Relationship Id="unresolved" Type="test" Target="target.bin#fragment"/>
<Relationship Id="unsupported" Type="test" Target="target.bin" unknown="data"/>
</Relationships>`
		parts["word/target.bin"] = "payload"
		parts["word/clash.bin"] = "a"
		parts["word/CLASH.bin"] = "b"
	})
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
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
	bytes := map[string][]byte{}
	for _, o := range p.Admission.Outcomes.Parts {
		bytes[o.Part.Name] = o.Part.Bytes
	}
	docs := map[string]v4.XML{}
	for i, part := range p.DOCX.OPC.Parts {
		if part.XML == nil {
			continue
		}
		mapped, err := xmlparts.MapParsed(context.Background(), bytes[part.Part], part.XML, xmlparts.MaxScalarMappings)
		if err != nil {
			t.Fatal(err)
		}
		x, err := BuildXMLRecord(mapped, parts[part.Part], i)
		if err != nil {
			t.Fatal(err)
		}
		docs[part.Part] = x
	}
	seen := map[string]bool{}
	for i, native := range p.DOCX.OPC.Relationships {
		r, err := RelationshipRecord(native, parts, docs[native.Anchor.Part], i)
		if err != nil {
			t.Fatal(native.ID, err)
		}
		if _, err := RelationshipRecord(native, parts, v4.XML{}, i); err == nil {
			t.Fatal("missing XML accepted")
		}
		if native.State == "resolved" {
			corrupted := native
			corrupted.TargetSHA256 = evidence.Hash([]byte("different"))
			if _, err := RelationshipRecord(corrupted, parts, docs[native.Anchor.Part], i); err == nil {
				t.Fatal("wrong target digest accepted")
			}
		}
		seen[native.State] = true
		if r.ID != native.ID || r.Type != native.Type || r.Target != native.Target || r.Mode != native.TargetMode || r.SourceOwner != native.SourcePart || r.Location.Span.Start != native.Anchor.Span.Start {
			t.Fatal("declaration changed")
		}
		if native.State == "resolved" {
			if r.TargetPartRef == nil || *r.TargetPartRef != parts[native.ResolvedPart].PartRef {
				t.Fatal("target identity missing")
			}
		} else if r.TargetPartRef != nil {
			t.Fatal("unverified target acquired identity")
		}
		if native.Code != "" && (r.ResolutionCode == nil || *r.ResolutionCode != native.Code) {
			t.Fatal("resolution code lost")
		}
		if native.State == "ambiguous" && r.ResolutionState != "ambiguous" {
			t.Fatal("ambiguity wire mapping")
		}
	}
	for _, state := range []string{"resolved", "external", "missing", "ambiguous", "invalid", "unresolved", "unsupported"} {
		if !seen[state] {
			t.Fatal("fixture lacks resolution state", state, seen)
		}
	}
}
