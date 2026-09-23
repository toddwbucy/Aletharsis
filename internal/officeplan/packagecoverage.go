package officeplan

import (
	"fmt"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// PackageCoverage constructs the source-coordinate coverage of package parsing.
// It does not describe XML/text analysis. Unverified compressed payloads are
// excluded; verified payloads and the inspected ZIP framing remain assessed.
// Empty unavailable payloads have an unknown remainder: their verification gap
// cannot honestly be represented as a positive source-byte interval.
// The caller owns configuration identity and graph ordinal allocation.
func PackageCoverage(records *PackageRecords, config v2.Config, execution, firstDiagnostic int) (v2.Execution, []v2.Diagnostic, error) {
	fail := func() (v2.Execution, []v2.Diagnostic, error) { return v2.Execution{}, nil, v4.ErrLinkage }
	if records == nil || len(records.Evidence.Packages) != 1 || execution < 0 || firstDiagnostic < 0 {
		return fail()
	}
	p := records.Evidence.Packages[0]
	index, err := v4.IndexEvidence(records.Evidence, p.SourceSHA256, p.SourceByteLength)
	if err != nil {
		return fail()
	}
	if err := index.ValidateArtifactBindings(records.Artifacts); err != nil {
		return fail()
	}
	e := v2.Execution{Ref: fmt.Sprintf("exec/%d", execution), CapabilityRef: capability.ParseOfficeID, Config: config,
		RequestedScope: v2.Scope{ArtifactRef: p.SourceArtifactRef, Unit: "whole_artifact"},
		AnalyzedScope:  []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: v2.State(p.State), DiagnosticRefs: []string{}}
	if err := validatePartialOutcomes(index, records.Evidence, &v4.TraceIndex{Executions: map[string]v2.Execution{e.Ref: e}}); err != nil {
		return fail()
	}
	diagnostics := []v2.Diagnostic{}
	gaps := []identity.Region{}
	for i, part := range p.Parts {
		if part.State == "completed" {
			continue
		}
		// BuildPackageRecords emits a parse outcome for each part in canonical order.
		if i >= len(p.Outcomes) || p.Outcomes[i].PartRef == nil || *p.Outcomes[i].PartRef != part.PartRef || len(p.Outcomes[i].Codes) != 1 {
			return fail()
		}
		code := failure.Code(p.Outcomes[i].Codes[0])
		d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", firstDiagnostic+len(diagnostics)), ExecutionRef: &e.Ref,
			Stage: failure.Parsing, Code: code, Message: "Package payload was not verified; retained identity is compressed source evidence only.",
			Details: v2.ErrorDetails{ErrorType: "OfficePartUnavailable"}}
		x := v2.Exclusion{ReasonCode: code}
		span := part.CompressedSpan
		if span.Start == span.End {
			x.UnknownRemainder = true
		} else {
			region := identity.Region{Start: uint64(span.Start), End: uint64(span.End)}
			scope := v2.Scope{ArtifactRef: p.SourceArtifactRef, Unit: "byte", Regions: []identity.Region{region}}
			x.Scope = &scope
			d.Scope = &scope
			gaps = append(gaps, region)
		}
		if err := d.Validate(); err != nil {
			return fail()
		}
		diagnostics = append(diagnostics, d)
		e.DiagnosticRefs = append(e.DiagnosticRefs, d.Ref)
		e.Exclusions = append(e.Exclusions, x)
		if e.ReasonCode == nil {
			reason := code
			e.ReasonCode = &reason
		}
	}
	if len(e.Exclusions) == 0 {
		e.AnalyzedScope = append(e.AnalyzedScope, e.RequestedScope)
	} else {
		spans := make([]v4.Span, 0, len(gaps))
		for _, gap := range gaps {
			spans = append(spans, v4.Span{Start: int64(gap.Start), End: int64(gap.End)})
		}
		assessed, _, err := partitionCoverage(p.SourceByteLength, spans, false, false)
		if err != nil {
			return fail()
		}
		regions := identityRegions(assessed)
		if len(regions) > 0 {
			e.AnalyzedScope = append(e.AnalyzedScope, v2.Scope{ArtifactRef: p.SourceArtifactRef, Unit: "byte", Regions: regions})
		}
	}
	if err := e.Validate(); err != nil {
		return v2.Execution{}, nil, err
	}
	if err := v4.ValidateCoverage(v4.NativeTrace{Executions: []v2.Execution{e}, Diagnostics: diagnostics}, records.Artifacts, index, evidence.Document{}); err != nil {
		return v2.Execution{}, nil, err
	}
	return e, diagnostics, nil
}
