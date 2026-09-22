package v4

import "github.com/toddwbucy/Aletharsis/internal/identity"

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
		if r.ResolutionState == "resolved" {
			if r.TargetPartRef == nil {
				return ErrLinkage
			}
			target, ok := x.Parts[*r.TargetPartRef]
			owner := x.Parts[r.Location.PartRef]
			if !ok || target.SHA256 == nil || target.PackageRef != owner.PackageRef {
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
			if !ok || seen[ref] || r.TargetPartRef == nil || *r.TargetPartRef != o.PartRef {
				return ErrLinkage
			}
			seen[ref] = true
		}
	}
	occurrences := map[[3]string]int64{}
	for _, m := range e.Metadata {
		doc, ok := x.XML[m.XMLRef]
		if !ok || doc.PartRef != m.PartRef || m.Element < 0 || m.Element >= int64(len(doc.Elements)) {
			return ErrLinkage
		}
		element := doc.Elements[m.Element]
		if element.Namespace != m.Namespace || element.LocalName != m.LocalName {
			return ErrLinkage
		}
		key := [3]string{m.PartRef, m.Namespace, m.LocalName}
		if m.DuplicateOrdinal != occurrences[key] {
			return ErrLinkage
		}
		occurrences[key]++
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
