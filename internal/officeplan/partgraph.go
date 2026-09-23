package officeplan

import (
	"context"
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

var partOperations = []string{capability.OfficeRelationshipsID, capability.OfficeMetadataID, capability.OfficeTextID}

// PartGraph binds part producers to parent executions. It is a graph fragment,
// not a complete report: acquisition, identification, objects, profiles and
// scope analyzers have separate records. Outcomes must be appended to the package.
type PartGraph struct {
	Trace       v4.NativeTrace
	Outcomes    []v4.Outcome
	Limitations []string
}

// BuildPartGraph preserves explicit unrun operations when format identification
// is incomplete or conflicting. It does not rerun producers or mutate evidence.
func BuildPartGraph(ctx context.Context, p *Prepared, a *Analysis, assembly *Assembly, catalog []v2.Capability, configs map[string]v2.Config, firstExecution, firstDiagnostic int) (*PartGraph, error) {
	if ctx == nil || p == nil || p.Admission == nil || a == nil || assembly == nil || len(assembly.Evidence.Packages) != 1 || firstExecution < 0 || firstDiagnostic < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pkg := assembly.Evidence.Packages[0]
	format := "unknown"
	switch p.Identity {
	case IdentityDOCX, IdentityODT:
		format = string(p.Identity)
	case IdentityConflicting, IdentityUnidentified:
	default:
		return nil, v4.ErrLinkage
	}
	if pkg.SourceSHA256 != p.Admission.Outcomes.SourceSHA256 || pkg.Format != format {
		return nil, v4.ErrLinkage
	}
	if p.DOCX == nil || p.DOCX.OPC == nil {
		return nil, v4.ErrLinkage
	}
	operations := partOperations
	ordinals := map[string]int{}
	descriptors := map[string]v2.Capability{}
	for i, op := range operations {
		ordinals[op] = firstExecution + i
	}
	for _, c := range catalog {
		if !slices.Contains(operations, c.ID) {
			continue
		}
		if _, exists := descriptors[c.ID]; exists {
			return nil, v4.ErrLinkage
		}
		config, ok := configs[c.ID]
		if !ok || config.Validate() != nil || c.Validate() != nil || c.Role != v2.Parser || c.Availability.State != "available" || c.Participation == v2.Disabled {
			return nil, v4.ErrLinkage
		}
		descriptors[c.ID] = c
	}
	if len(descriptors) != len(operations) {
		return nil, v4.ErrLinkage
	}
	// Object inspection is owned by its separate graph fragment. Keep its
	// already assembled records out of this fragment's outcome validation only.
	partEvidence := assembly.Evidence
	partEvidence.Objects = nil
	base := &PackageRecords{Evidence: partEvidence, Artifacts: assembly.Artifacts}
	content, err := CollectAnalysisCoverage(ctx, base, a, ordinals, firstDiagnostic)
	if err != nil {
		return nil, err
	}
	byOperation := map[string]*AnalysisCoverage{}
	for _, op := range operations {
		byOperation[op] = &AnalysisCoverage{Outcomes: []v4.Outcome{}, Diagnostics: []v2.Diagnostic{}, Limitations: []string{}}
	}
	for _, o := range content.Outcomes {
		byOperation[o.Operation].Outcomes = append(byOperation[o.Operation].Outcomes, o)
	}
	for _, d := range content.Diagnostics {
		for _, op := range []string{capability.OfficeMetadataID, capability.OfficeTextID} {
			if d.ExecutionRef != nil && *d.ExecutionRef == fmt.Sprintf("exec/%d", ordinals[op]) {
				byOperation[op].Diagnostics = append(byOperation[op].Diagnostics, d)
			}
		}
	}
	nextDiagnostic := firstDiagnostic + len(content.Diagnostics)
	if p.Identity != IdentityODT && len(p.DOCX.OPC.Parts) > 0 {
		relationships, err := CollectRelationshipCoverage(ctx, base, p.DOCX.OPC, ordinals[capability.OfficeRelationshipsID], nextDiagnostic)
		if err != nil {
			return nil, err
		}
		byOperation[capability.OfficeRelationshipsID] = relationships
		nextDiagnostic += len(relationships.Diagnostics)
	}
	addGap := func(op, part, code string) error {
		if p.Identity == IdentityODT && op != capability.OfficeTextID {
			return nil
		}
		ref := fmt.Sprintf("exec/%d", ordinals[op])
		d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", nextDiagnostic), ExecutionRef: &ref, Stage: failure.Parsing, Code: failure.Code(code), Message: "Office operation retains incomplete selection or evidence mapping.", Details: v2.ErrorDetails{ErrorType: "OfficeOperationGap"}}
		if part != "" {
			for _, candidate := range pkg.Parts {
				if candidate.Name == part {
					d.Scope = &v2.Scope{ArtifactRef: candidate.ArtifactRef, Unit: "whole_artifact"}
					break
				}
			}
		}
		if err := d.Validate(); err != nil {
			return err
		}
		nextDiagnostic++
		byOperation[op].Diagnostics = append(byOperation[op].Diagnostics, d)
		return nil
	}
	if p.hasDOCXDeclarations() {
		// Identification's retained declaration issues can mean the target set is
		// incomplete. Do not convert a zero selected count into assessed absence.
		for _, issue := range p.DOCX.Issues {
			// OPC's aggregate marker may describe an unrelated relationship part.
			// Its concrete owner/part reasons are handled by relationship coverage and
			// the main-relationship selection check below.
			if issue.Code == "opc.relationship_inventory_partial" {
				continue
			}
			for _, op := range []string{capability.OfficeMetadataID, capability.OfficeTextID} {
				if err := addGap(op, issue.Part, issue.Code); err != nil {
					return nil, err
				}
			}
		}
	}
	if p.hasDOCXDeclarations() {
		for _, part := range p.DOCX.OPC.Parts {
			if part.SourcePart != p.DOCX.MainPart || part.State == "completed" {
				continue
			}
			codes := []string{}
			for _, code := range []string{part.Code, part.SourceCode, part.LimitCode} {
				if code != "" && !slices.Contains(codes, code) {
					codes = append(codes, code)
					if err := addGap(capability.OfficeTextID, part.Part, code); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	for _, gap := range assembly.InventoryGaps {
		if slices.Contains(operations, gap.Operation) {
			if err := addGap(gap.Operation, gap.Part, gap.Code); err != nil {
				return nil, err
			}
		}
	}
	// A relationship part with no retained relationships can still lose its XML
	// map. Inventory gaps alone would miss that zero-entry case.
	for _, gap := range assembly.XMLFailures {
		if p.hasDOCXDeclarations() {
			for _, part := range p.DOCX.OPC.Parts {
				if part.Part == gap.Part {
					if err := addGap(capability.OfficeRelationshipsID, gap.Part, "office.xml_mapping_unavailable"); err != nil {
						return nil, err
					}
					break
				}
			}
		}
	}
	result := &PartGraph{Trace: v4.NativeTrace{Capabilities: []v2.Capability{}, Executions: []v2.Execution{}, Diagnostics: []v2.Diagnostic{}, Results: []v2.Result{}, Anchors: []v4.Anchor{}}, Outcomes: []v4.Outcome{}, Limitations: slices.Clone(content.Limitations)}
	for _, op := range operations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.Trace.Capabilities = append(result.Trace.Capabilities, descriptors[op])
		coverage := byOperation[op]
		var execution v2.Execution
		if p.Identity == IdentityUnidentified && len(coverage.Outcomes) == 0 && len(coverage.Diagnostics) == 0 {
			reason := failure.Code("office.identity_unconfirmed")
			if err := addGap(op, "", string(reason)); err != nil {
				return nil, err
			}
			refs := []string{}
			for _, d := range coverage.Diagnostics {
				refs = append(refs, d.Ref)
			}
			execution = v2.Execution{Ref: fmt.Sprintf("exec/%d", ordinals[op]), CapabilityRef: op, Config: configs[op], RequestedScope: v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "whole_artifact"}, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: v2.NotRun, ReasonCode: &reason, DiagnosticRefs: refs}
		} else if p.Identity == IdentityODT && op != capability.OfficeTextID {
			if len(coverage.Outcomes) != 0 || len(coverage.Diagnostics) != 0 {
				return nil, v4.ErrLinkage
			}
			reason := failure.UnsupportedInput
			execution = v2.Execution{Ref: fmt.Sprintf("exec/%d", ordinals[op]), CapabilityRef: op, Config: configs[op], RequestedScope: v2.Scope{ArtifactRef: pkg.SourceArtifactRef, Unit: "whole_artifact"}, AnalyzedScope: []v2.Scope{}, Exclusions: []v2.Exclusion{}, State: v2.NotRun, ReasonCode: &reason, DiagnosticRefs: []string{}}
		} else {
			execution, err = OperationCoverage(base, coverage, op, configs[op], ordinals[op])
			if err != nil {
				return nil, err
			}
		}
		result.Trace.Executions = append(result.Trace.Executions, execution)
		result.Trace.Diagnostics = append(result.Trace.Diagnostics, coverage.Diagnostics...)
		result.Outcomes = append(result.Outcomes, coverage.Outcomes...)
	}
	// No surviving text scope exists to carry these part-qualified boundaries.
	// Retain them as located observations, without turning ordinary structural
	// boundaries into failed/partial analysis or inventing an empty text scope.
	hasScope := map[string]bool{}
	for _, scope := range assembly.Evidence.Scopes {
		hasScope[scope.PartRef] = true
	}
	for _, part := range pkg.Parts {
		if hasScope[part.PartRef] {
			continue
		}
		for _, boundary := range assembly.Boundaries[part.PartRef] {
			var doc *v4.XML
			for i := range assembly.Evidence.XML {
				if assembly.Evidence.XML[i].XMLRef == boundary.XMLRef {
					doc = &assembly.Evidence.XML[i]
					break
				}
			}
			if doc == nil || doc.PartRef != part.PartRef || boundary.Token < 0 || boundary.Token >= int64(len(doc.Tokens)) {
				return nil, v4.ErrLinkage
			}
			span := doc.Tokens[boundary.Token].Span
			ref := fmt.Sprintf("exec/%d", ordinals[capability.OfficeTextID])
			d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", nextDiagnostic), ExecutionRef: &ref, Stage: failure.Parsing, Code: failure.Code("office.boundary." + boundary.Reason), Message: "Located extraction boundary retained without a surviving text scope.", Details: v2.ErrorDetails{ErrorType: "OfficeBoundary"}, Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: identityRegions([]v4.Span{span})}}
			nextDiagnostic++
			result.Trace.Diagnostics = append(result.Trace.Diagnostics, d)
			for i := range result.Trace.Executions {
				if result.Trace.Executions[i].Ref == ref {
					result.Trace.Executions[i].DiagnosticRefs = append(result.Trace.Executions[i].DiagnosticRefs, d.Ref)
				}
			}
		}
	}
	if _, err := v4.IndexTrace(result.Trace); err != nil {
		return nil, err
	}
	index, err := v4.IndexEvidence(assembly.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	if err := v4.ValidateCoverage(result.Trace, assembly.Artifacts, index, evidence.Document{}); err != nil {
		return nil, err
	}
	return result, nil
}
