package v4

import (
	"encoding/json"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

// validateOfficeReport connects the Office-specific semantic stages after
// closed wire decoding. It never acquires a source or follows a relationship.
func validateOfficeReport(r Report) error {
	if r.File.SHA256 == nil || r.File.Size == nil || len(r.Evidence.Texts) != 0 || len(r.Evidence.Office.Packages) == 0 {
		return ErrLinkage
	}
	office, err := IndexEvidence(r.Evidence.Office, *r.File.SHA256, int64(*r.File.Size))
	if err != nil {
		return err
	}
	trace, err := IndexTrace(r.Trace)
	if err != nil {
		return err
	}
	roles := map[v2.Role]bool{}
	for _, c := range r.Trace.Capabilities {
		roles[c.Role] = true
	}
	if !roles[v2.Acquisition] || !roles[v2.Parser] {
		return ErrLinkage
	}
	artifacts, err := validateArtifacts(r, office)
	if err != nil {
		return err
	}
	if err := ValidateCoverage(r.Trace, r.Artifacts, office, r.Evidence.Document); err != nil {
		return err
	}
	if err := office.ValidateOutcomes(r.Evidence.Office, trace); err != nil {
		return err
	}
	coverage := coverageIndex{artifacts: artifacts, office: office, flat: r.Evidence.Document}
	objects := map[string]Object{}
	for _, o := range r.Evidence.Office.Objects {
		objects[o.ObjectRef] = o
	}
	for _, a := range r.Trace.Anchors {
		mapping, err := v2.DecodeMapping(a.Mapping)
		if err != nil {
			return err
		}
		if err := coverage.mapping(mapping); err != nil {
			return err
		}
		switch a.Kind {
		case "office_scope", "office_structure":
			if err := office.ValidateOfficeAnchor(a, artifacts, objects); err != nil {
				return err
			}
		case "legacy_unknown":
			raw, err := json.Marshal(a)
			if err != nil {
				return err
			}
			decoded, err := v2.DecodeAnchor(raw)
			if err != nil {
				return err
			}
			locator, ok := decoded.Locator.(v2.LegacyLocator)
			if !ok {
				return ErrLinkage
			}
			if _, ok := trace.Diagnostics[locator.DiagnosticRef]; !ok {
				return ErrLinkage
			}
		default:
			return ErrLinkage
		}
	}
	if err := validateOfficeFindings(r, trace, office); err != nil {
		return err
	}
	return validateSummary(r, trace)
}
