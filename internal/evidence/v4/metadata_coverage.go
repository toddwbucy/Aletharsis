package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

// A completed/partial metadata projection accounts for every direct property:
// either a retained scalar, or a diagnostic at the exact property's byte span.
// Attribute/markup diagnostics do not stand in for a missing property.
func (x *Index) validateMetadataProjection(e Evidence, trace *TraceIndex) error {
	retained := map[string]map[int64]bool{}
	for _, m := range e.Metadata {
		if retained[m.PartRef] == nil {
			retained[m.PartRef] = map[int64]bool{}
		}
		retained[m.PartRef][m.Element] = true
	}
	for _, pkg := range e.Packages {
		for _, outcome := range pkg.Outcomes {
			if outcome.Operation != "aletharsis.office.metadata" || outcome.PartRef == nil || (outcome.State != "completed" && outcome.State != "partial") {
				continue
			}
			part := x.Parts[*outcome.PartRef]
			var doc *XML
			for _, candidate := range e.XML {
				if candidate.PartRef == part.PartRef {
					copy := candidate
					doc = &copy
					break
				}
			}
			if doc == nil || len(doc.Elements) == 0 {
				return ErrLinkage
			}
			root := doc.Elements[0]
			if !((root.Namespace == officemetadata.CoreNamespace && root.LocalName == "coreProperties") || (root.Namespace == officemetadata.AppNamespace && root.LocalName == "Properties")) {
				return ErrLinkage
			}
			gaps := map[int64]bool{}
			for _, ref := range outcome.DiagnosticRefs {
				diagnostic := trace.Diagnostics[ref]
				switch diagnostic.Code {
				case "metadata.property_unassessed", "metadata.standard_property_unassessed", "metadata.structured_value_unassessed":
				default:
					continue
				}
				scope := diagnostic.Scope
				if diagnostic.ExecutionRef == nil || *diagnostic.ExecutionRef != outcome.ExecutionRef || scope == nil || scope.Unit != "byte" || scope.ArtifactRef != part.ArtifactRef || len(scope.Regions) != 1 {
					return ErrLinkage
				}
				matched := false
				for _, element := range doc.Elements {
					if element.Parent == nil || *element.Parent != 0 {
						continue
					}
					span := Span{int64(scope.Regions[0].Start), int64(scope.Regions[0].End)}
					if element.Span == span {
						if gaps[element.Index] || retained[part.PartRef][element.Index] || !spansCovered([]Span{span}, outcome.Excluded) {
							return ErrLinkage
						}
						gaps[element.Index] = true
						matched = true
						break
					}
				}
				if !matched {
					return ErrLinkage
				}
			}
			for _, element := range doc.Elements {
				if element.Parent != nil && *element.Parent == 0 && !retained[part.PartRef][element.Index] && !gaps[element.Index] {
					return ErrLinkage
				}
			}
		}
	}
	return nil
}
