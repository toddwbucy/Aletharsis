package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Graph is the execution/evidence portion of a report. Validation establishes
// internal consistency only; it does not authenticate sources or retained blobs.
type Graph struct {
	Capabilities []Capability `json:"capabilities"`
	Executions   []Execution  `json:"executions"`
	Artifacts    []Artifact   `json:"artifacts"`
	Anchors      []Anchor     `json:"anchors"`
	Results      []Result     `json:"results"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}
type graphIndex struct {
	caps        map[string]Capability
	executions  map[string]Execution
	artifacts   map[string]Artifact
	anchors     map[string]Anchor
	diagnostics map[string]Diagnostic
	document    evidence.Document
}

func unique[T any](values []T, ref func(T) string, validate func(T) error) (map[string]T, error) {
	if values == nil {
		return nil, errors.New("graph arrays required")
	}
	result := make(map[string]T, len(values))
	for _, v := range values {
		if err := validate(v); err != nil {
			return nil, err
		}
		key := ref(v)
		if _, ok := result[key]; ok {
			return nil, fmt.Errorf("duplicate reference: %s", key)
		}
		result[key] = v
	}
	return result, nil
}
func (g Graph) index(d evidence.Document) (*graphIndex, error) {
	x := &graphIndex{document: d}
	var err error
	x.caps, err = unique(g.Capabilities, func(v Capability) string { return v.ID }, Capability.Validate)
	if err != nil {
		return nil, err
	}
	x.executions, err = unique(g.Executions, func(v Execution) string { return v.Ref }, Execution.Validate)
	if err != nil {
		return nil, err
	}
	x.artifacts, err = unique(g.Artifacts, func(v Artifact) string { return v.Ref }, Artifact.Validate)
	if err != nil {
		return nil, err
	}
	x.anchors, err = unique(g.Anchors, func(v Anchor) string { return v.Ref }, Anchor.Validate)
	if err != nil {
		return nil, err
	}
	x.diagnostics, err = unique(g.Diagnostics, func(v Diagnostic) string { return v.Ref }, Diagnostic.Validate)
	if err != nil {
		return nil, err
	}
	_, err = unique(g.Results, func(v Result) string { return v.Ref }, Result.Validate)
	if err != nil {
		return nil, err
	}
	return x, nil
}
func (x *graphIndex) text(pointer, suffix string) (*evidence.Text, error) {
	index := strings.TrimSuffix(strings.TrimPrefix(pointer, "/evidence/texts/"), suffix)
	n, err := strconv.ParseUint(index, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != index || n >= uint64(len(x.document.Texts)) || pointer != "/evidence/texts/"+index+suffix || x.document.Texts[n] == nil {
		return nil, errors.New("invalid content pointer")
	}
	return x.document.Texts[n], nil
}
func (x *graphIndex) mapping(m Mapping) error {
	if m.Quality == "unavailable" {
		return nil
	}
	if _, ok := x.artifacts[m.FromArtifactRef]; !ok {
		return errors.New("dangling mapping source")
	}
	if _, ok := x.artifacts[m.ToArtifactRef]; !ok {
		return errors.New("dangling mapping target")
	}
	if m.DataRef != nil {
		if _, err := x.text(*m.DataRef, "/byte_offsets"); err != nil {
			return err
		}
	}
	return nil
}
func (x *graphIndex) intervals(s Scope, known bool) ([]identity.Region, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	a, ok := x.artifacts[s.ArtifactRef]
	if !ok {
		return nil, errors.New("dangling scope artifact")
	}
	if s.Unit == "whole_artifact" {
		if a.ByteLength == nil {
			if known {
				return nil, errors.New("unknown analyzed extent")
			}
			return nil, nil
		}
		return []identity.Region{{Start: 0, End: *a.ByteLength}}, nil
	}
	bound := a.ByteLength
	if s.Unit == "scalar" {
		if a.ContentRef == nil || a.ContentRef.Kind != "report_pointer" {
			return nil, errors.New("unverifiable scalar extent")
		}
		t, err := x.text(*a.ContentRef.Pointer, "/text")
		if err != nil {
			return nil, err
		}
		n := uint64(utf8.RuneCountInString(t.Text))
		bound = &n
	}
	if bound == nil {
		return nil, errors.New("unknown region extent")
	}
	for _, r := range s.Regions {
		if r.End > *bound {
			return nil, errors.New("region exceeds artifact")
		}
	}
	return s.Regions, nil
}
func mergeRegions(regions []identity.Region) []identity.Region {
	regions = slices.Clone(regions)
	slices.SortFunc(regions, func(a, b identity.Region) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	merged := []identity.Region{}
	for _, r := range regions {
		if len(merged) > 0 && merged[len(merged)-1].End == r.Start {
			merged[len(merged)-1].End = r.End
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// Validate checks linkage, retained identities, coordinate widths and coverage.
// Unknown mapping methods remain descriptive claims; they grant no edit authority.
func (g Graph) Validate(file evidence.File, d evidence.Document) error {
	x, err := g.index(d)
	if err != nil {
		return err
	}
	roles := map[Role]bool{}
	planned := map[string]bool{}
	for _, c := range g.Capabilities {
		roles[c.Role] = true
	}
	if !roles[Acquisition] || !roles[Parser] {
		return errors.New("missing operational capabilities")
	}
	seen := map[string]bool{}
	sources := 0
	for _, a := range g.Artifacts {
		for _, p := range a.Parents {
			if !seen[p] {
				return errors.New("cyclic, missing or unordered artifact parent")
			}
		}
		seen[a.Ref] = true
		if err := x.mapping(a.Mapping); err != nil {
			return err
		}
		if a.Kind == "source" {
			sources++
			if !reflect.DeepEqual(a.SHA256, file.SHA256) || (a.ByteLength == nil) != (file.Size == nil) || file.Size != nil && (*file.Size < 0 || *a.ByteLength != uint64(*file.Size)) {
				return errors.New("source/file identity mismatch")
			}
		}
		if a.Transform != nil {
			for _, ex := range a.Transform.Exclusions {
				if ex.Scope != nil {
					if _, err := x.intervals(*ex.Scope, false); err != nil {
						return err
					}
				}
			}
		}
		if a.ContentRef != nil && a.ContentRef.Kind == "report_pointer" {
			t, err := x.text(*a.ContentRef.Pointer, "/text")
			if err != nil {
				return err
			}
			if !utf8.ValidString(t.Text) || a.Representation.Encoding == nil || *a.Representation.Encoding != "utf-8" || identity.ExactBytes([]byte(t.Text)) != *a.SHA256 || uint64(len(t.Text)) != *a.ByteLength {
				return errors.New("retained text identity mismatch")
			}
		}
	}
	if sources != 1 {
		return errors.New("exactly one source artifact required")
	}
	for _, t := range d.Texts {
		if t == nil || !utf8.ValidString(t.Text) || len(t.ByteOffsets) != utf8.RuneCountInString(t.Text)+1 || file.Size == nil {
			return errors.New("invalid text boundary map")
		}
		for _, b := range t.ByteOffsets {
			if b < 0 || b > *file.Size {
				return errors.New("text offset outside source")
			}
		}
		i := 0
		for _, c := range t.Text {
			width := 0
			switch t.Encoding {
			case "utf-8":
				width = utf8.RuneLen(c)
			case "utf-16-le", "utf-16-be":
				width = 2
				if c > 0xffff {
					width = 4
				}
			case "utf-32-le", "utf-32-be":
				width = 4
			default:
				return errors.New("unsupported text coordinate encoding")
			}
			if t.ByteOffsets[i+1]-t.ByteOffsets[i] != width {
				return errors.New("text coordinate width mismatch")
			}
			i++
		}
	}
	for _, e := range g.Executions {
		c, ok := x.caps[e.CapabilityRef]
		if !ok {
			return errors.New("dangling capability")
		}
		planned[c.ID] = true
		if c.Participation == Disabled {
			if e.State != NotRun || *e.ReasonCode != "execution.disabled" {
				return errors.New("disabled capability ran")
			}
		} else if c.Availability.State != "available" {
			if e.State != NotRun || *e.ReasonCode != *c.Availability.ReasonCode {
				return errors.New("unavailable capability ran")
			}
		}
		requested, err := x.intervals(e.RequestedScope, false)
		if err != nil {
			return err
		}
		regions := []identity.Region{}
		for _, s := range e.AnalyzedScope {
			r, err := x.intervals(s, true)
			if err != nil {
				return err
			}
			regions = append(regions, r...)
		}
		unknown := false
		for _, ex := range e.Exclusions {
			if ex.UnknownRemainder {
				unknown = true
			} else {
				r, err := x.intervals(*ex.Scope, true)
				if err != nil {
					return err
				}
				regions = append(regions, r...)
			}
		}
		if e.State == Partial && !unknown && !reflect.DeepEqual(mergeRegions(regions), mergeRegions(requested)) {
			return errors.New("unaccounted partial coverage")
		}
		for _, ref := range e.DiagnosticRefs {
			diag, ok := x.diagnostics[ref]
			if !ok || diag.ExecutionRef == nil || *diag.ExecutionRef != e.Ref {
				return errors.New("diagnostic/execution mismatch")
			}
		}
	}
	if len(planned) != len(x.caps) {
		return errors.New("capability planning incomplete")
	}
	for _, a := range g.Anchors {
		if a.ArtifactRef != nil {
			if _, ok := x.artifacts[*a.ArtifactRef]; !ok {
				return errors.New("dangling anchor artifact")
			}
		}
		if a.ExecutionRef != nil {
			if _, ok := x.executions[*a.ExecutionRef]; !ok {
				return errors.New("dangling anchor execution")
			}
		}
		if err := x.mapping(a.Mapping); err != nil {
			return err
		}
		switch l := a.Locator.(type) {
		case TextLocator:
			t, err := x.text(l.SegmentPointer, "")
			if err != nil {
				return err
			}
			art := x.artifacts[*a.ArtifactRef]
			if art.Kind != "text" || art.ContentRef == nil || art.ContentRef.Kind != "report_pointer" || *art.ContentRef.Pointer != l.SegmentPointer+"/text" {
				return errors.New("text anchor artifact mismatch")
			}
			scalars := []rune(t.Text)
			selection := make([]struct {
				Span identity.TextSpan `json:"span"`
				Text string            `json:"text"`
			}, 0, len(l.Spans))
			for _, s := range l.Spans {
				if s.Scalar.End > uint64(len(scalars)) || s.Byte.Start != uint64(t.ByteOffsets[s.Scalar.Start]) || s.Byte.End != uint64(t.ByteOffsets[s.Scalar.End]) {
					return errors.New("anchor coordinate mismatch")
				}
				selection = append(selection, struct {
					Span identity.TextSpan `json:"span"`
					Text string            `json:"text"`
				}{s, string(scalars[s.Scalar.Start:s.Scalar.End])})
			}
			// Scope byte regions refer to UTF-8 artifact bytes, not the
			// original-source byte coordinates stored on the locator.
			utf8Offsets := make([]uint64, len(scalars)+1)
			for i, c := range scalars {
				utf8Offsets[i+1] = utf8Offsets[i] + uint64(utf8.RuneLen(c))
			}
			execution := x.executions[*a.ExecutionRef]
			for _, span := range l.Spans {
				covered := false
				for _, scope := range execution.AnalyzedScope {
					if scope.ArtifactRef != *a.ArtifactRef {
						continue
					}
					if scope.Unit == "whole_artifact" {
						covered = true
						break
					}
					start, end := span.Scalar.Start, span.Scalar.End
					if scope.Unit == "byte" {
						start, end = utf8Offsets[start], utf8Offsets[end]
					}
					for _, region := range scope.Regions {
						if region.Start <= start && end <= region.End {
							covered = true
							break
						}
					}
				}
				if !covered {
					return errors.New("text anchor outside analyzed scope")
				}
			}
			digest, err := selectionDigest(selection)
			if err != nil {
				return err
			}
			if digest != l.SelectedTextSHA256 {
				return errors.New("selection digest mismatch")
			}
		case CredentialLocator:
			if x.artifacts[l.ManifestRef].Kind != "manifest" {
				return errors.New("credential manifest missing")
			}
			if l.VerificationExecutionRef != nil {
				if _, ok := x.executions[*l.VerificationExecutionRef]; !ok {
					return errors.New("credential execution missing")
				}
			}
		case StatisticalLocator:
			sample := x.artifacts[l.SampleRef]
			if sample.Kind != "sample" || sample.SHA256 == nil {
				return errors.New("sample identity missing")
			}
			if _, err := x.intervals(l.Scope, true); err != nil {
				return err
			}
		case LegacyLocator:
			if _, ok := x.diagnostics[l.DiagnosticRef]; !ok {
				return errors.New("legacy diagnostic missing")
			}
		}
	}
	resultScopes := map[string][]Scope{}
	for _, r := range g.Results {
		e, ok := x.executions[r.ExecutionRef]
		if !ok {
			return errors.New("dangling result execution")
		}
		c := x.caps[e.CapabilityRef]
		if (e.State != Completed && e.State != Partial) || c.Role != Analyzer || c.Mechanism == nil || *c.Mechanism != Structural || r.Payload.Operation != c.ID {
			return errors.New("result from ineligible operation")
		}
		if !slices.ContainsFunc(e.AnalyzedScope, func(s Scope) bool { return reflect.DeepEqual(s, r.Payload.Scope) }) {
			return errors.New("result outside analyzed scope")
		}
		for _, ref := range r.AnchorRefs {
			a, ok := x.anchors[ref]
			if !ok || a.ExecutionRef == nil || *a.ExecutionRef != e.Ref {
				return errors.New("result anchor mismatch")
			}
		}
		resultScopes[e.Ref] = append(resultScopes[e.Ref], r.Payload.Scope)
	}
	for _, e := range g.Executions {
		c := x.caps[e.CapabilityRef]
		if e.State == Partial && len(resultScopes[e.Ref]) == 0 {
			return errors.New("partial execution without usable result")
		}
		if (e.State == Completed || e.State == Partial) && c.Role == Analyzer && c.Mechanism != nil && *c.Mechanism == Structural {
			for _, s := range e.AnalyzedScope {
				if !slices.ContainsFunc(resultScopes[e.Ref], func(r Scope) bool { return reflect.DeepEqual(s, r) }) {
					return errors.New("analyzed scope without result")
				}
			}
		}
	}
	for _, diag := range g.Diagnostics {
		if diag.ExecutionRef != nil {
			if _, ok := x.executions[*diag.ExecutionRef]; !ok {
				return errors.New("dangling diagnostic execution")
			}
		}
		if diag.Scope != nil {
			if _, err := x.intervals(*diag.Scope, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func selectionDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return identity.Digest(identity.SelectionDomain, raw, RecordLimits())
}
