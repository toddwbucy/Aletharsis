package v4

import (
	"encoding/json"
	"errors"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

type Document struct {
	evidence.Document
	Office Evidence `json:"office"`
}

// Report is the decoded native 4.0 view. Construction alone is not validation.
// Use DecodeReport at the import boundary; records alone are not validation.
type Report struct {
	Version            string            `json:"aletharsis_version"`
	Schema             string            `json:"schema_version"`
	File               evidence.File     `json:"file"`
	Status             v2.State          `json:"status"`
	Summary            map[string]int    `json:"summary"`
	Evidence           Document          `json:"evidence"`
	Findings           []v2.Finding      `json:"findings"`
	Limitations        []string          `json:"limitations"`
	CatalogVersion     string            `json:"catalog_version"`
	ProfileAssessments []struct{}        `json:"profile_assessments"`
	View               v2.View           `json:"view"`
	Trace              NativeTrace       `json:"-"`
	Artifacts          []Artifact        `json:"artifacts"`
	AdapterRuns        []json.RawMessage `json:"adapter_runs"`
}

var ErrAdapterUnsupported = errors.New("native 4.0 adapter semantics unavailable")

// decodeReportRecords checks wire shape before bounded native record decoding.
// It is deliberately private: successful record decoding is not full semantic
// validation, and must never be exposed as a validated import by itself.
func decodeReportRecords(raw []byte, limits identity.Limits) (Report, error) {
	canonical, err := wire.Validate("4.0", raw, limits)
	if err != nil {
		return Report{}, err
	}
	var r Report
	if err := json.Unmarshal(canonical, &r); err != nil {
		return Report{}, err
	}
	if len(r.AdapterRuns) != 0 {
		return Report{}, ErrAdapterUnsupported
	}
	var records struct {
		Capabilities []json.RawMessage `json:"capabilities"`
		Executions   []json.RawMessage `json:"executions"`
		Diagnostics  []json.RawMessage `json:"diagnostics"`
		Results      []json.RawMessage `json:"results"`
		Anchors      []Anchor          `json:"anchors"`
	}
	if err := json.Unmarshal(canonical, &records); err != nil {
		return Report{}, err
	}
	if r.Trace.Capabilities, err = decodeRecords(records.Capabilities, v2.DecodeCapability); err != nil {
		return Report{}, err
	}
	if r.Trace.Executions, err = decodeRecords(records.Executions, v2.DecodeExecution); err != nil {
		return Report{}, err
	}
	if r.Trace.Diagnostics, err = decodeRecords(records.Diagnostics, v2.DecodeDiagnostic); err != nil {
		return Report{}, err
	}
	if r.Trace.Results, err = decodeRecords(records.Results, v2.DecodeResult); err != nil {
		return Report{}, err
	}
	r.Trace.Anchors = records.Anchors
	return r, nil
}
func decodeRecords[T any](raw []json.RawMessage, decode func([]byte) (T, error)) ([]T, error) {
	result := make([]T, 0, len(raw))
	for _, record := range raw {
		value, err := decode(record)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

// DecodeReport validates bounded native 4.0 evidence without acquiring source
// bytes. Internal consistency does not authenticate source correspondence.
func DecodeReport(raw []byte, limits identity.Limits) (Report, error) {
	r, err := decodeReportRecords(raw, limits)
	if err != nil {
		return Report{}, err
	}
	office := r.Evidence.Office
	hasOffice := len(office.Packages)+len(office.XML)+len(office.Scopes)+len(office.Metadata)+len(office.Relationships)+len(office.Objects) > 0
	if hasOffice {
		if err := validateOfficeReport(r); err != nil {
			return Report{}, err
		}
	} else {
		// A native flat 4.0 report retains the exact frozen 2.0 semantics. Only
		// empty additive fields and the version selector differ in this view.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return Report{}, err
		}
		fields["schema_version"] = json.RawMessage(`"2.0"`)
		delete(fields, "adapter_runs")
		var document map[string]json.RawMessage
		if err := json.Unmarshal(fields["evidence"], &document); err != nil {
			return Report{}, err
		}
		delete(document, "office")
		fields["evidence"], err = json.Marshal(document)
		if err != nil {
			return Report{}, err
		}
		legacy, err := json.Marshal(fields)
		if err != nil {
			return Report{}, err
		}
		if _, err := v2.DecodeReport(legacy, limits); err != nil {
			return Report{}, err
		}
	}
	return r, nil
}
