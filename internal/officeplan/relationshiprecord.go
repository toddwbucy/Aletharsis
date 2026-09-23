package officeplan

import (
	"fmt"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

// RelationshipRecord preserves the original declaration and binds its location
// and any verified target to named package parts. It performs no target reads.
// Missing XML evidence is an error for the coordinator to record as a gap.
func RelationshipRecord(native opcrels.Relationship, parts map[string]v4.Part, x v4.XML, ordinal int) (v4.Relationship, error) {
	owner, ok := parts[native.Anchor.Part]
	if !ok {
		return v4.Relationship{}, v4.ErrLinkage
	}
	if err := v4.ValidateXML(x, owner); err != nil {
		return v4.Relationship{}, err
	}
	return relationshipRecord(native, parts, x, ordinal)
}

// Shared XML is already validated once per part by the collection coordinator.
func relationshipRecord(native opcrels.Relationship, parts map[string]v4.Part, x v4.XML, ordinal int) (v4.Relationship, error) {
	owner, ok := parts[native.Anchor.Part]
	if !ok || ordinal < 0 || owner.SHA256 == nil || *owner.SHA256 != native.Anchor.PartSHA256 {
		return v4.Relationship{}, v4.ErrLinkage
	}
	element := int64(native.Anchor.Element)
	span := wireSpan(native.Anchor.Span)
	r := v4.Relationship{RelationshipRef: fmt.Sprintf("office-relationship/%d", ordinal), ID: native.ID, Type: native.Type, Target: native.Target, Mode: native.TargetMode, SourceOwner: native.SourcePart, ResolutionState: native.State,
		Location: v4.StructuralLocation{Kind: "office_structure", PartRef: owner.PartRef, PartSHA256: *owner.SHA256, XMLRef: &x.XMLRef, Element: &element, Span: &span}}
	if native.Code != "" {
		code := native.Code
		r.ResolutionCode = &code
	}
	switch native.State {
	case "ambiguous":
		// Wire 4.0 groups unsuccessful resolution under unresolved; the exact OPC
		// ambiguity code remains evidence and no target reference is manufactured.
		if native.Code != "opc.target_ambiguous" {
			return v4.Relationship{}, v4.ErrLinkage
		}

	case "resolved":
		target, ok := parts[native.ResolvedPart]
		if !ok || (target.SHA256 == nil && native.TargetSHA256 != "") || (target.SHA256 != nil && *target.SHA256 != native.TargetSHA256) || target.PackageRef != owner.PackageRef {
			return v4.Relationship{}, v4.ErrLinkage
		}
		ref := target.PartRef
		r.TargetPartRef = &ref
	case "external", "unresolved", "invalid", "unsupported", "missing":
	default:
		return v4.Relationship{}, v4.ErrLinkage
	}
	index := &v4.Index{Parts: map[string]v4.Part{owner.PartRef: owner}, XML: map[string]v4.XML{x.XMLRef: x}}
	if r.TargetPartRef != nil {
		target := parts[native.ResolvedPart]
		index.Parts[target.PartRef] = target
	}
	if err := index.ValidateStructuralLocation(r.Location, nil); err != nil {
		return v4.Relationship{}, err
	}
	return r, nil
}
