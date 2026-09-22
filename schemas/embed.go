// Package schemas bundles the published wire contracts. No report-supplied URL
// participates in schema selection or resource loading.
//
// Only contracts validated by Go participate in the embed set below. Go's
// regexp package implements RE2, which has no lookahead, so a JSON Schema
// "pattern" using a lookahead assertion cannot compile here even though it is
// valid for a Python jsonschema validator. adoption-record-v1 and
// reuse-registry anchor with (?![\s\S]) rather than $ (PR #53, to reject a
// trailing newline that Python's re accepts before $); both are validated only
// by the offline Python harness and are deliberately not embedded. Converting
// either to a Go-validated contract requires replacing those anchors first.
package schemas

import _ "embed"

//go:embed report.schema.json
var report1 string

//go:embed report-v2.schema.json
var report2 string

//go:embed report-v4.schema.json
var report4 string

// Report returns an independent copy of a known, bundled contract.
func Report(version string) ([]byte, bool) {
	switch version {
	case "1.0":
		return []byte(report1), true
	case "2.0":
		return []byte(report2), true
	case "4.0":
		return []byte(report4), true
	default:
		return nil, false
	}
}
