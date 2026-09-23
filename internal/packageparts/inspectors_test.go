package packageparts_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func TestVerifiedInspectorsRetainGoodEvidenceAndMissingPrerequisites(t *testing.T) {
	cases := []struct {
		name, format  string
		prerequisites []string
	}{
		{"office-minimal.docx", "docx", []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"}},
		{"office-partial-crc.docx", "docx", []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"}},
		{"office-odt-minimal.odt", "odt", []string{"mimetype", "META-INF/manifest.xml", "content.xml"}},
	}
	for _, tc := range cases {
		for _, skip := range append([]string{""}, tc.prerequisites...) {
			t.Run(tc.name+"/skip="+skip, func(t *testing.T) {
				raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + tc.name)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				names := []string{}
				for _, o := range reader.View().Parts {
					if o.Part.Name != skip {
						names = append(names, o.Part.Name)
					}
				}
				if err := reader.Admit(context.Background(), names); err != nil {
					t.Fatal(err)
				}
				before := reader.View()
				clear(raw)
				var format, state string
				if tc.format == "docx" {
					opc, err := opcrels.InspectVerified(context.Background(), reader)
					if err != nil {
						t.Fatal(err)
					}
					r, err := docxidentify.InspectVerified(context.Background(), opc)
					if err != nil {
						t.Fatal(err)
					}
					if r.OPC != opc {
						t.Fatal("did not retain shared OPC inventory")
					}
					format, state = r.Format, r.State
					if skip != "" {
						want := map[string]string{"[Content_Types].xml": "opc.content_types_unavailable", "_rels/.rels": "docx.root_relationships_incomplete", "word/document.xml": "docx.main_part_unavailable"}[skip]
						found := false
						for _, issue := range r.Issues {
							if issue.Code == want {
								found = true
							}
						}
						if !found {
							t.Fatalf("missing %s: %+v", want, r.Issues)
						}
					}
				} else {
					r, err := odtidentify.InspectVerified(context.Background(), reader)
					if err != nil {
						t.Fatal(err)
					}
					format, state = r.Format, r.State
					if r.Outcomes == nil {
						t.Fatal("lost package outcomes")
					}
					if skip != "" {
						want := map[string]string{"mimetype": "odt.mimetype_unavailable", "META-INF/manifest.xml": "odt.manifest_unavailable", "content.xml": "odt.content_identity_unavailable"}[skip]
						found := false
						for _, issue := range r.Issues {
							if issue.Code == want {
								found = true
							}
						}
						if !found {
							t.Fatalf("missing %s: %+v", want, r.Issues)
						}
					}
				}
				if skip == "" && format != tc.format {
					t.Fatal("failed identification despite verified prerequisites", format, state)
				}
				if skip != "" && (state != "partial" || format == tc.format) {
					t.Fatal("claimed identity with unassessed prerequisite", format, state)
				}
				if !reflect.DeepEqual(before, reader.View()) {
					t.Fatal("inspector changed admission evidence")
				}
			})
		}
	}
}
