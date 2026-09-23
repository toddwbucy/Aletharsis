package v4

import (
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"testing"
)

func TestMetadataProjectionRequiresLocatedOmission(t *testing.T) {
	part := Part{PartRef: "office-part/0", ArtifactRef: "artifact/1"}
	doc := XML{PartRef: part.PartRef, Elements: []Element{
		{Index: 0, Namespace: officemetadata.CoreNamespace, LocalName: "coreProperties", Span: Span{0, 100}},
		{Index: 1, Parent: wide(0), LocalName: "creator", Span: Span{10, 30}},
		{Index: 2, Parent: wide(0), LocalName: "creator", Span: Span{40, 60}},
	}}
	outcome := Outcome{Operation: "aletharsis.office.metadata", ExecutionRef: "exec/0", PartRef: &part.PartRef, State: "partial", Assessed: []Span{{40, 60}}, Excluded: []Span{{10, 30}}}
	e := Evidence{XML: []XML{doc}, Metadata: []Metadata{{PartRef: part.PartRef, Element: 2}}, Packages: []Package{{Outcomes: []Outcome{outcome}}}}
	x := Index{Parts: map[string]Part{part.PartRef: part}}
	trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {CapabilityRef: "aletharsis.office.metadata", State: v2.Partial}, "exec/1": {CapabilityRef: "aletharsis.office.metadata", State: v2.Partial}}, Diagnostics: map[string]v2.Diagnostic{}}
	if x.validateMetadataProjection(e, &trace) == nil {
		t.Fatal("silently omitted sibling")
	}
	diagnostic := v2.Diagnostic{Ref: "diagnostic/0", ExecutionRef: str("exec/0"), Code: "metadata.structured_value_unassessed", Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: 10, End: 30}}}}
	trace.Diagnostics[diagnostic.Ref] = diagnostic
	e.Packages[0].Outcomes[0].DiagnosticRefs = []string{diagnostic.Ref}
	if err := x.validateMetadataProjection(e, &trace); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"wrong_part", "wrong_span", "wrong_execution", "retained_and_omitted", "not_excluded"} {
		t.Run(mode, func(t *testing.T) {
			d := diagnostic
			scope := *d.Scope
			d.Scope = &scope
			switch mode {
			case "wrong_part":
				d.Scope.ArtifactRef = "artifact/99"
			case "wrong_span":
				d.Scope.Regions = []identity.Region{{Start: 11, End: 30}}
			case "wrong_execution":
				d.ExecutionRef = str("exec/99")
			case "retained_and_omitted":
				d.Scope.Regions = []identity.Region{{Start: 40, End: 60}}
			case "not_excluded":
				e.Packages[0].Outcomes[0].Excluded = nil
			}
			trace.Diagnostics[d.Ref] = d
			if x.validateMetadataProjection(e, &trace) == nil {
				t.Fatal("accepted unbound omission")
			}
		})
	}
}

func TestMetadataProjectionUnionsSeparateExecutionOmissions(t *testing.T) {
	part := Part{PartRef: "office-part/0", ArtifactRef: "artifact/1"}
	doc := XML{PartRef: part.PartRef, Elements: []Element{
		{Index: 0, Namespace: officemetadata.CoreNamespace, LocalName: "coreProperties", Span: Span{0, 100}},
		{Index: 1, Parent: wide(0), Span: Span{10, 30}},
		{Index: 2, Parent: wide(0), Span: Span{40, 60}},
	}}
	e := Evidence{XML: []XML{doc}, Packages: []Package{{Outcomes: []Outcome{}}}}
	trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {CapabilityRef: "aletharsis.office.metadata", State: v2.Partial}, "exec/1": {CapabilityRef: "aletharsis.office.metadata", State: v2.Partial}}, Diagnostics: map[string]v2.Diagnostic{}}
	for i, span := range []Span{{10, 30}, {40, 60}} {
		exec, ref := []string{"exec/0", "exec/1"}[i], []string{"diagnostic/0", "diagnostic/1"}[i]
		e.Packages[0].Outcomes = append(e.Packages[0].Outcomes, Outcome{Operation: "aletharsis.office.metadata", ExecutionRef: exec, PartRef: &part.PartRef, State: "partial", Excluded: []Span{span}, DiagnosticRefs: []string{ref}})
		trace.Diagnostics[ref] = v2.Diagnostic{Ref: ref, ExecutionRef: &exec, Code: "metadata.structured_value_unassessed", Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(span.Start), End: uint64(span.End)}}}}
	}
	x := Index{Parts: map[string]Part{part.PartRef: part}}
	if err := x.validateMetadataProjection(e, &trace); err != nil {
		t.Fatal("split metadata assessment rejected", err)
	}
	e.Packages[0].Outcomes = e.Packages[0].Outcomes[:1]
	if x.validateMetadataProjection(e, &trace) == nil {
		t.Fatal("unaccounted property accepted")
	}
}

func TestMetadataOmissionCannotBorrowAnotherExecutionAuthority(t *testing.T) {
	for _, mode := range []string{"valid partial pair", "completed sibling", "partially overlapping sibling", "failed parent", "canceled parent", "unrun parent", "borrowed exclusion", "borrowed diagnostic", "duplicate omission", "coincident properties", "empty property", "completed omission"} {
		t.Run(mode, func(t *testing.T) {
			part := Part{PartRef: "office-part/0", ArtifactRef: "artifact/1"}
			doc := XML{PartRef: part.PartRef, Elements: []Element{{Index: 0, Namespace: officemetadata.CoreNamespace, LocalName: "coreProperties", Span: Span{0, 100}}, {Index: 1, Parent: wide(0), Span: Span{10, 30}}, {Index: 2, Parent: wide(0), Span: Span{40, 60}}}}
			a := Outcome{Operation: "aletharsis.office.metadata", ExecutionRef: "exec/0", PartRef: &part.PartRef, State: "partial", Excluded: []Span{{10, 30}}, DiagnosticRefs: []string{"diagnostic/0"}}
			b := Outcome{Operation: a.Operation, ExecutionRef: "exec/1", PartRef: &part.PartRef, State: "partial", Assessed: []Span{{40, 60}}}
			d := v2.Diagnostic{Ref: "diagnostic/0", ExecutionRef: &a.ExecutionRef, Code: "metadata.structured_value_unassessed", Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: 10, End: 30}}}}
			trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {CapabilityRef: a.Operation, State: v2.Partial}, "exec/1": {CapabilityRef: a.Operation, State: v2.Partial}}, Diagnostics: map[string]v2.Diagnostic{d.Ref: d}}
			switch mode {
			case "coincident properties":
				doc.Elements[1].Span = doc.Elements[2].Span
				a.DiagnosticRefs = nil
				a.Excluded = nil
			case "empty property":
				doc.Elements[1].Span = Span{50, 50}
				d.Scope.Regions = []identity.Region{{Start: 50, End: 50}}
				trace.Diagnostics[d.Ref] = d
				a.Excluded = nil
			case "completed omission":
				a.State = "completed"
			case "duplicate omission":
				b.Excluded = a.Excluded
				b.DiagnosticRefs = []string{"diagnostic/1"}
				d.Ref = "diagnostic/1"
				d.ExecutionRef = &b.ExecutionRef
				trace.Diagnostics[d.Ref] = d
			case "completed sibling":
				b.State = "completed"
				b.Assessed = []Span{{0, 100}}
				trace.Executions[b.ExecutionRef] = v2.Execution{CapabilityRef: b.Operation, State: v2.Completed}
			case "partially overlapping sibling":
				b.Assessed = []Span{{29, 60}}
			case "failed parent", "canceled parent", "unrun parent":
				state := map[string]v2.State{"failed parent": v2.Failed, "canceled parent": v2.Canceled, "unrun parent": v2.NotRun}[mode]
				trace.Executions[a.ExecutionRef] = v2.Execution{CapabilityRef: a.Operation, State: state}
			case "borrowed exclusion":
				a.Excluded = nil
				b.Excluded = []Span{{10, 30}}
			case "borrowed diagnostic":
				a.DiagnosticRefs = nil
				b.DiagnosticRefs = []string{d.Ref}
				d.ExecutionRef = &b.ExecutionRef
				trace.Diagnostics[d.Ref] = d
			}
			e := Evidence{XML: []XML{doc}, Metadata: []Metadata{{PartRef: part.PartRef, Element: 2}}, Packages: []Package{{Outcomes: []Outcome{a, b}}}}
			x := Index{Parts: map[string]Part{part.PartRef: part}}
			err := x.validateMetadataProjection(e, &trace)
			if (err == nil) != (mode == "valid partial pair") {
				t.Fatal(mode, err)
			}
		})
	}
}
