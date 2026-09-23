package officeplan

import (
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// OperationCoverage summarizes a completed package-level selection pass and its
// child assessments. The source scope describes selection/processing of the ZIP;
// it does not assert that untargeted parts were text-analyzed. Child outcomes
// remain authoritative for decompressed assessment. Any incomplete child excludes
// its entire compressed representation: sub-part offsets are never translated.
// Diagnostics not attributed to a child denote an unknown enumeration remainder.
// Do not call this for unstarted selection or fatal container/acquisition failure.
func OperationCoverage(base *PackageRecords, coverage *AnalysisCoverage, operation string, config v2.Config, execution int) (v2.Execution, error) {
	fail := func() (v2.Execution, error) { return v2.Execution{}, v4.ErrLinkage }
	if base == nil || coverage == nil || len(base.Evidence.Packages) != 1 || execution < 0 {
		return fail()
	}
	switch operation {
	case capability.OfficeIdentifyID, capability.OfficeRelationshipsID, capability.OfficeMetadataID, capability.OfficeObjectsID, capability.OfficeTextID:
	default:
		return fail()
	}
	pkg := base.Evidence.Packages[0]
	index, err := v4.IndexEvidence(base.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return fail()
	}
	if err := index.ValidateArtifactBindings(base.Artifacts); err != nil {
		return fail()
	}
	e := v2.Execution{Ref: fmt.Sprintf("exec/%d", execution), CapabilityRef: operation, Config: config, State: v2.Completed, RequestedScope: v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "whole_artifact"}, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, DiagnosticRefs: []string{}}
	diagnostics := map[string]v2.Diagnostic{}
	for _, d := range coverage.Diagnostics {
		if d.ExecutionRef == nil || *d.ExecutionRef != e.Ref || diagnostics[d.Ref].Ref != "" {
			return fail()
		}
		if err := d.Validate(); err != nil {
			return fail()
		}
		diagnostics[d.Ref] = d
		e.DiagnosticRefs = append(e.DiagnosticRefs, d.Ref)
	}
	used := map[string]bool{}
	seen := map[string]bool{}
	type gap struct {
		span v4.Span
		code failure.Code
	}
	gaps := []gap{}
	unknown := []failure.Code{}
	for _, o := range coverage.Outcomes {
		if o.Operation != operation || o.ExecutionRef != e.Ref {
			return fail()
		}
		for _, ref := range o.DiagnosticRefs {
			d, ok := diagnostics[ref]
			if !ok || !slices.Contains(o.Codes, string(d.Code)) {
				return fail()
			}
			used[ref] = true
		}
		if o.PartRef != nil {
			p, ok := index.Parts[*o.PartRef]
			if !ok || seen[p.PartRef] {
				return fail()
			}
			seen[p.PartRef] = true
		}
		if o.State == "completed" {
			continue
		}
		if len(o.Codes) == 0 || len(o.DiagnosticRefs) == 0 {
			return fail()
		}
		code := failure.Code(o.Codes[0])
		if o.PartRef == nil {
			unknown = append(unknown, code)
			continue
		}
		span := index.Parts[*o.PartRef].CompressedSpan
		if span.Start == span.End {
			unknown = append(unknown, code)
		} else {
			gaps = append(gaps, gap{span, code})
		}
	}
	for _, d := range coverage.Diagnostics {
		if !used[d.Ref] {
			unknown = append(unknown, d.Code)
		}
	}
	slices.SortFunc(gaps, func(a, b gap) int {
		if a.span.Start < b.span.Start {
			return -1
		}
		if a.span.Start > b.span.Start {
			return 1
		}
		return 0
	})
	spans := make([]v4.Span, 0, len(gaps))
	for _, g := range gaps {
		spans = append(spans, g.span)
	}
	assessed, _, err := partitionCoverage(pkg.SourceByteLength, spans, false, false)
	if err != nil {
		return fail()
	}
	regions := identityRegions(assessed)
	for _, g := range gaps {
		scope := v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(g.span.Start), End: uint64(g.span.End)}}}
		e.Exclusions = append(e.Exclusions, v2.Exclusion{Scope: &scope, ReasonCode: g.code})
	}
	for _, code := range unknown {
		e.Exclusions = append(e.Exclusions, v2.Exclusion{UnknownRemainder: true, ReasonCode: code})
	}
	if len(e.Exclusions) == 0 {
		e.AnalyzedScope = append(e.AnalyzedScope, e.RequestedScope)
	} else {
		e.State = v2.Partial
		reason := e.Exclusions[0].ReasonCode
		e.ReasonCode = &reason
		if len(unknown) > 0 {
			// Only verified ZIP framing was necessarily consumed by selection.
			// Do not credit every payload merely because the missing extent is unknown.
			payloads := []v4.Span{}
			for _, part := range pkg.Parts {
				if part.CompressedSpan.Start < part.CompressedSpan.End {
					payloads = append(payloads, part.CompressedSpan)
				}
			}
			framing, _, err := partitionCoverage(pkg.SourceByteLength, payloads, false, false)
			if err != nil {
				return fail()
			}
			if len(framing) > 0 {
				e.AnalyzedScope = append(e.AnalyzedScope, v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "byte", Regions: identityRegions(framing)})
			}
		} else if len(gaps) == 0 {
			e.AnalyzedScope = append(e.AnalyzedScope, e.RequestedScope)
		} else if len(regions) > 0 {
			e.AnalyzedScope = append(e.AnalyzedScope, v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "byte", Regions: regions})
		}
	}
	if err := e.Validate(); err != nil {
		return v2.Execution{}, err
	}
	// Include existing parse outcomes only to validate their shared part identities;
	// their parent is independent of the operation being summarized here.
	trace := &v4.TraceIndex{Executions: map[string]v2.Execution{}, Diagnostics: diagnostics}
	for _, o := range pkg.Outcomes {
		trace.Executions[o.ExecutionRef] = v2.Execution{CapabilityRef: o.Operation, State: v2.Partial}
	}
	if _, exists := trace.Executions[e.Ref]; exists {
		return fail()
	}
	trace.Executions[e.Ref] = e
	records := base.Evidence
	records.Packages = slices.Clone(records.Packages)
	records.Packages[0].Outcomes = append(slices.Clone(pkg.Outcomes), coverage.Outcomes...)
	if err := validatePartialOutcomes(index, records, trace); err != nil {
		return v2.Execution{}, err
	}
	if err := v4.ValidateCoverage(v4.NativeTrace{Executions: []v2.Execution{e}, Diagnostics: coverage.Diagnostics}, base.Artifacts, index, evidence.Document{}); err != nil {
		return v2.Execution{}, err
	}
	return e, nil
}
