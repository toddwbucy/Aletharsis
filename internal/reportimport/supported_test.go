package reportimport

import (
	"bytes"
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func supportLimits() identity.Limits {
	return identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
}
func TestSupportedPreservesLegacyImports(t *testing.T) {
	paths, err := filepath.Glob("../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight fixtures required", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		legacy, err := Read(raw, supportLimits())
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadSupported(raw, supportLimits())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Imported, legacy) || got.SupportReason != nil || got.V4 != nil {
			t.Fatal("legacy behavior changed")
		}
	}
}
func TestSupportedNativeOfficeAndFlat(t *testing.T) {
	paths, err := filepath.Glob("../../tests/contracts_v4/fixtures/*.json")
	if err != nil || len(paths) != 11 {
		t.Fatal("eleven fixtures required", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadSupported(raw, supportLimits())
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got.Status != "validated" || got.SupportReason != nil || got.V4 == nil || got.Coverage != "declared" || !bytes.Equal(got.Bytes(), raw) {
			t.Fatal("incorrect validated import")
		}
		copy := got.Bytes()
		copy[0] = 'X'
		if !bytes.Equal(got.Bytes(), raw) {
			t.Fatal("mutable artifact bytes")
		}
		old, err := Read(raw, supportLimits())
		if err != nil || old.Status != "unsupported_version" {
			t.Fatal("legacy Read changed", err)
		}
	}
}
func TestSupportedUnknownAndMalformed(t *testing.T) {
	for version, reason := range map[string]string{"3.0": "known_version_unimplemented", "9.0": "unknown_version"} {
		raw := []byte(`{"schema_version":"` + version + `"}`)
		got, err := ReadSupported(raw, supportLimits())
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "unsupported_version" || got.SupportReason == nil || *got.SupportReason != reason || got.V4 != nil || got.Coverage != "unknown" {
			t.Fatal("incorrect unsupported result")
		}
	}
	for _, raw := range []string{`{"schema_version":"4.0"}`, `{"schema_version":"9.0","schema_version":"4.0"}`, `null`, `[]`} {
		if _, err := ReadSupported([]byte(raw), supportLimits()); err == nil {
			t.Fatal("accepted malformed report")
		}
	}
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	scopes := value["evidence"].(map[string]any)["office"].(map[string]any)["scopes"].([]any)
	scopes[0].(map[string]any)["hashes"].(map[string]any)["nfc_text_sha256"] = identity.ExactBytes([]byte("wrong"))
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSupported(raw, supportLimits()); err == nil {
		t.Fatal("accepted damaged analytic hash")
	}
}

func TestSupportedAdapterFeatureIsNotValidation(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts_v3/fixtures/adapter-extracted.json")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	report["schema_version"] = "4.0"
	report["evidence"].(map[string]any)["office"] = map[string]any{"packages": []any{}, "xml": []any{}, "scopes": []any{}, "metadata": []any{}, "relationships": []any{}, "objects": []any{}}
	raw, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadSupported(raw, supportLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "unsupported_feature" || got.SupportReason == nil || *got.SupportReason != "known_feature_unimplemented" || got.V4 != nil || got.V2 != nil || got.Coverage != "unknown" || !bytes.Equal(raw, got.Bytes()) {
		t.Fatal("adapter report falsely validated")
	}
	report["unexpected"] = true
	raw, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSupported(raw, supportLimits()); err == nil {
		t.Fatal("wire-invalid adapter bypassed validation")
	}
}
