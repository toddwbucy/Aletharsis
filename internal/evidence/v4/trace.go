package v4

import (
	"reflect"
	"slices"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

// NativeTrace contains the unchanged native execution records. Adapter records
// must be screened before decoding this view; they have different semantics.
type NativeTrace struct {
	Capabilities []v2.Capability
	Executions   []v2.Execution
	Diagnostics  []v2.Diagnostic
	Results      []v2.Result
	Anchors      []Anchor
}
type TraceIndex struct {
	Capabilities map[string]v2.Capability
	Executions   map[string]v2.Execution
	Diagnostics  map[string]v2.Diagnostic
	Anchors      map[string]Anchor
}

// IndexTrace checks record invariants and cross-record ownership. Artifact bounds,
// analyzed/excluded coverage, findings and aggregate status are separate stages.
func IndexTrace(t NativeTrace) (*TraceIndex, error) {
	x := &TraceIndex{Capabilities: map[string]v2.Capability{}, Executions: map[string]v2.Execution{}, Diagnostics: map[string]v2.Diagnostic{}, Anchors: map[string]Anchor{}}
	for _, c := range t.Capabilities {
		if err := c.Validate(); err != nil {
			return nil, err
		}
		if _, ok := x.Capabilities[c.ID]; ok {
			return nil, ErrLinkage
		}
		x.Capabilities[c.ID] = c
	}
	for _, d := range t.Diagnostics {
		if err := d.Validate(); err != nil {
			return nil, err
		}
		if _, ok := x.Diagnostics[d.Ref]; ok {
			return nil, ErrLinkage
		}
		x.Diagnostics[d.Ref] = d
	}
	planned := map[string]bool{}
	for _, e := range t.Executions {
		if err := e.Validate(); err != nil {
			return nil, err
		}
		if _, ok := x.Executions[e.Ref]; ok {
			return nil, ErrLinkage
		}
		c, ok := x.Capabilities[e.CapabilityRef]
		if !ok {
			return nil, ErrLinkage
		}
		if c.Participation == v2.Disabled {
			if e.State != v2.NotRun || e.ReasonCode == nil || *e.ReasonCode != "execution.disabled" {
				return nil, ErrLinkage
			}
		} else if c.Availability.State != "available" {
			if e.State != v2.NotRun || e.ReasonCode == nil || c.Availability.ReasonCode == nil || *e.ReasonCode != *c.Availability.ReasonCode {
				return nil, ErrLinkage
			}
		}
		refs := map[string]bool{}
		for _, ref := range e.DiagnosticRefs {
			d, ok := x.Diagnostics[ref]
			if !ok || refs[ref] || d.ExecutionRef == nil || *d.ExecutionRef != e.Ref {
				return nil, ErrLinkage
			}
			refs[ref] = true
		}
		x.Executions[e.Ref] = e
		planned[c.ID] = true
	}
	if len(planned) != len(x.Capabilities) {
		return nil, ErrLinkage
	}
	for _, d := range t.Diagnostics {
		if d.ExecutionRef != nil {
			if _, ok := x.Executions[*d.ExecutionRef]; !ok {
				return nil, ErrLinkage
			}
		}
	}
	for _, a := range t.Anchors {
		if _, ok := x.Anchors[a.Ref]; ok {
			return nil, ErrLinkage
		}
		if a.ExecutionRef != nil {
			if _, ok := x.Executions[*a.ExecutionRef]; !ok {
				return nil, ErrLinkage
			}
		}
		x.Anchors[a.Ref] = a
	}
	resultRefs := map[string]bool{}
	resultScopes := map[string][]v2.Scope{}
	for _, r := range t.Results {
		if err := r.Validate(); err != nil {
			return nil, err
		}
		if resultRefs[r.Ref] {
			return nil, ErrLinkage
		}
		resultRefs[r.Ref] = true
		e, ok := x.Executions[r.ExecutionRef]
		if !ok {
			return nil, ErrLinkage
		}
		c := x.Capabilities[e.CapabilityRef]
		if (e.State != v2.Completed && e.State != v2.Partial) || c.Role != v2.Analyzer || c.Mechanism == nil ||
			*c.Mechanism != v2.Structural || r.Payload.Operation != c.ID {
			return nil, ErrLinkage
		}
		if !slices.ContainsFunc(e.AnalyzedScope, func(s v2.Scope) bool { return reflect.DeepEqual(s, r.Payload.Scope) }) {
			return nil, ErrLinkage
		}
		for _, ref := range r.AnchorRefs {
			a, ok := x.Anchors[ref]
			if !ok || a.ExecutionRef == nil || *a.ExecutionRef != e.Ref {
				return nil, ErrLinkage
			}
		}
		resultScopes[e.Ref] = append(resultScopes[e.Ref], r.Payload.Scope)
	}
	for _, e := range t.Executions {
		c := x.Capabilities[e.CapabilityRef]
		if (e.State == v2.Completed || e.State == v2.Partial) && c.Role == v2.Analyzer && c.Mechanism != nil && *c.Mechanism == v2.Structural {
			for _, s := range e.AnalyzedScope {
				if !slices.ContainsFunc(resultScopes[e.Ref], func(r v2.Scope) bool { return reflect.DeepEqual(s, r) }) {
					return nil, ErrLinkage
				}
			}
		}
	}
	return x, nil
}
