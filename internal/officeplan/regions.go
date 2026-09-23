package officeplan

import (
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"slices"
)

// partitionCoverage computes one bounded complement without mutating its input.
// Analytical gaps may overlap; compressed package members must not. Empty
// assessed parts retain [0,0), whereas package execution regions must be nonempty.
func partitionCoverage(length int64, gaps []v4.Span, merge, retainEmpty bool) (assessed, excluded []v4.Span, err error) {
	if length < 0 {
		return nil, nil, v4.ErrLinkage
	}
	gaps = slices.Clone(gaps)
	slices.SortFunc(gaps, func(a, b v4.Span) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	excluded = []v4.Span{}
	for _, gap := range gaps {
		if !gap.Within(length) {
			return nil, nil, v4.ErrLinkage
		}
		if len(excluded) > 0 && gap.Start <= excluded[len(excluded)-1].End {
			last := &excluded[len(excluded)-1]
			if !merge && gap.Start < last.End {
				return nil, nil, v4.ErrLinkage
			}
			if merge {
				last.End = max(last.End, gap.End)
				continue
			}
		}
		excluded = append(excluded, gap)
	}
	assessed = []v4.Span{}
	var cursor int64
	for _, gap := range excluded {
		if cursor < gap.Start {
			assessed = append(assessed, v4.Span{Start: cursor, End: gap.Start})
		}
		cursor = gap.End
	}
	if cursor < length || length == 0 && retainEmpty {
		assessed = append(assessed, v4.Span{Start: cursor, End: length})
	}
	return assessed, excluded, nil
}
func identityRegions(spans []v4.Span) []identity.Region {
	result := make([]identity.Region, 0, len(spans))
	for _, s := range spans {
		result = append(result, identity.Region{Start: uint64(s.Start), End: uint64(s.End)})
	}
	return result
}
