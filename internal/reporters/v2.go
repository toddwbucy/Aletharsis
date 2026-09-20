package reporters

import (
	"fmt"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

// ConsoleV2 renders a validated native report. Coverage precedes findings so an
// empty finding view cannot conceal unrun, failed or unavailable operations.
func ConsoleV2(r *v2.Report, verbose bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Schema: 2.0 | View: %s | Operational status: %s\n\nCoverage\n", u.Escaped(r.View.Name), u.Escaped(string(r.Status)))
	capabilities := map[string]v2.Capability{}
	for _, c := range r.Capabilities {
		capabilities[c.ID] = c
	}
	for _, e := range r.Executions {
		c := capabilities[e.CapabilityRef]
		mechanism := "operational"
		if c.Mechanism != nil {
			mechanism = string(*c.Mechanism)
		}
		fmt.Fprintf(&b, "  %s [%s]\n    Availability: %s; participation: %s; execution: %s\n", u.Escaped(c.ID), u.Escaped(mechanism), u.Escaped(c.Availability.State), u.Escaped(string(c.Participation)), u.Escaped(string(e.State)))
		if c.Availability.ReasonCode != nil {
			fmt.Fprintf(&b, "    Availability reason: %s\n", u.Escaped(string(*c.Availability.ReasonCode)))
		}
		if e.ReasonCode != nil {
			fmt.Fprintf(&b, "    Execution reason: %s\n", u.Escaped(string(*e.ReasonCode)))
		}
		fmt.Fprintf(&b, "    Requested scope: %s; analyzed scopes: %d; exclusions: %d\n", compact(e.RequestedScope), len(e.AnalyzedScope), len(e.Exclusions))
		if verbose {
			fmt.Fprintf(&b, "    Analyzed: %s\n    Exclusions: %s\n", compact(e.AnalyzedScope), compact(e.Exclusions))
		}
	}
	for _, d := range r.Diagnostics {
		fmt.Fprintf(&b, "  Diagnostic %s: %s — %s\n", u.Escaped(d.Ref), u.Escaped(string(d.Code)), u.Escaped(d.Message))
	}
	b.WriteString("\nNo reported findings does not establish the absence of a watermark.\nUnavailable or unrun detectors provide no negative result.\nCoverage and results apply to the full audit; findings and severity exits reflect the selected view.\n\n")
	legacy := evidence.Report{Version: r.Version, Schema: r.Schema, File: r.File, Status: string(r.Status), Summary: r.Summary, Evidence: r.Evidence, Limitations: r.Limitations, Findings: []evidence.Finding{}}
	for _, f := range r.Findings {
		legacy.Findings = append(legacy.Findings, f.Finding)
	}
	b.WriteString(Console(&legacy, verbose))
	return b.String()
}
