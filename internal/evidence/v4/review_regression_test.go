package v4

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestAdjacentRequestedCoverage(t *testing.T) {
	size := uint64(10)
	requested := v2.Scope{ArtifactRef: "artifact/0", Unit: "byte", Regions: []identity.Region{{Start: 0, End: 5}, {Start: 5, End: 10}}}
	trace := NativeTrace{Executions: []v2.Execution{{State: v2.Completed, RequestedScope: requested, AnalyzedScope: []v2.Scope{requested}}}}
	artifacts := []Artifact{{Ref: "artifact/0", ByteLength: &size}}
	if err := ValidateCoverage(trace, artifacts, &Index{}, evidence.Document{}); err != nil {
		t.Fatal(err)
	}
	// Adjacent regions merge; a genuine hole must not be bridged.
	trace.Executions[0].RequestedScope.Regions = []identity.Region{{Start: 0, End: 4}, {Start: 5, End: 10}}
	if ValidateCoverage(trace, artifacts, &Index{}, evidence.Document{}) == nil {
		t.Fatal("coverage bridged an unrequested gap")
	}
}
func TestRawMessageProducerBudgetCountsJSONNotBytes(t *testing.T) {
	limits := identity.Limits{InputBytes: 8192, OutputBytes: 8192, Nodes: 16, Depth: 8}
	for _, tc := range []struct {
		raw   json.RawMessage
		valid bool
	}{
		{json.RawMessage(`{"text":"` + strings.Repeat("x", 4030) + `"}`), true},
		{json.RawMessage(`{"text":`), false},
		{json.RawMessage(`[` + strings.Repeat(`0,`, 20) + `0]`), false},
	} {
		nodes, size := 0, 0
		err := checkValue(reflect.ValueOf(tc.raw), 0, &nodes, &size, limits)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v err=%v", tc.valid, err)
		}
	}
}
func TestXMLSingleRootIndependentGuard(t *testing.T) {
	hash := strings.Repeat("a", 64)
	size := int64(8)
	part := Part{PartRef: "office-part/0", SHA256: &hash, ByteLength: &size}
	x := XML{PartRef: part.PartRef, Elements: []Element{{Index: 0, Span: Span{0, 4}}}, Tokens: []Token{
		{Index: 0, Kind: "start", Element: wide(0), Span: Span{0, 4}},
		{Index: 1, Kind: "end", Element: wide(0), Span: Span{4, 4}},
	}, Segments: []Segment{}, Controls: []Control{}}
	if err := ValidateXML(x, part); err != nil {
		t.Fatal("valid single root", err)
	}
	// Two individually coherent token trees; only the single-document-root
	// invariant rejects this shape. No metadata guard can mask the experiment.
	x.Elements = append(x.Elements, Element{Index: 1, Span: Span{4, 8}})
	x.Tokens = append(x.Tokens, Token{Index: 2, Kind: "start", Element: wide(1), Span: Span{4, 8}}, Token{Index: 3, Kind: "end", Element: wide(1), Span: Span{8, 8}})
	if err := ValidateXML(x, part); err == nil {
		t.Fatalf("xml-root: %v", err)
	}
}
func TestMutationNullDiscoveryForms(t *testing.T) {
	for _, schema := range []any{
		map[string]any{"enum": []any{nil, "x"}}, map[string]any{"const": nil},
		map[string]any{"if": map[string]any{"type": "null"}},
		map[string]any{"then": map[string]any{"type": []any{"null", "string"}}},
		map[string]any{"else": map[string]any{"enum": []any{nil, "x"}}},
	} {
		found := false
		for _, m := range generateMutations("x", schema) {
			found = found || m.op == "null"
		}
		if !found {
			t.Fatalf("nullable schema missed: %v", schema)
		}
	}
}
