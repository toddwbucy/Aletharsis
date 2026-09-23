package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

// Every direct property is retained or has an omission bound to one producing
// execution's own exclusion. No execution may assess an omitted region.
func (x *Index) validateMetadataProjection(e Evidence, trace *TraceIndex) error {
	retained := map[string]map[int64]bool{}
	for _, m := range e.Metadata {
		if retained[m.PartRef] == nil {
			retained[m.PartRef] = map[int64]bool{}
		}
		retained[m.PartRef][m.Element] = true
	}
	groups := map[string][]Outcome{}
	for _, pkg := range e.Packages {
		for _, o := range pkg.Outcomes {
			if o.Operation != "aletharsis.office.metadata" || o.PartRef == nil || (o.State != "completed" && o.State != "partial") {
				continue
			}
			parent, ok := trace.Executions[o.ExecutionRef]
			if !ok || parent.CapabilityRef != o.Operation || (parent.State != v2.Completed && parent.State != v2.Partial) {
				return ErrLinkage
			}
			groups[*o.PartRef] = append(groups[*o.PartRef], o)
		}
	}
	documents := map[string]XML{}
	for _, doc := range e.XML {
		documents[doc.PartRef] = doc
	}
	for partRef, outcomes := range groups {
		part := x.Parts[partRef]
		doc, ok := documents[partRef]
		if !ok || len(doc.Elements) == 0 || officemetadata.ProjectionKind(doc.Elements[0].Namespace, doc.Elements[0].LocalName) == "" {
			return ErrLinkage
		}
		assessed, excluded := []Span{}, []Span{}
		for _, o := range outcomes {
			assessed = append(assessed, o.Assessed...)
			excluded = append(excluded, o.Excluded...)
		}
		assessed, excluded = normalizedSpans(assessed), normalizedSpans(excluded)
		if normalizedSpansOverlap(assessed, excluded) {
			return ErrLinkage
		}
		children := map[Span]int64{}
		for _, element := range doc.Elements {
			if element.Parent != nil && *element.Parent == 0 {
				children[element.Span] = element.Index
			}
		}
		gaps := map[int64]bool{}
		for _, o := range outcomes {
			ownExcluded := normalizedSpans(o.Excluded)
			for _, ref := range o.DiagnosticRefs {
				diagnostic := trace.Diagnostics[ref]
				switch diagnostic.Code {
				case "metadata.property_unassessed", "metadata.standard_property_unassessed", "metadata.structured_value_unassessed":
				default:
					continue
				}
				scope := diagnostic.Scope
				if diagnostic.ExecutionRef == nil || *diagnostic.ExecutionRef != o.ExecutionRef || scope == nil || scope.Unit != "byte" || scope.ArtifactRef != part.ArtifactRef || len(scope.Regions) != 1 {
					return ErrLinkage
				}
				span := Span{int64(scope.Regions[0].Start), int64(scope.Regions[0].End)}
				element, ok := children[span]
				if !ok || retained[partRef][element] || !coveredByNormalized([]Span{span}, ownExcluded) {
					return ErrLinkage
				}
				gaps[element] = true
			}
		}
		for _, element := range children {
			if !retained[partRef][element] && !gaps[element] {
				return ErrLinkage
			}
		}
	}
	return nil
}

// Inputs are normalized, sorted, disjoint lists of half-open spans.
func normalizedSpansOverlap(a, b []Span) bool {
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if a[i].Start == a[i].End || a[i].End <= b[j].Start {
			i++
			continue
		}
		if b[j].Start == b[j].End || b[j].End <= a[i].Start {
			j++
			continue
		}
		return true
	}
	return false
}
