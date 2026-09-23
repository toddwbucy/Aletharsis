package corpusv2

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"testing"
)

func limits() identity.Limits {
	return identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
}
func sample(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/flat-partial-timeout.json")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	canonical, err := identity.Canonicalize(raw, limits())
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"header":  map[string]any{"type": "header", "contract": "aletharsis.corpus/2", "workspace": "fixtures", "report_schema": "4.0", "limits": map[string]any{"file_input_bytes": 8 << 20, "acquisition_bytes": 16 << 20, "output_bytes": 16 << 20, "concurrency": 1, "entries": 10, "depth": 8, "path_bytes": 4096}},
		"entries": []any{map[string]any{"type": "entry", "relative_path": "a.txt", "state": "partial", "reason": "", "report": report, "report_schema": "4.0", "report_canonical_sha256": identity.ExactBytes(canonical), "highest_finding_severity": nil}},
		"summary": map[string]any{"type": "summary", "state": "partial", "reason": "", "discovery_complete": true, "entries": 1, "counts": map[string]int{"no_reported_findings": 0, "requires_review": 0, "unsupported": 0, "failed": 0, "skipped": 0, "canceled": 0, "partial": 1}, "finding_severity_counts": map[string]int{"INFO": 0, "LOW": 0, "MEDIUM": 0, "HIGH": 0}, "exit_code": 4},
	}
}
func TestCorpusCrossRecordValidation(t *testing.T) {
	d := sample(t)
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(raw, limits(), limits()); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["header"].(map[string]any)["report_schema"] = "2.0" },
		func(d map[string]any) {
			d["entries"].([]any)[0].(map[string]any)["report_canonical_sha256"] = identity.ExactBytes([]byte("wrong"))
		},
		func(d map[string]any) { d["entries"].([]any)[0].(map[string]any)["highest_finding_severity"] = "HIGH" },
		func(d map[string]any) { d["summary"].(map[string]any)["counts"].(map[string]int)["partial"] = 0 },
		func(d map[string]any) { d["summary"].(map[string]any)["exit_code"] = 0 },
	} {
		d := sample(t)
		mutate(d)
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(raw, limits(), limits()); err == nil {
			t.Fatal("accepted inconsistent corpus")
		}
	}
}
