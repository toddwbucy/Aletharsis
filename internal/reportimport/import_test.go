package reportimport_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/reportimport"
)

func limits() identity.Limits {
	return identity.Limits{InputBytes: 8 << 20, OutputBytes: 8 << 20, Nodes: 500000, Depth: 64}
}
func TestAllLegacyReportsRemainUnknownCoverage(t *testing.T) {
	paths, err := filepath.Glob("../../reference/python-behavior/reports/*.json")
	if err != nil || len(paths) != 37 {
		t.Fatal("37 frozen reports required", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := reportimport.Read(raw, limits())
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "legacy" || got.Coverage != "unknown" || got.V2 != nil || got.ReportArtifactSHA256 != identity.ExactBytes(raw) || !bytes.Equal(got.Bytes(), raw) {
				t.Fatal("legacy report reinterpreted")
			}
			var r struct {
				Findings []struct {
					ID string `json:"id"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			failures := 0
			for _, f := range r.Findings {
				if f.ID == "parser.failure" {
					failures++
				}
			}
			if len(got.AdapterDiagnostics) != failures {
				t.Fatal("missing legacy diagnostic")
			}
			for _, d := range got.AdapterDiagnostics {
				if d.FailureCode != nil || d.Code != "legacy.failure_code_unavailable" {
					t.Fatal("inferred historical code")
				}
			}
		})
	}
}
func TestImportRetainsExactIdentityAndIndependentBytes(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts/fixtures/structural-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := reportimport.Read(raw, limits())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "validated" || got.Coverage != "declared" || got.V2 == nil {
		t.Fatal("wrong v2 state")
	}
	digest := identity.ExactBytes(raw)
	raw[0] = ' '
	copy := got.Bytes()
	copy[0] = ' '
	got.V2.File.Path = "consumer annotation"
	if got.ReportArtifactSHA256 != digest || identity.ExactBytes(got.Bytes()) != digest {
		t.Fatal("retained bytes mutated")
	}
	differentlySpaced := append(got.Bytes(), byte('\n'))
	other, err := reportimport.Read(differentlySpaced, limits())
	if err != nil {
		t.Fatal(err)
	}
	if other.ReportArtifactSHA256 == digest {
		t.Fatal("identity used reserialized report")
	}
}
func TestUnknownAndMalformedImports(t *testing.T) {
	raw := []byte(`{"schema_version":"99.0","file":{"path":"file:///etc/passwd"},"$schema":"https://example.invalid/schema"}`)
	got, err := reportimport.Read(raw, limits())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "unsupported_version" || got.Coverage != "unknown" || got.V2 != nil {
		t.Fatal("unknown version interpreted")
	}
	for _, bad := range [][]byte{[]byte(`{"schema_version":"2.0"}`), []byte(`{"schema_version":"1.0"}`), []byte(`{"schema_version":"99.0","schema_version":"2.0"}`), []byte(`{"schema_version":"99.0","text":"\ud800"}`), []byte(`[]`), []byte(`{"SCHEMA_VERSION":"99.0"}`), []byte(`{"schema_version":2}`)} {
		if _, err := reportimport.Read(bad, limits()); err == nil {
			t.Fatal("malformed import accepted")
		}
	}
	budget := limits()
	budget.InputBytes = len(raw) - 1
	if _, err := reportimport.Read(raw, budget); err == nil {
		t.Fatal("import byte limit ignored")
	}
}
func FuzzReportImport(f *testing.F) {
	for _, path := range []string{"../../tests/contracts/fixtures/structural-observation.json", "../../tests/contracts/fixtures/partial-timeout.json", "../../reference/python-behavior/reports/000.json"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Add([]byte(`{"schema_version":"future"}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 32768 {
			t.Skip()
		}
		original := bytes.Clone(raw)
		result, err := reportimport.Read(raw, limits())
		if err == nil && (result.ReportArtifactSHA256 != identity.ExactBytes(raw) || !bytes.Equal(raw, result.Bytes())) {
			t.Fatal("import identity changed")
		}
		if !bytes.Equal(raw, original) {
			t.Fatal("input mutated")
		}
	})
}
