package v4

import (
	"encoding/json"
	"reflect"
	"slices"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

func viewCategories(name string) ([]string, bool) {
	switch name {
	case "audit":
		return []string{}, true
	case "unicode":
		return []string{"possible_steganography", "unicode"}, true
	case "metadata":
		return []string{"identifier", "metadata", "provenance"}, true
	case "structure":
		return []string{"document_structure", "embedded_content", "hidden_content", "visual_watermark"}, true
	default:
		return nil, false
	}
}
func validateOfficeFindings(r Report, trace *TraceIndex, office *Index) error {
	categories, ok := viewCategories(r.View.Name)
	if !ok || !reflect.DeepEqual(categories, r.View.FindingCategories) {
		return ErrLinkage
	}
	positive := map[string]bool{}
	for _, result := range r.Trace.Results {
		if result.Payload.Outcome == "observations_present" {
			positive[result.ExecutionRef] = true
		}
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		if seen[f.Ref] {
			return ErrLinkage
		}
		seen[f.Ref] = true
		execution, ok := trace.Executions[f.ExecutionRef]
		if !ok {
			return ErrLinkage
		}
		capability := trace.Capabilities[execution.CapabilityRef]
		if !reflect.DeepEqual(f.Mechanism, capability.Mechanism) {
			return ErrLinkage
		}
		if r.View.Name != "audit" && f.Category != "parser" && !slices.Contains(categories, f.Category) {
			return ErrLinkage
		}
		anchors := map[string]bool{}
		for _, ref := range f.AnchorRefs {
			anchor, ok := trace.Anchors[ref]
			if !ok || anchors[ref] || anchor.ExecutionRef == nil || *anchor.ExecutionRef != execution.Ref {
				return ErrLinkage
			}
			anchors[ref] = true
		}
		if f.ID == "parser.failure" {
			code, ok := f.Evidence["failure_code"].(string)
			if !ok || execution.State != v2.Failed || (capability.Role != v2.Acquisition && capability.Role != v2.Parser) {
				return ErrLinkage
			}
			matched := false
			for _, ref := range execution.DiagnosticRefs {
				matched = matched || string(trace.Diagnostics[ref].Code) == code
			}
			if !matched {
				return ErrLinkage
			}
			continue
		}
		if (execution.State != v2.Completed && execution.State != v2.Partial) || capability.Role != v2.Analyzer || !positive[execution.Ref] {
			return ErrLinkage
		}
		if err := ValidateFindingCoordinates(f.Finding, office.Scopes); err != nil {
			return err
		}
		scope := f.Location["scope_ref"].(string)
		if len(f.AnchorRefs) == 0 {
			return ErrLinkage
		}
		for _, ref := range f.AnchorRefs {
			anchor := trace.Anchors[ref]
			var locator ScopeLocation
			if anchor.Kind != "office_scope" || json.Unmarshal(anchor.Locator, &locator) != nil || locator.ScopeRef != scope {
				return ErrLinkage
			}
			// The result that establishes a positive observation must cover this exact
			// artifact, not merely another scope scanned by the same capability.
			matched := false
			for _, result := range r.Trace.Results {
				if result.ExecutionRef == execution.Ref && result.Payload.Outcome == "observations_present" &&
					result.Payload.Scope.ArtifactRef == anchor.ArtifactRef && slices.Contains(result.AnchorRefs, ref) {
					matched = true
				}
			}
			if !matched {
				return ErrLinkage
			}
		}
	}
	return nil
}
