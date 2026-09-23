package docxidentify

import (
	"context"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// PreparedTypes retains one content-type parse for priority planning and final
// identification. Private fields prevent callers from replacing parsed declarations.
type PreparedTypes struct {
	result   *Result
	ready    bool
	source   string
	assigned map[string]string
}

func PrepareTypes(ctx context.Context, opc *opcrels.Result) (*PreparedTypes, error) {
	if ctx == nil || opc == nil || opc.Outcomes == nil || opc.Outcomes.SourceSHA256 == "" {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, ready, err := prepareTypes(ctx, opc)
	if err != nil {
		return nil, err
	}
	p := &PreparedTypes{result: r, ready: ready, source: opc.Outcomes.SourceSHA256, assigned: map[string]string{}}
	if ready {
		assigned := p.copy(opc)
		assigned.assign(opc.Outcomes.Parts)
		for _, a := range assigned.Assignments {
			if a.State == "assigned" {
				p.assigned[a.Part] = a.ContentType
			}
		}
	}
	return p, nil
}
func (p *PreparedTypes) copy(opc *opcrels.Result) *Result {
	r := *p.result
	r.OPC = opc
	r.Declarations = slices.Clone(r.Declarations)
	r.Assignments = []Assignment{}
	r.Issues = slices.Clone(r.Issues)
	return &r
}

// MainCandidate returns only a unique declared internal DOCX main part. Its bytes
// need not be admitted yet. The name is a scheduling hint, not identification.
func (p *PreparedTypes) MainCandidate() string {
	if p == nil || !p.ready {
		return ""
	}
	opc := p.result.OPC
	rootOK := acceptableRootRelationships(opc)
	rootType := false
	for part, contentType := range p.assigned {
		if opcrels.FoldName(part) == "_rels/.rels" && contentType == "application/vnd.openxmlformats-package.relationships+xml" {
			rootType = true
		}
	}
	if !rootOK || !rootType {
		return ""
	}
	candidates := []opcrels.Relationship{}
	for _, rel := range opc.Relationships {
		if opcrels.FoldName(rel.Anchor.Part) == "_rels/.rels" && (rel.Type == TransitionalRelationship || rel.Type == StrictRelationship) {
			candidates = append(candidates, rel)
		}
	}
	if len(candidates) != 1 {
		return ""
	}
	rel := candidates[0]
	if rel.TargetMode != "Internal" || rel.State != "resolved" {
		return ""
	}
	if p.assigned[rel.ResolvedPart] == MainContentType {
		return rel.ResolvedPart
	}
	return ""
}

// InspectPrepared performs final identification using the shared OPC result and
// previously parsed content types. No second content-type parse is performed.
func InspectPrepared(ctx context.Context, opc *opcrels.Result, p *PreparedTypes) (*Result, error) {
	if ctx == nil || opc == nil || opc.Outcomes == nil || p == nil || p.result == nil || p.source != opc.Outcomes.SourceSHA256 {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := p.copy(opc)
	if !p.ready {
		return r, nil
	}
	matches := find(opc.Outcomes.Parts, r.TypesPart)
	if len(matches) != 1 || opc.Outcomes.Parts[matches[0]].SHA256 != r.TypesSHA256 {
		return nil, packageparts.ErrIdentity
	}
	return finishOPC(ctx, opc, r)
}

// AssignedType is a declaration lookup, not validation of target content.
func (p *PreparedTypes) AssignedType(name string) string {
	if p == nil || !p.ready {
		return ""
	}
	return p.assigned[name]
}
