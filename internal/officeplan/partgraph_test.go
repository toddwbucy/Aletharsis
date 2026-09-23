package officeplan

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestPartGraphConnectsOfficeProducers(t *testing.T) {
	for _, name := range []string{"DOCX", "ODT", "omitted scopes", "bad metadata", "main relationships invalid", "relationship mapping limit", "objects", "canceled"} {
		t.Run(name, func(t *testing.T) {
			fixture := "office-minimal.docx"
			if name == "ODT" {
				fixture = "office-odt-minimal.odt"
			}
			parts := budgetBase(t, fixture)
			limits := DefaultLimits()
			switch name {
			case "omitted scopes":
				limits.ScopeScalarOrigins = 1
			case "bad metadata":
				budgetMetadata(parts)
				parts["docProps/core.xml"] = "<broken>"
			case "main relationships invalid":
				parts["word/_rels/document.xml.rels"] = "<broken>"
			case "relationship mapping limit":
				parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="` + opcrels.Namespace + `">` + strings.Repeat(" ", xmlparts.MaxScalarMappings+1) + `</Relationships>`
			case "objects":
				budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
			}
			raw := budgetArchive(t, parts, false)
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			analysisContext := context.Background()
			if name == "canceled" {
				canceled, cancel := context.WithCancel(analysisContext)
				cancel()
				analysisContext = canceled
			}
			a, err := AnalyzePrepared(analysisContext, p)
			if name == "canceled" && err != context.Canceled {
				t.Fatal("cancellation not retained", err)
			}
			if err != nil && name != "canceled" {
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
			configs := map[string]v2.Config{}
			for _, c := range catalog {
				configs[c.ID] = v2.UnknownConfig()
			}
			before, err := json.Marshal(assembly)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := BuildPartGraph(context.Background(), p, a, assembly, catalog, configs, 5, 0)
			if err != nil {
				t.Fatal(err)
			}
			again, err := BuildPartGraph(context.Background(), p, a, assembly, catalog, configs, 5, 0)
			if err != nil || !reflect.DeepEqual(graph, again) {
				t.Fatal("unstable graph", err)
			}
			after, err := json.Marshal(assembly)
			if err != nil || string(before) != string(after) {
				t.Fatal("assembly mutated", err)
			}
			if len(graph.Trace.Capabilities) != 3 || len(graph.Trace.Executions) != 3 {
				t.Fatal("operation omitted")
			}
			expected := map[string]v2.State{capability.OfficeTextID: v2.Completed, capability.OfficeMetadataID: v2.Completed, capability.OfficeRelationshipsID: v2.Completed}
			switch name {
			case "ODT":
				expected[capability.OfficeMetadataID] = v2.NotRun
				expected[capability.OfficeRelationshipsID] = v2.NotRun
			case "omitted scopes", "canceled":
				expected[capability.OfficeTextID] = v2.Partial
			case "bad metadata":
				expected[capability.OfficeMetadataID] = v2.Partial
			case "main relationships invalid":
				expected[capability.OfficeTextID] = v2.Partial
				expected[capability.OfficeRelationshipsID] = v2.Partial
			case "relationship mapping limit":
				expected[capability.OfficeRelationshipsID] = v2.Partial
			}
			for _, e := range graph.Trace.Executions {
				if e.State != expected[e.CapabilityRef] {
					t.Fatalf("%s: %s want %s; issues=%+v", e.CapabilityRef, e.State, expected[e.CapabilityRef], p.DOCX.Issues)
				}
			}
			trace, err := v4.IndexTrace(graph.Trace)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range assembly.Evidence.Packages[0].Outcomes {
				trace.Executions[o.ExecutionRef] = v2.Execution{CapabilityRef: o.Operation, State: v2.Partial}
			}
			for _, o := range assembly.Evidence.Objects {
				trace.Executions[o.Inspection.ExecutionRef] = v2.Execution{CapabilityRef: o.Inspection.Operation, State: v2.Partial}
			}
			// Validate all new child references alongside existing package/object records.
			assembled := assembly.Evidence
			assembled.Packages = append([]v4.Package{}, assembled.Packages...)
			assembled.Packages[0].Outcomes = append(append([]v4.Outcome{}, assembled.Packages[0].Outcomes...), graph.Outcomes...)
			index, err := v4.IndexEvidence(assembled, evidence.Hash(raw), int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			if err := validatePartialOutcomes(index, assembled, trace); err != nil {
				t.Fatal(err)
			}
			delete(configs, capability.OfficeTextID)
			if _, err := BuildPartGraph(context.Background(), p, a, assembly, catalog, configs, 5, 0); err == nil {
				t.Fatal("missing configuration accepted")
			}
		})
	}
}
