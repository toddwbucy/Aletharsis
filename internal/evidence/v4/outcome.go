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
	for _, p := range e.Packages {
		for _, o := range p.Outcomes {
			if err := check(o); err != nil {
				return err
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
	return nil
}
