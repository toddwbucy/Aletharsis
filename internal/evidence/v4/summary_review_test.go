package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"testing"
)

func TestSummarySeverityMatchesWireGrammar(t *testing.T) {
	for severity, rank := range map[string]int{"HIGH": 3, "MEDIUM": 2, "LOW": 1, "INFO": 1, "High": -1, "high": -1, "medium": -1, "Low": -1, "info": -1, "": -1} {
		t.Run(severity, func(t *testing.T) {
			r := Report{Status: v2.Completed, Findings: []v2.Finding{{Finding: evidence.Finding{Severity: severity}}}}
			counts, err := summaryCounts(r)
			if rank < 0 {
				if err == nil {
					t.Fatal("accepted noncanonical severity", counts)
				}
				return
			}
			if err != nil || counts["exit_code"] != rank || counts["findings"] != 1 {
				t.Fatal(counts, err)
			}
			r.Status = v2.Partial
			counts, err = summaryCounts(r)
			if err != nil || counts["exit_code"] != 4 {
				t.Fatal(counts, err)
			}
		})
	}
}
