package officeplan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestCanceledAnalysisRetainsEverySelectedTarget(t *testing.T) {
	for _, fixture := range []string{"office-minimal.docx", "office-odt-minimal.odt"} {
		for _, timeout := range []bool{false, true} {
			t.Run(fixture+map[bool]string{false: "/cancel", true: "/deadline"}[timeout], func(t *testing.T) {
				parts := budgetBase(t, fixture)
				if fixture == "office-minimal.docx" {
					budgetMetadata(parts)
					budgetStory(parts, "header.xml", "header", "hdr", `<w:p><w:r><w:t>header</w:t></w:r></w:p>`)
				}
				raw := budgetArchive(t, parts, false)
				prepared, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				wantErr, wantCode := context.Canceled, failure.Canceled
				if timeout {
					cancel()
					ctx, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
					wantErr, wantCode = context.DeadlineExceeded, failure.Timeout
				}
				cancel()
				result, err := AnalyzePrepared(ctx, prepared)
				if !errors.Is(err, wantErr) {
					t.Fatalf("error = %v; want %v", err, wantErr)
				}
				assertTargetSets(t, prepared, result)
				if len(result.Parts) == 0 {
					t.Fatal("cancellation erased selected targets")
				}
				for _, part := range result.Parts {
					if part.State != "not_run" || part.Code != string(wantCode) || !errors.Is(part.Error, wantErr) || part.Word != nil || part.ODT != nil || part.Metadata != nil {
						t.Fatalf("unstarted target has incorrect coverage: %+v", part)
					}
				}
				if fixture == "office-minimal.docx" && (result.Objects != nil || !errors.Is(result.ObjectsError, wantErr)) {
					t.Fatal("object inspection ran after cancellation")
				}
				base, err := BuildPackageRecords(context.Background(), raw, prepared, 0)
				if err != nil {
					t.Fatal(err)
				}
				coverage, err := CollectAnalysisCoverage(context.Background(), base, result, map[string]int{capability.OfficeTextID: 1, capability.OfficeMetadataID: 2}, 0)
				if err != nil {
					t.Fatal(err)
				}
				if len(coverage.Outcomes) != len(result.Parts) || len(coverage.Diagnostics) != len(result.Parts) {
					t.Fatal("coverage dropped canceled targets")
				}
				for _, outcome := range coverage.Outcomes {
					if outcome.State != "not_run" || len(outcome.Assessed) != 0 || len(outcome.Excluded) != 1 {
						t.Fatalf("unstarted target claims assessed bytes: %+v", outcome)
					}
				}
			})
		}
	}
}
