package v4

import (
	"encoding/json"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"path/filepath"
	"testing"
)

func traceFixture(t *testing.T, path string) NativeTrace {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := v2.DecodeReport(raw, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
	if err != nil {
		t.Fatal(err)
	}
	var anchors []Anchor
	encoded, err := json.Marshal(r.Anchors)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &anchors); err != nil {
		t.Fatal(err)
	}
	return NativeTrace{Capabilities: r.Capabilities, Executions: r.Executions, Diagnostics: r.Diagnostics, Results: r.Results, Anchors: anchors}
}
func TestNativeTracePreservesPublishedFixtureLinks(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight fixtures required", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			trace := traceFixture(t, path)
			if _, err := IndexTrace(trace); err != nil {
				t.Fatal(err)
			}
			trace.Executions[0].CapabilityRef = "missing"
			if _, err := IndexTrace(trace); err == nil {
				t.Fatal("accepted dangling capability")
			}
		})
	}
}
func TestTraceRejectsOwnershipAndPlanningMutations(t *testing.T) {
	const path = "../../../tests/contracts/fixtures/structural-observation.json"
	for _, mutate := range []func(*NativeTrace){
		func(t *NativeTrace) { t.Capabilities = append(t.Capabilities, t.Capabilities[0]) },
		func(t *NativeTrace) { t.Executions = append(t.Executions, t.Executions[0]) },
		func(t *NativeTrace) { t.Executions = nil },
		func(t *NativeTrace) { t.Results = nil },
		func(t *NativeTrace) { t.Results[0].ExecutionRef = "exec/999" },
		func(t *NativeTrace) { t.Results[0].Payload.Operation = "wrong" },
		func(t *NativeTrace) { t.Results[0].AnchorRefs = []string{"anchor/999"} },
		func(t *NativeTrace) { t.Results = append(t.Results, t.Results[0]) },
	} {
		trace := traceFixture(t, path)
		mutate(&trace)
		if _, err := IndexTrace(trace); err == nil {
			t.Fatal("accepted damaged trace")
		}
	}
	const failed = "../../../tests/contracts/fixtures/acquisition-failed.json"
	trace := traceFixture(t, failed)
	trace.Diagnostics[0].ExecutionRef = str("exec/999")
	if _, err := IndexTrace(trace); err == nil {
		t.Fatal("accepted mismatched diagnostic owner")
	}
}
