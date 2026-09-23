package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"slices"
)

// ValidateOutcomes binds retained per-part outcomes to their parent operation.
// Child failure is allowed under partial coverage, never under completed coverage.
func (x *Index) ValidateOutcomes(e Evidence, t *TraceIndex) error {
	return x.validateOutcomes(e, t, true)
}

// ValidateOutcomeLinks checks intermediate operation links only. Complete report
// encoding and import always call ValidateOutcomes.
func (x *Index) ValidateOutcomeLinks(e Evidence, t *TraceIndex) error {
	return x.validateOutcomes(e, t, false)
}
func (x *Index) validateOutcomes(e Evidence, t *TraceIndex, complete bool) error {
	check := func(o Outcome) error {
		ex, ok := t.Executions[o.ExecutionRef]
		if !ok || ex.CapabilityRef != o.Operation {
			return ErrLinkage
		}
		if ex.State == v2.Completed && o.State != "completed" {
			return ErrLinkage
		}
		if (ex.State == v2.NotRun || ex.State == v2.Failed || ex.State == v2.Canceled) && len(o.Assessed) > 0 {
			return ErrLinkage
		}
		if (o.State == "not_run" || o.State == "failed" || o.State == "canceled") && len(o.Assessed) > 0 {
			return ErrLinkage
		}
		if o.State == "completed" && len(o.Excluded) > 0 {
			return ErrLinkage
		}
		var part *Part
		if o.PartRef != nil {
			p, ok := x.Parts[*o.PartRef]
			if !ok {
				return ErrLinkage
			}
			part = &p
		}
		for _, regions := range [][]Span{o.Assessed, o.Excluded} {
			for _, r := range regions {
				if part == nil || part.ByteLength == nil || !r.Within(*part.ByteLength) {
					return ErrLinkage
				}
			}
		}
		// A byte cannot be both assessed and explicitly excluded. Sorting a
		// copy keeps validation independent of producer ordering and mutation.
		regions := append(slices.Clone(o.Assessed), o.Excluded...)
		slices.SortFunc(regions, func(a, b Span) int {
			if a.Start < b.Start {
				return -1
			}
			if a.Start > b.Start {
				return 1
			}
			return 0
		})
		var end int64
		for _, r := range regions {
			if r.Start == r.End {
				continue
			}
			if r.Start < end {
				return ErrLinkage
			}
			end = r.End
		}
		seen := map[string]bool{}
		for _, ref := range o.DiagnosticRefs {
			d, ok := t.Diagnostics[ref]
			if !ok || seen[ref] || d.ExecutionRef == nil || *d.ExecutionRef != o.ExecutionRef || !slices.Contains(o.Codes, string(d.Code)) {
				return ErrLinkage
			}
			seen[ref] = true
		}
		return nil
	}
	produced := map[[2]string][]Outcome{}
	for _, p := range e.Packages {
		decompressed := map[string]bool{}
		seen := map[[2]string]bool{}
		for _, o := range p.Outcomes {
			if err := check(o); err != nil {
				return err
			}
			if o.PartRef == nil {
				continue
			}
			part := x.Parts[*o.PartRef]
			key := [2]string{o.ExecutionRef, *o.PartRef}
			if part.PackageRef != p.PackageRef || seen[key] {
				return ErrLinkage
			}
			seen[key] = true
			parent := t.Executions[o.ExecutionRef]
			if (o.State == "completed" || o.State == "partial") &&
				(parent.State == v2.Completed || parent.State == v2.Partial) {
				key := [2]string{o.Operation, *o.PartRef}
				produced[key] = append(produced[key], o)
			}
			if o.Operation == "aletharsis.parse.office_package" {
				if decompressed[*o.PartRef] || o.State != part.State {
					return ErrLinkage
				}
				if o.State == "completed" {
					if part.ByteLength == nil {
						return ErrLinkage
					}
					regions := slices.Clone(o.Assessed)
					slices.SortFunc(regions, func(a, b Span) int {
						if a.Start < b.Start {
							return -1
						}
						if a.Start > b.Start {
							return 1
						}
						return 0
					})
					var end int64
					for _, region := range regions {
						if region.Start == region.End {
							continue
						}
						if region.Start != end {
							return ErrLinkage
						}
						end = region.End
					}
					if end != *part.ByteLength {
						return ErrLinkage
					}
				}
				decompressed[*o.PartRef] = true
			}
		}
		for _, part := range p.Parts {
			if !decompressed[part.PartRef] || (p.State == "completed" && part.State != "completed") {
				return ErrLinkage
			}
		}
	}
	if complete {
		// Successful executions may contribute separate assessed regions to one
		// producer inventory. Normalize once per operation/part, then binary-search
		// each wanted span; record order cannot trigger a quadratic scan.
		coverage := map[[2]string][]Span{}
		for key, outcomes := range produced {
			regions, excluded := []Span{}, []Span{}
			for _, outcome := range outcomes {
				regions = append(regions, outcome.Assessed...)
				excluded = append(excluded, outcome.Excluded...)
			}
			coverage[key] = normalizedSpans(regions)
			if normalizedSpansOverlap(coverage[key], normalizedSpans(excluded)) {
				return ErrLinkage
			}
		}
		covers := func(operation, part string, spans []Span) bool {
			key := [2]string{operation, part}
			_, exists := produced[key]
			return exists && coveredByNormalized(spans, coverage[key])
		}
		origins := func(values []Origin) []Span {
			spans := make([]Span, 0, len(values))
			for _, value := range values {
				spans = append(spans, value.Source)
			}
			return spans
		}
		for _, scope := range e.Scopes {
			if !covers("aletharsis.office.text", scope.PartRef, origins(scope.Origins)) {
				return ErrLinkage
			}
		}
		for _, item := range e.Metadata {
			doc := x.XML[item.XMLRef]
			if item.Element < 0 || item.Element >= int64(len(doc.Elements)) {
				return ErrLinkage
			}
			spans := origins(item.ValueOrigins)
			if !covers("aletharsis.office.metadata", item.PartRef, spans) {
				return ErrLinkage
			}
		}
		for _, item := range e.Relationships {
			if item.Location.Span == nil || !covers("aletharsis.office.relationships", item.Location.PartRef, []Span{*item.Location.Span}) {
				return ErrLinkage
			}
		}
		// XML mapping may retain lexical structure for excluded analytical regions.
		// Bind the map to a parsing operation without claiming those regions were
		// assessed for text, metadata, or relationship interpretation.
		for _, doc := range e.XML {
			matched := false
			for _, operation := range []string{"aletharsis.office.identify", "aletharsis.office.text", "aletharsis.office.metadata", "aletharsis.office.relationships"} {
				if slices.ContainsFunc(coverage[[2]string{operation, doc.PartRef}], func(span Span) bool { return span.Start < span.End }) {
					matched = true
					break
				}
			}
			if !matched {
				return ErrLinkage
			}
		}
	}
	for _, o := range e.Objects {
		if err := check(o.Inspection); err != nil {
			return err
		}
	}
	checkIssue := func(issue Issue) error {
		if issue.PartRef != nil {
			if _, ok := x.Parts[*issue.PartRef]; !ok {
				return ErrLinkage
			}
		}
		if issue.DiagnosticRef != nil {
			d, ok := t.Diagnostics[*issue.DiagnosticRef]
			if !ok || string(d.Code) != issue.Code {
				return ErrLinkage
			}
		}
		return nil
	}
	for _, p := range e.Packages {
		for _, issue := range p.Issues {
			if err := checkIssue(issue); err != nil {
				return err
			}
		}
		for _, part := range p.Parts {
			for _, issue := range part.Issues {
				if err := checkIssue(issue); err != nil {
					return err
				}
			}
		}
	}
	for _, doc := range e.XML {
		for _, issue := range doc.Issues {
			if err := checkIssue(issue); err != nil {
				return err
			}
		}
	}
	for _, s := range e.Scopes {
		for _, issue := range s.Issues {
			if err := checkIssue(issue); err != nil {
				return err
			}
		}
	}
	if complete {
		return x.validateMetadataProjection(e, t)
	}
	return nil
}

func normalizedSpans(spans []Span) []Span {
	regions := slices.Clone(spans)
	slices.SortFunc(regions, func(a, b Span) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	merged := regions[:0]
	for _, span := range regions {
		if len(merged) > 0 && span.Start <= merged[len(merged)-1].End {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, span.End)
		} else {
			merged = append(merged, span)
		}
	}
	return merged
}

func coveredByNormalized(wanted, regions []Span) bool {
	for _, span := range wanted {
		if span.Start == span.End {
			continue
		}
		i, _ := slices.BinarySearchFunc(regions, span.Start, func(r Span, start int64) int {
			if r.End < start {
				return -1
			}
			if r.End > start {
				return 1
			}
			return 0
		})
		if i < len(regions) && regions[i].End == span.Start {
			i++
		}
		if i >= len(regions) || regions[i].Start > span.Start || regions[i].End < span.End {
			return false
		}
	}
	return true
}
