package v4

import "encoding/json"

type Anchor struct {
	Ref          string          `json:"anchor_ref"`
	Kind         string          `json:"kind"`
	ArtifactRef  string          `json:"artifact_ref"`
	ExecutionRef *string         `json:"execution_ref"`
	Mapping      json.RawMessage `json:"mapping"`
	Locator      json.RawMessage `json:"locator"`
}

// ValidateOfficeAnchor binds an already wire-validated Office locator to the
// exact graph artifact. Common execution and mapping checks run at report level.
func (x *Index) ValidateOfficeAnchor(a Anchor, artifacts map[string]Artifact, objects map[string]Object) error {
	art, ok := artifacts[a.ArtifactRef]
	if !ok || a.ExecutionRef == nil {
		return ErrLinkage
	}
	switch a.Kind {
	case "office_scope":
		var l ScopeLocation
		if json.Unmarshal(a.Locator, &l) != nil || l.Kind != "office_scope" {
			return ErrLinkage
		}
		scope, ok := x.Scopes[l.ScopeRef]
		if !ok || !ScopeArtifactEquivalent(art, scope) {
			return ErrLinkage
		}
	case "office_structure":
		var l StructuralLocation
		if json.Unmarshal(a.Locator, &l) != nil || l.Kind != "office_structure" {
			return ErrLinkage
		}
		if err := x.ValidateStructuralLocation(l, objects); err != nil {
			return err
		}
		if x.Parts[l.PartRef].ArtifactRef != a.ArtifactRef {
			return ErrLinkage
		}
	default:
		return ErrLinkage
	}
	return nil
}
