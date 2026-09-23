package officeplan

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/scopelimits"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

// AnalysisCoverage retains producer gaps in decompressed part coordinates.
// It does not allocate parent executions or claim source-byte equivalence.
type AnalysisCoverage struct {
	Outcomes    []v4.Outcome
	Diagnostics []v2.Diagnostic
	Limitations []string
}
type analysisGap struct {
	code, detail string
	span         *v4.Span
}

// CollectAnalysisCoverage translates text/metadata producer outcomes. Callers
// must also account for untargeted parts and enumeration/identification gaps.
// Regions describe the producer's assessment, not authorization to edit them.
func CollectAnalysisCoverage(ctx context.Context, base *PackageRecords, analysis *Analysis, executions map[string]int, firstDiagnostic int) (*AnalysisCoverage, error) {
	if ctx == nil || base == nil || analysis == nil || len(base.Evidence.Packages) != 1 || firstDiagnostic < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &AnalysisCoverage{Outcomes: []v4.Outcome{}, Diagnostics: []v2.Diagnostic{}, Limitations: []string{}}
	pkg := base.Evidence.Packages[0]
	index, err := v4.IndexEvidence(base.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	parts := map[string]v4.Part{}
	for _, part := range pkg.Parts {
		parts[part.Name] = part
	}
	records := map[string]map[string]PartAnalysis{}
	for _, a := range analysis.Parts {
		if a.Operation != capability.OfficeTextID && a.Operation != capability.OfficeMetadataID {
			return nil, v4.ErrLinkage
		}
		if _, exists := parts[a.Part]; !exists && a.Declaration == nil {
			return nil, v4.ErrLinkage
		}
		if records[a.Operation] == nil {
			records[a.Operation] = map[string]PartAnalysis{}
		}
		if _, ok := records[a.Operation][a.Part]; ok {
			return nil, v4.ErrLinkage
		}
		records[a.Operation][a.Part] = a
	}
	for _, op := range []string{capability.OfficeMetadataID, capability.OfficeTextID} {
		names := []string{}
		for name := range records[op] {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			a := records[op][name]
			part, present := parts[name]
			ordinal, exists := executions[op]
			if !exists || ordinal < 0 {
				return nil, v4.ErrLinkage
			}
			exec := fmt.Sprintf("exec/%d", ordinal)
			if !present {
				outcome, diagnostic, err := absentTargetCoverage(a, parts, exec, firstDiagnostic+len(result.Diagnostics))
				if err != nil {
					return nil, err
				}
				result.Outcomes = append(result.Outcomes, outcome)
				result.Diagnostics = append(result.Diagnostics, diagnostic)
				continue
			}
			outcome := v4.Outcome{Operation: op, ExecutionRef: exec, PartRef: &part.PartRef, State: a.State, Codes: []string{}, DiagnosticRefs: []string{}, Assessed: []v4.Span{}, Excluded: []v4.Span{}}
			gaps, limitations, err := partAnalysisGaps(a, part)
			if err != nil {
				return nil, err
			}
			result.Limitations = append(result.Limitations, limitations...)
			for _, gap := range gaps {
				diagnostic := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", firstDiagnostic+len(result.Diagnostics)), ExecutionRef: &outcome.ExecutionRef,
					Stage: failure.Parsing, Code: failure.Code(gap.code), Message: "Office part analysis retained an explicit coverage gap.", Details: v2.ErrorDetails{ErrorType: gap.detail}}
				if gap.span != nil {
					if part.ByteLength == nil || !gap.span.Within(*part.ByteLength) {
						return nil, v4.ErrLinkage
					}
					if gap.span.Start < gap.span.End {
						diagnostic.Scope = &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(gap.span.Start), End: uint64(gap.span.End)}}}
						outcome.Excluded = append(outcome.Excluded, *gap.span)
					}
				} else {
					diagnostic.Scope = &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "whole_artifact"}
				}
				if err := diagnostic.Validate(); err != nil {
					return nil, err
				}
				if !slices.Contains(outcome.Codes, gap.code) {
					outcome.Codes = append(outcome.Codes, gap.code)
				}
				outcome.DiagnosticRefs = append(outcome.DiagnosticRefs, diagnostic.Ref)
				result.Diagnostics = append(result.Diagnostics, diagnostic)
			}
			switch a.State {
			case "completed", "partial":
				if part.ByteLength == nil {
					return nil, v4.ErrLinkage
				}
				if (a.State == "partial") != (len(gaps) > 0) {
					return nil, v4.ErrLinkage
				}
				var err error
				outcome.Assessed, outcome.Excluded, err = partitionCoverage(*part.ByteLength, outcome.Excluded, true, true)
				if err != nil {
					return nil, err
				}
			case "failed", "not_run", "canceled":
				if part.ByteLength != nil {
					outcome.Excluded = []v4.Span{{Start: 0, End: *part.ByteLength}}
				}
			default:
				return nil, v4.ErrLinkage
			}
			result.Outcomes = append(result.Outcomes, outcome)
		}
	}
	slices.Sort(result.Limitations)
	result.Limitations = slices.Compact(result.Limitations)
	// Validate outcome linkage with temporary parent slots. Actual execution
	// coverage/state is assembled by the host; partial permits mixed children.
	trace := &v4.TraceIndex{Executions: map[string]v2.Execution{}, Diagnostics: map[string]v2.Diagnostic{}}
	evidence := base.Evidence
	evidence.Packages = slices.Clone(evidence.Packages)
	evidence.Packages[0].Outcomes = append(slices.Clone(pkg.Outcomes), result.Outcomes...)
	for _, o := range evidence.Packages[0].Outcomes {
		if prior, ok := trace.Executions[o.ExecutionRef]; ok && prior.CapabilityRef != o.Operation {
			return nil, v4.ErrLinkage
		}
		trace.Executions[o.ExecutionRef] = v2.Execution{CapabilityRef: o.Operation, State: v2.Partial}
	}
	for _, d := range result.Diagnostics {
		trace.Diagnostics[d.Ref] = d
	}
	if err := validatePartialOutcomes(index, evidence, trace); err != nil {
		return nil, err
	}
	return result, nil
}

func partAnalysisGaps(a PartAnalysis, part v4.Part) ([]analysisGap, []string, error) {
	gaps := []analysisGap{}
	limitations := []string{}
	add := func(code, detail string, span xmlparts.Span) {
		s := v4.Span{Start: span.Start, End: span.End}
		gaps = append(gaps, analysisGap{code, detail, &s})
	}
	element := func(mapped *xmlparts.MappedDocument, code string, n int) error {
		if mapped == nil || mapped.Document == nil || n < 0 || n >= len(mapped.Document.Elements) {
			return v4.ErrLinkage
		}
		add(code, "OfficeExtractionGap", mapped.Document.Elements[n].Full)
		return nil
	}
	omitted := func(items []scopelimits.Omission) {
		for _, o := range items {
			add(string(o.Code), o.Dimension, o.Source)
		}
	}
	if a.Error != nil {
		if a.State != "failed" && a.State != "not_run" && a.State != "canceled" {
			return nil, nil, v4.ErrLinkage
		}
		if a.Word != nil || a.ODT != nil || a.Metadata != nil {
			return nil, nil, v4.ErrLinkage
		}
		code := failure.ExecutionFailed
		switch {
		case errors.Is(a.Error, context.Canceled):
			code = failure.Canceled
		case errors.Is(a.Error, context.DeadlineExceeded):
			code = failure.Timeout
		case errors.Is(a.Error, xmlparts.ErrLimit):
			code = failure.ResourceLimit
		case a.State == "not_run":
			code = failure.PrerequisiteFailed
		}
		if a.Code != "" {
			code = failure.Code(a.Code)
		}
		return []analysisGap{{code: string(code), detail: "OfficeAnalysisUnavailable"}}, limitations, nil
	}
	check := func(mapped *xmlparts.MappedDocument, state string) bool {
		return mapped != nil && mapped.Document != nil && part.SHA256 != nil && mapped.Document.PartSHA256 == *part.SHA256 && state == a.State
	}
	count := 0
	if a.Word != nil {
		count++
		r := a.Word
		if a.Operation != capability.OfficeTextID || r.Extraction == nil || !check(r.Extraction.XML, r.State) {
			return nil, nil, v4.ErrLinkage
		}
		for _, i := range r.Extraction.Issues {
			if err := element(r.Extraction.XML, i.Code, i.Element); err != nil {
				return nil, nil, err
			}
		}
		omitted(r.Omitted)
		limitations = append(limitations, r.Limitations...)
		limitations = append(limitations, r.Extraction.Limitations...)
	}
	if a.ODT != nil {
		count++
		r := a.ODT
		if a.Operation != capability.OfficeTextID || r.Extraction == nil || !check(r.Extraction.XML, r.State) {
			return nil, nil, v4.ErrLinkage
		}
		for _, i := range r.Extraction.Issues {
			if err := element(r.Extraction.XML, i.Code, i.Element); err != nil {
				return nil, nil, err
			}
		}
		omitted(r.Omitted)
		for _, b := range r.Boundaries {
			if b.Reason == "control_expansion_limit" {
				if b.Token < 0 || b.Token >= len(r.Extraction.XML.Document.Tokens) {
					return nil, nil, v4.ErrLinkage
				}
				add(string(failure.ResourceLimit), "ODTControlExpansionLimit", r.Extraction.XML.Document.Tokens[b.Token].Span)
			}
		}
		limitations = append(limitations, r.Limitations...)
		limitations = append(limitations, r.Extraction.Limitations...)
	}
	if a.Metadata != nil {
		count++
		r := a.Metadata
		if a.Operation != capability.OfficeMetadataID || !check(r.XML, r.State) {
			return nil, nil, v4.ErrLinkage
		}
		for _, i := range r.Issues {
			add(i.Code, "OfficeMetadataGap", i.Span)
		}
		limitations = append(limitations, r.Limitations...)
	}
	if count != 1 {
		return nil, nil, v4.ErrLinkage
	}
	return gaps, limitations, nil
}

// An absent target has no package part identity or assessed bytes. Locate the
// gap in its verified declaration instead; the original relationship preserves
// the target string and its resolution evidence.
func absentTargetCoverage(a PartAnalysis, parts map[string]v4.Part, exec string, ordinal int) (v4.Outcome, v2.Diagnostic, error) {
	fail := func() (v4.Outcome, v2.Diagnostic, error) {
		return v4.Outcome{}, v2.Diagnostic{}, v4.ErrLinkage
	}
	if a.State != "not_run" || a.Code == "" || a.Error == nil || a.Word != nil || a.ODT != nil || a.Metadata != nil || a.Declaration == nil {
		return fail()
	}
	declaration := a.Declaration
	part, ok := parts[declaration.Part]
	span := v4.Span{Start: declaration.Span.Start, End: declaration.Span.End}
	if !ok || part.SHA256 == nil || *part.SHA256 != declaration.PartSHA256 || part.ByteLength == nil || !span.Within(*part.ByteLength) || span.Start == span.End {
		return fail()
	}
	diagnostic := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", ordinal), ExecutionRef: &exec, Stage: failure.Parsing, Code: failure.Code(a.Code),
		Message: "Declared Office target is absent: " + a.Part, Details: v2.ErrorDetails{ErrorType: "OfficeTargetAbsent"},
		Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(span.Start), End: uint64(span.End)}}}}
	if err := diagnostic.Validate(); err != nil {
		return fail()
	}
	outcome := v4.Outcome{Operation: a.Operation, ExecutionRef: exec, State: "not_run", Codes: []string{a.Code}, DiagnosticRefs: []string{diagnostic.Ref}, Assessed: []v4.Span{}, Excluded: []v4.Span{}}
	return outcome, diagnostic, nil
}
