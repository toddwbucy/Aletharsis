package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// V2Options keeps input acquisition and report serialization budgets independent.
// Cancellation is cooperative at native operation boundaries, not a hard timeout.
type V2Options struct {
	InputBytes   int
	ReportLimits identity.Limits
	View         string
}

// V2Output contains a validated serialization and its presentation model. Callers
// own both after return; changes to Report do not rewrite the JSON snapshot.
type V2Output struct {
	Report v2.Report
	JSON   []byte
}

func DefaultV2Options() V2Options {
	return V2Options{InputBytes: MaxBytes, ReportLimits: identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 1000000, Depth: 64}, View: "audit"}
}

// RunV2 performs a fresh native audit. It never upgrades an imported v1 report
// into invented execution coverage and never invokes imported capabilities.
func RunV2(ctx context.Context, path string, options V2Options) (*V2Output, error) {
	return runV2WithReader(ctx, path, options, readSnapshot, nil)
}
func runV2WithReader(ctx context.Context, path string, options V2Options, read func(string, int) ([]byte, error), catalog []v2.Capability) (*V2Output, error) {
	if ctx == nil || options.InputBytes <= 0 || options.InputBytes > MaxBytes || !slices.Contains([]string{"audit", "unicode", "metadata", "structure"}, options.View) {
		return nil, errors.New("invalid native v2 options")
	}
	if _, err := identity.Canonicalize([]byte(`{}`), options.ReportLimits); err != nil {
		return nil, err
	}
	var err error
	if catalog == nil {
		catalog, err = capability.Native(Version, uint64(options.InputBytes))
		if err != nil {
			return nil, err
		}
	}
	trace := &nativeTrace{ctx: ctx, completed: map[string][]evidence.Finding{}}
	// The compiled platform capability is authoritative before acquisition; an
	// unavailable no-atime reader is not called and reported as if it had run.
	canAcquire := false
	for _, c := range catalog {
		if c.ID == capability.AcquireID {
			canAcquire = c.Availability.State == "available" && c.Participation != v2.Disabled
		}
	}
	outcome := Outcome{Report: newReport(path)}
	if canAcquire {
		outcome = inspectWithTrace(path, options.InputBytes, read, trace)
	}
	report, err := assembleV2(outcome, trace, catalog, options)
	if err != nil {
		return nil, err
	}
	encoded, err := report.Encode(options.ReportLimits)
	if err != nil {
		return nil, err
	}
	return &V2Output{Report: report, JSON: encoded}, nil
}
func stage(role v2.Role) int {
	switch role {
	case v2.Acquisition:
		return 0
	case v2.Parser:
		return 1
	default:
		return 2
	}
}
func whole(artifact string) v2.Scope { return v2.Scope{ArtifactRef: artifact, Unit: "whole_artifact"} }
func pointer[T any](v T) *T          { return &v }

func assembleV2(outcome Outcome, trace *nativeTrace, catalog []v2.Capability, options V2Options) (v2.Report, error) {
	legacy := outcome.Report
	r := v2.Report{Version: Version, Schema: "2.0", File: legacy.File, Evidence: legacy.Evidence, Findings: []v2.Finding{}, Limitations: slices.Clone(legacy.Limitations), CatalogVersion: capability.CatalogVersion, ProfileAssessments: []struct{}{}, View: v2.View{Name: "audit", FindingCategories: []string{}}, Graph: v2.Graph{Capabilities: slices.Clone(catalog), Executions: []v2.Execution{}, Artifacts: []v2.Artifact{}, Anchors: []v2.Anchor{}, Results: []v2.Result{}, Diagnostics: []v2.Diagnostic{}}}
	sort.Slice(r.Capabilities, func(i, j int) bool {
		a, b := r.Capabilities[i], r.Capabilities[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Revision < b.Revision
	})
	operations := slices.Clone(r.Capabilities)
	sort.Slice(operations, func(i, j int) bool {
		a, b := operations[i], operations[j]
		if stage(a.Role) != stage(b.Role) {
			return stage(a.Role) < stage(b.Role)
		}
		return a.ID < b.ID
	})
	acquired := legacy.File.SHA256 != nil
	_, parsed := trace.completed[capability.ParseTextID]
	if len(r.Evidence.Texts) > 1 || parsed && len(r.Evidence.Texts) != 1 {
		return v2.Report{}, errors.New("native coordinator requires one literal text segment")
	}
	dataRevision, err := capability.NativeDataRevision()
	if err != nil {
		return v2.Report{}, err
	}
	failureID := ""
	if outcome.Failure != nil {
		failureID = capability.ParseTextID
		if outcome.Failure.Stage() == failure.Acquisition {
			failureID = capability.AcquireID
		}
	}
	for _, c := range operations {
		scope := whole("artifact/0")
		if parsed && c.Role == v2.Analyzer && c.ID != "aletharsis.c2pa.carrier" {
			scope = whole("artifact/1")
		}
		config := v2.UnknownConfig()
		if c.Implementation != nil {
			var err error
			config, err = v2.NewNativeConfig("1", dataRevision, c.Limits)
			if err != nil {
				return v2.Report{}, err
			}
		}
		e := v2.Execution{Ref: fmt.Sprintf("exec/%d", len(r.Executions)), CapabilityRef: c.ID, Config: config, RequestedScope: scope, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, DiagnosticRefs: []string{}, State: v2.NotRun}
		reason, err := capability.NonExecutionReason(c, true, true, true)
		if err != nil {
			return v2.Report{}, err
		}
		_, completed := trace.completed[c.ID]
		switch {
		case reason != nil:
			e.ReasonCode = reason
		case c.ID == failureID:
			e.State = v2.Failed
			e.ReasonCode = pointer(outcome.Failure.Code())
			d, err := outcome.Diagnostic(fmt.Sprintf("diagnostic/%d", len(r.Diagnostics)), e.Ref, &scope)
			if err != nil {
				return v2.Report{}, err
			}
			r.Diagnostics = append(r.Diagnostics, *d)
			e.DiagnosticRefs = append(e.DiagnosticRefs, d.Ref)
		case c.ID == trace.canceled:
			e.State = v2.Canceled
			e.ReasonCode = pointer(trace.stopReason)
			if trace.stopReason == failure.Timeout {
				e.State = v2.Failed
			}
			d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", len(r.Diagnostics)), ExecutionRef: &e.Ref, Stage: failure.Execution, Code: trace.stopReason, Message: "Native audit stopped at an operation boundary.", Scope: &scope, Details: v2.ExecutionDetails{}}
			r.Diagnostics = append(r.Diagnostics, d)
			e.DiagnosticRefs = append(e.DiagnosticRefs, d.Ref)
		case completed:
			e.State = v2.Completed
			e.AnalyzedScope = []v2.Scope{scope}
		default:
			e.ReasonCode = pointer(failure.PrerequisiteFailed)
		}
		r.Executions = append(r.Executions, e)
	}
	source := v2.Artifact{Ref: "artifact/0", Kind: "source", Representation: v2.Representation{Serialization: "original-bytes/1", Normalization: "none", LineEndings: "preserved"}, SHA256: r.File.SHA256, UnavailableReason: pointer("not_acquired"), Parents: []string{}, Mapping: v2.Mapping{Quality: "unavailable", ReasonCode: pointer(failure.Code("mapping.not_applicable"))}}
	if acquired {
		source.ByteLength = pointer(uint64(*r.File.Size))
		source.UnavailableReason = pointer("not_retained")
	}
	r.Artifacts = append(r.Artifacts, source)
	var verified *identity.VerifiedText
	if parsed {
		text := r.Evidence.Texts[0]
		var err error
		verified, err = identity.VerifyText(trace.source, text, options.InputBytes)
		if err != nil {
			return v2.Report{}, err
		}
		var configSHA *string
		for _, e := range r.Executions {
			if e.CapabilityRef == capability.ParseTextID {
				configSHA = e.Config.SHA256
			}
		}
		r.Artifacts = append(r.Artifacts, v2.Artifact{Ref: "artifact/1", Kind: "text", Representation: v2.Representation{Serialization: "unicode-scalars-utf8/1", Encoding: pointer("utf-8"), Normalization: "none", LineEndings: "preserved"}, SHA256: pointer(identity.ExactBytes([]byte(text.Text))), ByteLength: pointer(uint64(len(text.Text))), ContentRef: &v2.ContentRef{Kind: "report_pointer", Pointer: pointer("/evidence/texts/0/text")}, Parents: []string{"artifact/0"}, Transform: &v2.Transform{Operation: "decode", Version: *r.File.Parser, ConfigSHA256: configSHA, Inputs: []string{"artifact/0"}, Exclusions: []v2.Exclusion{}}, Mapping: nativeMapping()})
	}
	if err := assembleFindings(&r, outcome, trace, verified); err != nil {
		return v2.Report{}, err
	}
	for i, e := range r.Executions {
		observations, ran := trace.completed[e.CapabilityRef]
		if !ran || e.State != v2.Completed {
			continue
		}
		c := operations[i]
		if c.Role != v2.Analyzer {
			continue
		}
		outcome := "no_observations"
		if len(observations) > 0 {
			outcome = "observations_present"
		}
		anchors := []string{}
		for _, a := range r.Anchors {
			if a.ExecutionRef != nil && *a.ExecutionRef == e.Ref {
				anchors = append(anchors, a.Ref)
			}
		}
		r.Results = append(r.Results, v2.Result{Ref: fmt.Sprintf("result/%d", len(r.Results)), ExecutionRef: e.Ref, Kind: "structural_scan", ContractVersion: "1", AnchorRefs: anchors, Payload: v2.StructuralPayload{Operation: c.ID, Scope: e.RequestedScope, Outcome: outcome}, Limitations: []string{}})
	}
	r.Status, err = r.Graph.AggregateStatus(r.File, r.Evidence)
	if err != nil {
		return v2.Report{}, err
	}
	r.Recount()
	r, err = r.SelectView(options.View)
	if err != nil {
		return v2.Report{}, err
	}
	return r, nil
}
func nativeMapping() v2.Mapping {
	return v2.Mapping{Quality: "exact", FromArtifactRef: "artifact/1", ToArtifactRef: "artifact/0", Method: "scalar-to-source-byte-boundaries", Version: "1", DataRef: pointer("/evidence/texts/0/byte_offsets")}
}

// canonicalKey is used only on small bounded, trusted native ordering records.
func canonicalKey(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := identity.Canonicalize(raw, v2.RecordLimits())
	return string(canonical), err
}
