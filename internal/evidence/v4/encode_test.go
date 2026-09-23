package v4

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestNativeReportRoundTripPreservesCompleteWireEvidence(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts_v4/fixtures/*.json")
	if err != nil || len(paths) < 9 {
		t.Fatal("complete flat and Office fixtures required", err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			report, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := report.Encode(limits)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := identity.Canonicalize(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, expected) {
				t.Fatal("producer changed retained wire evidence")
			}
			decoded, err := DecodeReport(encoded, limits)
			if err != nil {
				t.Fatal(err)
			}
			again, err := decoded.Encode(limits)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, again) {
				t.Fatal("nondeterministic producer round trip")
			}
		})
	}
}

func TestEncodeRejectsDamagedProducerValues(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	cases := map[string]func(*Report){
		"invalid UTF8":       func(r *Report) { r.Version = string([]byte{0xff}) },
		"raw surrogate":      func(r *Report) { r.Artifacts[0].Mapping = json.RawMessage(`{"quality":"\ud800"}`) },
		"duplicate raw keys": func(r *Report) { r.Artifacts[0].Mapping = json.RawMessage(`{"quality":"exact","quality":"exact"}`) },
		"broken linkage":     func(r *Report) { r.Trace.Results[0].AnchorRefs = []string{"anchor/999"} },
		"incorrect summary":  func(r *Report) { r.Summary["findings"]++ },
		"missing graph":      func(r *Report) { r.Trace.Executions = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&report)
			if _, err := report.Encode(limits); err == nil {
				t.Fatal("accepted damaged producer")
			}
		})
	}
	for _, name := range []string{"input", "output", "nodes", "depth"} {
		t.Run(name, func(t *testing.T) {
			report, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			small := limits
			switch name {
			case "input":
				small.InputBytes = 10
			case "output":
				small.OutputBytes = 10
			case "nodes":
				small.Nodes = 10
			case "depth":
				small.Depth = 1
			}
			if _, err := report.Encode(small); err == nil {
				t.Fatal("ignored producer budget")
			}
		})
	}
}
