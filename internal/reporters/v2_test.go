package reporters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

func TestV2ConsoleCoverageFixtures(t *testing.T) {
	paths, err := filepath.Glob("../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatalf("fixture collection: %v %d", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			report, err := v2.DecodeReport(raw, audit.DefaultV2Options().ReportLimits)
			if err != nil {
				t.Fatal(err)
			}
			output := ConsoleV2(&report, true)
			if strings.Index(output, "Coverage") > strings.Index(output, "Summary:") || !strings.Contains(output, "Unavailable or unrun detectors provide no negative result.") {
				t.Fatal("coverage caveat missing")
			}
			for _, e := range report.Executions {
				if !strings.Contains(output, e.CapabilityRef) || !strings.Contains(output, "execution: "+string(e.State)) {
					t.Fatal("execution hidden")
				}
			}
			for _, d := range report.Diagnostics {
				if !strings.Contains(output, string(d.Code)) {
					t.Fatal("diagnostic hidden")
				}
			}
			report.File.Path = "bad\x1b[31m\u202e"
			output = ConsoleV2(&report, false)
			if strings.ContainsAny(output, "\x1b\u202e") {
				t.Fatal("terminal controls rendered")
			}
		})
	}
}
