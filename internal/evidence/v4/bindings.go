package v4

import "slices"

// ValidateArtifactBindings establishes all three coordinate identities: source
// container, decompressed named part, and retained scope text. Content equality
// alone never authorizes substituting one part or scope for another.
func (x *Index) ValidateArtifactBindings(artifacts []Artifact) error {
	byRef := map[string]Artifact{}
	for _, a := range artifacts {
		if _, exists := byRef[a.Ref]; exists {
			return ErrLinkage
		}
		seen := map[string]bool{}
		for _, parent := range a.Parents {
			if _, ok := byRef[parent]; !ok || seen[parent] {
				return ErrLinkage
			}
			seen[parent] = true
		}
		byRef[a.Ref] = a
	}
	for _, p := range x.Packages {
		source, ok := byRef[p.SourceArtifactRef]
		if !ok || source.Kind != "source" || source.SHA256 == nil || *source.SHA256 != p.SourceSHA256 ||
			source.ByteLength == nil || p.SourceByteLength < 0 || *source.ByteLength != uint64(p.SourceByteLength) || len(source.Parents) != 0 {
			return ErrLinkage
		}
	}
	usedParts := map[string]bool{}
	for _, p := range x.Parts {
		a, ok := byRef[p.ArtifactRef]
		container, known := x.Packages[p.PackageRef]
		if !ok || !known || usedParts[a.Ref] || a.Kind != "package_part" || !sameString(a.SHA256, p.SHA256) ||
			len(a.Parents) != 1 || a.Parents[0] != container.SourceArtifactRef || (a.ByteLength == nil) != (p.ByteLength == nil) {
			return ErrLinkage
		}
		if p.ByteLength != nil && (*p.ByteLength < 0 || *a.ByteLength != uint64(*p.ByteLength)) {
			return ErrLinkage
		}
		usedParts[a.Ref] = true
	}
	if err := x.ValidateScopeArtifacts(artifacts); err != nil {
		return err
	}
	for _, a := range artifacts {
		if a.ContentRef == nil || a.ContentRef.Kind != "office_scope" {
			continue
		}
		s := x.Scopes[*a.ContentRef.ScopeRef]
		part, ok := x.Parts[s.PartRef]
		if !ok || !slices.Equal(a.Parents, []string{part.ArtifactRef}) {
			return ErrLinkage
		}
	}
	return nil
}
