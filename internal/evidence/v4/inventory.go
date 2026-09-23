package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

// ValidateStructuralLocation checks a part-qualified XML or opaque-object
// anchor. Coordinates always index decompressed part bytes, never ZIP bytes.
func (x *Index) ValidateStructuralLocation(l StructuralLocation, objects map[string]Object) error {
	p, ok := x.Parts[l.PartRef]
	if !ok || p.SHA256 == nil || p.ByteLength == nil || *p.SHA256 != l.PartSHA256 {
		return ErrLinkage
	}
	if l.ObjectRef != nil {
		o, ok := objects[*l.ObjectRef]
		if !ok || o.PartRef != l.PartRef || l.XMLRef != nil || l.Element != nil || l.Token != nil || l.Span != nil {
			return ErrLinkage
		}
		return nil
	}
	if l.XMLRef == nil || l.Span == nil || !l.Span.Within(*p.ByteLength) || (l.Element == nil && l.Token == nil) {
		return ErrLinkage
	}
	doc, ok := x.XML[*l.XMLRef]
	if !ok || doc.PartRef != p.PartRef {
		return ErrLinkage
	}
	if l.Element != nil {
		if *l.Element < 0 || *l.Element >= int64(len(doc.Elements)) {
			return ErrLinkage
		}
		e := doc.Elements[*l.Element]
		if l.Span.Start < e.Span.Start || l.Span.End > e.Span.End {
			return ErrLinkage
		}
	}
	if l.Token != nil {
		if *l.Token < 0 || *l.Token >= int64(len(doc.Tokens)) {
			return ErrLinkage
		}
		t := doc.Tokens[*l.Token]
		if l.Span.Start < t.Span.Start || l.Span.End > t.Span.End {
			return ErrLinkage
		}
		if l.Element != nil && (t.Element == nil || *t.Element != *l.Element) {
			return ErrLinkage
		}
	}
	return nil
}

// ValidateInventories binds metadata and relationships to the indexed XML and
// objects to verified parts. No external relationship is followed.
func (x *Index) ValidateInventories(e Evidence) error {
	objects := map[string]Object{}
	for _, o := range e.Objects {
		p, ok := x.Parts[o.PartRef]
		if !ok || !canonicalRef(o.ObjectRef, "object") || !sameString(o.SHA256, p.SHA256) ||
			o.Inspection.PartRef == nil || *o.Inspection.PartRef != p.PartRef || len(o.InclusionEvidence) == 0 {
			return ErrLinkage
		}
		if _, exists := objects[o.ObjectRef]; exists {
			return ErrLinkage
		}
		objects[o.ObjectRef] = o
	}
	// Index names once per package. Resolution links are checked against the
	// declaring part's owner and this inventory, including unverified targets.
	names := map[[2]string][]string{}
	for ref, part := range x.Parts {
		key := [2]string{part.PackageRef, opcrels.FoldName(part.Name)}
		names[key] = append(names[key], ref)
	}
	rels := map[string]Relationship{}
	for _, r := range e.Relationships {
		if !canonicalRef(r.RelationshipRef, "relationship") {
			return ErrLinkage
		}
		if _, exists := rels[r.RelationshipRef]; exists {
			return ErrLinkage
		}
		if err := x.ValidateStructuralLocation(r.Location, objects); err != nil {
			return err
		}
		if r.Location.ObjectRef != nil {
			return ErrLinkage
		}
		declaring := x.Parts[r.Location.PartRef]
		owner, valid := opcrels.RelationshipOwner(declaring.Name)
		if !valid || opcrels.FoldName(owner) != opcrels.FoldName(r.SourceOwner) {
			return ErrLinkage
		}
		if r.ResolutionState == "resolved" {
			if owner != "" && len(names[[2]string{declaring.PackageRef, opcrels.FoldName(owner)}]) != 1 {
				return ErrLinkage
			}
			target, code := opcrels.ResolveInternalTarget(owner, r.Target)
			matches := names[[2]string{declaring.PackageRef, opcrels.FoldName(target)}]
			if r.Mode != "Internal" || r.ResolutionCode != nil || code != "" || len(matches) != 1 ||
				r.TargetPartRef == nil || *r.TargetPartRef != matches[0] {
				return ErrLinkage
			}
		} else if r.TargetPartRef != nil {
			return ErrLinkage
		}
		rels[r.RelationshipRef] = r
	}
	for _, o := range e.Objects {
		seen := map[string]bool{}
		for _, ref := range o.RelationshipRefs {
			r, ok := rels[ref]
			if !ok || seen[ref] {
				return ErrLinkage
			}
			if r.ResolutionState != "resolved" || r.TargetPartRef == nil || *r.TargetPartRef != o.PartRef {
				return ErrLinkage
			}
			seen[ref] = true
		}
	}
	// Duplicate ordinals describe original siblings, including structured values
	// excluded from the scalar metadata projection. Derive them from retained XML
	// rather than renumbering only the successfully extracted metadata values.
	ordinals := map[string][]int64{}
	for ref, doc := range x.XML {
		counts := map[struct {
			parent           int64
			namespace, local string
		}]int64{}
		values := make([]int64, len(doc.Elements))
		for i, e := range doc.Elements {
			parent := int64(-1)
			if e.Parent != nil {
				parent = *e.Parent
			}
			key := struct {
				parent           int64
				namespace, local string
			}{parent, e.Namespace, e.LocalName}
			values[i] = counts[key]
			counts[key]++
		}
		ordinals[ref] = values
	}
	seenMetadata := map[struct {
		xml     string
		element int64
	}]bool{}
	for _, m := range e.Metadata {
		key := struct {
			xml     string
			element int64
		}{m.PartRef, m.Element}
		if seenMetadata[key] {
			return ErrLinkage
		}
		seenMetadata[key] = true
		doc, ok := x.XML[m.XMLRef]
		if !ok || doc.PartRef != m.PartRef || m.Element < 0 || m.Element >= int64(len(doc.Elements)) {
			return ErrLinkage
		}
		element := doc.Elements[m.Element]
		if element.Namespace != m.Namespace || element.LocalName != m.LocalName {
			return ErrLinkage
		}
		if m.DuplicateOrdinal != ordinals[m.XMLRef][m.Element] {
			return ErrLinkage
		}
		if err := ValidateStoredOrigins(m.LexicalValue, identity.ExactBytes([]byte(m.LexicalValue)),
			m.ValueOrigins, map[string][]Segment{m.XMLRef: doc.Segments}); err != nil {
			return err
		}
		for _, o := range m.ValueOrigins {
			if o.XMLRef != m.XMLRef {
				return ErrLinkage
			}
			segment := doc.Segments[*o.Segment]
			if segment.Element == nil || int64(*segment.Element) != m.Element {
				return ErrLinkage
			}
		}
	}
	return nil
}
func sameString(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
