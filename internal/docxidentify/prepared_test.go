package docxidentify

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func prepare(t *testing.T, parts map[string]string) (*packageparts.OutcomeReader, *opcrels.Session, *PreparedTypes) {
	t.Helper()
	raw := archive(t, parts)
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"[Content_Types].xml", "_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	session, err := opcrels.NewSession(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Parse(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	opc, err := session.Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareTypes(context.Background(), opc)
	if err != nil {
		t.Fatal(err)
	}
	return reader, session, prepared
}
func TestPreparedTypesChooseNondefaultMainAndReuseParse(t *testing.T) {
	parts := base("custom/main.XML")
	parts["word/document.xml"] = "<decoy/>"
	reader, session, prepared := prepare(t, parts)
	if prepared.MainCandidate() != "custom/main.XML" {
		t.Fatal("did not honor declared main", prepared.MainCandidate())
	}
	for _, o := range reader.View().Parts {
		if o.Part.Name == "custom/main.XML" && o.State != "not_run" {
			t.Fatal("preparation admitted content")
		}
	}
	names := []string{}
	for _, o := range reader.View().Parts {
		names = append(names, o.Part.Name)
	}
	if err := reader.Admit(context.Background(), names); err != nil {
		t.Fatal(err)
	}
	opc, err := session.Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := InspectPrepared(context.Background(), opc, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if got.TypesXML != prepared.result.TypesXML || got.MainPart != "custom/main.XML" || got.Format != "docx" {
		t.Fatal("prepared inspection lost identity")
	}
	legacy := inspect(t, parts)
	got.OPC.Outcomes = nil
	legacy.OPC.Outcomes = nil
	legacy.OPC.Package = nil
	if !reflect.DeepEqual(legacy, got) {
		t.Fatal("prepared identification differs from legacy")
	}
}
func TestPreparedMainRejectsAmbiguousExternalOrWrongType(t *testing.T) {
	cases := map[string]func(map[string]string){
		"external": func(p map[string]string) {
			p["_rels/.rels"] = strings.Replace(p["_rels/.rels"], `Target="custom/main.xml"`, `Target="custom/main.xml" TargetMode="External"`, 1)
		},
		"wrong type": func(p map[string]string) {
			p["[Content_Types].xml"] = strings.ReplaceAll(p["[Content_Types].xml"], MainContentType, "application/xml")
		},
		"duplicate relationship": func(p map[string]string) {
			p["_rels/.rels"] = strings.Replace(p["_rels/.rels"], "</Relationships>", `<Relationship Id="another" Type="`+TransitionalRelationship+`" Target="custom/main.xml"/></Relationships>`, 1)
		},
		"ambiguous part": func(p map[string]string) { p["CUSTOM/MAIN.XML"] = p["custom/main.xml"] },
		"invalid root":   func(p map[string]string) { p["_rels/.rels"] = `<wrong/>` },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			parts := base("custom/main.xml")
			mutate(parts)
			_, _, prepared := prepare(t, parts)
			if prepared.MainCandidate() != "" {
				t.Fatal("invented main prerequisite")
			}
		})
	}
}
