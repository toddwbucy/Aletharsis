package officeplan

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

type ReportOutput struct {
	Report v4.Report
	JSON   []byte
}

// BuildReport assembles a fresh native audit from an acquired snapshot and the
// producers that inspected it. Conflicting/unidentified containers retain their
// evidence with explicit unrun dependent operations. Unavailable acquisition
// emits pre-acquisition framing without attesting the supplied bytes. Acquisition
// I/O failures and preparation interrupted before identification need separate framing.
// The caller must have acquired the bytes using the declared native acquisition
// capability; this function neither reads a file nor upgrades imported evidence.
func BuildReport(ctx context.Context, source []byte, file evidence.File, p *Prepared, a *Analysis, version string) (*ReportOutput, error) {
	if ctx == nil || p == nil || a == nil || file.SHA256 == nil || file.Size == nil || *file.Size != len(source) || *file.SHA256 != evidence.Hash(source) {
		return nil, v4.ErrLinkage
	}
	catalog, err := p.Limits.Catalog(version)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return buildReportWithCatalog(ctx, source, file, p, a, version, catalog)
}

func buildReportWithCatalog(ctx context.Context, source []byte, file evidence.File, p *Prepared, a *Analysis, version string, catalog []v2.Capability) (*ReportOutput, error) {
	descriptors := map[string]v2.Capability{}
	configs := map[string]v2.Config{}
	data, err := capability.NativeDataRevision()
	if err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}
	for _, c := range catalog {
		descriptors[c.ID] = c
		config, err := v2.NewNativeConfig(c.Revision, data, c.Limits)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		configs[c.ID] = config
	}
	acquisition := descriptors[capability.AcquireID]
	if acquisition.Availability.State != "available" || acquisition.Participation == v2.Disabled {
		return unavailableAcquisitionReport(file, p.Limits, version, catalog)
	}
	nextExecution := 0
	reserve := func(count int) int { first := nextExecution; nextExecution += count; return first }
	acquireOrdinal, parseOrdinal := reserve(1), reserve(1)
	identityOrdinal, objectOrdinal := reserve(1), reserve(1)
	profileOrdinal := reserve(1)
	partOrdinal := reserve(len(partOperations))
	assembly, err := Assemble(ctx, source, p, a, parseOrdinal, objectOrdinal)
	if err != nil {
		return nil, fmt.Errorf("assembly: %w", err)
	}
	partEvidence := assembly.Evidence
	partEvidence.Objects = nil
	base := &PackageRecords{Evidence: partEvidence, Artifacts: assembly.Artifacts}
	parse, parseDiagnostics, err := PackageCoverage(base, configs[capability.ParseOfficeID], parseOrdinal, 0)
	if err != nil {
		return nil, fmt.Errorf("parse, parseDiagnostics: %w", err)
	}
	identification, err := BuildIdentityGraph(ctx, p, assembly, descriptors[capability.OfficeIdentifyID], configs[capability.OfficeIdentifyID], identityOrdinal, len(parseDiagnostics))
	if err != nil {
		return nil, fmt.Errorf("identification: %w", err)
	}
	nextDiagnostic := len(parseDiagnostics) + len(identification.Trace.Diagnostics)
	objects, err := BuildObjectGraph(ctx, p, a, assembly, descriptors[capability.OfficeObjectsID], configs[capability.OfficeObjectsID], objectOrdinal, nextDiagnostic)
	if err != nil {
		return nil, fmt.Errorf("objects: %w", err)
	}
	nextDiagnostic += len(objects.Trace.Diagnostics)
	parts, err := BuildPartGraph(ctx, p, a, assembly, catalog, configs, partOrdinal, nextDiagnostic)
	if err != nil {
		return nil, fmt.Errorf("parts: %w", err)
	}
	nextDiagnostic += len(parts.Trace.Diagnostics)
	scopes, err := BuildScopeGraph(ctx, assembly, catalog, configs, TraceOffsets{Execution: nextExecution}, p.Limits.Report)
	if err != nil {
		return nil, fmt.Errorf("scopes: %w", err)
	}
	reserve(len(scopes.Trace.Executions))
	pkg := assembly.Evidence.Packages[0]
	parser := pkg.ParserVersion
	file.Format = pkg.Format
	file.Parser = &parser
	file.Basis = "verified package declarations and namespace-qualified content"
	switch p.Identity {
	case IdentityDOCX:
		file.MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case IdentityODT:
		file.MIME = "application/vnd.oasis.opendocument.text"
	default:
		file.MIME = "application/zip"
		file.Basis = "verified ZIP container; Office format identity is " + string(p.Identity)
	}
	requested := v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "whole_artifact"}
	acquire := v2.Execution{Ref: fmt.Sprintf("exec/%d", acquireOrdinal), CapabilityRef: capability.AcquireID, Config: configs[capability.AcquireID], RequestedScope: requested, AnalyzedScope: []v2.Scope{requested}, Exclusions: []v2.Exclusion{}, State: v2.Completed, DiagnosticRefs: []string{}}
	report := v4.Report{Version: version, Schema: "4.0", File: file, Evidence: v4.Document{Document: evidence.EmptyDocument(), Office: assembly.Evidence}, Findings: scopes.Findings, Limitations: []string{}, CatalogVersion: capability.OfficeCatalogVersion, ProfileAssessments: []struct{}{}, View: v2.View{Name: "audit", FindingCategories: []string{}}, Artifacts: assembly.Artifacts, AdapterRuns: []json.RawMessage{}, Trace: v4.NativeTrace{Capabilities: catalog, Executions: []v2.Execution{acquire, parse}, Diagnostics: parseDiagnostics, Results: []v2.Result{}, Anchors: []v4.Anchor{}}}
	for _, fragment := range []v4.NativeTrace{identification.Trace, objects.Trace, parts.Trace, scopes.Trace} {
		report.Trace.Executions = append(report.Trace.Executions, fragment.Executions...)
		report.Trace.Diagnostics = append(report.Trace.Diagnostics, fragment.Diagnostics...)
		report.Trace.Results = append(report.Trace.Results, fragment.Results...)
		report.Trace.Anchors = append(report.Trace.Anchors, fragment.Anchors...)
	}
	report.Evidence.Office.Objects = objects.Objects
	report.Evidence.Office.Packages = slices.Clone(report.Evidence.Office.Packages)
	report.Evidence.Office.Packages[0].Issues = identification.Issues
	outcomes, err := bindParseDiagnostics(pkg, parseDiagnostics)
	if err != nil {
		return nil, err
	}
	report.Evidence.Office.Packages[0].Outcomes = append(append(outcomes, identification.Outcomes...), parts.Outcomes...)
	report.Limitations = append(report.Limitations, parts.Limitations...)
	report.Limitations = append(report.Limitations, objects.Limitations...)
	planned := map[string]bool{}
	for _, e := range report.Trace.Executions {
		planned[e.CapabilityRef] = true
	}

	for _, c := range catalog {
		if planned[c.ID] {
			continue
		}
		reason := failure.UnsupportedInput
		ordinal := profileOrdinal
		if c.ID == capability.OfficeProfilesID {
			reason = "profile.no_applicable_profile"
		} else {
			ordinal = reserve(1)
		}
		if c.Participation == v2.Disabled {
			reason = failure.Disabled
		} else if c.Availability.State != "available" {
			if c.Availability.ReasonCode == nil {
				return nil, v4.ErrLinkage
			}
			reason = *c.Availability.ReasonCode
		}
		e := v2.Execution{Ref: fmt.Sprintf("exec/%d", ordinal), CapabilityRef: c.ID, Config: configs[c.ID], RequestedScope: requested, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: v2.NotRun, ReasonCode: &reason, DiagnosticRefs: []string{}}
		if c.ID == capability.OfficeProfilesID {
			d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", nextDiagnostic), ExecutionRef: &e.Ref, Stage: failure.Parsing, Code: reason, Message: "No approved Office profile bundle is configured; synthetic profile fixtures are not production policies.", Details: v2.ErrorDetails{ErrorType: "OfficeProfileUnavailable"}}
			nextDiagnostic++
			e.DiagnosticRefs = append(e.DiagnosticRefs, d.Ref)
			report.Trace.Diagnostics = append(report.Trace.Diagnostics, d)
			report.Limitations = append(report.Limitations, "office.no_applicable_profile")
		}
		report.Trace.Executions = append(report.Trace.Executions, e)
	}
	slices.Sort(report.Limitations)
	report.Limitations = slices.Compact(report.Limitations)
	if err := report.Summarize(); err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	encoded, err := report.Encode(p.Limits.Report)
	if err != nil {
		return nil, fmt.Errorf("encoded: %w", err)
	}
	return &ReportOutput{Report: report, JSON: encoded}, nil
}
