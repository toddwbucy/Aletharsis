package officeplan

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestBuildOfficeReportsWithoutWireFixtureTemplates(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx", "inventory", "omitted", "high findings", "report limit"} {
		t.Run(name, func(t *testing.T) {
			fixture := name
			if name == "inventory" || name == "omitted" || name == "high findings" || name == "report limit" {
				fixture = "office-minimal.docx"
			}
			baseFixture := fixture
			if name == "office-partial-crc.docx" {
				baseFixture = "office-minimal.docx"
			}
			parts := budgetBase(t, baseFixture)
			limits := DefaultLimits()
			if name == "inventory" {
				budgetMetadata(parts)
				budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
			}
			if name == "omitted" {
				limits.ScopeScalarOrigins = 1
			}
			if name == "high findings" {
				parts["word/document.xml"] = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + strings.Repeat("\u200b\u200c", 64) + `</w:t></w:r></w:p></w:body></w:document>`
			}
			if name == "report limit" {
				limits.Report.OutputBytes = 1024
			}
			raw := budgetArchive(t, parts, false)
			if name == "office-partial-crc.docx" {
				raw = coverageFixture(t, name, nil)
			}
			hash, size := evidence.Hash(raw), len(raw)
			file := evidence.File{Path: fixture, Filename: fixture, Extension: filepath.Ext(fixture), SHA256: &hash, Size: &size}
			p, err := Prepare(context.Background(), raw, hash, limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			result, err := buildFixtureReport(context.Background(), raw, file, p, a, "test")
			if name == "report limit" {
				if result != nil || !errors.Is(err, identity.ErrLimit) {
					t.Fatal("over-limit report returned output", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			again, err := buildFixtureReport(context.Background(), raw, file, p, a, "test")
			if err != nil || string(result.JSON) != string(again.JSON) {
				t.Fatal("report not deterministic", err)
			}
			decoded, err := v4.DecodeReport(result.JSON, limits.Report)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.CatalogVersion != capability.OfficeCatalogVersion || decoded.Status != v2.Partial || decoded.Summary["exit_code"] != 4 {
				t.Fatal("unavailable checks hidden", decoded.Status, decoded.Summary)
			}
			refs := map[string]bool{}
			for _, e := range decoded.Trace.Executions {
				refs[e.Ref] = true
			}
			for i := range decoded.Trace.Executions {
				if !refs[fmt.Sprintf("exec/%d", i)] {
					t.Fatal("reserved execution ordinal missing", i)
				}
			}
			refs = map[string]bool{}
			for _, d := range decoded.Trace.Diagnostics {
				refs[d.Ref] = true
			}
			for i := range decoded.Trace.Diagnostics {
				if !refs[fmt.Sprintf("diagnostic/%d", i)] {
					t.Fatal("allocated diagnostic missing", i)
				}
			}
			planned := map[string]bool{}
			for _, e := range decoded.Trace.Executions {
				planned[e.CapabilityRef] = true
				if e.CapabilityRef == capability.OfficeProfilesID && (e.State != v2.NotRun || e.ReasonCode == nil || *e.ReasonCode != "profile.no_applicable_profile") {
					t.Fatal("synthetic profiles became production policy")
				}
			}
			catalog, err := limits.Catalog("test")
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range catalog {
				if !planned[c.ID] {
					t.Fatal("catalog operation omitted", c.ID)
				}
			}
			if name == "omitted" {
				if len(decoded.Findings) != 0 || len(decoded.Evidence.Office.Scopes) != 0 {
					t.Fatal("omitted scope gained findings")
				}
			} else if len(decoded.Findings) == 0 {
				t.Fatal("lost actual Unicode finding")
			}
			if name == "inventory" && (len(decoded.Evidence.Office.Metadata) != 2 || len(decoded.Evidence.Office.Objects) != 1) {
				t.Fatal("lost inventories")
			}
			if name == "high findings" && decoded.Summary["high"] == 0 {
				t.Fatal("high-severity findings lost")
			}
			if !reflect.DeepEqual(decoded.Summary, result.Report.Summary) {
				t.Fatal("summary changed on import")
			}
			if evidence.Hash(raw) != hash {
				t.Fatal("source changed")
			}
		})
	}
}

func TestBuildReportRetainsUnidentifiedAndConflictingContainers(t *testing.T) {
	for _, name := range []string{"unidentified", "invalid main", "conflict", "conflict reordered"} {
		t.Run(name, func(t *testing.T) {
			var raw []byte
			code := "office.identity_unconfirmed"
			switch name {
			case "conflict", "conflict reordered":
				raw = hybridFixture(t, name == "conflict reordered")
				code = IdentityConflictCode
			case "invalid main":
				parts := budgetBase(t, "office-minimal.docx")
				parts["word/document.xml"] = "<unrecognized>text\u200b</unrecognized>"
				raw = budgetArchive(t, parts, false)
			default:
				raw = budgetArchive(t, map[string]string{"payload.txt": "text\u200bwith concealed Unicode"}, false)
			}
			digest, size := evidence.Hash(raw), len(raw)
			file := evidence.File{Path: "misleading.docx", Filename: "misleading.docx", Extension: ".docx", SHA256: &digest, Size: &size}
			p, err := Prepare(context.Background(), raw, digest, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			output, err := buildFixtureReport(context.Background(), raw, file, p, a, "test")
			if err != nil {
				t.Fatal(err)
			}
			report, err := v4.DecodeReport(output.JSON, DefaultLimits().Report)
			if err != nil {
				t.Fatal(err)
			}
			if report.File.Format != "unknown" || report.File.MIME != "application/zip" || report.Status != v2.Partial || report.Summary["exit_code"] != 4 {
				t.Fatal("ambiguous ZIP misclassified", report.File, report.Status)
			}
			if !strings.HasPrefix(name, "conflict") && (len(report.Findings) != 0 || len(report.Evidence.Texts) != 0 || len(report.Evidence.Office.Scopes) != 0 || len(report.Trace.Results) != 0) {
				t.Fatal("unidentified container fell back to text or negative scan")
			}
			if len(report.Evidence.Office.Packages) != 1 || len(report.Evidence.Office.Packages[0].Parts) != len(p.Admission.Outcomes.Parts) {
				t.Fatal("retained package evidence lost")
			}
			seen := map[string]bool{}
			for _, e := range report.Trace.Executions {
				if e.CapabilityRef == capability.OfficeTextID || e.CapabilityRef == capability.OfficeMetadataID || e.CapabilityRef == capability.OfficeRelationshipsID {
					seen[e.CapabilityRef] = true
					if (e.CapabilityRef == capability.OfficeRelationshipsID && name != "unidentified") || strings.HasPrefix(name, "conflict") || name == "invalid main" {
						if (e.State != v2.Completed && e.State != v2.Partial) || len(e.AnalyzedScope) == 0 {
							t.Fatal("verified operation suppressed", e)
						}
					} else if e.State != v2.NotRun || e.ReasonCode == nil || string(*e.ReasonCode) != code || len(e.AnalyzedScope) != 0 {
						t.Fatal("dependent operation claimed coverage", e)
					}
				}
			}
			if len(seen) != 3 {
				t.Fatal("missing dependent execution")
			}
			if strings.HasPrefix(name, "conflict") {
				if len(report.Evidence.Office.Scopes) != 2 || len(report.Trace.Results) != 6 || len(report.Findings) == 0 {
					t.Fatal("independent hybrid scans lost")
				}
				count := 0
				for _, o := range report.Evidence.Office.Packages[0].Outcomes {
					if o.Operation == capability.OfficeTextID {
						count++
						if o.State != "completed" || len(o.Assessed) == 0 || len(o.Codes) != 0 {
							t.Fatal("conflicting content path lost its gap", o)
						}
					}
				}
				if count != 2 {
					t.Fatal("one conflicting content path silently disappeared")
				}
			}
		})
	}
}
