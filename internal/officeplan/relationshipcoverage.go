package officeplan

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

// CollectRelationshipCoverage preserves every enumerated OPC relationship-part
// outcome. Resolution gaps refer to decompressed declaration bytes, never ZIP
// offsets. Unlocated structural/limit gaps conservatively exclude the whole part.
// The host still accounts for format applicability and enumeration failure.
func CollectRelationshipCoverage(ctx context.Context, base *PackageRecords, native *opcrels.Result, execution, firstDiagnostic int) (*AnalysisCoverage, error) {
	if ctx == nil || base == nil || native == nil || native.Outcomes == nil || len(base.Evidence.Packages) != 1 || execution < 0 || firstDiagnostic < 0 {
		return nil, v4.ErrLinkage
	}
	pkg := base.Evidence.Packages[0]
	if native.Outcomes.SourceSHA256 != pkg.SourceSHA256 {
		return nil, v4.ErrLinkage
	}
	parts := map[string]v4.Part{}
	for _, p := range pkg.Parts {
		parts[p.Name] = p
	}
	byPart := map[string][]opcrels.Relationship{}
	for _, rel := range native.Relationships {
		byPart[rel.Anchor.Part] = append(byPart[rel.Anchor.Part], rel)
	}
	entries := slices.Clone(native.Parts)
	slices.SortFunc(entries, func(a, b opcrels.PartResult) int { return strings.Compare(a.Part, b.Part) })
	result := &AnalysisCoverage{Outcomes: []v4.Outcome{}, Diagnostics: []v2.Diagnostic{}, Limitations: []string{}}
	exec := fmt.Sprintf("exec/%d", execution)
	for i, n := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p, ok := parts[n.Part]
		if !ok || i > 0 && entries[i-1].Part == n.Part || p.SHA256 != nil && n.PartSHA256 != *p.SHA256 || p.SHA256 == nil && n.PartSHA256 != "" {
			return nil, v4.ErrLinkage
		}
		o := v4.Outcome{Operation: capability.OfficeRelationshipsID, ExecutionRef: exec, PartRef: &p.PartRef, State: n.State, Codes: []string{}, DiagnosticRefs: []string{}, Assessed: []v4.Span{}, Excluded: []v4.Span{}}
		// Unavailable payloads prevented parsing; their failure remains on the
		// package outcome, while this dependent operation did not run.
		if p.State != "completed" {
			o.State = "not_run"
		}
		add := func(code string, span *v4.Span, exclude bool) error {
			if code == "" {
				return v4.ErrLinkage
			}
			d := v2.Diagnostic{Ref: fmt.Sprintf("diagnostic/%d", firstDiagnostic+len(result.Diagnostics)), ExecutionRef: &exec, Stage: failure.Parsing, Code: failure.Code(code), Message: "OPC relationship assessment retained an explicit coverage gap.", Details: v2.ErrorDetails{ErrorType: "OfficeRelationshipGap"}, Scope: &v2.Scope{ArtifactRef: p.ArtifactRef, Unit: "whole_artifact"}}
			if span != nil {
				if p.ByteLength == nil || !span.Within(*p.ByteLength) || span.Start == span.End {
					return v4.ErrLinkage
				}
				d.Scope = &v2.Scope{ArtifactRef: p.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(span.Start), End: uint64(span.End)}}}
				if exclude {
					o.Excluded = append(o.Excluded, *span)
				}
			} else if exclude && p.ByteLength != nil {
				o.Excluded = append(o.Excluded, v4.Span{Start: 0, End: *p.ByteLength})
			}
			if err := d.Validate(); err != nil {
				return err
			}
			if !slices.Contains(o.Codes, code) {
				o.Codes = append(o.Codes, code)
			}
			o.DiagnosticRefs = append(o.DiagnosticRefs, d.Ref)
			result.Diagnostics = append(result.Diagnostics, d)
			return nil
		}
		codes := []string{}
		for _, code := range []string{n.Code, n.SourceCode, n.LimitCode} {
			if code == "" || slices.Contains(codes, code) {
				continue
			}
			codes = append(codes, code)
			if err := add(code, nil, code != "opc.relationship_unresolved"); err != nil {
				return nil, err
			}
		}
		unresolved := 0
		for _, rel := range byPart[n.Part] {
			if n.XML == nil || p.SHA256 == nil || rel.Anchor.PartSHA256 != *p.SHA256 || n.XML.PartSHA256 != *p.SHA256 {
				return nil, v4.ErrLinkage
			}
			if rel.Code == "" {
				continue
			}
			unresolved++
			span := v4.Span{Start: rel.Anchor.Span.Start, End: rel.Anchor.Span.End}
			if err := add(rel.Code, &span, false); err != nil {
				return nil, err
			}
		}
		if n.Code == "opc.relationship_unresolved" && unresolved == 0 {
			return nil, v4.ErrLinkage
		}
		switch o.State {
		case "completed", "partial":
			if p.ByteLength == nil || n.XML == nil || n.XML.PartSHA256 != n.PartSHA256 || (o.State == "partial") != (len(o.Codes) > 0) {
				return nil, v4.ErrLinkage
			}
			// Retained declarations were inspected even when target resolution
			// failed. Keep their diagnostic, but do not call their bytes unread.
			// A part-wide structural gap still excludes unrecognized regions.
			for _, rel := range byPart[n.Part] {
				span := wireSpan(rel.Anchor.Span)
				remaining := []v4.Span{}
				for _, gap := range o.Excluded {
					if span.End <= gap.Start || span.Start >= gap.End {
						remaining = append(remaining, gap)
						continue
					}
					if gap.Start < span.Start {
						remaining = append(remaining, v4.Span{Start: gap.Start, End: span.Start})
					}
					if span.End < gap.End {
						remaining = append(remaining, v4.Span{Start: span.End, End: gap.End})
					}
				}
				o.Excluded = remaining
			}
			var err error
			o.Assessed, o.Excluded, err = partitionCoverage(*p.ByteLength, o.Excluded, true, true)
			if err != nil {
				return nil, err
			}
		case "failed", "not_run", "canceled", "unsupported":
			if o.State == "unsupported" {
				o.State = "not_run"
			}
			if len(o.Codes) == 0 {
				return nil, v4.ErrLinkage
			}
			o.Excluded = []v4.Span{}
			if p.ByteLength != nil {
				o.Excluded = append(o.Excluded, v4.Span{Start: 0, End: *p.ByteLength})
			}
		default:
			return nil, v4.ErrLinkage
		}
		result.Outcomes = append(result.Outcomes, o)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
