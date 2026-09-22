package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func TestOfficeScalarCoverageUsesUTF8Bytes(t *testing.T) {
	size := uint64(5)
	x := coverageIndex{artifacts: map[string]Artifact{"artifact/0": {Ref: "artifact/0", ByteLength: &size, ContentRef: &ContentRef{Kind: "office_scope", ScopeRef: str("office-scope/0")}}}, office: &Index{Scopes: map[string]Scope{"office-scope/0": {Text: "😀A"}}}}
	s := v2.Scope{ArtifactRef: "artifact/0", Unit: "scalar", Regions: []identity.Region{{Start: 0, End: 1}}}
	got, err := x.intervals(s, true)
	if err != nil || len(got) != 1 || got[0] != (identity.Region{Start: 0, End: 4}) {
		t.Fatal(got, err)
	}
	s.Regions[0].End = 3
	if _, err := x.intervals(s, true); err == nil {
		t.Fatal("accepted scalar past EOF")
	}
	s.Unit = "byte"
	s.Regions[0].End = 6
	if _, err := x.intervals(s, true); err == nil {
		t.Fatal("accepted byte past EOF")
	}
}
func TestPartialCoverageRequiresCompleteAccounting(t *testing.T) {
	size := uint64(5)
	a := Artifact{Ref: "artifact/0", ByteLength: &size, ContentRef: &ContentRef{Kind: "office_scope", ScopeRef: str("office-scope/0")}}
	office := &Index{Scopes: map[string]Scope{"office-scope/0": {Text: "😀A"}}}
	tcase := NativeTrace{Executions: []v2.Execution{{State: v2.Partial, RequestedScope: v2.Scope{ArtifactRef: a.Ref, Unit: "whole_artifact"},
		AnalyzedScope: []v2.Scope{{ArtifactRef: a.Ref, Unit: "scalar", Regions: []identity.Region{{Start: 0, End: 1}}}},
		Exclusions:    []v2.Exclusion{{Scope: &v2.Scope{ArtifactRef: a.Ref, Unit: "byte", Regions: []identity.Region{{Start: 4, End: 5}}}}}}}}
	if err := ValidateCoverage(tcase, []Artifact{a}, office, evidence.Document{}); err != nil {
		t.Fatal(err)
	}
	tcase.Executions[0].Exclusions[0].Scope.Regions[0].Start = 3
	if ValidateCoverage(tcase, []Artifact{a}, office, evidence.Document{}) == nil {
		t.Fatal("accepted cross-unit overlap")
	}
	tcase.Executions[0].Exclusions = append(tcase.Executions[0].Exclusions, v2.Exclusion{UnknownRemainder: true})
	if ValidateCoverage(tcase, []Artifact{a}, office, evidence.Document{}) == nil {
		t.Fatal("unknown remainder concealed cross-unit overlap")
	}
	tcase.Executions[0].Exclusions = tcase.Executions[0].Exclusions[:1]
	tcase.Executions[0].Exclusions[0].Scope.Regions[0].Start = 5
	if ValidateCoverage(tcase, []Artifact{a}, office, evidence.Document{}) == nil {
		t.Fatal("accepted unaccounted byte")
	}
}
