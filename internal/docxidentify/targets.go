package docxidentify

import (
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

// PartTarget is a declaration-selected candidate, not validation of its payload.
// An empty Namespace on a related story admits either WT-001 Word namespace;
// the relationship URI family alone is not a document conformance assertion.
// Missing targets retain the declaring relationship anchor, never a part digest.
type PartTarget struct {
	Name, Kind, Namespace string
	Declaration           opcrels.Anchor
	Missing               bool
}

func (p *PreparedTypes) MainTarget() (PartTarget, bool) {
	name := p.MainCandidate()
	if name == "" {
		return PartTarget{}, false
	}
	for _, rel := range p.result.OPC.Relationships {
		if opcrels.FoldName(rel.Anchor.Part) == "_rels/.rels" && rel.ResolvedPart == name && (rel.Type == TransitionalRelationship || rel.Type == StrictRelationship) {
			ns := TransitionalWord
			if rel.Type == StrictRelationship {
				ns = StrictWord
			}
			return PartTarget{Name: name, Kind: "main", Namespace: ns, Declaration: rel.Anchor}, true
		}
	}
	return PartTarget{}, false
}
func selectedTarget(rel opcrels.Relationship, kind string) (PartTarget, bool) {
	if rel.TargetMode != "Internal" {
		return PartTarget{}, false
	}
	target := PartTarget{Name: rel.ResolvedPart, Kind: kind, Declaration: rel.Anchor}
	if rel.State == "resolved" && target.Name != "" {
		return target, true
	}
	if rel.State == "missing" && rel.Code == "opc.target_missing" {
		name, code := opcrels.ResolveInternalTarget(rel.SourcePart, rel.Target)
		if code == "" && name != "" {
			target.Name = name
			target.Missing = true
			return target, true
		}
	}
	return PartTarget{}, false
}

// MetadataTargets is the single core/app selection rule for scheduling and
// extraction. A missing target is retained as a gap, not as admitted metadata.
func (p *PreparedTypes) MetadataTargets() []PartTarget {
	targets := []PartTarget{}
	if p == nil || !p.ready {
		return targets
	}
	for _, rel := range p.result.OPC.Relationships {
		if opcrels.FoldName(rel.Anchor.Part) != "_rels/.rels" {
			continue
		}
		kind, want := "", ""
		switch rel.Type {
		case "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties", "http://purl.oclc.org/ooxml/package/relationships/metadata/core-properties":
			kind, want = "core", "application/vnd.openxmlformats-package.core-properties+xml"
		case "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties", "http://purl.oclc.org/ooxml/officeDocument/relationships/extended-properties":
			kind, want = "app", "application/vnd.openxmlformats-officedocument.extended-properties+xml"
		default:
			continue
		}
		target, ok := selectedTarget(rel, kind)
		if ok && (target.Missing || p.AssignedType(target.Name) == want) {
			targets = append(targets, target)
		}
	}
	return targets
}

// RelatedTargets selects only direct main-part declarations. Both relationship
// URI families are recognized; payload roots still pass WT-001 namespace checks.
// Missing declarations are returned so the coordinator can preserve their gaps.
func (p *PreparedTypes) RelatedTargets(opc *opcrels.Result, main, relationshipPart string) (texts, embedded []PartTarget) {
	texts, embedded = []PartTarget{}, []PartTarget{}
	if p == nil || !p.ready || opc == nil || main == "" || relationshipPart == "" {
		return
	}
	for _, rel := range opc.Relationships {
		if rel.Anchor.Part != relationshipPart || rel.SourcePart != main {
			continue
		}
		kind := ""
		for _, prefix := range []string{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/", "http://purl.oclc.org/ooxml/officeDocument/relationships/"} {
			if strings.HasPrefix(rel.Type, prefix) {
				kind = strings.TrimPrefix(rel.Type, prefix)
				break
			}
		}
		target, ok := selectedTarget(rel, kind)
		if !ok {
			continue
		}
		switch kind {
		case "header", "footer", "comments", "footnotes", "endnotes":
			if target.Missing || p.AssignedType(target.Name) == "application/vnd.openxmlformats-officedocument.wordprocessingml."+kind+"+xml" {
				texts = append(texts, target)
			}
		case "oleObject", "package":
			embedded = append(embedded, target)
		}
	}
	return
}

// MetadataCandidates preserves the existing names-only scheduling API.
func (p *PreparedTypes) MetadataCandidates() []string {
	names := []string{}
	for _, t := range p.MetadataTargets() {
		if !t.Missing {
			names = append(names, t.Name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}
