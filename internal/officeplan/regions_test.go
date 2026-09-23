package officeplan

import (
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"reflect"
	"testing"
)

func TestCoveragePartitionsPreserveCoordinatePolicy(t *testing.T) {
	cases := []struct {
		name    string
		length  int64
		gaps    []v4.Span
		want    []v4.Span
		overlap bool
	}{
		{"none", 10, nil, []v4.Span{{Start: 0, End: 10}}, false},
		{"unordered adjacent", 10, []v4.Span{{Start: 4, End: 6}, {Start: 2, End: 4}}, []v4.Span{{Start: 0, End: 2}, {Start: 6, End: 10}}, false},
		{"nested", 10, []v4.Span{{Start: 2, End: 8}, {Start: 3, End: 4}}, []v4.Span{{Start: 0, End: 2}, {Start: 8, End: 10}}, true},
		{"whole", 10, []v4.Span{{Start: 0, End: 10}}, []v4.Span{}, false},
	}
	for _, tc := range cases {
		for _, merge := range []bool{false, true} {
			before := append([]v4.Span(nil), tc.gaps...)
			got, _, err := partitionCoverage(tc.length, tc.gaps, merge, false)
			if tc.overlap && !merge {
				if err == nil {
					t.Fatal("overlapping package spans accepted")
				}
				continue
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(tc.gaps, before) {
				t.Fatal(tc.name, merge, got, err)
			}
		}
	}
	for _, retain := range []bool{false, true} {
		got, _, err := partitionCoverage(0, nil, true, retain)
		if err != nil || len(got) != map[bool]int{false: 0, true: 1}[retain] {
			t.Fatal("empty part policy", got, err)
		}
	}
	for _, gap := range []v4.Span{{Start: -1, End: 2}, {Start: 3, End: 2}, {Start: 0, End: 11}} {
		if _, _, err := partitionCoverage(10, []v4.Span{gap}, true, false); err == nil {
			t.Fatal("out-of-bounds gap accepted")
		}
	}
}
