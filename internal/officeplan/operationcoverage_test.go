package officeplan

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestOperationCoverageAggregatesRealPartOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		limit         int
		state         v2.State
	}{
		{"Word completed", "office-minimal.docx", 0, v2.Completed},
		{"ODT completed", "office-odt-minimal.odt", 0, v2.Completed},
		{"Word omitted scope", "office-minimal.docx", 1, v2.Partial},
		{"ODT omitted scope", "office-odt-minimal.odt", 1, v2.Partial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := coverageFixture(t, tc.fixture, nil)
			limits := DefaultLimits()
			if tc.limit > 0 {
				limits.ScopeScalarOrigins = tc.limit
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			base, err := BuildPackageRecords(context.Background(), raw, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			coverage, err := CollectAnalysisCoverage(context.Background(), base, a, map[string]int{capability.OfficeTextID: 1, capability.OfficeMetadataID: 2}, 0)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(coverage)
			if err != nil {
				t.Fatal(err)
			}
			e, err := OperationCoverage(base, coverage, capability.OfficeTextID, v2.UnknownConfig(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if e.State != tc.state {
				t.Fatalf("state %s; want %s", e.State, tc.state)
			}
			again, err := OperationCoverage(base, coverage, capability.OfficeTextID, v2.UnknownConfig(), 1)
			if err != nil || !reflect.DeepEqual(e, again) {
				t.Fatal("unstable aggregate", err)
			}
			after, err := json.Marshal(coverage)
			if err != nil || string(before) != string(after) {
				t.Fatal("child coverage mutated", err)
			}
			if tc.state == v2.Partial {
				if len(e.Exclusions) != 1 || e.Exclusions[0].Scope == nil {
					t.Fatal("lost compressed part gap")
				}
				part := base.Evidence.Packages[0].Parts[0]
				for _, candidate := range base.Evidence.Packages[0].Parts {
					if coverage.Outcomes[0].PartRef != nil && candidate.PartRef == *coverage.Outcomes[0].PartRef {
						part = candidate
					}
				}
				excluded := e.Exclusions[0].Scope
				if excluded.ArtifactRef != base.Evidence.Packages[0].SourceArtifactRef || len(excluded.Regions) != 1 || excluded.Regions[0].Start != uint64(part.CompressedSpan.Start) || excluded.Regions[0].End != uint64(part.CompressedSpan.End) {
					t.Fatal("decompressed offsets reused as ZIP offsets")
				}
			}
			// Empty selected inventory is an assessed absence after selection, whereas
			// an enumeration diagnostic prevents an empty inventory claiming completion.
			empty := &AnalysisCoverage{}
			e, err = OperationCoverage(base, empty, capability.OfficeMetadataID, v2.UnknownConfig(), 2)
			if err != nil || e.State != v2.Completed {
				t.Fatal("assessed absence lost", err)
			}
			ref := "exec/2"
			empty.Diagnostics = []v2.Diagnostic{{Ref: "diagnostic/0", ExecutionRef: &ref, Stage: failure.Parsing, Code: failure.ResourceLimit, Message: "Enumeration incomplete", Details: v2.ErrorDetails{ErrorType: "OfficeEnumerationLimit"}}}
			e, err = OperationCoverage(base, empty, capability.OfficeMetadataID, v2.UnknownConfig(), 2)
			if err != nil || e.State != v2.Partial || len(e.Exclusions) != 1 || !e.Exclusions[0].UnknownRemainder {
				t.Fatal("enumeration gap reported as absence", err)
			}
			if _, err := OperationCoverage(base, coverage, capability.OfficeMetadataID, v2.UnknownConfig(), 1); err == nil {
				t.Fatal("accepted foreign child operation")
			}
			if len(coverage.Diagnostics) > 0 {
				coverage.Diagnostics = nil
				if _, err := OperationCoverage(base, coverage, capability.OfficeTextID, v2.UnknownConfig(), 1); err == nil {
					t.Fatal("accepted unexplained incomplete child")
				}
			}
		})
	}
}
