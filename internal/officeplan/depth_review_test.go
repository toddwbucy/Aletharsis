package officeplan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestPublishedBoundariesSurviveWithAndWithoutScopes(t *testing.T) {
	for _, withText := range []bool{false, true} {
		t.Run(fmt.Sprint(withText), func(t *testing.T) {
			parts := budgetBase(t, "office-minimal.docx")
			body := `<w:sdt><w:p><w:r><w:t>blocked</w:t></w:r></w:p></w:sdt>`
			if withText {
				body = `<w:p><w:r><w:t>before</w:t></w:r></w:p>` + body + `<w:p><w:r><w:t>after</w:t></w:r></w:p>`
			}
			parts["word/document.xml"] = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`
			raw := budgetArchive(t, parts, false)
			hash, size := evidence.Hash(raw), len(raw)
			p, err := Prepare(context.Background(), raw, hash, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]int{}
			for _, part := range a.Parts {
				if part.Word != nil {
					for _, b := range part.Word.Boundaries {
						want[fmt.Sprintf("%d/%s", b.Token, b.Reason)]++
					}
				}
			}
			if len(want) == 0 {
				t.Fatal("probe lacks boundaries")
			}
			out, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "boundary.docx", Filename: "boundary.docx", SHA256: &hash, Size: &size}, p, a, "test")
			if err != nil {
				t.Fatal(err)
			}
			for _, execution := range out.Report.Trace.Executions {
				if execution.CapabilityRef == capability.OfficeTextID && execution.State != v2.Partial {
					t.Fatal("unrecognized context lost its coverage gap")
				}
			}
			got := map[string]int{}
			for _, scope := range out.Report.Evidence.Office.Scopes {
				for _, b := range scope.Boundaries {
					got[fmt.Sprintf("%d/%s", b.Token, b.Reason)]++
				}
			}
			if withText {
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("boundaries lost or duplicated: %v want %v", got, want)
				}
			} else {
				count := 0
				for _, d := range out.Report.Trace.Diagnostics {
					if strings.HasPrefix(string(d.Code), "office.boundary.") {
						if d.Scope == nil || d.Scope.Unit != "byte" || len(d.Scope.Regions) != 1 {
							t.Fatal("boundary is not located")
						}
						count++
					}
				}
				total := 0
				for _, n := range want {
					total += n
				}
				if count != total || len(out.Report.Evidence.Office.Scopes) != 0 {
					t.Fatal("boundary loss or fabricated scope", count, total)
				}
			}
		})
	}
}

func odtWithFailedDOCX(t *testing.T) ([]byte, *Prepared, *Analysis) {
	t.Helper()
	parts := budgetBase(t, "office-odt-minimal.odt")
	docx := budgetBase(t, "office-minimal.docx")
	budgetMetadata(docx)
	budgetEmbedding(docx, "word/embeddings/obj1.bin", "PK\x03\x04payload")
	budgetRelationship(docx, "word/_rels/document.xml.rels", `<Relationship Id="embed" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/obj1.bin"/>`)
	for k, v := range docx {
		parts[k] = v
	}
	parts["word/document.xml"] = "<foreign/>"
	raw := budgetArchive(t, parts, false)
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if p.Identity != IdentityODT || !p.hasDOCXDeclarations() {
		t.Fatal("hybrid precondition")
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return raw, p, a
}
func TestODTHybridDoesNotClaimDOCXOnlyCoverage(t *testing.T) {
	raw, p, a := odtWithFailedDOCX(t)
	hash, size := evidence.Hash(raw), len(raw)
	out, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "hybrid.odt", Filename: "hybrid.odt", SHA256: &hash, Size: &size}, p, a, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range out.Report.Trace.Executions {
		if e.CapabilityRef == capability.OfficeMetadataID || e.CapabilityRef == capability.OfficeRelationshipsID || e.CapabilityRef == capability.OfficeObjectsID {
			if e.State != v2.Partial || e.ReasonCode == nil || len(e.Exclusions) == 0 || len(e.AnalyzedScope) != 1 || e.AnalyzedScope[0].Unit != "byte" {
				t.Fatal("DOCX-only coverage claimed ODT", e)
			}
		}
	}
	if len(out.Report.Evidence.Office.XML) == 0 || len(out.Report.Evidence.Office.Metadata) == 0 || len(out.Report.Evidence.Office.Objects) != 1 || len(out.Report.Evidence.Office.Relationships) == 0 {
		t.Fatal("declaration evidence lost")
	}
}
func TestODTHybridPreservesObjectInterruption(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		raw, p, a := odtWithFailedDOCX(t)
		a.Objects = nil
		a.ObjectsError = cause
		assembly, err := Assemble(context.Background(), raw, p, a, 1, 3)
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := p.Limits.Catalog("test")
		if err != nil {
			t.Fatal(err)
		}
		data, err := capability.NativeDataRevision()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range catalog {
			if c.ID == capability.OfficeObjectsID {
				config, err := v2.NewNativeConfig(c.Revision, data, c.Limits)
				if err != nil {
					t.Fatal(err)
				}
				graph, err := BuildObjectGraph(context.Background(), p, a, assembly, c, config, 3, 0)
				if err != nil {
					t.Fatal(err)
				}
				e := graph.Trace.Executions[0]
				want := failure.Canceled
				if errors.Is(cause, context.DeadlineExceeded) {
					want = failure.Timeout
				}
				if e.ReasonCode == nil || *e.ReasonCode != want {
					t.Fatal("interruption replaced with unsupported", e)
				}
			}
		}
	}
}
func TestEqualTargetsKeepDeclarationOrderAndIdentityFailureCode(t *testing.T) {
	parts := budgetBase(t, "office-minimal.docx")
	parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="a" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="missing.xml"/><Relationship Id="b" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="missing.xml"/></Relationships>`
	raw := budgetArchive(t, parts, false)
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	first := int64(-1)
	for _, target := range p.Targets {
		if strings.HasSuffix(target.Name, "missing.xml") && first < 0 {
			first = target.Declaration.Span.Start
		}
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, part := range a.Parts {
		if strings.HasSuffix(part.Part, "missing.xml") {
			found = true
			if part.Declaration == nil || part.Declaration.Span.Start != first {
				t.Fatal("declaration order changed")
			}
		}
	}
	if !found || first < 0 {
		t.Fatal("missing duplicate-target probe")
	}
	for i := range p.Admission.Outcomes.Parts {
		if p.Admission.Outcomes.Parts[i].Part.Name == p.DOCX.MainPart {
			p.Admission.Outcomes.Parts[i].Part.SHA256 = ""
		}
	}
	a, err = AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range a.Parts {
		if part.Part == p.DOCX.MainPart && part.Code != string(failure.ExecutionFailed) {
			t.Fatal("identity failure mislabeled", part.Code)
		}
	}
}

func TestUnknownMetadataRemainderDoesNotCreditPayloads(t *testing.T) {
	parts := budgetBase(t, "office-minimal.docx")
	parts["stray.unknown"] = "unclassified"
	raw := budgetArchive(t, parts, false)
	hash, size := evidence.Hash(raw), len(raw)
	p, err := Prepare(context.Background(), raw, hash, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "stray.docx", Filename: "stray.docx", SHA256: &hash, Size: &size}, p, a, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range out.Report.Trace.Executions {
		if ex.CapabilityRef != capability.OfficeMetadataID {
			continue
		}
		if ex.State != v2.Partial {
			t.Fatal("unknown remainder lost")
		}
		for _, scope := range ex.AnalyzedScope {
			if scope.Unit != "byte" {
				t.Fatal("whole-source assessment claimed")
			}
			for _, region := range scope.Regions {
				for _, part := range out.Report.Evidence.Office.Packages[0].Parts {
					span := part.CompressedSpan
					if span.Start < span.End && region.Start < uint64(span.End) && uint64(span.Start) < region.End {
						t.Fatal("unassessed payload credited")
					}
				}
			}
		}
		return
	}
	t.Fatal("missing metadata operation")
}
