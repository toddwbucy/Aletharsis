package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"slices"
)

// ValidateOutcomes binds retained per-part outcomes to their parent operation.
// Child failure is allowed under partial coverage, never under completed coverage.
func (x *Index) ValidateOutcomes(e Evidence, t *TraceIndex) error {
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
	produced := map[[2]string]bool{}
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
				produced[[2]string{o.Operation, *o.PartRef}] = true
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
	// Retained extraction records require an actual successful per-part
	// operation; package decompression alone cannot authorize their presence.
	for _, scope := range e.Scopes {
		if !produced[[2]string{"aletharsis.office.text", scope.PartRef}] {
			return ErrLinkage
		}
	}
	for _, item := range e.Metadata {
		if !produced[[2]string{"aletharsis.office.metadata", item.PartRef}] {
			return ErrLinkage
		}
	}
	for _, item := range e.Relationships {
		if !produced[[2]string{"aletharsis.office.relationships", item.Location.PartRef}] {
			return ErrLinkage
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
	return nil
}
