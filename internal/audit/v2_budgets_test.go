package audit

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestDenseNativeAnchorsPreserveMembership(t *testing.T) {
	for _, source := range []string{strings.Repeat("a\u2060", 1200), strings.Repeat("\u200b\u200c", 1200), "A" + strings.Repeat("\u0305", 20000)} {
		out := memoryV2(t, []byte(source), "audit")
		r, err := v2.DecodeReport(out.JSON, DefaultV2Options().ReportLimits)
		if err != nil {
			t.Fatal(err)
		}
		split := false
		anchors := map[string]v2.Anchor{}
		for _, a := range r.Anchors {
			anchors[a.Ref] = a
		}
		for _, f := range r.Findings {
			if len(f.AnchorRefs) > 1 {
				split = true
			}
			counts := map[uint64]int{}
			for _, ref := range f.AnchorRefs {
				loc := anchors[ref].Locator.(v2.TextLocator)
				if len(loc.Spans) > 512 {
					t.Fatal("unbounded locator")
				}
				for _, s := range loc.Spans {
					for i := s.Scalar.Start; i < s.Scalar.End; i++ {
						counts[i]++
					}
				}
			}
			// Use independently captured native positions, not the decoded JSON types.
			for _, native := range out.Report.Findings {
				if native.Ref == f.Ref {
					if positions, ok := native.Location["character_offsets"].([]int); ok {
						if len(counts) != len(positions) {
							t.Fatal("added unrelated positions")
						}
						for _, p := range positions {
							if counts[uint64(p)] != 1 {
								t.Fatal("lost or duplicated occurrence")
							}
						}
					}
				}
			}
		}
		if !split {
			t.Fatal("fixture did not exercise multiple anchors")
		}
	}
}

func TestV2RejectsImpossibleTextBudgetBeforeAnalyzers(t *testing.T) {
	options := DefaultV2Options()
	options.ReportLimits.InputBytes = 100
	options.ReportLimits.OutputBytes = 100
	trace := &nativeTrace{ctx: context.Background(), completed: map[string][]evidence.Finding{}, reportLimits: options.ReportLimits}
	source := bytes.Repeat([]byte("ordinary ASCII "), 1000)
	outcome := inspectWithTrace("test.txt", MaxBytes, func(string, int) ([]byte, error) { return source, nil }, trace)
	if !errors.Is(trace.budgetErr, identity.ErrLimit) || outcome.Failure != nil {
		t.Fatal("wrong budget boundary")
	}
	if len(trace.completed) != 2 {
		t.Fatal("analysis ran for impossible text budget")
	}
	if _, ok := trace.completed[capability.ParseTextID]; !ok {
		t.Fatal("missing completed parse")
	}
	// Disabling the optional trace retains the legacy audit's full analysis.
	legacy := inspectWithReader("test.txt", MaxBytes, func(string, int) ([]byte, error) { return source, nil })
	if legacy.Failure != nil || legacy.Report.Status != "completed" {
		t.Fatal("legacy changed")
	}
}
