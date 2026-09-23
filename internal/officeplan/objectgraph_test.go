package officeplan

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestObjectGraphPreservesInspectionAndEnumerationStates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   v2.State
		objects int
		code    string
	}{
		{"empty", v2.Completed, 0, ""},
		{"available", v2.Completed, 1, ""},
		{"part limit", v2.Partial, 1, "office.part_limit"},
		{"CRC", v2.Partial, 1, "office.part_crc_failed"},
		{"missing", v2.Partial, 0, "opc.target_missing"},
		{"external", v2.Completed, 0, ""},
		{"mapping gap", v2.Partial, 1, "office.xml_mapping_unavailable"},
		{"empty mapping gap", v2.Partial, 0, "office.xml_mapping_unavailable"},
		{"canceled", v2.Canceled, 0, "execution.canceled"},
		{"ODT", v2.NotRun, 0, "execution.unsupported_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := "office-minimal.docx"
			if tc.name == "ODT" {
				fixture = "office-odt-minimal.odt"
			}
			parts := budgetBase(t, fixture)
			limits := DefaultLimits()
			if tc.objects > 0 || tc.name == "missing" || tc.name == "external" {
				budgetEmbedding(parts, "object.bin", strings.Repeat("payload", 512))
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
			}
			switch tc.name {
			case "part limit":
				limits.Package.PartBytes = 2048
			case "missing":
				delete(parts, "word/embeddings/object.bin")
			case "external":
				delete(parts, "word/embeddings/object.bin")
				parts["word/_rels/document.xml.rels"] = strings.Replace(parts["word/_rels/document.xml.rels"], `Target="embeddings/object.bin"`, `Target="https://example.invalid/payload" TargetMode="External"`, 1)
			case "mapping gap":
				parts["word/_rels/document.xml.rels"] = strings.Replace(parts["word/_rels/document.xml.rels"], "</Relationships>", strings.Repeat(" ", xmlparts.MaxScalarMappings+1)+"</Relationships>", 1)
			case "empty mapping gap":
				parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="` + opcrels.Namespace + `">` + strings.Repeat(" ", xmlparts.MaxScalarMappings+1) + `</Relationships>`
			}
			raw := budgetArchive(t, parts, false)
			if tc.name == "CRC" {
				raw = wrongCRCFixture(t, raw, "word/embeddings/object.bin")
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			analysisContext := context.Background()
			if tc.name == "canceled" {
				c, cancel := context.WithCancel(analysisContext)
				cancel()
				analysisContext = c
			}
			a, err := AnalyzePrepared(analysisContext, p)
			if err != nil && tc.name != "canceled" {
				t.Fatal(err)
			}
			assembly, err := Assemble(context.Background(), raw, p, a, 1, 4)
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := limits.Catalog("test")
			if err != nil {
				t.Fatal(err)
			}
			var descriptor v2.Capability
			for _, c := range catalog {
				if c.ID == capability.OfficeObjectsID {
					descriptor = c
				}
			}
			before, err := json.Marshal(assembly)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := BuildObjectGraph(context.Background(), p, a, assembly, descriptor, v2.UnknownConfig(), 4, 20)
			if err != nil {
				t.Fatal(err)
			}
			again, err := BuildObjectGraph(context.Background(), p, a, assembly, descriptor, v2.UnknownConfig(), 4, 20)
			if err != nil || !reflect.DeepEqual(graph, again) {
				t.Fatal("nondeterministic object graph", err)
			}
			after, err := json.Marshal(assembly)
			if err != nil || string(before) != string(after) {
				t.Fatal("input mutated", err)
			}
			if len(graph.Trace.Executions) != 1 || graph.Trace.Executions[0].State != tc.state || len(graph.Objects) != tc.objects {
				t.Fatalf("unexpected state/count: %+v", graph)
			}
			if tc.code != "" && !slices.ContainsFunc(graph.Trace.Diagnostics, func(d v2.Diagnostic) bool { return string(d.Code) == tc.code }) {
				t.Fatal("missing diagnostic", tc.code)
			}
			for _, o := range graph.Objects {
				for _, span := range o.Inspection.Assessed {
					if span.Start != 0 || span.End > 8 {
						t.Fatal("claims uninspected payload bytes")
					}
				}
				if tc.name == "part limit" || tc.name == "CRC" {
					if o.SHA256 != nil || o.Inspection.State != "not_run" || len(o.Inspection.DiagnosticRefs) == 0 || len(o.Inspection.Assessed) != 0 {
						t.Fatal("failed prerequisite gained object scan", o)
					}
				}
			}
			if a.Objects != nil && !reflect.DeepEqual(graph.Limitations, a.Objects.Limitations) {
				t.Fatal("lost producer limitations")
			}
			descriptor.Participation = v2.Disabled
			if _, err := BuildObjectGraph(context.Background(), p, a, assembly, descriptor, v2.UnknownConfig(), 4, 20); err == nil {
				t.Fatal("disabled producer accepted")
			}
		})
	}
}
