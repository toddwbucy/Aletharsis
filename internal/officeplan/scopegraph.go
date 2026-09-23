package officeplan

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/odttext"
	"github.com/toddwbucy/Aletharsis/internal/wordtext"
)

type TraceOffsets struct{ Execution, Result, Anchor, Finding int }
type ScopeGraph struct {
	Trace    v4.NativeTrace
	Findings []v2.Finding
}

// BuildScopeGraph binds completed WA/OA scans to their retained text artifacts.
// It runs no detector and invents no negative result for an omitted scope. The
// host separately records omissions, unrun operations, and the other Office
// capabilities. No scopes means no analyzer execution in this graph fragment.
// Executions remain per artifact: the current wire contract has one requested
// artifact and requires analyzed scopes within it. Coalescing paragraph artifacts
// needs a versioned multi-artifact execution contract, not only fewer records.
func BuildScopeGraph(ctx context.Context, assembly *Assembly, catalog []v2.Capability, configs map[string]v2.Config, offset TraceOffsets, limits identity.Limits) (*ScopeGraph, error) {
	if ctx == nil || assembly == nil || len(assembly.Evidence.Packages) != 1 || offset.Execution < 0 || offset.Result < 0 || offset.Anchor < 0 || offset.Finding < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pkg := assembly.Evidence.Packages[0]
	index, err := v4.IndexEvidence(assembly.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	if err := index.ValidateArtifactBindings(assembly.Artifacts); err != nil {
		return nil, err
	}
	artifacts := map[string]v4.Artifact{}
	for _, a := range assembly.Artifacts {
		artifacts[a.Ref] = a
	}
	observations := map[string]ScopedFindings{}
	for _, f := range assembly.Findings {
		scope, ok := index.Scopes[f.ScopeRef]
		if !ok || scope.PartRef != f.PartRef || !v4.ScopeArtifactEquivalent(artifacts[f.ArtifactRef], scope) {
			return nil, v4.ErrLinkage
		}
		if _, ok := observations[f.ScopeRef]; ok {
			return nil, v4.ErrLinkage
		}
		observations[f.ScopeRef] = f
	}
	if len(observations) != len(index.Scopes) {
		return nil, v4.ErrLinkage
	}
	result := &ScopeGraph{Trace: v4.NativeTrace{Capabilities: []v2.Capability{}, Executions: []v2.Execution{}, Diagnostics: []v2.Diagnostic{}, Results: []v2.Result{}, Anchors: []v4.Anchor{}}, Findings: []v2.Finding{}}
	if len(index.Scopes) == 0 {
		return result, nil
	}
	operations := []string{capability.UnicodeInventoryID, capability.EmojiID, capability.PatternsID}
	formats := map[string]bool{}
	for _, scope := range assembly.Evidence.Scopes {
		switch scope.ExtractorVersion {
		case wordtext.Version:
			formats["docx"] = true
		case odttext.Version:
			formats["odt"] = true
		default:
			return nil, v4.ErrLinkage
		}
	}
	descriptors := map[string]v2.Capability{}
	for _, c := range catalog {
		if !slices.Contains(operations, c.ID) {
			continue
		}
		if _, exists := descriptors[c.ID]; exists {
			return nil, v4.ErrLinkage
		}
		if err := c.Validate(); err != nil {
			return nil, err
		}
		if c.Role != v2.Analyzer || c.Mechanism == nil || *c.Mechanism != v2.Structural || c.Availability.State != "available" || c.Participation == v2.Disabled || !slices.Contains(c.SupportedScope.Kinds, "office_scope") {
			return nil, v4.ErrLinkage
		}
		for format := range formats {
			if !slices.Contains(c.SupportedScope.Formats, format) {
				return nil, v4.ErrLinkage
			}
		}
		config, ok := configs[c.ID]
		if !ok || config.Validate() != nil {
			return nil, v4.ErrLinkage
		}
		descriptors[c.ID] = c
	}
	if len(descriptors) != len(operations) {
		return nil, v4.ErrLinkage
	}
	for _, id := range operations {
		result.Trace.Capabilities = append(result.Trace.Capabilities, descriptors[id])
	}
	type orderedFinding struct {
		finding v2.Finding
		key     string
	}
	findings := []orderedFinding{}
	for _, scope := range assembly.Evidence.Scopes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		observed := observations[scope.ScopeRef]
		byOperation := map[string][]evidence.Finding{}
		for _, original := range observed.Findings {
			op := ""
			switch {
			case original.ID == "unicode.emoji":
				op = capability.EmojiID
			case strings.HasPrefix(original.ID, "unicode."):
				op = capability.UnicodeInventoryID
			case strings.HasPrefix(original.ID, "pattern."):
				op = capability.PatternsID
			default:
				return nil, v4.ErrLinkage
			}
			translated, err := v4.OfficeFinding(original, scope)
			if err != nil {
				return nil, err
			}
			// Copy nested payload/offset data before publishing a mutable graph result.
			raw, err := json.Marshal(translated)
			if err != nil {
				return nil, err
			}
			var owned evidence.Finding
			if err := json.Unmarshal(raw, &owned); err != nil {
				return nil, err
			}
			byOperation[op] = append(byOperation[op], owned)
		}
		for _, op := range operations {
			requested := v2.Scope{ArtifactRef: observed.ArtifactRef, Unit: "whole_artifact"}
			execution := v2.Execution{Ref: fmt.Sprintf("exec/%d", offset.Execution+len(result.Trace.Executions)), CapabilityRef: op, Config: configs[op], RequestedScope: requested,
				AnalyzedScope: []v2.Scope{requested}, Exclusions: []v2.Exclusion{}, State: v2.Completed, DiagnosticRefs: []string{}}
			if err := execution.Validate(); err != nil {
				return nil, err
			}
			result.Trace.Executions = append(result.Trace.Executions, execution)
			scan := v2.Result{Ref: fmt.Sprintf("result/%d", offset.Result+len(result.Trace.Results)), ExecutionRef: execution.Ref, Kind: "structural_scan", ContractVersion: "1", AnchorRefs: []string{},
				Payload: v2.StructuralPayload{Operation: op, Scope: requested, Outcome: "no_observations"}, Limitations: []string{"office.analysis_scope_only", "office.not_rendered_text"}}
			if len(byOperation[op]) > 0 {
				locator, err := json.Marshal(v4.ScopeLocation{Kind: "office_scope", ScopeRef: scope.ScopeRef})
				if err != nil {
					return nil, err
				}
				anchor := v4.Anchor{Ref: fmt.Sprintf("anchor/%d", offset.Anchor+len(result.Trace.Anchors)), Kind: "office_scope", ArtifactRef: observed.ArtifactRef, ExecutionRef: &execution.Ref,
					Mapping: slices.Clone(artifacts[observed.ArtifactRef].Mapping), Locator: locator}
				if err := index.ValidateOfficeAnchor(anchor, artifacts, nil); err != nil {
					return nil, err
				}
				result.Trace.Anchors = append(result.Trace.Anchors, anchor)
				scan.Payload.Outcome = "observations_present"
				scan.AnchorRefs = append(scan.AnchorRefs, anchor.Ref)
				for _, f := range byOperation[op] {
					mechanism := v2.Structural
					finding := v2.Finding{Finding: f, ExecutionRef: execution.Ref, Mechanism: &mechanism, AnchorRefs: []string{anchor.Ref}}
					raw, err := json.Marshal(finding)
					if err != nil {
						return nil, err
					}
					key, err := identity.Canonicalize(raw, limits)
					if err != nil {
						return nil, err
					}
					findings = append(findings, orderedFinding{finding, string(key)})
				}
			}
			if err := scan.Validate(); err != nil {
				return nil, err
			}
			result.Trace.Results = append(result.Trace.Results, scan)
		}
	}
	rank := func(severity string) int {
		if severity == "INFO" {
			return 0
		}
		return evidence.Rank(severity)
	}
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if rank(a.finding.Severity) != rank(b.finding.Severity) {
			return rank(a.finding.Severity) > rank(b.finding.Severity)
		}
		if a.finding.ID != b.finding.ID {
			return a.finding.ID < b.finding.ID
		}
		return a.key < b.key
	})
	for _, f := range findings {
		f.finding.Ref = fmt.Sprintf("finding/%d", offset.Finding+len(result.Findings))
		result.Findings = append(result.Findings, f.finding)
	}
	if _, err := v4.IndexTrace(result.Trace); err != nil {
		return nil, err
	}
	if err := v4.ValidateCoverage(result.Trace, assembly.Artifacts, index, evidence.Document{}); err != nil {
		return nil, err
	}
	return result, nil
}
