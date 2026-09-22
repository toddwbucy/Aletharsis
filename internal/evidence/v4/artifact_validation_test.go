package v4

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"testing"
)

func TestFullArtifactGraphRejectsIdentityAndMappingMutations(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/flat-structural-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	office := &Index{Packages: map[string]Package{}, Parts: map[string]Part{}, Scopes: map[string]Scope{}}
	for _, mutate := range []func(*Report){
		func(r *Report) { r.Artifacts[0].SHA256 = str(identity.ExactBytes([]byte("different"))) },
		func(r *Report) { r.Artifacts[1].SHA256 = str(identity.ExactBytes([]byte("different"))) },
		func(r *Report) { r.Artifacts[1].Parents = []string{"artifact/999"} },
		func(r *Report) { r.Artifacts = append(r.Artifacts, r.Artifacts[0]) },
		func(r *Report) { r.Artifacts[1].ContentRef.Pointer = str("/evidence/texts/00/text") },
		func(r *Report) {
			var m map[string]any
			if err := json.Unmarshal(r.Artifacts[1].Mapping, &m); err != nil {
				t.Fatal(err)
			}
			m["from_artifact_ref"] = "artifact/999"
			r.Artifacts[1].Mapping, _ = json.Marshal(m)
		},
	} {
		r, err := decodeReportRecords(raw, limits)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&r)
		if _, err := validateArtifacts(r, office); err == nil {
			t.Fatal("accepted damaged artifact graph")
		}
	}
}
