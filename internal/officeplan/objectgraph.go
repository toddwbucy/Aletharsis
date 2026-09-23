package officeplan

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

type ObjectGraph struct {
	Trace       v4.NativeTrace
	Objects     []v4.Object
	Limitations []string
}

// BuildObjectGraph links inert candidate inspection and declaration gaps. The
// returned objects replace assembly.Objects so their diagnostic references are
// retained. No payload is opened and no nested-content coverage is claimed.
func BuildObjectGraph(ctx context.Context, p *Prepared, a *Analysis, assembly *Assembly, descriptor v2.Capability, config v2.Config, execution, firstDiagnostic int) (*ObjectGraph, error) {
	if ctx == nil || p == nil || p.Admission == nil || a == nil || assembly == nil || len(assembly.Evidence.Packages) != 1 || execution < 0 || firstDiagnostic < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if descriptor.ID != capability.OfficeObjectsID || descriptor.Role != v2.Parser || descriptor.Validate() != nil || descriptor.Availability.State != "available" || descriptor.Participation == v2.Disabled || config.Validate() != nil {
		return nil, v4.ErrLinkage
	}
	pkg := assembly.Evidence.Packages[0]
	if pkg.SourceSHA256 != p.Admission.Outcomes.SourceSHA256 {
		return nil, v4.ErrLinkage
	}
	index, err := v4.IndexEvidence(assembly.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	if err := index.ValidateArtifactBindings(assembly.Artifacts); err != nil {
		return nil, err
	}
	result := &ObjectGraph{Trace: v4.NativeTrace{Capabilities: []v2.Capability{descriptor}, Executions: []v2.Execution{}, Diagnostics: []v2.Diagnostic{}, Results: []v2.Result{}, Anchors: []v4.Anchor{}}, Objects: []v4.Object{}, Limitations: []string{}}
	ref := fmt.Sprintf("exec/%d", execution)
	for _, outcome := range pkg.Outcomes {
		if outcome.ExecutionRef == ref {
			return nil, v4.ErrLinkage
		}
	}
	coverage := &AnalysisCoverage{Outcomes: []v4.Outcome{}, Diagnostics: []v2.Diagnostic{}}
	add := func(code, part string) (string, error) {
		d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", firstDiagnostic+len(coverage.Diagnostics)), ExecutionRef: &ref, Stage: failure.Parsing, Code: failure.Code(code), Message: "Embedded-object inventory or bounded inspection is incomplete.", Details: v2.ErrorDetails{ErrorType: "OfficeObjectGap"}}
		if part != "" {
			for _, candidate := range pkg.Parts {
				if candidate.Name == part {
					d.Scope = &v2.Scope{ArtifactRef: candidate.ArtifactRef, Unit: "whole_artifact"}
					break
				}
			}
		}
		if err := d.Validate(); err != nil {
			return "", err
		}
		coverage.Diagnostics = append(coverage.Diagnostics, d)
		return d.Ref, nil
	}
	var parent v2.Execution
	if a.Objects == nil {
		if len(assembly.Evidence.Objects) != 0 {
			return nil, v4.ErrLinkage
		}
		state, reason := v2.NotRun, failure.PrerequisiteFailed
		switch {
		case p.Identity == IdentityODT:
			reason = failure.UnsupportedInput
		case errors.Is(a.ObjectsError, context.Canceled):
			state, reason = v2.Canceled, failure.Canceled
		case errors.Is(a.ObjectsError, context.DeadlineExceeded):
			state, reason = v2.Failed, failure.Timeout
		case a.ObjectsError != nil:
			state, reason = v2.Failed, failure.ExecutionFailed
		case p.hasDOCXDeclarations():
			return nil, v4.ErrLinkage
		}
		diagnostic, err := add(string(reason), "")
		if err != nil {
			return nil, err
		}
		parent = v2.Execution{Ref: ref, CapabilityRef: descriptor.ID, Config: config, RequestedScope: v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "whole_artifact"}, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: state, ReasonCode: &reason, DiagnosticRefs: []string{diagnostic}}
	} else {
		if a.ObjectsError != nil || !p.hasDOCXDeclarations() || a.Objects.SourceSHA256 != pkg.SourceSHA256 || len(a.Objects.Objects) != len(assembly.Evidence.Objects) || p.DOCX == nil || p.DOCX.OPC == nil {
			return nil, v4.ErrLinkage
		}
		result.Limitations = slices.Clone(a.Objects.Limitations)
		for i, original := range assembly.Evidence.Objects {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if original.ObjectRef != a.Objects.Objects[i].ID || original.Inspection.ExecutionRef != ref {
				return nil, v4.ErrLinkage
			}
			o := original
			o.Inspection.Codes = slices.Clone(original.Inspection.Codes)
			o.Inspection.Assessed = slices.Clone(original.Inspection.Assessed)
			o.Inspection.Excluded = slices.Clone(original.Inspection.Excluded)
			o.Inspection.DiagnosticRefs = []string{}
			part := index.Parts[o.PartRef]
			// A failed decompression is a failed prerequisite, not an object scan.
			if part.State != "completed" {
				o.Inspection.State = "not_run"
			}
			for _, code := range o.Inspection.Codes {
				diagnostic, err := add(code, part.Name)
				if err != nil {
					return nil, err
				}
				o.Inspection.DiagnosticRefs = append(o.Inspection.DiagnosticRefs, diagnostic)
			}
			coverage.Outcomes = append(coverage.Outcomes, o.Inspection)
			result.Objects = append(result.Objects, o)
		}
		// Partial OPC enumeration can hide even the existence of object declarations.
		if p.DOCX.OPC.State == "partial" {
			if _, err := add("opc.relationship_inventory_partial", ""); err != nil {
				return nil, err
			}
		}
		for _, declaration := range a.Objects.Declarations {
			if declaration.Code != "" {
				if _, err := add(declaration.Code, declaration.Anchor.Part); err != nil {
					return nil, err
				}
			}
		}
		for _, gap := range assembly.InventoryGaps {
			if gap.Operation == descriptor.ID {
				if _, err := add(gap.Code, gap.Part); err != nil {
					return nil, err
				}
			}
		}
		for _, gap := range assembly.XMLFailures {
			for _, part := range p.DOCX.OPC.Parts {
				if part.Part == gap.Part {
					if _, err := add("office.xml_mapping_unavailable", gap.Part); err != nil {
						return nil, err
					}
					break
				}
			}
		}
		if a.Objects.State != "completed" && a.Objects.State != "partial" {
			return nil, v4.ErrLinkage
		}
		if a.Objects.State == "partial" && len(coverage.Diagnostics) == 0 {
			return nil, v4.ErrLinkage
		}
		// Object outcomes are validated under this parent; leave unrelated object
		// slots out of the package-layer helper's temporary execution index.
		partEvidence := assembly.Evidence
		partEvidence.Objects = nil
		parent, err = OperationCoverage(&PackageRecords{Evidence: partEvidence, Artifacts: assembly.Artifacts}, coverage, descriptor.ID, config, execution)
		if err != nil {
			return nil, err
		}
	}
	result.Trace.Executions = append(result.Trace.Executions, parent)
	result.Trace.Diagnostics = coverage.Diagnostics
	trace, err := v4.IndexTrace(result.Trace)
	if err != nil {
		return nil, err
	}
	if err := v4.ValidateCoverage(result.Trace, assembly.Artifacts, index, evidence.Document{}); err != nil {
		return nil, err
	}
	for _, o := range pkg.Outcomes {
		trace.Executions[o.ExecutionRef] = v2.Execution{CapabilityRef: o.Operation, State: v2.Partial}
	}
	retained := assembly.Evidence
	retained.Objects = result.Objects
	if err := validatePartialOutcomes(index, retained, trace); err != nil {
		return nil, err
	}
	return result, nil
}
