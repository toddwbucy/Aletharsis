package officeplan

import (
	"encoding/json"
	"fmt"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// ScopeArtifact names the retained coordinate text and its verified parent part.
// Mapping is derived: XML decoding and generated controls are not byte identity.
func ScopeArtifact(s v4.Scope, part v4.Part, ordinal int) (v4.Artifact, error) {
	if ordinal < 0 || s.PartRef != part.PartRef || part.SHA256 == nil || identity.ExactBytes([]byte(s.Text)) != s.SHA256 {
		return v4.Artifact{}, v4.ErrLinkage
	}
	ref := fmt.Sprintf("artifact/%d", ordinal)
	encoding := "utf-8"
	size := uint64(len(s.Text))
	native := v2.Artifact{Ref: ref, Kind: "text", Representation: v2.Representation{Serialization: "unicode-scalars-utf8/1", Encoding: &encoding, Normalization: "none", LineEndings: "preserved"}, SHA256: &s.SHA256, ByteLength: &size,
		ContentRef: &v2.ContentRef{Kind: "retained_blob", SHA256: &s.SHA256}, Parents: []string{part.ArtifactRef}, Mapping: v2.Mapping{Quality: "derived", FromArtifactRef: ref, ToArtifactRef: part.ArtifactRef, Method: "office-scalar-origins", Version: "1"}}
	raw, err := json.Marshal(native)
	if err != nil {
		return v4.Artifact{}, err
	}
	var artifact v4.Artifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return v4.Artifact{}, err
	}
	artifact.ContentRef = &v4.ContentRef{Kind: "office_scope", ScopeRef: &s.ScopeRef}
	return artifact, nil
}
