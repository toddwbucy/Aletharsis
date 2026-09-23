package officeplan

import (
	"context"
	"errors"
	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"slices"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func TestUnavailableTextAndMetadataPreservePrerequisiteReason(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/word/header.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`, 1)
		parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="core.xml"/></Relationships>`, 1)
		parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="h" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header.xml"/></Relationships>`
		parts["core.xml"] = `<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>` + strings.Repeat("x", 8192) + `</dc:creator></cp:coreProperties>`
		parts["word/header.xml"] = `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>` + strings.Repeat("x", 8192) + `</w:t></w:r></w:p></w:hdr>`
	})
	for _, code := range []string{"office.part_limit", "office.aggregate_limit"} {
		t.Run(code, func(t *testing.T) {
			limits := DefaultLimits()
			if code == "office.part_limit" {
				limits.Package.PartBytes = 4096
			} else {
				limits.Package.TotalBytes = 4096
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			if p.Identity != IdentityDOCX {
				t.Fatal("unrelated target gap invalidated main identity")
			}
			// The relaxation is only for a fully enumerated root inventory
			// with one resolved main declaration; each independent gap blocks it.
			for _, damage := range []func(*opcrels.Result){
				func(o *opcrels.Result) {
					for i := range o.Parts {
						if o.Parts[i].Part == "_rels/.rels" {
							o.Parts[i].LimitCode = "opc.resource_limit"
						}
					}
				},
				func(o *opcrels.Result) {
					for i := range o.Parts {
						if o.Parts[i].Part == "_rels/.rels" {
							o.Parts[i].SourceCode = "opc.source_missing"
						}
					}
				},
				func(o *opcrels.Result) {
					for i := range o.Parts {
						if o.Parts[i].Part == "_rels/.rels" {
							o.Parts[i].Code = "opc.relationship_structure_unknown"
						}
					}
				},
				func(o *opcrels.Result) {
					for i := range o.Relationships {
						if o.Relationships[i].Type == docxidentify.TransitionalRelationship {
							o.Relationships[i].State = "missing"
						}
					}
				},
				func(o *opcrels.Result) {
					for _, r := range o.Relationships {
						if r.Type == docxidentify.TransitionalRelationship {
							o.Relationships = append(o.Relationships, r)
							break
						}
					}
				},
			} {
				changed := *p.DOCX.OPC
				changed.Parts = slices.Clone(changed.Parts)
				changed.Relationships = slices.Clone(changed.Relationships)
				damage(&changed)
				prepared, err := docxidentify.PrepareTypes(context.Background(), &changed)
				if err != nil || prepared.MainCandidate() != "" {
					t.Fatal("priority candidate accepted incomplete root", err)
				}
				identified, err := docxidentify.InspectVerified(context.Background(), &changed)
				if err != nil || identified.Format == "docx" {
					t.Fatal("incomplete/ambiguous root accepted", err)
				}
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			unavailable, usable := 0, 0
			for _, record := range a.Parts {
				if record.Part == "core.xml" || record.Part == "word/header.xml" {
					unavailable++
					if record.State != "not_run" || record.Code != code || record.Error == nil || errors.Is(record.Error, packageparts.ErrIdentity) || record.Word != nil || record.Metadata != nil {
						t.Fatal("prerequisite reason collapsed", record)
					}
				} else if record.Word != nil && len(record.Word.Scopes) > 0 && len(record.Word.Scopes[0].Findings) > 0 {
					usable++
				}
			}
			if unavailable != 2 || usable != 1 {
				t.Fatal("lost unavailable target or usable sibling", a.Parts)
			}
			base, err := BuildPackageRecords(context.Background(), raw, p, 1)
			if err != nil {
				t.Fatal(err)
			}
			coverage, err := CollectAnalysisCoverage(context.Background(), base, a, map[string]int{capability.OfficeTextID: 4, capability.OfficeMetadataID: 3}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(coverage.Diagnostics) != 2 {
				t.Fatal("prerequisite diagnostics lost")
			}
			for _, d := range coverage.Diagnostics {
				if string(d.Code) != code {
					t.Fatal("reason changed during wire conversion", d)
				}
			}
		})
	}
}
