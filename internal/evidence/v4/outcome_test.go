package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"testing"
)

func TestBadChildRequiresIncompleteParent(t *testing.T) {
	outcome := Outcome{Operation: "office.parse", ExecutionRef: "exec/0", PartRef: str("office-part/0"), State: "failed", Codes: []string{"office.bad_part"}, DiagnosticRefs: []string{"diagnostic/0"}}
	e := Evidence{Packages: []Package{{Outcomes: []Outcome{outcome}}}}
	x := Index{Parts: map[string]Part{"office-part/0": {PartRef: "office-part/0", State: "failed"}}}
	trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {Ref: "exec/0", CapabilityRef: "office.parse", State: v2.Partial}},
		Diagnostics: map[string]v2.Diagnostic{"diagnostic/0": {Ref: "diagnostic/0", ExecutionRef: str("exec/0"), Code: "office.bad_part"}}}
	if err := x.ValidateOutcomes(e, &trace); err != nil {
		t.Fatal(err)
	}
	execution := trace.Executions["exec/0"]
	execution.State = v2.Completed
	trace.Executions[execution.Ref] = execution
	if x.ValidateOutcomes(e, &trace) == nil {
		t.Fatal("completed parent concealed bad child")
	}
	execution.State = v2.Partial
	trace.Executions[execution.Ref] = execution
	e.Packages[0].Outcomes[0].ExecutionRef = "exec/999"
	if x.ValidateOutcomes(e, &trace) == nil {
		t.Fatal("accepted dangling parent")
	}
	e.Packages[0].Outcomes[0] = outcome
	e.Packages[0].Outcomes[0].Assessed = []Span{{0, 1}}
	if x.ValidateOutcomes(e, &trace) == nil {
		t.Fatal("claimed assessed bytes from failed decompression")
	}
}
func TestOfficeInventoryRemainsUsableOnCancellation(t *testing.T) {
	trace := NativeTrace{Executions: []v2.Execution{{Ref: "exec/0", CapabilityRef: "office.parse", State: v2.Canceled}}}
	x := TraceIndex{Capabilities: map[string]v2.Capability{"office.parse": {Role: v2.Parser, Participation: v2.Required}}}
	if aggregateNativeStatus(trace, &x, Evidence{}) != v2.Canceled {
		t.Fatal("empty cancellation changed")
	}
	office := Evidence{Packages: []Package{{Parts: []Part{{PartRef: "office-part/0"}}}}}
	if aggregateNativeStatus(trace, &x, office) != v2.Partial {
		t.Fatal("lost usable inventory on cancellation")
	}
}
