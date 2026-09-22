package v4

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"testing"
)

func findingGraphCase() (Report, *TraceIndex, *Index) {
	mechanism := v2.Structural
	locator, _ := json.Marshal(ScopeLocation{Kind: "office_scope", ScopeRef: "office-scope/0"})
	anchor := Anchor{Ref: "anchor/0", Kind: "office_scope", ArtifactRef: "artifact/2", ExecutionRef: str("exec/0"), Locator: locator}
	trace := &TraceIndex{Capabilities: map[string]v2.Capability{"analyze": {ID: "analyze", Role: v2.Analyzer, Mechanism: &mechanism}},
		Executions: map[string]v2.Execution{"exec/0": {Ref: "exec/0", CapabilityRef: "analyze", State: v2.Completed}}, Anchors: map[string]Anchor{anchor.Ref: anchor}}
	office := &Index{Scopes: map[string]Scope{"office-scope/0": {ScopeRef: "office-scope/0", Text: "\u200b"}}}
	report := Report{View: v2.View{Name: "audit", FindingCategories: []string{}}, Findings: []v2.Finding{{Ref: "finding/0", ExecutionRef: "exec/0", Mechanism: &mechanism, AnchorRefs: []string{anchor.Ref},
		Finding: evidence.Finding{ID: "unicode.zero_width", Category: "unicode", Evidence: evidence.Object{"count": 1}, Location: evidence.Object{"kind": "office_offsets", "scope_ref": "office-scope/0", "scope_character_offsets": []int{0}, "scope_byte_offsets": []int{0}}}}},
		Trace: NativeTrace{Results: []v2.Result{{ExecutionRef: "exec/0", AnchorRefs: []string{anchor.Ref}, Payload: v2.StructuralPayload{Outcome: "observations_present", Scope: v2.Scope{ArtifactRef: anchor.ArtifactRef}}}}}}
	return report, trace, office
}
func TestFindingRequiresMatchingPositiveResultAndScopeAnchor(t *testing.T) {
	r, tr, of := findingGraphCase()
	if err := validateOfficeFindings(r, tr, of); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Report, *TraceIndex){
		func(r *Report, tr *TraceIndex) { r.Findings[0].AnchorRefs = nil },
		func(r *Report, tr *TraceIndex) { r.Findings[0].ExecutionRef = "exec/99" },
		func(r *Report, tr *TraceIndex) { r.Trace.Results[0].Payload.Outcome = "no_observations" },
		func(r *Report, tr *TraceIndex) { r.Trace.Results[0].Payload.Scope.ArtifactRef = "artifact/99" },
		func(r *Report, tr *TraceIndex) { r.Trace.Results[0].AnchorRefs = nil },
		func(r *Report, tr *TraceIndex) { r.Findings = append(r.Findings, r.Findings[0]) },
		func(r *Report, tr *TraceIndex) {
			a := tr.Anchors["anchor/0"]
			a.Locator = json.RawMessage(`{"kind":"office_scope","scope_ref":"office-scope/1"}`)
			tr.Anchors[a.Ref] = a
		},
		func(r *Report, tr *TraceIndex) {
			r.View = v2.View{Name: "metadata", FindingCategories: []string{"identifier", "metadata", "provenance"}}
		},
	} {
		r, tr, of := findingGraphCase()
		mutate(&r, tr)
		if validateOfficeFindings(r, tr, of) == nil {
			t.Fatal("accepted disconnected Office finding")
		}
	}
}
