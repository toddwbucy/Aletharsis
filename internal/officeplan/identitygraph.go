package officeplan

import (
	"context"
	"fmt"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

type IdentityGraph struct {
	Outcomes []v4.Outcome
	Trace    v4.NativeTrace
	Issues   []v4.Issue // Replaces package.Issues with diagnostic-linked observations.
}

// BuildIdentityGraph records the shared format decision after both inspectors
// return. Unknown or conflicting format is incomplete identification of a valid
// container, not a failed source hash check or permission to parse ZIP as text.
// A preparation interrupted before inspection needs an unrun execution instead.
func BuildIdentityGraph(ctx context.Context, p *Prepared, assembly *Assembly, descriptor v2.Capability, config v2.Config, execution, firstDiagnostic int) (*IdentityGraph, error) {
	if ctx == nil || p == nil || p.Admission == nil || p.DOCX == nil || p.ODT == nil || assembly == nil || len(assembly.Evidence.Packages) != 1 || execution < 0 || firstDiagnostic < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if descriptor.ID != capability.OfficeIdentifyID || descriptor.Role != v2.Parser || descriptor.Validate() != nil || descriptor.Availability.State != "available" || descriptor.Participation == v2.Disabled || config.Validate() != nil {
		return nil, v4.ErrLinkage
	}
	pkg := assembly.Evidence.Packages[0]
	if pkg.SourceSHA256 != p.Admission.Outcomes.SourceSHA256 || identifyFormat(p.DOCX, p.ODT) != p.Identity {
		return nil, v4.ErrLinkage
	}
	format := "unknown"
	switch p.Identity {
	case IdentityDOCX, IdentityODT:
		format = string(p.Identity)
	case IdentityConflicting, IdentityUnidentified:
	default:
		return nil, v4.ErrLinkage
	}
	if pkg.Format != format {
		return nil, v4.ErrLinkage
	}
	parts := map[string]v4.Part{}
	for _, part := range pkg.Parts {
		parts[part.Name] = part
	}
	documents := map[string]*xmlparts.Document{p.DOCX.TypesPart: p.DOCX.TypesXML, p.DOCX.MainPart: p.DOCX.MainXML, "META-INF/manifest.xml": p.ODT.ManifestXML, "content.xml": p.ODT.ContentXML}
	if p.DOCX.OPC != nil {
		for _, part := range p.DOCX.OPC.Parts {
			if part.XML != nil {
				documents[part.Part] = part.XML
			}
		}
	}
	result := &IdentityGraph{Trace: v4.NativeTrace{Capabilities: []v2.Capability{descriptor}, Executions: []v2.Execution{}, Diagnostics: []v2.Diagnostic{}, Results: []v2.Result{}, Anchors: []v4.Anchor{}}, Issues: []v4.Issue{}}
	coverage := &AnalysisCoverage{Outcomes: []v4.Outcome{}, Diagnostics: []v2.Diagnostic{}}
	ref := fmt.Sprintf("exec/%d", execution)
	add := func(code, name string, element int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", firstDiagnostic+len(coverage.Diagnostics)), ExecutionRef: &ref, Stage: failure.Parsing, Code: failure.Code(code), Message: "Office format identification retained an explicit uncertainty or declaration gap.", Details: v2.ErrorDetails{ErrorType: "OfficeIdentityGap"}}
		issue := v4.Issue{Code: code, DiagnosticRef: &d.Ref}
		if part, ok := parts[name]; ok {
			issue.PartRef = &part.PartRef
			d.Scope = &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "whole_artifact"}
			if element >= 0 {
				doc := documents[name]
				if doc == nil || element >= len(doc.Elements) || part.SHA256 == nil || doc.PartSHA256 != *part.SHA256 || part.ByteLength == nil {
					return v4.ErrLinkage
				}
				span := doc.Elements[element].Full
				if span.Start < 0 || span.End <= span.Start || span.End > *part.ByteLength {
					return v4.ErrLinkage
				}
				d.Scope = &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(span.Start), End: uint64(span.End)}}}
			}
		}
		if err := d.Validate(); err != nil {
			return err
		}
		result.Issues = append(result.Issues, issue)
		coverage.Diagnostics = append(coverage.Diagnostics, d)
		return nil
	}
	// Stable decision first, then each inspector's original issue order. Repeated
	// declaration issues remain distinct rather than deduplicating their evidence.
	switch p.Identity {
	case IdentityConflicting:
		if err := add(IdentityConflictCode, "", -1); err != nil {
			return nil, err
		}
	case IdentityUnidentified:
		if err := add("office.identity_unconfirmed", "", -1); err != nil {
			return nil, err
		}
	}
	for _, issue := range p.DOCX.Issues {
		if err := add(issue.Code, issue.Part, issue.Element); err != nil {
			return nil, err
		}
	}
	for _, issue := range p.ODT.Issues {
		if err := add(issue.Code, issue.Part, issue.Element); err != nil {
			return nil, err
		}
	}
	for _, gap := range assembly.XMLFailures {
		if documents[gap.Part] != nil {
			if err := add("office.xml_mapping_unavailable", gap.Part, -1); err != nil {
				return nil, err
			}
		}
	}
	// Existing package-level observations must not disappear as identity issues
	// gain graph links (the conflict marker is already represented above).
	for _, issue := range pkg.Issues {
		if !slices.ContainsFunc(result.Issues, func(i v4.Issue) bool { return i.Code == issue.Code && samePartRef(i.PartRef, issue.PartRef) }) {
			result.Issues = append(result.Issues, issue)
		}
	}
	// A related story can have a recognized XML root but no admissible prose.
	// Identify that root independently of analytical text coverage, so its parsed
	// XML is not attested by a text outcome that assessed zero bytes.
	identifiedStories := map[string]bool{}
	roots := map[string]v4.Element{}
	for _, mapped := range assembly.Evidence.XML {
		if len(mapped.Elements) > 0 {
			roots[mapped.PartRef] = mapped.Elements[0]
		}
	}
	storyRoots := map[string]string{"main": "document", "header": "hdr", "footer": "ftr", "comments": "comments", "footnotes": "footnotes", "endnotes": "endnotes"}
	for _, target := range p.Targets {
		if target.Operation != capability.OfficeTextID || target.Format != IdentityDOCX {
			continue
		}
		part, exists := parts[target.Name]
		if !exists {
			continue
		}
		root, mapped := roots[part.PartRef]
		want := storyRoots[target.Kind]
		if mapped && want != "" && root.LocalName == want && (root.Namespace == docxidentify.TransitionalWord || root.Namespace == docxidentify.StrictWord) && (target.Namespace == "" || target.Namespace == root.Namespace) {
			identifiedStories[part.Name] = true
		}
	}
	for _, part := range pkg.Parts {
		if (documents[part.Name] != nil || identifiedStories[part.Name]) && part.ByteLength != nil {
			refPart := part.PartRef
			outcome := v4.Outcome{Operation: descriptor.ID, ExecutionRef: ref, PartRef: &refPart, State: "completed", Codes: []string{}, DiagnosticRefs: []string{}, Assessed: []v4.Span{{Start: 0, End: *part.ByteLength}}, Excluded: []v4.Span{}}
			coverage.Outcomes = append(coverage.Outcomes, outcome)
		}
	}
	result.Outcomes = coverage.Outcomes
	parent, err := OperationCoverage(partLayer(assembly), coverage, descriptor.ID, config, execution)
	if err != nil {
		return nil, err
	}
	result.Trace.Executions = append(result.Trace.Executions, parent)
	result.Trace.Diagnostics = coverage.Diagnostics
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

func samePartRef(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
