package evidence_test

import (
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"reflect"
	"testing"
)

func TestSummarySeverityAndFailurePrecedence(t *testing.T) {
	for _, tc := range []struct {
		status     string
		severities []string
		want       int
	}{
		{"completed", nil, 0}, {"completed", []string{"INFO"}, 1}, {"completed", []string{"LOW"}, 1},
		{"completed", []string{"INFO", "MEDIUM"}, 2}, {"completed", []string{"LOW", "HIGH"}, 3},
		{"failed", nil, 4}, {"failed", []string{"HIGH"}, 4},
	} {
		r := evidence.Report{Status: tc.status}
		for _, s := range tc.severities {
			r.Findings = append(r.Findings, evidence.Finding{Severity: s})
		}
		if got := r.Summarize(); got != tc.want || r.Summary["findings"] != len(tc.severities) || r.Summary["exit_code"] != tc.want {
			t.Fatal(r.Summary)
		}
	}
	r := evidence.Report{Status: "completed", Findings: []evidence.Finding{{Severity: "HIGH"}, {Severity: "HIGH"}, {Severity: "MEDIUM"}, {Severity: "LOW"}, {Severity: "INFO"}}}
	r.Summarize()
	if !reflect.DeepEqual(r.Summary, map[string]int{"findings": 5, "high": 2, "medium": 1, "low": 1, "info": 1, "exit_code": 3}) {
		t.Fatal(r.Summary)
	}
	r.Findings = nil
	if r.Summarize() != 0 || r.Summary["high"] != 0 {
		t.Fatal("stale summary retained")
	}
}

func TestLocationIncludesEndBoundary(t *testing.T) {
	text := &evidence.Text{Source: "file", Text: "é😀e\u0301", ByteOffsets: []int{0, 2, 6, 7, 9}}
	want := evidence.Object{"source": "file", "character_offsets": []int{0, 1, 3, 4}, "byte_offsets": []int{0, 2, 7, 9}}
	if got := evidence.Location(text, []int{0, 1, 3, 4}); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}
