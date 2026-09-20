package audit

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type nativeFinding struct {
	finding     v2.Finding
	execution   int
	payloadKey  string
	locationKey string
	anchorKey   string
}
type nativeAnchor struct {
	anchor    v2.Anchor
	execution int
	key       string
}

func assembleFindings(r *v2.Report, outcome Outcome, trace *nativeTrace, verified *identity.VerifiedText) error {
	items := []nativeFinding{}
	anchors := map[string]nativeAnchor{}
	var scalars []rune
	if verified != nil {
		scalars = []rune(r.Evidence.Texts[0].Text)
	}
	for i, e := range r.Executions {
		observations := trace.completed[e.CapabilityRef]
		if e.State == v2.Failed && outcome.Failure != nil {
			observations = outcome.Report.Findings
		}
		for _, old := range observations {
			f := v2.Finding{Finding: old, ExecutionRef: e.Ref, Mechanism: pointer(v2.Structural), AnchorRefs: []string{}}
			if old.ID == "parser.failure" {
				f.Mechanism = nil
				f.Evidence = maps.Clone(old.Evidence)
				f.Evidence["failure_code"] = string(outcome.Failure.Code())
			}
			payloadKey, err := canonicalKey(f.Finding)
			if err != nil {
				return err
			}
			item := nativeFinding{finding: f, execution: i, payloadKey: payloadKey, locationKey: locationKey(old.Location)}
			if verified != nil {
				spans, err := nativeSpans(old, r.Evidence.Texts[0], scalars)
				if err != nil {
					return err
				}
				if len(spans) > 0 {
					digest, err := verified.SelectionDigest(spans, v2.RecordLimits())
					if err != nil {
						return err
					}
					locator := v2.TextLocator{Version: "1", SegmentPointer: "/evidence/texts/0", Spans: spans, SelectedTextSHA256: digest, DigestDomain: identity.SelectionDomain}
					key, err := canonicalKey(locator)
					if err != nil {
						return err
					}
					// Every native anchor targets artifact/1 with the same text mapping/kind.
					// The remaining EC-001 sort keys are execution order and canonical locator.
					item.anchorKey = fmt.Sprintf("%020d:%s", i, key)
					anchors[item.anchorKey] = nativeAnchor{anchor: v2.Anchor{Kind: "text", ArtifactRef: pointer("artifact/1"), ExecutionRef: pointer(e.Ref), Mapping: nativeMapping(), Locator: locator}, execution: i, key: key}
				}
			}
			items = append(items, item)
		}
	}
	ordered := make([]nativeAnchor, 0, len(anchors))
	for _, a := range anchors {
		ordered = append(ordered, a)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.execution != b.execution {
			return a.execution < b.execution
		}
		return a.key < b.key
	})
	references := map[string]string{}
	for _, a := range ordered {
		a.anchor.Ref = fmt.Sprintf("anchor/%d", len(r.Anchors))
		references[fmt.Sprintf("%020d:%s", a.execution, a.key)] = a.anchor.Ref
		r.Anchors = append(r.Anchors, a.anchor)
	}
	rank := func(s string) int {
		if s == "INFO" {
			return 0
		}
		return evidence.Rank(s)
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if rank(a.finding.Severity) != rank(b.finding.Severity) {
			return rank(a.finding.Severity) > rank(b.finding.Severity)
		}
		if a.finding.ID != b.finding.ID {
			return a.finding.ID < b.finding.ID
		}
		if a.locationKey != b.locationKey {
			return a.locationKey < b.locationKey
		}
		if a.finding.Title != b.finding.Title {
			return a.finding.Title < b.finding.Title
		}
		if a.payloadKey != b.payloadKey {
			return a.payloadKey < b.payloadKey
		}
		return a.execution < b.execution
	})
	for _, item := range items {
		f := item.finding
		f.Ref = fmt.Sprintf("finding/%d", len(r.Findings))
		if item.anchorKey != "" {
			f.AnchorRefs = append(f.AnchorRefs, references[item.anchorKey])
		}
		r.Findings = append(r.Findings, f)
	}
	return nil
}

// nativeSpans describes observed extents, not normalization or remediation. Full
// token extents are verified against source text; escaped strings are compared
// as inert representations and never evaluated as commands or language syntax.
func nativeSpans(f evidence.Finding, text *evidence.Text, scalars []rune) ([]identity.TextSpan, error) {
	positions, ok := f.Location["character_offsets"].([]int)
	if !ok {
		if f.ID == "text.bom" {
			positions = []int{0}
		} else {
			return nil, nil
		}
	}
	spans := make([]identity.TextSpan, 0, len(positions))
	for _, start := range positions {
		if start < 0 || start >= len(scalars) {
			return nil, errors.New("invalid native finding position")
		}
		width := 1
		switch f.ID {
		case "identifier.uuid", "text.encoded_candidate":
			value, ok := f.Evidence["value"].(string)
			if !ok {
				return nil, errors.New("native token value missing")
			}
			width = utf8.RuneCountInString(value)
			if width == 0 || start+width > len(scalars) || string(scalars[start:start+width]) != value {
				return nil, errors.New("native token does not match source")
			}
		case "text.mixed_script", "provenance.text_marker":
			field := "escaped_token"
			if f.ID == "provenance.text_marker" {
				field = "escaped_marker"
			}
			escaped, ok := f.Evidence[field].(string)
			if !ok || escaped == "" {
				return nil, errors.New("native escaped token missing")
			}
			offset, end := 0, start
			for offset < len(escaped) && end < len(scalars) {
				piece := u.Escaped(string(scalars[end]))
				if !strings.HasPrefix(escaped[offset:], piece) {
					return nil, errors.New("native escaped token does not match source")
				}
				offset += len(piece)
				end++
			}
			if offset != len(escaped) {
				return nil, errors.New("native escaped token exceeds source")
			}
			width = end - start
		}
		end := start + width
		span := identity.TextSpan{Scalar: identity.Region{Start: uint64(start), End: uint64(end)}, Byte: identity.Region{Start: uint64(text.ByteOffsets[start]), End: uint64(text.ByteOffsets[end])}}
		if len(spans) > 0 && spans[len(spans)-1].Scalar.End > span.Scalar.Start {
			return nil, errors.New("overlapping native occurrences")
		}
		// Coalesce adjoining observed scalars; the finding retains each occurrence's
		// coordinates while the selection identity preserves all gaps between runs.
		if len(spans) > 0 && spans[len(spans)-1].Scalar.End == span.Scalar.Start {
			spans[len(spans)-1].Scalar.End = span.Scalar.End
			spans[len(spans)-1].Byte.End = span.Byte.End
		} else {
			spans = append(spans, span)
		}
	}
	return spans, nil
}
