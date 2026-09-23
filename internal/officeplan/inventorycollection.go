package officeplan

import (
	"context"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

type InventoryGap struct {
	Operation, Part, ObjectRef, Code string
	Relationship                     *int
}
type Inventories struct {
	Metadata         []v4.Metadata
	Relationships    []v4.Relationship
	Objects          []v4.Object
	RelationshipRefs map[int]string
	Gaps             []InventoryGap
}

// CollectInventories assembles cross-linked observations without modifying the
// producer evidence. Missing XML caused by an explicit mapping failure is a
// coverage gap; unexplained missing references remain assembly errors.
func CollectInventories(ctx context.Context, p *Prepared, a *Analysis, base *PackageRecords, xml *XMLInventory, objectExecution int) (*Inventories, error) {
	return collectInventories(ctx, p, a, base, xml, objectExecution, true)
}

// Internal assembly uses XML already validated by CollectXML.
func collectInventories(ctx context.Context, p *Prepared, a *Analysis, base *PackageRecords, xml *XMLInventory, objectExecution int, validateXML bool) (*Inventories, error) {
	if ctx == nil || p == nil || p.Admission == nil || a == nil || base == nil || xml == nil || len(base.Evidence.Packages) != 1 || objectExecution < 0 {
		return nil, v4.ErrLinkage
	}
	if base.Evidence.Packages[0].SourceSHA256 != p.Admission.Outcomes.SourceSHA256 {
		return nil, v4.ErrLinkage
	}
	r := &Inventories{Metadata: []v4.Metadata{}, Relationships: []v4.Relationship{}, Objects: []v4.Object{}, RelationshipRefs: map[int]string{}, Gaps: []InventoryGap{}}
	parts := map[string]v4.Part{}
	indexed := map[string]v4.Part{}
	for _, part := range base.Evidence.Packages[0].Parts {
		parts[part.Name] = part
		indexed[part.PartRef] = part
	}
	docs := map[string]v4.XML{}
	byRef := map[string]v4.XML{}
	for name, i := range xml.ByPart {
		if i < 0 || i >= len(xml.Documents) {
			return nil, v4.ErrLinkage
		}
		if validateXML {
			if err := v4.ValidateXML(xml.Documents[i], parts[name]); err != nil {
				return nil, err
			}
		}
		docs[name] = xml.Documents[i]
		byRef[xml.Documents[i].XMLRef] = xml.Documents[i]
	}
	missing := map[string]bool{}
	for _, gap := range xml.Failures {
		missing[gap.Part] = true
	}
	for _, analysis := range a.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if analysis.Metadata == nil {
			continue
		}
		doc, ok := docs[analysis.Part]
		if !ok {
			if !missing[analysis.Part] {
				return nil, v4.ErrLinkage
			}
			r.Gaps = append(r.Gaps, InventoryGap{Operation: capability.OfficeMetadataID, Part: analysis.Part, Code: "office.xml_mapping_unavailable"})
			continue
		}
		records, err := metadataRecords(analysis.Metadata, parts[analysis.Part], doc)
		if err != nil {
			return nil, err
		}
		r.Metadata = append(r.Metadata, records...)
	}
	skipped := map[int]bool{}
	if p.DOCX != nil && p.DOCX.OPC != nil {
		for i, native := range p.DOCX.OPC.Relationships {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			doc, ok := docs[native.Anchor.Part]
			if !ok {
				if !missing[native.Anchor.Part] {
					return nil, v4.ErrLinkage
				}
				index := i
				r.Gaps = append(r.Gaps, InventoryGap{Operation: capability.OfficeRelationshipsID, Part: native.Anchor.Part, Relationship: &index, Code: "office.xml_mapping_unavailable"})
				skipped[i] = true
				continue
			}
			record, err := relationshipRecord(native, parts, doc, i)
			if err != nil {
				return nil, err
			}
			r.RelationshipRefs[i] = record.RelationshipRef
			r.Relationships = append(r.Relationships, record)
		}
	}
	if a.Objects != nil {
		if a.Objects.SourceSHA256 != p.Admission.Outcomes.SourceSHA256 {
			return nil, v4.ErrLinkage
		}
		for _, original := range a.Objects.Objects {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			native := original
			native.Relationships = []int{}
			incomplete := false
			for _, i := range original.Relationships {
				if skipped[i] {
					incomplete = true
					index := i
					r.Gaps = append(r.Gaps, InventoryGap{Operation: capability.OfficeObjectsID, Part: original.Anchor.Part, ObjectRef: original.ID, Relationship: &index, Code: "office.relationship_evidence_unavailable"})
				} else {
					native.Relationships = append(native.Relationships, i)
				}
			}
			record, err := ObjectRecord(native, parts[native.Anchor.Part], r.RelationshipRefs, objectExecution)
			if err != nil {
				return nil, err
			}
			if incomplete {
				if record.Inspection.State == "completed" {
					record.Inspection.State = "partial"
				}
				record.Inspection.Codes = append(record.Inspection.Codes, "office.relationship_evidence_unavailable")
			}
			r.Objects = append(r.Objects, record)
		}
	}
	index := &v4.Index{Parts: indexed, XML: byRef}
	if err := index.ValidateInventories(v4.Evidence{Metadata: r.Metadata, Relationships: r.Relationships, Objects: r.Objects}); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
