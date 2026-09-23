package officeplan

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/scopelimits"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
)

func TestPartGapsDoNotPreventOfficeReport(t *testing.T) {
	for _, name := range []string{"foo.rels", "word/custom.rels", "nested/other.rels", "decoy metadata", "encrypted ODT"} {
		t.Run(name, func(t *testing.T) {
			fixture := "office-minimal.docx"
			if name == "decoy metadata" || name == "encrypted ODT" {
				fixture = "office-odt-minimal.odt"
			}
			parts := budgetBase(t, fixture)
			switch name {
			case "decoy metadata":
				docx := budgetBase(t, "office-minimal.docx")
				budgetMetadata(docx)
				for _, n := range []string{"[Content_Types].xml", "_rels/.rels", "docProps/core.xml", "docProps/app.xml"} {
					parts[n] = docx[n]
				}
			case "encrypted ODT":
				parts["META-INF/manifest.xml"] = strings.Replace(parts["META-INF/manifest.xml"], `manifest:media-type="text/xml"/>`, `manifest:media-type="text/xml"><manifest:encryption-data/></manifest:file-entry>`, 1)
			default:
				parts[name] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`
			}
			raw := budgetArchive(t, parts, false)
			digest, size := evidence.Hash(raw), len(raw)
			p, err := Prepare(context.Background(), raw, digest, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			output, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: fixture, Filename: fixture, SHA256: &digest, Size: &size}, p, a, "test")
			if err != nil {
				t.Fatal(err)
			}
			report, err := v4.DecodeReport(output.JSON, DefaultLimits().Report)
			if err != nil {
				t.Fatal(err)
			}
			if name != "encrypted ODT" && len(report.Findings) == 0 {
				t.Fatal("usable text findings lost")
			}
			found := 0
			for _, d := range report.Trace.Diagnostics {
				switch name {
				case "encrypted ODT":
					if d.Code == "odt.content_encrypted" {
						found++
					}
				case "decoy metadata":
					// Retain independently declared OOXML evidence; the parent
					// reports only restricted hybrid coverage, not a complete ODT scan.
					if len(report.Evidence.Office.Metadata) != 2 || len(report.Evidence.Office.Relationships) == 0 {
						t.Fatal("hybrid inventory lost decoy evidence")
					}
					if len(report.Evidence.Office.XML) > 0 {
						found++
					}
				default:
					if d.Code == "opc.relationship_name_unsupported" {
						found++
					}
				}
			}
			if found == 0 {
				t.Fatal("part diagnostic lost")
			}
			if name == "encrypted ODT" && len(report.Evidence.Office.Scopes) != 0 {
				t.Fatal("encrypted content analyzed")
			}
		})
	}
}

func TestPreparedReusesPayloadsAndIdentificationXML(t *testing.T) {
	for _, fixture := range []string{"office-minimal.docx", "office-odt-minimal.odt"} {
		raw := coverageFixture(t, fixture, nil)
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		for _, view := range []*packageparts.OutcomeView{p.DOCX.OPC.Outcomes, p.ODT.Outcomes} {
			for i, part := range p.Admission.Outcomes.Parts {
				if len(part.Bytes) > 0 && &part.Bytes[0] != &view.Parts[i].Bytes[0] {
					t.Fatal("retained payload copy")
				}
			}
		}
		for _, part := range a.Parts {
			source := []byte(budgetBase(t, fixture)[part.Part])
			limits := scopelimits.Limits{TextUTF8Bytes: p.Limits.ScopeTextUTF8Bytes, ScalarOrigins: p.Limits.ScopeScalarOrigins}
			if part.Word != nil {
				if part.Word.Extraction.XML.Document != p.DOCX.MainXML {
					t.Fatal("Word main parsed again")
				}
				expected, err := wordanalysis.AnalyzeWithLimits(context.Background(), source, evidence.Hash(source), limits)
				if err != nil || !reflect.DeepEqual(expected, part.Word) {
					t.Fatal("Word parsed entry changed semantics", err)
				}
			}
			if part.ODT != nil {
				if part.ODT.Extraction.XML.Document != p.ODT.ContentXML {
					t.Fatal("ODT main parsed again")
				}
				expected, err := odtanalysis.AnalyzeWithLimits(context.Background(), source, evidence.Hash(source), limits)
				if err != nil || !reflect.DeepEqual(expected, part.ODT) {
					t.Fatal("ODT parsed entry changed semantics", err)
				}
			}
		}
	}
}

func TestUnavailableMainKeepsAdmissionReason(t *testing.T) {
	for _, fixture := range []string{"office-minimal.docx", "office-odt-minimal.odt"} {
		for _, aggregate := range []bool{false, true} {
			parts := budgetBase(t, fixture)
			main := "word/document.xml"
			if strings.HasSuffix(fixture, "odt") {
				main = "content.xml"
			}
			parts[main] += strings.Repeat(" ", 8192)
			raw := budgetArchive(t, parts, false)
			limits := DefaultLimits()
			code := "office.part_limit"
			if aggregate {
				limits.Package.TotalBytes = 4096
				code = "office.aggregate_limit"
			} else {
				limits.Package.PartBytes = 4096
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, part := range a.Parts {
				if part.Part == main && part.Operation == capability.OfficeTextID {
					found = true
					if part.Code != code || part.State != "not_run" {
						t.Fatal("admission reason masked", part)
					}
				}
			}
			if !found {
				t.Fatal("unavailable main omitted")
			}
		}
	}
}

func TestPackageRecordArgumentsAreLinkageErrors(t *testing.T) {
	if _, err := BuildPackageRecords(context.Background(), nil, nil, -1); !errors.Is(err, v4.ErrLinkage) || errors.Is(err, packageparts.ErrIdentity) {
		t.Fatal(err)
	}
}

func TestHybridPreservesDOCXObjectInventory(t *testing.T) {
	for _, hybrid := range []bool{false, true} {
		parts := budgetBase(t, "office-minimal.docx")
		budgetMetadata(parts)
		budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
		if hybrid {
			for n, b := range budgetBase(t, "office-odt-minimal.odt") {
				parts[n] = b
			}
		}
		raw := budgetArchive(t, parts, false)
		digest, size := evidence.Hash(raw), len(raw)
		p, err := Prepare(context.Background(), raw, digest, DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		output, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "hybrid.docx", Filename: "hybrid.docx", SHA256: &digest, Size: &size}, p, a, "test")
		if err != nil {
			t.Fatal(err)
		}
		if len(output.Report.Evidence.Office.Objects) != 1 || len(output.Report.Evidence.Office.Metadata) != 2 {
			t.Fatal("hybrid suppressed inventory")
		}
		found := false
		for _, e := range output.Report.Trace.Executions {
			if e.CapabilityRef == capability.OfficeObjectsID {
				found = true
				if e.State != "completed" && e.State != "partial" {
					t.Fatal("object inspection unrun", e)
				}
			}
		}
		if !found {
			t.Fatal("missing object operation")
		}
	}
}

func TestUnavailableMainPreservesVerifiedSiblingEvidence(t *testing.T) {
	for _, mode := range []string{"healthy", "part limit", "malformed", "unassessed header"} {
		t.Run(mode, func(t *testing.T) {
			parts := budgetBase(t, "office-minimal.docx")
			budgetMetadata(parts)
			budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
			budgetStory(parts, "header1.xml", "header", "hdr", "<w:p><w:r><w:t>header\u200b</w:t></w:r></w:p>")
			parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
			limits := DefaultLimits()
			switch mode {
			case "part limit":
				parts["word/document.xml"] += strings.Repeat(" ", 8192)
				limits.Package.PartBytes = 4096
			case "malformed":
				parts["word/document.xml"] = "<broken"
			case "unassessed header":
				parts["word/header1.xml"] = `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">unrecognized direct text</w:hdr>`
			}
			raw := budgetArchive(t, parts, false)
			digest, size := evidence.Hash(raw), len(raw)
			p, err := Prepare(context.Background(), raw, digest, limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			output, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "siblings.docx", Filename: "siblings.docx", SHA256: &digest, Size: &size}, p, a, "test")
			if err != nil {
				t.Fatal(err)
			}
			r, err := v4.DecodeReport(output.JSON, limits.Report)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Evidence.Office.Metadata) != 2 || len(r.Evidence.Office.Objects) != 1 || len(r.Findings) == 0 {
				t.Fatal("lost independent sibling evidence")
			}
			wantScopes := 1
			if mode == "healthy" {
				wantScopes = 2
			} else if mode != "unassessed header" && r.File.Format != "unknown" {
				t.Fatal("invented format identity", r.File.Format)
			}
			if len(r.Evidence.Office.Scopes) != wantScopes {
				t.Fatal("lost sibling text", len(r.Evidence.Office.Scopes))
			}
			if mode == "part limit" {
				found := false
				for _, o := range r.Evidence.Office.Packages[0].Outcomes {
					if o.Operation == capability.OfficeTextID {
						for _, code := range o.Codes {
							if code == "office.part_limit" {
								found = true
							}
							if code == "office.identity_unconfirmed" {
								t.Fatal("masked real admission reason")
							}
						}
					}
				}
				if !found {
					t.Fatal("missing main admission reason")
				}
			}
		})
	}
}

// Reproduce the review's default-limit case without a surviving text scope:
// metadata and objects must survive, and the parent must retain the actual cause.
func TestDefaultMainLimitPreservesInventoryAndExecutionReason(t *testing.T) {
	parts := budgetBase(t, "office-minimal.docx")
	budgetMetadata(parts)
	budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
	parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
	limits := DefaultLimits()
	parts["word/document.xml"] += strings.Repeat(" ", int(limits.Package.PartBytes)+1)
	raw := budgetArchive(t, parts, false)
	digest, size := evidence.Hash(raw), len(raw)
	p, err := Prepare(context.Background(), raw, digest, limits)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	output, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "limited.docx", Filename: "limited.docx", SHA256: &digest, Size: &size}, p, a, "test")
	if err != nil {
		t.Fatal(err)
	}
	r, err := v4.DecodeReport(output.JSON, limits.Report)
	if err != nil {
		t.Fatal(err)
	}
	if r.File.Format != "unknown" || len(r.Evidence.Office.Metadata) != 2 || len(r.Evidence.Office.Objects) != 1 || len(r.Evidence.Office.Scopes) != 0 {
		t.Fatal("lost verified inventories or invented main text")
	}
	found := false
	for _, e := range r.Trace.Executions {
		if e.CapabilityRef == capability.OfficeTextID {
			found = true
			if e.State != "partial" || e.ReasonCode == nil || *e.ReasonCode != "office.part_limit" {
				t.Fatal("parent execution masked its child admission failure", e)
			}
		}
	}
	if !found {
		t.Fatal("missing text execution")
	}
}
