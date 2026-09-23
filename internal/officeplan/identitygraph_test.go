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
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func TestIdentityGraphRetainsDecisionAndLocatedIssues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state v2.State
		code  string
	}{
		{"DOCX", v2.Completed, ""}, {"ODT", v2.Completed, ""},
		{"conflict", v2.Partial, IdentityConflictCode},
		{"unidentified", v2.Partial, "office.identity_unconfirmed"},
		{"declaration issue", v2.Partial, "opc.content_types_structure_unknown"},
		{"mapping gap", v2.Partial, "office.xml_mapping_unavailable"},
		{"ODT with damaged OPC candidate", v2.Partial, "xml.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := "office-minimal.docx"
			if strings.HasPrefix(tc.name, "ODT") {
				fixture = "office-odt-minimal.odt"
			}
			parts := budgetBase(t, fixture)
			switch tc.name {
			case "unidentified":
				parts = map[string]string{"ordinary.txt": "plain data in ZIP"}
			case "declaration issue":
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", "<unknown/></Types>", 1)
			case "mapping gap":
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", strings.Repeat(" ", xmlparts.MaxScalarMappings+1)+"</Types>", 1)
			case "ODT with damaged OPC candidate":
				parts["[Content_Types].xml"] = "<broken>"
			}
			raw := budgetArchive(t, parts, false)
			if tc.name == "conflict" {
				raw = hybridFixture(t, false)
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			assembly, err := Assemble(context.Background(), raw, p, a, 1, 4)
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := DefaultLimits().Catalog("test")
			if err != nil {
				t.Fatal(err)
			}
			var descriptor v2.Capability
			for _, c := range catalog {
				if c.ID == capability.OfficeIdentifyID {
					descriptor = c
				}
			}
			before, err := json.Marshal(assembly)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := BuildIdentityGraph(context.Background(), p, assembly, descriptor, v2.UnknownConfig(), 2, 10)
			if err != nil {
				t.Fatal(err)
			}
			again, err := BuildIdentityGraph(context.Background(), p, assembly, descriptor, v2.UnknownConfig(), 2, 10)
			if err != nil || !reflect.DeepEqual(graph, again) {
				t.Fatal("unstable identity graph", err)
			}
			after, err := json.Marshal(assembly)
			if err != nil || string(before) != string(after) {
				t.Fatal("input mutated", err)
			}
			if len(graph.Trace.Executions) != 1 || graph.Trace.Executions[0].State != tc.state {
				t.Fatal("incorrect parent state", graph.Trace.Executions)
			}
			if tc.code != "" && !slices.ContainsFunc(graph.Trace.Diagnostics, func(d v2.Diagnostic) bool { return string(d.Code) == tc.code }) {
				t.Fatalf("missing %s: %+v", tc.code, graph.Trace.Diagnostics)
			}
			for _, issue := range graph.Issues {
				if issue.DiagnosticRef == nil || !slices.ContainsFunc(graph.Trace.Diagnostics, func(d v2.Diagnostic) bool { return d.Ref == *issue.DiagnosticRef && string(d.Code) == issue.Code }) {
					t.Fatal("unlinked identity issue", issue)
				}
			}
			if tc.name == "declaration issue" {
				found := false
				for _, d := range graph.Trace.Diagnostics {
					if string(d.Code) == tc.code && d.Scope != nil && d.Scope.Unit == "byte" {
						found = true
					}
				}
				if !found {
					t.Fatal("lost original XML element location")
				}
			}
			p.Identity = IdentityUnidentified
			if tc.name != "unidentified" {
				if _, err := BuildIdentityGraph(context.Background(), p, assembly, descriptor, v2.UnknownConfig(), 2, 10); err == nil {
					t.Fatal("mismatched identity decision accepted")
				}
			}
		})
	}
}
