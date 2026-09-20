// Package schemas bundles the published wire contracts. No report-supplied URL
// participates in schema selection or resource loading.
package schemas

import _ "embed"

//go:embed report.schema.json
var report1 string

//go:embed report-v2.schema.json
var report2 string

// Report returns an independent copy of a known, bundled contract.
func Report(version string) ([]byte, bool) {
	switch version {
	case "1.0":
		return []byte(report1), true
	case "2.0":
		return []byte(report2), true
	default:
		return nil, false
	}
}
