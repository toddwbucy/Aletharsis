package officeplan

import (
	"context"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

// These fixtures are already acquired in memory. Make their synthetic
// acquisition explicit rather than depending on the test host's native reader.
func buildFixtureReport(ctx context.Context, source []byte, file evidence.File, p *Prepared, a *Analysis, version string) (*ReportOutput, error) {
	catalog, err := p.Limits.Catalog(version)
	if err != nil {
		return nil, err
	}
	for i := range catalog {
		if catalog[i].ID == capability.AcquireID {
			catalog[i].Availability = v2.Availability{State: "available"}
		}
	}
	return buildReportWithCatalog(ctx, source, file, p, a, version, catalog)
}
func TestUnavailableAcquisitionIsCoverageNotLinkage(t *testing.T) {
	raw := budgetArchive(t, budgetBase(t, "office-minimal.docx"), false)
	hash, size := evidence.Hash(raw), len(raw)
	p, err := Prepare(context.Background(), raw, hash, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"unavailable", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			catalog, err := p.Limits.Catalog("test")
			if err != nil {
				t.Fatal(err)
			}
			reason := failure.Code("integrity.no_atime_unavailable")
			for i := range catalog {
				if catalog[i].ID == capability.AcquireID {
					if mode == "disabled" {
						catalog[i].Participation = v2.Disabled
						reason = failure.Disabled
					} else {
						catalog[i].Availability = v2.Availability{State: "unavailable", ReasonCode: &reason}
					}
				}
			}
			report, err := buildReportWithCatalog(context.Background(), raw, evidence.File{Path: "input.docx", Filename: "input.docx", Extension: ".docx", SHA256: &hash, Size: &size}, p, a, "test", catalog)
			if err != nil {
				t.Fatal(err)
			}
			if report.Report.File.SHA256 != nil || len(report.Report.Evidence.Office.Packages) != 0 || report.Report.Summary["exit_code"] != 4 {
				t.Fatal("unavailable acquisition attested supplied evidence")
			}
			if report.Report.CatalogVersion != capability.OfficeCatalogVersion || len(report.Report.Trace.Capabilities) != len(catalog) {
				t.Fatal("Office request catalog lost")
			}
			found := false
			for _, e := range report.Report.Trace.Executions {
				if e.State != v2.NotRun || len(e.AnalyzedScope) != 0 {
					t.Fatal("operation ran without acquisition")
				}
				if e.CapabilityRef == capability.AcquireID {
					found = true
					if e.ReasonCode == nil || *e.ReasonCode != reason {
						t.Fatal("wrong acquisition reason")
					}
				}
			}
			if !found {
				t.Fatal("missing acquisition execution")
			}
		})
	}
}

// Exercise the public entry point on each CI host, in addition to the injected
// availability cases above. Non-Linux must emit coverage, not ErrLinkage.
func TestBuildReportHonorsNativeAcquisitionAvailability(t *testing.T) {
	raw := budgetArchive(t, budgetBase(t, "office-minimal.docx"), false)
	hash, size := evidence.Hash(raw), len(raw)
	p, err := Prepare(context.Background(), raw, hash, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := p.Limits.Catalog("test")
	if err != nil {
		t.Fatal(err)
	}
	out, err := BuildReport(context.Background(), raw, evidence.File{Path: "input.docx", Filename: "input.docx", SHA256: &hash, Size: &size}, p, a, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range catalog {
		if c.ID != capability.AcquireID {
			continue
		}
		for _, e := range out.Report.Trace.Executions {
			if e.CapabilityRef != c.ID {
				continue
			}
			if c.Availability.State == "available" {
				if e.State != v2.Completed || out.Report.File.SHA256 == nil {
					t.Fatal("available acquisition lost evidence")
				}
			} else if e.State != v2.NotRun || e.ReasonCode == nil || c.Availability.ReasonCode == nil || *e.ReasonCode != *c.Availability.ReasonCode || out.Report.File.SHA256 != nil {
				t.Fatal("unavailable acquisition not represented faithfully")
			}
			return
		}
	}
	t.Fatal("missing acquisition")
}
