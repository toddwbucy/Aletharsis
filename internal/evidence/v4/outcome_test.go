package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
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

func TestPackageCoverageCannotOmitOrContradictPartOutcomes(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	mutations := map[string]func(*Report){
		"completed without assessed bytes":          func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].Assessed = []Span{} },
		"completed with leading gap":                func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].Assessed[0].Start = 1 },
		"completed with trailing gap":               func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].Assessed[0].End-- },
		"all outcomes omitted":                      func(r *Report) { r.Evidence.Office.Packages[0].Outcomes = []Outcome{} },
		"one part omitted":                          func(r *Report) { p := &r.Evidence.Office.Packages[0]; p.Outcomes = p.Outcomes[1:] },
		"duplicate part outcome":                    func(r *Report) { p := &r.Evidence.Office.Packages[0]; p.Outcomes = append(p.Outcomes, p.Outcomes[0]) },
		"another operation conceals parse omission": func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].Operation = "aletharsis.office.metadata" },
		"excluded completed bytes":                  func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].Excluded = []Span{{0, 1}} },
		"overlapping assessed bytes": func(r *Report) {
			p := &r.Evidence.Office.Packages[0]
			p.Outcomes[0].Assessed = append(p.Outcomes[0].Assessed, Span{0, 1})
		},
		"failed child assessed": func(r *Report) {
			r.Evidence.Office.Packages[0].Outcomes[0].State = "failed"
			r.Trace.Executions[1].State = v2.Partial
		},
	}
	// Coverage may be split and unordered; only its exact union matters.
	complete, err := DecodeReport(raw, limits)
	if err != nil {
		t.Fatal(err)
	}
	outcome := &complete.Evidence.Office.Packages[0].Outcomes[0]
	end := outcome.Assessed[0].End
	outcome.Assessed = []Span{{1, end}, {0, 1}}
	if _, err := complete.Encode(limits); err != nil {
		t.Fatal("rejected complete partition", err)
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&r)
			if _, err := r.Encode(limits); err == nil {
				t.Fatal("accepted incomplete or contradictory part coverage")
			}
		})
	}
}

func TestEveryRetainedExtractionRequiresSuccessfulPartOperation(t *testing.T) {
	for _, operation := range []string{"aletharsis.office.text", "aletharsis.office.metadata", "aletharsis.office.relationships"} {
		for _, state := range []v2.State{v2.Completed, v2.Partial, v2.Failed, v2.Canceled, v2.NotRun} {
			t.Run(operation+"/"+string(state), func(t *testing.T) {
				part := "office-part/0"
				e := Evidence{Packages: []Package{{Outcomes: []Outcome{{Operation: operation, ExecutionRef: "exec/0", PartRef: &part, State: string(state)}}}}}
				switch operation {
				case "aletharsis.office.text":
					e.Scopes = []Scope{{PartRef: part}}
				case "aletharsis.office.metadata":
					e.Metadata = []Metadata{{PartRef: part, XMLRef: "xml"}}
					e.XML = []XML{{PartRef: part, Elements: []Element{{Namespace: "http://schemas.openxmlformats.org/package/2006/metadata/core-properties", LocalName: "coreProperties"}}}}
				case "aletharsis.office.relationships":
					e.Relationships = []Relationship{{Location: StructuralLocation{PartRef: part, Span: &Span{}}}}
				}
				x := Index{Parts: map[string]Part{part: {PartRef: part}}, XML: map[string]XML{"xml": {Elements: []Element{{}}}}}
				trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {Ref: "exec/0", CapabilityRef: operation, State: state}}}
				good := state == v2.Completed || state == v2.Partial
				if err := x.ValidateOutcomes(e, &trace); (err == nil) != good {
					t.Fatalf("state %s: %v", state, err)
				}
				e.Packages[0].Outcomes = nil
				if x.ValidateOutcomes(e, &trace) == nil {
					t.Fatal("accepted retained evidence without producing outcome")
				}
			})
		}
	}
}

func TestRetainedScopeCannotEscapeAssessedCoverage(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"empty", "gap", "xml_without_producer"} {
		t.Run(mode, func(t *testing.T) {
			r, err := decodeReportRecords(raw, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
			if err != nil {
				t.Fatal(err)
			}
			p := &r.Evidence.Office.Packages[0]
			for i := range p.Outcomes {
				if p.Outcomes[i].Operation == "aletharsis.office.text" {
					p.Outcomes[i].Assessed = nil
					if mode == "gap" {
						s := r.Evidence.Office.Scopes[0].Origins[0].Source
						p.Outcomes[i].Assessed = []Span{{0, s.Start}, {s.End, *p.Parts[len(p.Parts)-1].ByteLength}}
					}
					if mode == "xml_without_producer" {
						p.Outcomes = append(p.Outcomes[:i], p.Outcomes[i+1:]...)
					}
					break
				}
			}
			x, err := IndexEvidence(r.Evidence.Office, *r.File.SHA256, int64(*r.File.Size))
			if err != nil {
				t.Fatal(err)
			}
			trace, err := IndexTrace(r.Trace)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "xml_without_producer" {
				r.Evidence.Office.Scopes = nil
			}
			if x.ValidateOutcomes(r.Evidence.Office, trace) == nil {
				t.Fatal("unassessed evidence accepted")
			}
		})
	}
}

func TestCoverageLookupHandlesUnorderedRepeatedOrigins(t *testing.T) {
	regions := make([]Span, 20000)
	for i := range regions {
		regions[i] = Span{int64(i * 2), int64(i*2 + 1)}
	}
	wanted := make([]Span, 80000)
	for i := range wanted {
		wanted[i] = regions[(i*7919)%len(regions)]
	}
	if !spansCovered(wanted, regions) {
		t.Fatal("lost unordered or repeated origins")
	}
	wanted[0] = Span{1, 2}
	if spansCovered(wanted, regions) {
		t.Fatal("bridged a gap")
	}
	if !spansCovered([]Span{{0, 3}}, []Span{{2, 3}, {0, 1}, {1, 2}}) {
		t.Fatal("adjacent ranges not united")
	}
}

func TestRetainedRecordCanUseMultipleProducingExecutions(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeReport(raw, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
	if err != nil {
		t.Fatal(err)
	}
	scope := r.Evidence.Office.Scopes[0]
	split := scope.Origins[len(scope.Origins)-1].Source.Start
	pkg := &r.Evidence.Office.Packages[0]
	for i, o := range pkg.Outcomes {
		if o.Operation != "aletharsis.office.text" {
			continue
		}
		next := o
		next.ExecutionRef = "exec/99"
		end := o.Assessed[len(o.Assessed)-1].End
		pkg.Outcomes[i].Assessed = []Span{{0, split}}
		next.Assessed = []Span{{split, end}}
		pkg.Outcomes = append(pkg.Outcomes, next)
		for _, e := range r.Trace.Executions {
			if e.Ref == o.ExecutionRef {
				e.Ref = next.ExecutionRef
				r.Trace.Executions = append(r.Trace.Executions, e)
				break
			}
		}
		break
	}
	x, err := IndexEvidence(r.Evidence.Office, *r.File.SHA256, int64(*r.File.Size))
	if err != nil {
		t.Fatal(err)
	}
	trace, err := IndexTrace(r.Trace)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.ValidateOutcomes(r.Evidence.Office, trace); err != nil {
		t.Fatal("split coverage rejected", err)
	}
}
