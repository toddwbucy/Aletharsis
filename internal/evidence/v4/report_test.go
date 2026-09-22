package v4

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestCompleteFlatReportRecordDecoding(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts_v4/fixtures/flat-*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight complete flat fixtures required", err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := decodeReportRecords(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			if r.Schema != "4.0" || len(r.AdapterRuns) != 0 {
				t.Fatal("incorrect native view")
			}
			if _, err := IndexTrace(r.Trace); err != nil {
				t.Fatal(err)
			}
			if len(r.Evidence.Office.Packages) != 0 {
				t.Fatal("flat report gained Office data")
			}
			var bad map[string]any
			if err := json.Unmarshal(raw, &bad); err != nil {
				t.Fatal(err)
			}
			bad["undeclared"] = true
			changed, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeReportRecords(changed, limits); err == nil {
				t.Fatal("accepted unknown top-level field")
			}
		})
	}
}
