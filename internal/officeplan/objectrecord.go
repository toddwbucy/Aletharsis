package officeplan

import (
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/officeobjects"
)

// ObjectRecord preserves candidate inclusion and bounded inspection evidence.
// Relationship ordinals belong to the shared OPC inventory, not document Id
// attributes (which can repeat). The coordinator must bind the final graph.
func ObjectRecord(native officeobjects.Object, part v4.Part, relationships map[int]string, execution int) (v4.Object, error) {
	if execution < 0 || native.Anchor.Part != part.Name || native.Anchor.ObjectID != native.ID || native.State != part.State || len(native.DeclaredTypes) > 1 || len(native.Reasons) == 0 {
		return v4.Object{}, v4.ErrLinkage
	}
	switch native.State {
	case "completed":
		if part.SHA256 == nil {
			return v4.Object{}, v4.ErrLinkage
		}
	case "failed", "not_run", "canceled":
		if part.SHA256 != nil {
			return v4.Object{}, v4.ErrLinkage
		}
	default:
		return v4.Object{}, v4.ErrLinkage
	}
	if (part.SHA256 == nil) != (part.ByteLength == nil) || (native.ByteLength == nil) != (part.ByteLength == nil) {
		return v4.Object{}, v4.ErrLinkage
	}
	if part.SHA256 == nil {
		if native.Anchor.PartSHA256 != "" || native.Signature != "" || len(native.Assessed) != 0 {
			return v4.Object{}, v4.ErrLinkage
		}
	} else if native.Anchor.PartSHA256 != *part.SHA256 || *part.ByteLength < 0 || *native.ByteLength != uint64(*part.ByteLength) {
		return v4.Object{}, v4.ErrLinkage
	}
	r := v4.Object{ObjectRef: native.ID, PartRef: part.PartRef, InclusionEvidence: slices.Clone(native.Reasons), RelationshipRefs: []string{},
		Inspection: v4.Outcome{Operation: capability.OfficeObjectsID, ExecutionRef: fmt.Sprintf("exec/%d", execution), PartRef: &part.PartRef, State: native.State, Codes: []string{}, DiagnosticRefs: []string{}, Assessed: []v4.Span{}, Excluded: []v4.Span{}}}
	if part.SHA256 != nil {
		digest := *part.SHA256
		r.SHA256 = &digest
	}
	if len(native.DeclaredTypes) == 1 {
		value := native.DeclaredTypes[0]
		r.DeclaredType = &value
	}
	if native.Signature != "" {
		value := native.Signature
		r.ObservedSignature = &value
	}
	if native.Code != "" {
		r.Inspection.Codes = append(r.Inspection.Codes, native.Code)
	}
	seen := map[string]bool{}
	for _, index := range native.Relationships {
		ref, ok := relationships[index]
		if !ok || ref == "" || seen[ref] {
			return v4.Object{}, v4.ErrLinkage
		}
		seen[ref] = true
		r.RelationshipRefs = append(r.RelationshipRefs, ref)
	}
	for _, span := range native.Assessed {
		s := v4.Span{Start: span.Start, End: span.End}
		if part.ByteLength == nil || !s.Within(*part.ByteLength) || s.End > officeobjects.SignatureBytes {
			return v4.Object{}, v4.ErrCoordinates
		}
		r.Inspection.Assessed = append(r.Inspection.Assessed, s)
	}
	return r, nil
}
