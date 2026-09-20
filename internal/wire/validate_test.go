package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func limits() identity.Limits {
	return identity.Limits{InputBytes: 8 << 20, OutputBytes: 8 << 20, Nodes: 500000, Depth: 64}
}
func TestOriginalNumericTokensAreValidated(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts/fixtures/structural-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	needle := []byte(`"count": 1`)
	if !bytes.Contains(raw, needle) {
		t.Fatal("fixture changed")
	}
	for _, token := range []string{"true", "1.000000000000000000000000001", "9007199254740992", "1e-1000000000"} {
		bad := bytes.Replace(raw, needle, []byte(`"count": `+token), 1)
		if _, err := Validate("2.0", bad, limits()); err == nil {
			t.Fatal("invalid count accepted", token)
		}
	}
	integral := bytes.Replace(raw, needle, []byte(`"count": 1.0`), 1)
	if _, err := Validate("2.0", integral, limits()); err != nil {
		t.Fatal("integral spelling rejected", err)
	}
}
func TestNoReportSelectedSchemas(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts/fixtures/clean-unavailable.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"file:///etc/passwd", "https://example.invalid/report.json"} {
		if _, err := Validate(url, raw, limits()); !errors.Is(err, ErrVersion) {
			t.Fatal(err)
		}
		if _, err := (denyLoader{}).Load(url); err == nil {
			t.Fatal("external loader permitted")
		}
	}
	if _, err := Validate("2.0", raw, limits()); err != nil {
		t.Fatal(err)
	}
}
func TestSchemaErrorsDoNotEchoPayloads(t *testing.T) {
	raw := []byte(`{"schema_version":"2.0","secret":"do not disclose this"}`)
	_, err := Validate("2.0", raw, limits())
	if !errors.Is(err, ErrSchema) || bytes.Contains([]byte(err.Error()), []byte("disclose")) {
		t.Fatal(err)
	}
}

func TestConcurrentBundledValidation(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts/fixtures/clean-unavailable.json")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		t.Run("reader", func(t *testing.T) {
			t.Parallel()
			if _, err := Validate("2.0", raw, limits()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAllNativeFindingWireVariants(t *testing.T) {
	raw, err := os.ReadFile("../../tests/schema-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Findings map[string]map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases.Findings) != 23 {
		t.Fatal("23 native finding variants required")
	}
	base, err := os.ReadFile("../../tests/contracts/fixtures/clean-unavailable.json")
	if err != nil {
		t.Fatal(err)
	}
	for id, finding := range cases.Findings {
		t.Run(id, func(t *testing.T) {
			var report map[string]any
			if err := json.Unmarshal(base, &report); err != nil {
				t.Fatal(err)
			}
			finding["finding_ref"] = "finding/0"
			finding["execution_ref"] = "exec/2"
			finding["mechanism"] = "structural"
			finding["anchor_refs"] = []string{}
			if id == "parser.failure" {
				finding["mechanism"] = nil
				finding["evidence"].(map[string]any)["failure_code"] = "audit.failed"
			}
			report["findings"] = []any{finding}
			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Validate("2.0", raw, limits()); err != nil {
				t.Fatal(err)
			}
			finding["evidence"].(map[string]any)["unsupported_property"] = true
			raw, err = json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Validate("2.0", raw, limits()); err == nil {
				t.Fatal("unknown evidence key accepted")
			}
		})
	}
}
