package officeplan

import (
	"context"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/scopelimits"
)

type ScopedFindings struct {
	ScopeRef, ArtifactRef, PartRef string
	Findings                       []evidence.Finding // Borrowed producer findings; coordinator must not mutate.
}

// ScopeCollection keeps omitted scopes and extraction boundaries available for
// coverage assembly. Neither is converted into a retained text artifact.
type ScopeCollection struct {
	Scopes     []v4.Scope
	Artifacts  []v4.Artifact
	Findings   []ScopedFindings
	Boundaries map[string][]v4.Boundary
	Omitted    map[string][]scopelimits.Omission
}

func CollectScopes(ctx context.Context, base *PackageRecords, a *Analysis, xml *XMLInventory) (*ScopeCollection, error) {
	return collectScopes(ctx, base, a, xml, true)
}

// Internal assembly uses XML already validated by CollectXML.
func collectScopes(ctx context.Context, base *PackageRecords, a *Analysis, xml *XMLInventory, validateXML bool) (*ScopeCollection, error) {
	if ctx == nil || base == nil || a == nil || xml == nil || len(base.Evidence.Packages) != 1 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	analyses := map[string]PartAnalysis{}
	for _, part := range a.Parts {
		if part.Word != nil || part.ODT != nil {
			if _, exists := analyses[part.Part]; exists || part.Word != nil && part.ODT != nil {
				return nil, v4.ErrLinkage
			}
			analyses[part.Part] = part
		}
	}
	r := &ScopeCollection{Scopes: []v4.Scope{}, Artifacts: []v4.Artifact{}, Findings: []ScopedFindings{}, Boundaries: map[string][]v4.Boundary{}, Omitted: map[string][]scopelimits.Omission{}}
	pkg := base.Evidence.Packages[0]
	appendScope := func(s v4.Scope, part v4.Part, findings []evidence.Finding) error {
		artifact, err := ScopeArtifact(s, part, len(base.Artifacts)+len(r.Artifacts))
		if err != nil {
			return err
		}
		r.Scopes = append(r.Scopes, s)
		r.Artifacts = append(r.Artifacts, artifact)
		r.Findings = append(r.Findings, ScopedFindings{s.ScopeRef, artifact.Ref, part.PartRef, findings})
		return nil
	}
	for _, part := range pkg.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		analysis, ok := analyses[part.Name]
		if !ok {
			continue
		}
		delete(analyses, part.Name)
		index, ok := xml.ByPart[part.Name]
		if !ok || index < 0 || index >= len(xml.Documents) {
			return nil, v4.ErrLinkage
		}
		doc := xml.Documents[index]
		// Validate the shared XML once per part, not once per paragraph/scope.
		if validateXML {
			if err := v4.ValidateXML(doc, part); err != nil {
				return nil, err
			}
		}
		r.Boundaries[part.PartRef] = []v4.Boundary{}
		if word := analysis.Word; word != nil {
			r.Omitted[part.PartRef] = append([]scopelimits.Omission{}, word.Omitted...)
			for _, b := range word.Boundaries {
				r.Boundaries[part.PartRef] = append(r.Boundaries[part.PartRef], v4.Boundary{XMLRef: doc.XMLRef, Token: int64(b.Token), Reason: b.Reason})
			}
			for _, native := range word.Scopes {
				s, err := wordScopeRecord(native, part, pkg.SourceSHA256, doc, len(r.Scopes))
				if err != nil {
					return nil, err
				}
				if err := appendScope(s, part, native.Findings); err != nil {
					return nil, err
				}
			}
		}
		if odt := analysis.ODT; odt != nil {
			r.Omitted[part.PartRef] = append([]scopelimits.Omission{}, odt.Omitted...)
			for _, b := range odt.Boundaries {
				r.Boundaries[part.PartRef] = append(r.Boundaries[part.PartRef], v4.Boundary{XMLRef: doc.XMLRef, Token: int64(b.Token), Reason: b.Reason})
			}
			for _, native := range odt.Scopes {
				s, err := odtScopeRecord(native, part, pkg.SourceSHA256, doc, xml.ControlRefs[part.Name], len(r.Scopes))
				if err != nil {
					return nil, err
				}
				if err := appendScope(s, part, native.Findings); err != nil {
					return nil, err
				}
			}
		}
	}
	// Boundaries are part-qualified lexical context, not offsets in a scope's
	// text. Retain each part's list once on its first surviving scope; copying
	// every boundary onto every paragraph would grow quadratically.
	attached := map[string]bool{}
	for i := range r.Scopes {
		scope := &r.Scopes[i]
		if !attached[scope.PartRef] {
			scope.Boundaries = slices.Clone(r.Boundaries[scope.PartRef])
			if scope.Boundaries == nil {
				scope.Boundaries = []v4.Boundary{}
			}
			attached[scope.PartRef] = true
		}
	}
	if len(analyses) != 0 {
		return nil, v4.ErrLinkage
	}
	return r, nil
}
