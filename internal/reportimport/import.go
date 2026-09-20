// Package reportimport preserves exact report artifacts separately from decoded
// evidence. It neither opens source paths nor invokes capabilities in a report.
package reportimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

type Diagnostic struct {
	Code          string  `json:"code"`
	ReportPointer string  `json:"report_pointer"`
	FailureCode   *string `json:"failure_code"`
}

// Imported owns its original bytes; Bytes returns a defensive copy. The decoded
// V2 report is a separate, mutable consumer view, never the artifact's identity.
type Imported struct {
	ReportArtifactSHA256 string
	Status               string
	Coverage             string
	V2                   *v2.Report
	AdapterDiagnostics   []Diagnostic
	original             []byte
}

func (r Imported) Bytes() []byte { return bytes.Clone(r.original) }

// Read retains a valid report under explicit caller budgets. Unsupported versions
// are inert bytes with unknown coverage. Malformed known versions return an error,
// not partial evidence; callers may independently quarantine their input bytes.
func Read(raw []byte, limits identity.Limits) (Imported, error) {
	if limits.InputBytes <= 0 || len(raw) > limits.InputBytes {
		return Imported{}, identity.ErrLimit
	}
	digest := identity.ExactBytes(raw)
	canonical, err := identity.Canonicalize(raw, limits)
	if err != nil {
		return Imported{}, err
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &header); err != nil {
		return Imported{}, errors.New("report version required")
	}
	var version string
	if err := json.Unmarshal(header["schema_version"], &version); err != nil || version == "" {
		return Imported{}, errors.New("report version required")
	}
	result := Imported{ReportArtifactSHA256: digest, Coverage: "unknown", AdapterDiagnostics: []Diagnostic{}}
	switch version {
	case "2.0":
		report, err := v2.DecodeReport(raw, limits)
		if err != nil {
			return Imported{}, err
		}
		result.Status = "validated"
		result.Coverage = "declared"
		result.V2 = &report
	case "1.0":
		canonical, err = wire.Validate("1.0", raw, limits)
		if err != nil {
			return Imported{}, err
		}
		var legacy struct {
			Findings []struct {
				ID string `json:"id"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(canonical, &legacy); err != nil {
			return Imported{}, err
		}
		result.Status = "legacy"
		for i, f := range legacy.Findings {
			if f.ID == "parser.failure" {
				result.AdapterDiagnostics = append(result.AdapterDiagnostics, Diagnostic{Code: "legacy.failure_code_unavailable", ReportPointer: findingPointer(i)})
			}
		}
	default:
		result.Status = "unsupported_version"
	}
	result.original = bytes.Clone(raw)
	return result, nil
}

func findingPointer(i int) string { return "/findings/" + strconv.Itoa(i) }
