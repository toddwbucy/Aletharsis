package v4

import (
	"encoding/json"
)

// Artifact retains v3's wire shape; only Office scope content adds a native
// discriminator. Adapter-specific records remain unsupported by native import.
type Artifact struct {
	Ref               string          `json:"artifact_ref"`
	Kind              string          `json:"kind"`
	Representation    json.RawMessage `json:"representation"`
	SHA256            *string         `json:"sha256"`
	ByteLength        *uint64         `json:"byte_length"`
	ContentRef        *ContentRef     `json:"content_ref"`
	UnavailableReason *string         `json:"unavailable_reason"`
	Parents           []string        `json:"parents"`
	Transform         json.RawMessage `json:"transform"`
	Mapping           json.RawMessage `json:"mapping"`
}
type ContentRef struct {
	Kind     string  `json:"kind"`
	Pointer  *string `json:"pointer,omitempty"`
	SHA256   *string `json:"sha256,omitempty"`
	ScopeRef *string `json:"scope_ref,omitempty"`
}

// ValidateScopeArtifacts validates the scope-text portion of the report graph.
// Common artifact identities and transforms are checked by the report validator.
// The Office scope ref resolves its own coordinate artifact without a second,
// competing coordinate identifier in the finding or scope record.
func (x *Index) ValidateScopeArtifacts(artifacts []Artifact) error {
	seen := map[string]bool{}
	for _, a := range artifacts {
		if a.ContentRef == nil || a.ContentRef.Kind != "office_scope" {
			continue
		}
		c := a.ContentRef
		if c.ScopeRef == nil || c.Pointer != nil || c.SHA256 != nil || a.Kind != "text" ||
			a.SHA256 == nil || a.ByteLength == nil || a.UnavailableReason != nil {
			return ErrLinkage
		}
		scope, ok := x.Scopes[*c.ScopeRef]
		if !ok || seen[*c.ScopeRef] || *a.SHA256 != scope.SHA256 || *a.ByteLength != uint64(len(scope.Text)) {
			return ErrLinkage
		}
		seen[*c.ScopeRef] = true
		var representation map[string]any
		if json.Unmarshal(a.Representation, &representation) != nil || representation["encoding"] != "utf-8" {
			return ErrLinkage
		}
	}
	// Every retained scope participates in the same graph used for coverage.
	if len(seen) != len(x.Scopes) {
		return ErrLinkage
	}
	return nil
}

// ScopeArtifactEquivalent is used when linking a scope anchor to a graph asset.
func ScopeArtifactEquivalent(a Artifact, s Scope) bool {
	return a.ContentRef != nil && a.ContentRef.Kind == "office_scope" && a.ContentRef.ScopeRef != nil &&
		*a.ContentRef.ScopeRef == s.ScopeRef && a.SHA256 != nil && *a.SHA256 == s.SHA256 &&
		a.ByteLength != nil && *a.ByteLength == uint64(len(s.Text)) && a.UnavailableReason == nil
}
