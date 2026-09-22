package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"testing"
)

func TestCompleteOfficeReportValidation(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	r, err := decodeReportRecords(raw, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOfficeReport(r); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Report){
		func(r *Report) { r.Evidence.Office.Scopes[0].IdentitySHA256 = "bad" },
		func(r *Report) { r.Evidence.Office.Packages[0].Parts[0].ArtifactRef = "artifact/999" },
		func(r *Report) { r.Trace.Results[0].AnchorRefs = nil },
		func(r *Report) { r.Evidence.Office.XML[0].Segments = nil },
		func(r *Report) { r.Evidence.Office.Scopes[0].Origins = nil },
		func(r *Report) { r.Evidence.Office.Packages[0].Outcomes[0].ExecutionRef = "exec/999" },
		func(r *Report) { r.Summary["findings"] = 0 },
	} {
		changed, err := decodeReportRecords(raw, limits)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if validateOfficeReport(changed) == nil {
			t.Fatal("accepted damaged complete Office report")
		}
	}
}
