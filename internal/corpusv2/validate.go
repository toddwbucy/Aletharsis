// Package corpusv2 validates the versioned Office-capable corpus envelope.
// It performs no discovery, source acquisition or presentation work.
package corpusv2

import (
	"encoding/json"
	"errors"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"reflect"

	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/reportimport"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

var ErrLinkage = errors.New("corpus-v2 cross-record linkage is invalid")

type Header struct {
	Type         string          `json:"type"`
	Contract     string          `json:"contract"`
	Workspace    string          `json:"workspace"`
	ReportSchema string          `json:"report_schema"`
	Limits       json.RawMessage `json:"limits"`
}
type Entry struct {
	// Import support is a runtime view, never producer-supplied wire data.
	// Unsupported-feature entries retain opaque report bytes; they are not
	// promoted to validated native evidence or interpreted as negative results.
	ImportStatus           string          `json:"-"`
	SupportReason          *string         `json:"-"`
	Type                   string          `json:"type"`
	RelativePath           string          `json:"relative_path"`
	State                  string          `json:"state"`
	Reason                 string          `json:"reason"`
	Report                 json.RawMessage `json:"report,omitempty"`
	ReportSchema           string          `json:"report_schema,omitempty"`
	ReportCanonicalSHA256  string          `json:"report_canonical_sha256,omitempty"`
	HighestFindingSeverity *string         `json:"highest_finding_severity,omitempty"`
}
type Summary struct {
	Type                  string         `json:"type"`
	State                 string         `json:"state"`
	Reason                string         `json:"reason"`
	DiscoveryComplete     bool           `json:"discovery_complete"`
	Entries               int            `json:"entries"`
	Counts                map[string]int `json:"counts"`
	FindingSeverityCounts map[string]int `json:"finding_severity_counts"`
	ExitCode              int            `json:"exit_code"`
}
type Document struct {
	Header  Header  `json:"header"`
	Entries []Entry `json:"entries"`
	Summary Summary `json:"summary"`
}

// Decode validates each embedded report under the independent report budget.
// Caller budgets apply to the whole envelope before record allocation.
func Decode(raw []byte, envelopeLimits, reportLimits identity.Limits) (Document, error) {
	canonical, err := wire.ValidateEnvelope("corpus-document-v2", raw, envelopeLimits)
	if err != nil {
		return Document{}, err
	}
	var d Document
	if err := json.Unmarshal(canonical, &d); err != nil {
		return Document{}, err
	}
	return validateDocument(d, reportLimits)
}

func validateDocument(d Document, reportLimits identity.Limits) (Document, error) {
	counts := map[string]int{"no_reported_findings": 0, "requires_review": 0, "unsupported": 0, "failed": 0, "skipped": 0, "canceled": 0, "partial": 0}
	severities := map[string]int{"INFO": 0, "LOW": 0, "MEDIUM": 0, "HIGH": 0}
	paths := map[string]bool{}
	for i := range d.Entries {
		entry := &d.Entries[i]
		if paths[entry.RelativePath] {
			return Document{}, ErrLinkage
		}
		paths[entry.RelativePath] = true
		counts[entry.State]++
		if len(entry.Report) == 0 {
			continue
		}
		if entry.ReportSchema != d.Header.ReportSchema {
			return Document{}, ErrLinkage
		}
		imported, err := reportimport.ReadSupported(entry.Report, reportLimits)
		if err != nil {
			return Document{}, err
		}
		entry.ImportStatus, entry.SupportReason = imported.Status, imported.SupportReason
		if imported.Status != "validated" && imported.Status != "unsupported_feature" {
			return Document{}, ErrLinkage
		}
		reportCanonical, err := identity.Canonicalize(entry.Report, reportLimits)
		if err != nil {
			return Document{}, err
		}
		if identity.ExactBytes(reportCanonical) != entry.ReportCanonicalSHA256 {
			return Document{}, ErrLinkage
		}
		var report struct {
			Status      string         `json:"status"`
			Schema      string         `json:"schema_version"`
			Summary     map[string]int `json:"summary"`
			Diagnostics []struct {
				Code         string  `json:"code"`
				ExecutionRef *string `json:"execution_ref"`
			} `json:"diagnostics"`
			Executions   []v2.Execution  `json:"executions"`
			Capabilities []v2.Capability `json:"capabilities"`
		}
		if err := json.Unmarshal(entry.Report, &report); err != nil {
			return Document{}, err
		}
		if report.Schema != entry.ReportSchema {
			return Document{}, ErrLinkage
		}
		severity := ""
		for _, pair := range [][2]string{{"high", "HIGH"}, {"medium", "MEDIUM"}, {"low", "LOW"}, {"info", "INFO"}} {
			if report.Summary[pair[0]] > 0 {
				severity = pair[1]
				break
			}
		}
		if (entry.HighestFindingSeverity == nil) != (severity == "") || entry.HighestFindingSeverity != nil && *entry.HighestFindingSeverity != severity {
			return Document{}, ErrLinkage
		}
		if severity != "" {
			severities[severity]++
		}
		expected := report.Status
		if expected == "completed" {
			expected = "no_reported_findings"
			if severity != "" {
				expected = "requires_review"
			}
		}
		// A source-limit failure in the observer retains the detection report.
		observerFailure := entry.State == "failed" && entry.Reason == "execution.resource_limit"
		formatUnsupported := false
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Code != "format.unsupported" || diagnostic.ExecutionRef == nil {
				continue
			}
			for _, execution := range report.Executions {
				if execution.Ref != *diagnostic.ExecutionRef || execution.State != v2.Failed {
					continue
				}
				for _, capability := range report.Capabilities {
					if capability.ID == execution.CapabilityRef && capability.Role == v2.Parser {
						formatUnsupported = true
					}
				}
			}
		}
		unsupported := entry.State == "unsupported" && report.Status == "failed" && entry.Reason == "format.unsupported" && formatUnsupported
		if entry.State != expected && !observerFailure && !unsupported {
			return Document{}, ErrLinkage
		}
	}
	if d.Summary.Entries != len(d.Entries) || !reflect.DeepEqual(d.Summary.Counts, counts) || !reflect.DeepEqual(d.Summary.FindingSeverityCounts, severities) {
		return Document{}, ErrLinkage
	}
	incomplete := !d.Summary.DiscoveryComplete || counts["partial"]+counts["failed"]+counts["unsupported"]+counts["canceled"] > 0
	expectedExit := 0
	if severities["INFO"]+severities["LOW"] > 0 {
		expectedExit = 1
	}
	if severities["MEDIUM"] > 0 {
		expectedExit = 2
	}
	if severities["HIGH"] > 0 {
		expectedExit = 3
	}
	if incomplete || d.Summary.State == "failed" || d.Summary.State == "canceled" {
		expectedExit = 4
	}
	if d.Summary.ExitCode != expectedExit || (incomplete && d.Summary.State == "completed") {
		return Document{}, ErrLinkage
	}
	if !incomplete && d.Summary.State == "partial" {
		return Document{}, ErrLinkage
	}
	if (d.Summary.State == "failed" || d.Summary.State == "canceled") && d.Summary.Reason == "" {
		return Document{}, ErrLinkage
	}
	return d, nil
}
