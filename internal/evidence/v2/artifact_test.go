package v2_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestAcceptedEvidenceRecords(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight fixtures required", err)
	}
	counts := map[string]int{}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		for _, key := range []string{"artifacts", "anchors", "results"} {
			for _, raw := range records(t, name, key) {
				counts[key]++
				var value any
				switch key {
				case "artifacts":
					value, err = v2.DecodeArtifact(raw)
				case "anchors":
					value, err = v2.DecodeAnchor(raw)
				case "results":
					value, err = v2.DecodeResult(raw)
				}
				if err != nil {
					t.Fatalf("%s %s: %v", name, key, err)
				}
				sameJSON(t, raw, value)
			}
		}
	}
	for _, key := range []string{"artifacts", "anchors", "results"} {
		if counts[key] == 0 {
			t.Fatal("empty fixture coverage", key)
		}
	}
}
func TestArtifactRejectsContradictions(t *testing.T) {
	raw := records(t, "structural-observation", "artifacts")[1]
	for name, change := range map[string]func(map[string]any){
		"missing length":           func(m map[string]any) { delete(m, "byte_length") },
		"null identity":            func(m map[string]any) { m["sha256"] = nil },
		"unavailable retained":     func(m map[string]any) { m["unavailable_reason"] = "not_retained" },
		"unsafe length":            func(m map[string]any) { m["byte_length"] = uint64(1) << 53 },
		"boolean length":           func(m map[string]any) { m["byte_length"] = true },
		"extra key":                func(m map[string]any) { m["trusted"] = true },
		"self parent":              func(m map[string]any) { m["parents"] = []string{m["artifact_ref"].(string)} },
		"null parents":             func(m map[string]any) { m["parents"] = nil },
		"transform mismatch":       func(m map[string]any) { m["transform"].(map[string]any)["inputs"] = []string{} },
		"pointer leading zero":     func(m map[string]any) { m["content_ref"].(map[string]any)["pointer"] = "/evidence/texts/00/text" },
		"content extra key":        func(m map[string]any) { m["content_ref"].(map[string]any)["path"] = "/tmp/private" },
		"mapping extra key":        func(m map[string]any) { m["mapping"].(map[string]any)["trusted"] = true },
		"mapping missing nullable": func(m map[string]any) { delete(m["mapping"].(map[string]any), "data_ref") },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			change(m)
			encoded, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeArtifact(encoded); err == nil {
				t.Fatal("accepted invalid artifact")
			}
		})
	}
}
func TestUnavailableArtifactVariants(t *testing.T) {
	a, err := v2.DecodeArtifact(records(t, "clean-unavailable", "artifacts")[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"not_retained", "redacted"} {
		a.UnavailableReason = &reason
		if err := a.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	a.SHA256 = nil
	a.ByteLength = nil
	for _, reason := range []string{"not_acquired", "extraction_unavailable"} {
		a.UnavailableReason = &reason
		if err := a.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	a.UnavailableReason = nil
	if err := a.Validate(); err == nil {
		t.Fatal("unknown identity without reason")
	}
}
func TestAllLocatorVariants(t *testing.T) {
	artifact, exec := "artifact/1", "exec/1"
	reason := failure.Code("mapping.unavailable")
	base := v2.Anchor{Ref: "anchor/0", ArtifactRef: &artifact, ExecutionRef: &exec, Mapping: v2.Mapping{Quality: "unavailable", ReasonCode: &reason}}
	for kind, locator := range map[string]v2.Locator{
		"structural_object":  v2.StructuralLocator{Version: "1", Format: "pdf", ObjectID: "12 0 R"},
		"credential":         v2.CredentialLocator{Version: "1", ManifestRef: "artifact/1"},
		"statistical_sample": v2.StatisticalLocator{Version: "1", SampleRef: artifact, Scope: v2.Scope{ArtifactRef: artifact, Unit: "whole_artifact"}},
		"legacy_unknown":     v2.LegacyLocator{Version: "1", ReportPointer: "/findings/0", DiagnosticRef: "diagnostic/0"},
	} {
		t.Run(kind, func(t *testing.T) {
			a := base
			a.Kind = kind
			a.Locator = locator
			if kind == "legacy_unknown" {
				a.ArtifactRef = nil
				a.ExecutionRef = nil
			}
			raw, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := v2.DecodeAnchor(raw)
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, raw, decoded)
			var w map[string]any
			if err := json.Unmarshal(raw, &w); err != nil {
				t.Fatal(err)
			}
			w["locator"].(map[string]any)["byte_start"] = 0
			invalid, err := json.Marshal(w)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeAnchor(invalid); err == nil {
				t.Fatal("fabricated byte span accepted")
			}
		})
	}
}
func TestEvidenceRecordInputBounds(t *testing.T) {
	oversized := bytes.Repeat([]byte(" "), v2.RecordLimits().InputBytes+1)
	for _, decode := range []func([]byte) error{
		func(b []byte) error { _, e := v2.DecodeArtifact(b); return e },
		func(b []byte) error { _, e := v2.DecodeMapping(b); return e },
		func(b []byte) error { _, e := v2.DecodeAnchor(b); return e },
		func(b []byte) error { _, e := v2.DecodeResult(b); return e },
	} {
		for _, raw := range [][]byte{oversized, []byte(`{"kind":"text","kind":"sample"}`), []byte(`{"kind":"\ud800"}`)} {
			if err := decode(raw); err == nil {
				t.Fatal("invalid record accepted")
			}
		}
	}
}
func FuzzEvidenceRecords(f *testing.F) {
	raw, err := os.ReadFile("../../../tests/contracts/fixtures/structural-observation.json")
	if err != nil {
		f.Fatal(err)
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal(raw, &report); err != nil {
		f.Fatal(err)
	}
	for _, key := range []string{"artifacts", "anchors", "results"} {
		var records []json.RawMessage
		if err := json.Unmarshal(report[key], &records); err != nil {
			f.Fatal(err)
		}
		for _, record := range records {
			f.Add([]byte(record))
		}
	}

	f.Add([]byte(`{}`))
	f.Add([]byte(`{"quality":"unavailable","reason_code":"mapping.unknown"}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 8192 {
			t.Skip()
		}
		before := bytes.Clone(raw)
		if a, e := v2.DecodeArtifact(raw); e == nil {
			sameJSON(t, raw, a)
		}
		if a, e := v2.DecodeAnchor(raw); e == nil {
			sameJSON(t, raw, a)
		}
		if r, e := v2.DecodeResult(raw); e == nil {
			sameJSON(t, raw, r)
		}
		if m, e := v2.DecodeMapping(raw); e == nil {
			sameJSON(t, raw, m)
		}
		if !bytes.Equal(raw, before) {
			t.Fatal("mutated input")
		}
	})
}

func TestResultRejectsInventedVendorSemantics(t *testing.T) {
	raw := records(t, "clean-unavailable", "results")[0]
	for name, change := range map[string]func(map[string]any){
		"statistical kind":              func(m map[string]any) { m["kind"] = "statistical_watermark" },
		"universal probability":         func(m map[string]any) { m["payload"].(map[string]any)["probability"] = 0.99 },
		"missing limitation disclosure": func(m map[string]any) { delete(m, "limitations") },
		"null anchors":                  func(m map[string]any) { m["anchor_refs"] = nil },
		"wrong outcome":                 func(m map[string]any) { m["payload"].(map[string]any)["outcome"] = "human_written" },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			change(m)
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeResult(b); err == nil {
				t.Fatal("unsupported result semantics accepted")
			}
		})
	}
}
func TestRetainedBlobIdentityIsNotAPath(t *testing.T) {
	a, err := v2.DecodeArtifact(records(t, "clean-unavailable", "artifacts")[0])
	if err != nil {
		t.Fatal(err)
	}
	a.ContentRef = &v2.ContentRef{Kind: "retained_blob", SHA256: a.SHA256}
	a.UnavailableReason = nil
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := v2.DecodeArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, raw, decoded)
	wrong := strings.Repeat("0", 64)
	a.ContentRef.SHA256 = &wrong
	if err := a.Validate(); err == nil {
		t.Fatal("mismatched blob digest accepted")
	}
	path := "/tmp/private-source"
	a.ContentRef.Pointer = &path
	if err := a.Validate(); err == nil {
		t.Fatal("blob path accepted")
	}
}
