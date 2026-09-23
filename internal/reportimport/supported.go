package reportimport

import (
	"bytes"
	"encoding/json"
	"errors"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Supported adds explicit runtime support information without extending the
// legacy Imported status contract. Bytes always returns the original artifact.
type Supported struct {
	Imported
	SupportReason *string
	V4            *v4.Report
}

// ReadSupported is a superset of Read. Unknown versions and unsupported adapter
// semantics retain bounded exact bytes but never expose a validated decoded view.
func ReadSupported(raw []byte, limits identity.Limits) (Supported, error) {
	canonical, err := identity.Canonicalize(raw, limits)
	if err != nil {
		return Supported{}, err
	}
	var header struct {
		Version string `json:"schema_version"`
	}
	if json.Unmarshal(canonical, &header) != nil || header.Version == "" {
		return Supported{}, errors.New("report version required")
	}
	if header.Version == "1.0" || header.Version == "2.0" {
		imported, err := Read(raw, limits)
		if err != nil {
			return Supported{}, err
		}
		return Supported{Imported: imported}, nil
	}
	result := Supported{Imported: Imported{ReportArtifactSHA256: identity.ExactBytes(raw), Status: "unsupported_version", Coverage: "unknown", AdapterDiagnostics: []Diagnostic{}, original: bytes.Clone(raw)}}
	reason := "unknown_version"
	switch header.Version {
	case "3.0":
		reason = "known_version_unimplemented"
	case "4.0":
		report, err := v4.DecodeReport(raw, limits)
		if errors.Is(err, v4.ErrAdapterUnsupported) {
			result.Status = "unsupported_feature"
			reason = "known_feature_unimplemented"
		} else if err != nil {
			return Supported{}, err
		} else {
			result.Status = "validated"
			result.Coverage = "declared"
			result.V4 = &report
			return result, nil
		}
	}
	result.SupportReason = &reason
	return result, nil
}
