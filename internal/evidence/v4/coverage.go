package v4

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

type coverageIndex struct {
	artifacts map[string]Artifact
	office    *Index
	flat      evidence.Document
}

func (x coverageIndex) text(a Artifact) (string, error) {
	if a.ContentRef == nil {
		return "", ErrLinkage
	}
	switch a.ContentRef.Kind {
	case "office_scope":
		if a.ContentRef.ScopeRef == nil {
			return "", ErrLinkage
		}
		s, ok := x.office.Scopes[*a.ContentRef.ScopeRef]
		if !ok {
			return "", ErrLinkage
		}
		return s.Text, nil
	case "report_pointer":
		if a.ContentRef.Pointer == nil {
			return "", ErrLinkage
		}
		p := *a.ContentRef.Pointer
		index := strings.TrimSuffix(strings.TrimPrefix(p, "/evidence/texts/"), "/text")
		n, err := strconv.ParseUint(index, 10, 53)
		if err != nil || strconv.FormatUint(n, 10) != index || p != "/evidence/texts/"+index+"/text" ||
			n >= uint64(len(x.flat.Texts)) || x.flat.Texts[n] == nil {
			return "", ErrLinkage
		}
		return x.flat.Texts[n].Text, nil
	default:
		return "", ErrLinkage
	}
}

// intervals returns byte intervals in the named artifact, converting scalar
// positions through retained UTF-8 text. Cross-unit comparisons stay meaningful.
func (x coverageIndex) intervals(s v2.Scope, known bool) ([]identity.Region, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	a, ok := x.artifacts[s.ArtifactRef]
	if !ok {
		return nil, ErrLinkage
	}
	if s.Unit == "whole_artifact" {
		if a.ByteLength == nil {
			if known {
				return nil, ErrLinkage
			}
			return nil, nil
		}
		return []identity.Region{{Start: 0, End: *a.ByteLength}}, nil
	}
	if a.ByteLength == nil {
		return nil, ErrLinkage
	}
	if s.Unit == "byte" {
		for _, r := range s.Regions {
			if r.End > *a.ByteLength {
				return nil, ErrLinkage
			}
		}
		return slices.Clone(s.Regions), nil
	}
	text, err := x.text(a)
	if err != nil || !utf8.ValidString(text) || uint64(len(text)) != *a.ByteLength {
		return nil, ErrLinkage
	}
	starts := make([]uint64, 0, utf8.RuneCountInString(text)+1)
	for offset := range text {
		starts = append(starts, uint64(offset))
	}
	starts = append(starts, uint64(len(text)))
	result := make([]identity.Region, 0, len(s.Regions))
	for _, r := range s.Regions {
		if r.End >= uint64(len(starts)) {
			return nil, ErrLinkage
		}
		result = append(result, identity.Region{Start: starts[r.Start], End: starts[r.End]})
	}
	return result, nil
}
func mergeCoverage(regions []identity.Region) []identity.Region {
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
	out := []identity.Region{}
	for _, r := range regions {
		if len(out) > 0 && out[len(out)-1].End == r.Start {
			out[len(out)-1].End = r.End
		} else {
			out = append(out, r)
		}
	}
	return out
}

// ValidateCoverage runs after record validation and checks every execution and
// diagnostic scope against the actual graph artifact and retained text bounds.
func ValidateCoverage(t NativeTrace, artifacts []Artifact, office *Index, flat evidence.Document) error {
	x := coverageIndex{artifacts: map[string]Artifact{}, office: office, flat: flat}
	for _, a := range artifacts {
		x.artifacts[a.Ref] = a
	}
	for _, e := range t.Executions {
		requested, err := x.intervals(e.RequestedScope, false)
		if err != nil {
			return err
		}
		requested = mergeCoverage(requested)
		accounted := []identity.Region{}
		for _, s := range e.AnalyzedScope {
			regions, err := x.intervals(s, true)
			if err != nil {
				return err
			}
			accounted = append(accounted, regions...)
		}
		unknown := false
		for _, excluded := range e.Exclusions {
			if excluded.UnknownRemainder {
				unknown = true
				continue
			}
			if excluded.Scope == nil {
				return ErrLinkage
			}
			regions, err := x.intervals(*excluded.Scope, true)
			if err != nil {
				return err
			}
			accounted = append(accounted, regions...)
		}
		merged := mergeCoverage(accounted)
		for i, region := range merged {
			if i > 0 && region.Start < merged[i-1].End {
				return ErrLinkage
			}
			if !slices.ContainsFunc(requested, func(bound identity.Region) bool {
				return region.Start >= bound.Start && region.End <= bound.End
			}) {
				return ErrLinkage
			}
		}
		if e.State == v2.Partial && !unknown && !slices.Equal(mergeCoverage(accounted), requested) {
			return ErrLinkage
		}
	}
	for _, d := range t.Diagnostics {
		if d.Scope != nil {
			if _, err := x.intervals(*d.Scope, false); err != nil {
				return err
			}
		}
	}
	return nil
}
