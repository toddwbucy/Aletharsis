package v2_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

func reportLimits() identity.Limits {
	return identity.Limits{InputBytes: 8 << 20, OutputBytes: 8 << 20, Nodes: 500000, Depth: 64}
}
func readReport(t *testing.T, name string) v2.Report {
	t.Helper()
	raw, err := os.ReadFile("../../../tests/contracts/fixtures/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := v2.DecodeReport(raw, reportLimits())
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestFullReportFixtureRoundTrip(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal("eight fixtures required", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := v2.DecodeReport(raw, reportLimits())
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := r.Encode(reportLimits())
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := identity.Canonicalize(raw, reportLimits())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, canonical) {
				t.Fatal("full report wire changed")
			}
		})
	}
}
func TestReportRejectsBrokenFindingContracts(t *testing.T) {
	for name, mutate := range map[string]func(*v2.Report){
		"unknown payload":           func(r *v2.Report) { r.Findings[0].Evidence["malicious_extra"] = true },
		"missing payload":           func(r *v2.Report) { delete(r.Findings[0].Evidence, "count") },
		"wrong category":            func(r *v2.Report) { r.Findings[0].Category = "provenance" },
		"missing occurrence anchor": func(r *v2.Report) { r.Findings[0].AnchorRefs = []string{} },
		"cyclic producer map":       func(r *v2.Report) { r.Findings[0].Evidence["cycle"] = r.Findings[0].Evidence },
		"false raw text hash": func(r *v2.Report) {
			r.Evidence.Texts[0].Hashes["raw_text_sha256"] = string(bytes.Repeat([]byte("0"), 64))
		},
		"false occurrence count": func(r *v2.Report) { r.Findings[0].Evidence["count"] = 999 },
		"wrong execution":        func(r *v2.Report) { r.Findings[0].ExecutionRef = "exec/999" },
		"wrong mechanism":        func(r *v2.Report) { m := v2.Statistical; r.Findings[0].Mechanism = &m },
		"negative result":        func(r *v2.Report) { r.Results[0].Payload.Outcome = "no_observations" },
		"wrong counts":           func(r *v2.Report) { r.Summary["findings"]++ },
		"invalid UTF8":           func(r *v2.Report) { r.Findings[0].Evidence["name"] = string([]byte{0xff}) },
		"status mismatch":        func(r *v2.Report) { r.Status = v2.Failed },
		"missing view":           func(r *v2.Report) { r.View.FindingCategories = nil },
		"duplicate finding":      func(r *v2.Report) { r.Findings = append(r.Findings, r.Findings[0]); r.Recount() },
	} {
		t.Run(name, func(t *testing.T) {
			r := readReport(t, "structural-observation")
			mutate(&r)
			if _, err := r.Encode(reportLimits()); err == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
}
func TestFilteredViewRetainsCoverage(t *testing.T) {
	r := readReport(t, "structural-observation")
	before, err := r.Encode(reportLimits())
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := r.SelectView("metadata")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Findings) != 0 || filtered.Summary["exit_code"] != 0 {
		t.Fatal("wrong filtered counts")
	}
	if !reflect.DeepEqual(r.Graph, filtered.Graph) || !reflect.DeepEqual(r.Evidence, filtered.Evidence) {
		t.Fatal("filter changed evidence or coverage")
	}
	if filtered.Results[0].Payload.Outcome != "observations_present" {
		t.Fatal("filter turned positive result into negative")
	}
	after, err := r.Encode(reportLimits())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("filter mutated original", err)
	}
	if _, err := filtered.Encode(reportLimits()); err != nil {
		t.Fatal(err)
	}
	if _, err := filtered.SelectView("audit"); err == nil {
		t.Fatal("reconstructed missing findings")
	}
	failed := readReport(t, "decode-failed")
	filtered, err = failed.SelectView("metadata")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Summary["exit_code"] != 4 || len(filtered.Findings) != 1 {
		t.Fatal("filter hid failure")
	}
}
func TestReportBudgetAndStrictJSON(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts/fixtures/clean-unavailable.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{
		append(append([]byte{}, raw...), []byte(` {}`)...),
		bytes.Replace(raw, []byte(`"schema_version": "2.0"`), []byte(`"schema_version":"2.0","schema_version":"2.0"`), 1),
		bytes.Replace(raw, []byte(`"schema_version": "2.0"`), []byte(`"schema_version":"\ud800"`), 1),
	} {
		if _, err := v2.DecodeReport(bad, reportLimits()); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	limits := reportLimits()
	limits.InputBytes = len(raw) - 1
	if _, err := v2.DecodeReport(raw, limits); err == nil {
		t.Fatal("input budget ignored")
	}
	r := readReport(t, "clean-unavailable")
	limits = reportLimits()
	limits.Nodes = 10
	if _, err := r.Encode(limits); err == nil {
		t.Fatal("producer budget ignored")
	}
}
func TestParserFailureCodeMustMatchDiagnostic(t *testing.T) {
	r := readReport(t, "decode-failed")
	r.Findings[0].Evidence["failure_code"] = "file.not_found"
	if _, err := r.Encode(reportLimits()); err == nil {
		t.Fatal("mismatched typed failure accepted")
	}
}
func TestReportRoundTripDoesNotReinterpretFindingNumbers(t *testing.T) {
	r := readReport(t, "structural-observation")
	raw, err := r.Encode(reportLimits())
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	findings := value["findings"].([]any)
	findings[0].(map[string]any)["evidence"].(map[string]any)["count"] = true
	bad, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.DecodeReport(bad, reportLimits()); err == nil {
		t.Fatal("boolean count accepted")
	}
}

func TestBOMFindingMatchesRetainedSegment(t *testing.T) {
	r := readReport(t, "clean-unavailable")
	source := []byte("\ufeffhello\n")
	document, err := (parsers.TextParser{}).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	(analyzers.Unicode{}).Analyze(&document)
	r.Evidence = document
	size, hash := len(source), identity.ExactBytes(source)
	r.File.Size = &size
	r.File.SHA256 = &hash
	byteLength := uint64(size)
	r.Artifacts[0].SHA256 = &hash
	r.Artifacts[0].ByteLength = &byteLength
	textHash := identity.ExactBytes([]byte(document.Texts[0].Text))
	textLength := uint64(len(document.Texts[0].Text))
	r.Artifacts[1].SHA256 = &textHash
	r.Artifacts[1].ByteLength = &textLength
	mechanism := v2.Structural
	for _, f := range (analyzers.Text{}).Analyze(&document) {
		if f.ID == "text.bom" {
			r.Findings = []v2.Finding{{Finding: f, Ref: "finding/0", Mechanism: &mechanism, ExecutionRef: r.Results[0].ExecutionRef, AnchorRefs: []string{}}}
		}
	}
	if len(r.Findings) != 1 {
		t.Fatal("BOM finding missing")
	}
	r.Results[0].Payload.Outcome = "observations_present"
	r.Recount()
	if _, err := r.Encode(reportLimits()); err != nil {
		t.Fatal(err)
	}
	r.Findings[0].Location["byte_offset"] = 1
	if _, err := r.Encode(reportLimits()); err == nil {
		t.Fatal("wrong BOM byte offset accepted")
	}
}
