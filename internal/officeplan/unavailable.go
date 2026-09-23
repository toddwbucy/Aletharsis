package officeplan

import (
	"encoding/json"
	"fmt"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

// No bytes are attested when native acquisition cannot run. Use the frozen
// pre-acquisition (flat) report framing; no Office parser was authorized to run.
// Caller-supplied prepared evidence cannot override platform availability.
func unavailableAcquisitionReport(file evidence.File, limits Limits, version string, acquisition v2.Capability) (*ReportOutput, error) {
	catalog, err := capability.Native(version, uint64(limits.Package.SourceBytes))
	if err != nil {
		return nil, err
	}
	data, err := capability.NativeDataRevision()
	if err != nil {
		return nil, err
	}
	file.SHA256 = nil
	file.Size = nil
	file.Parser = nil
	file.Format = "unknown"
	file.MIME = "application/octet-stream"
	file.Basis = "acquisition unavailable"
	r := v4.Report{Version: version, Schema: "4.0", File: file, Evidence: v4.Document{Document: evidence.EmptyDocument(), Office: v4.Evidence{Packages: []v4.Package{}, XML: []v4.XML{}, Scopes: []v4.Scope{}, Metadata: []v4.Metadata{}, Relationships: []v4.Relationship{}, Objects: []v4.Object{}}}, Findings: []v2.Finding{}, Limitations: []string{"Office evidence was not assessed because native acquisition could not run."}, CatalogVersion: capability.CatalogVersion, ProfileAssessments: []struct{}{}, View: v2.View{Name: "audit", FindingCategories: []string{}}, AdapterRuns: []json.RawMessage{}, Trace: v4.NativeTrace{Capabilities: catalog, Executions: []v2.Execution{}, Diagnostics: []v2.Diagnostic{}, Results: []v2.Result{}, Anchors: []v4.Anchor{}}}
	reasonUnavailable := "not_acquired"
	mappingReason := failure.Code("mapping.not_applicable")
	mapping, err := json.Marshal(v2.Mapping{Quality: "unavailable", ReasonCode: &mappingReason})
	if err != nil {
		return nil, err
	}
	representation, err := json.Marshal(v2.Representation{Serialization: "original-bytes/1", Normalization: "none", LineEndings: "preserved"})
	if err != nil {
		return nil, err
	}
	r.Artifacts = []v4.Artifact{{Ref: "artifact/0", Kind: "source", Representation: representation, UnavailableReason: &reasonUnavailable, Parents: []string{}, Mapping: mapping}}
	for i, c := range catalog {
		if c.ID == capability.AcquireID {
			c = acquisition
			r.Trace.Capabilities[i] = c
		}
		config, err := v2.NewNativeConfig(c.Revision, data, c.Limits)
		if err != nil {
			return nil, err
		}
		reason := failure.PrerequisiteFailed
		if c.Participation == v2.Disabled {
			reason = failure.Disabled
		} else if c.Availability.State != "available" {
			if c.Availability.ReasonCode == nil {
				return nil, v4.ErrLinkage
			}
			reason = *c.Availability.ReasonCode
		}
		r.Trace.Executions = append(r.Trace.Executions, v2.Execution{Ref: fmt.Sprintf("exec/%d", i), CapabilityRef: c.ID, Config: config, RequestedScope: v2.Scope{ArtifactRef: "artifact/0", Unit: "whole_artifact"}, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: v2.NotRun, ReasonCode: &reason, DiagnosticRefs: []string{}})
	}
	if err := r.Summarize(); err != nil {
		return nil, err
	}
	encoded, err := r.Encode(limits.Report)
	if err != nil {
		return nil, err
	}
	return &ReportOutput{Report: r, JSON: encoded}, nil
}
