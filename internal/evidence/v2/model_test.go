package v2_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func fixture(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "contracts", "fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]json.RawMessage
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func records(t *testing.T, name, key string) []json.RawMessage {
	t.Helper()
	var r []json.RawMessage
	if err := json.Unmarshal(fixture(t, name)[key], &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func sameJSON(t *testing.T, before []byte, value any) {
	t.Helper()
	after, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	a, err := identity.Canonicalize(before, v2.RecordLimits())
	if err != nil {
		t.Fatal(err)
	}
	b, err := identity.Canonicalize(after, v2.RecordLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("wire changed:\n%s\n%s", a, b)
	}
}
func TestAcceptedRecordFixtures(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight fixtures required", err)
	}
	counts := map[string]int{}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var report map[string]json.RawMessage
			if err := json.Unmarshal(raw, &report); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"capabilities", "executions", "diagnostics"} {
				var values []json.RawMessage
				if err := json.Unmarshal(report[key], &values); err != nil {
					t.Fatal(err)
				}
				for _, raw := range values {
					counts[key]++
					switch key {
					case "capabilities":
						v, err := v2.DecodeCapability(raw)
						if err != nil {
							t.Fatal(err)
						}
						sameJSON(t, raw, v)
					case "executions":
						v, err := v2.DecodeExecution(raw)
						if err != nil {
							t.Fatal(err)
						}
						sameJSON(t, raw, v)
					case "diagnostics":
						v, err := v2.DecodeDiagnostic(raw)
						if err != nil {
							t.Fatal(err)
						}
						sameJSON(t, raw, v)
					}
				}
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"capabilities": 40, "executions": 40, "diagnostics": 3}) {
		t.Fatal(counts)
	}
}

func alter(t *testing.T, raw []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	edit(value)
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestCapabilityInvalidShapes(t *testing.T) {
	raw := records(t, "clean-unavailable", "capabilities")[0]
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"unknown", func(v map[string]any) { v["extra"] = true }},
		{"case-alias", func(v map[string]any) { v["ID"] = "other" }},
		{"required-nullable-missing", func(v map[string]any) { delete(v, "purpose") }},
		{"null-enum", func(v map[string]any) { v["role"] = nil }},
		{"null-list", func(v map[string]any) { v["supported_scope"].(map[string]any)["kinds"] = nil }},
		{"missing-limit", func(v map[string]any) { delete(v["limits"].(map[string]any), "depth") }},
		{"nested-unknown", func(v map[string]any) { v["availability"].(map[string]any)["extra"] = true }},
		{"mechanism", func(v map[string]any) { v["mechanism"] = "statistical" }},
		{"enum", func(v map[string]any) { v["participation"] = "automatic" }},
		{"unsafe-limit", func(v map[string]any) { v["limits"].(map[string]any)["input_bytes"] = identity.MaxSafeInteger + 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v2.DecodeCapability(alter(t, raw, tc.edit)); err == nil {
				t.Fatal("accepted invalid capability")
			}
		})
	}
	invalid := bytes.Replace(raw, []byte(`"aletharsis.acquire"`), []byte(`"\ud800"`), 1)
	if _, err := v2.DecodeCapability(invalid); err == nil {
		t.Fatal("accepted lone surrogate")
	}
	duplicate := append([]byte(`{"id":"different",`), raw[1:]...)
	if _, err := v2.DecodeCapability(duplicate); err == nil {
		t.Fatal("accepted duplicate identity")
	}
}

func TestConfigIdentityAndClosedVariants(t *testing.T) {
	n := uint64(8388608)
	c, err := v2.NewNativeConfig("conformance-1", "unicode-15.0.0", v2.Limits{InputBytes: &n})
	if err != nil {
		t.Fatal(err)
	}
	var execution map[string]json.RawMessage
	if err := json.Unmarshal(records(t, "clean-unavailable", "executions")[0], &execution); err != nil {
		t.Fatal(err)
	}
	sameJSON(t, execution["config"], c)
	unknown := v2.UnknownConfig()
	raw, err := json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.DecodeConfig(raw); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { delete(v, "schema") },
		func(v map[string]any) { v["key_ref"] = "secret" },
		func(v map[string]any) { v["settings"] = map[string]any{} },
	} {
		if _, err := v2.DecodeConfig(alter(t, raw, edit)); err == nil {
			t.Fatal("invalid unknown config")
		}
	}
	native, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { v["sha256"] = "bad" },
		func(v map[string]any) { v["unavailable_reason"] = nil },
		func(v map[string]any) { v["settings"].(map[string]any)["preprocessing"] = "normalize" },
	} {
		if _, err := v2.DecodeConfig(alter(t, native, edit)); err == nil {
			t.Fatal("invalid native config")
		}
	}
	c.Settings.Limits.InputBytes = ptr(uint64(12))
	if _, err := json.Marshal(c); err == nil {
		t.Fatal("stale configuration hash serialized")
	}
}

func TestExecutionStateRules(t *testing.T) {
	completed := records(t, "clean-unavailable", "executions")[2]
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { v["state"] = "running" },
		func(v map[string]any) { v["analyzed_scope"] = []any{} },
		func(v map[string]any) { v["state"] = "not_run"; v["reason_code"] = "execution.disabled" },
		func(v map[string]any) { v["analyzed_scope"] = nil },
		func(v map[string]any) { v["requested_scope"].(map[string]any)["regions"] = []any{} },
	} {
		if _, err := v2.DecodeExecution(alter(t, completed, edit)); err == nil {
			t.Fatal("invalid completed execution")
		}
	}
	partial := records(t, "partial-timeout", "executions")[2]
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { v["diagnostic_refs"] = []any{} },
		func(v map[string]any) { v["exclusions"] = []any{} },
		func(v map[string]any) {
			v["exclusions"].([]any)[0].(map[string]any)["scope"] = v["analyzed_scope"].([]any)[0]
		},
		func(v map[string]any) {
			v["analyzed_scope"] = append(v["analyzed_scope"].([]any), v["analyzed_scope"].([]any)[0])
		},
	} {
		if _, err := v2.DecodeExecution(alter(t, partial, edit)); err == nil {
			t.Fatal("invalid partial execution")
		}
	}
	canceled := records(t, "canceled", "executions")[2]
	if _, err := v2.DecodeExecution(alter(t, canceled, func(v map[string]any) { v["reason_code"] = "execution.disabled" })); err == nil {
		t.Fatal("invalid cancellation reason")
	}
}

func TestDiagnosticStageAndUnknownCode(t *testing.T) {
	raw := records(t, "decode-failed", "diagnostics")[0]
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { delete(v["details"].(map[string]any), "byte_start") },
		func(v map[string]any) { v["stage"] = "acquisition" },
		func(v map[string]any) { v["details"].(map[string]any)["command"] = "untrusted content" },
		func(v map[string]any) { v["details"] = nil },
	} {
		if _, err := v2.DecodeDiagnostic(alter(t, raw, edit)); err == nil {
			t.Fatal("invalid diagnostic")
		}
	}
	unknown := alter(t, raw, func(v map[string]any) { v["code"] = "future.new_failure" })
	d, err := v2.DecodeDiagnostic(unknown)
	if err != nil || d.Code != "future.new_failure" {
		t.Fatal(d, err)
	}
	sameJSON(t, unknown, d)
}

func TestProducerRejectsLossyOrInvalidValues(t *testing.T) {
	if _, err := json.Marshal(v2.Capability{}); err == nil {
		t.Fatal("zero capability serialized")
	}
	if _, err := json.Marshal(v2.Execution{}); err == nil {
		t.Fatal("zero execution serialized")
	}
	if _, err := json.Marshal(v2.Config{}); err == nil {
		t.Fatal("zero config serialized")
	}
	d := v2.Diagnostic{Ref: "diagnostic/0", Stage: failure.Acquisition, Code: failure.IOFailed, Message: string([]byte{0xff}), Details: v2.ErrorDetails{ErrorType: "OSError"}}
	if _, err := json.Marshal(d); err == nil {
		t.Fatal("invalid Unicode replaced")
	}
}

func TestBoundedRecordDecode(t *testing.T) {
	if _, err := v2.DecodeCapability(bytes.Repeat([]byte(" "), (1<<20)+1)); err == nil {
		t.Fatal("unbounded input")
	}
	if _, err := v2.DecodeCapability([]byte(`null`)); err == nil {
		t.Fatal("null record accepted")
	}
}
func ptr[T any](v T) *T { return &v }

func TestSafeNumericSpellings(t *testing.T) {
	raw := records(t, "clean-unavailable", "capabilities")[0]
	for _, number := range []string{"8388608.0", "8.388608e6"} {
		alternate := bytes.Replace(raw, []byte(`8388608`), []byte(number), 1)
		if _, err := v2.DecodeCapability(alternate); err != nil {
			t.Fatal("integral JSON spelling rejected", err)
		}
	}
	for _, number := range []string{"true", "-1", "1.5", "9007199254740992", "9007199254740993"} {
		alternate := bytes.Replace(raw, []byte(`8388608`), []byte(number), 1)
		if _, err := v2.DecodeCapability(alternate); err == nil {
			t.Fatal("unsafe coordinate/count accepted", number)
		}
	}
}

func TestOtherDiagnosticVariants(t *testing.T) {
	for _, d := range []v2.Diagnostic{
		{Ref: "diagnostic/0", Stage: failure.Audit, Code: failure.AuditFailed, Details: v2.ErrorDetails{ErrorType: "ValueError"}},
		{Ref: "diagnostic/0", Stage: "import", Code: "legacy.failure_code_unavailable", Details: v2.ImportDetails{ReportPointer: ptr("/findings/0")}},
		{Ref: "diagnostic/0", Stage: "output", Code: "output.write_failed", Details: v2.OutputDetails{Operation: "write"}},
	} {
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		got, err := v2.DecodeDiagnostic(raw)
		if err != nil {
			t.Fatal(err)
		}
		sameJSON(t, raw, got)
	}
}

func FuzzRecordDecode(f *testing.F) {
	f.Add([]byte(`null`))
	f.Add([]byte(`{"id":"x","id":"y"}`))
	f.Add([]byte(`{"schema":null,"revision":null,"settings":null,"sha256":null,"key_ref":null,"unavailable_reason":"configuration.unknown"}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 8192 {
			return
		}
		before := bytes.Clone(raw)
		if c, err := v2.DecodeConfig(raw); err == nil {
			round, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeConfig(round); err != nil {
				t.Fatal(err)
			}
		}
		_, _ = v2.DecodeCapability(raw)
		_, _ = v2.DecodeExecution(raw)
		_, _ = v2.DecodeDiagnostic(raw)
		if !bytes.Equal(raw, before) {
			t.Fatal("record decoder mutated bytes")
		}
	})
}
