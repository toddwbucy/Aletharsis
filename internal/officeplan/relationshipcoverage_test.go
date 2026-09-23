package officeplan

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
)

func TestRelationshipCoverageRetainsOPCFailuresAndCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name, xml, code, state string
		limit                  bool
	}{
		{name: "empty", xml: `<Relationships xmlns="` + opcrels.Namespace + `"/>`, state: "completed"},
		{name: "missing target", xml: `<Relationships xmlns="` + opcrels.Namespace + `"><Relationship Id="a" Type="example" Target="absent.xml"/></Relationships>`, code: "opc.target_missing", state: "partial"},
		{name: "external target", xml: `<Relationships xmlns="` + opcrels.Namespace + `"><Relationship Id="a" Type="example" Target="https://example.invalid/" TargetMode="External"/></Relationships>`, state: "completed"},
		{name: "unknown structure", xml: `<Relationships xmlns="` + opcrels.Namespace + `"><unknown/></Relationships>`, code: "opc.relationship_structure_unknown", state: "partial"},
		{name: "malformed", xml: `<broken>`, code: "xml.invalid", state: "failed"},
		{name: "invalid root", xml: `<unknown/>`, code: "opc.relationship_root_invalid", state: "failed"},
		{name: "unavailable prerequisite", xml: strings.Repeat("x", 4096), code: "office.part_limit", state: "not_run", limit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := budgetBase(t, "office-minimal.docx")
			const name = "word/_rels/document.xml.rels"
			parts[name] = tc.xml
			raw := budgetArchive(t, parts, false)
			limits := DefaultLimits()
			if tc.limit {
				limits.Package.PartBytes = 2048
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			base, err := BuildPackageRecords(context.Background(), raw, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(p.DOCX.OPC)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CollectRelationshipCoverage(context.Background(), base, p.DOCX.OPC, 1, 3)
			if err != nil {
				t.Fatal(err)
			}
			again, err := CollectRelationshipCoverage(context.Background(), base, p.DOCX.OPC, 1, 3)
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("nondeterministic coverage", err)
			}
			after, err := json.Marshal(p.DOCX.OPC)
			if err != nil || string(before) != string(after) {
				t.Fatal("producer mutated", err)
			}
			if len(result.Outcomes) != len(p.DOCX.OPC.Parts) {
				t.Fatal("lost relationship part")
			}
			parent, err := OperationCoverage(base, result, capability.OfficeRelationshipsID, v2.UnknownConfig(), 1)
			if err != nil {
				t.Fatal(err)
			}
			wantParent := v2.Completed
			if tc.state != "completed" {
				wantParent = v2.Partial
			}
			if parent.State != wantParent {
				t.Fatalf("recoverable child %s produced parent %s", tc.state, parent.State)
			}
			var selected *v4.Outcome
			for i := range result.Outcomes {
				if result.Outcomes[i].PartRef != nil && *result.Outcomes[i].PartRef == base.PartRefs[name] {
					selected = &result.Outcomes[i]
				}
			}
			if selected == nil || selected.State != tc.state || tc.code != "" && !slices.Contains(selected.Codes, tc.code) {
				t.Fatalf("outcome = %+v", selected)
			}
			if tc.state == "failed" || tc.state == "not_run" {
				if len(selected.Assessed) != 0 {
					t.Fatal("unavailable operation claims assessed bytes")
				}
			}
			if tc.name == "missing target" {
				if len(selected.Excluded) != 0 || len(selected.Assessed) != 1 {
					t.Fatal("observed declaration excluded because its target is missing")
				}
				var found bool
				for _, d := range result.Diagnostics {
					if string(d.Code) == tc.code {
						found = true
						if d.Scope == nil || d.Scope.Unit != "byte" || len(d.Scope.Regions) != 1 {
							t.Fatal("missing declaration coordinates")
						}
						for _, part := range base.Evidence.Packages[0].Parts {
							if part.Name == name && d.Scope.ArtifactRef != part.ArtifactRef {
								t.Fatal("used compressed coordinates")
							}
						}
						r := d.Scope.Regions[0]
						if !strings.HasPrefix(tc.xml[r.Start:r.End], "<Relationship ") {
							t.Fatal("span does not identify declaration")
						}
					}
				}
				if !found {
					t.Fatal("missing target diagnostic")
				}
			}
			index, err := v4.IndexEvidence(base.Evidence, base.Evidence.Packages[0].SourceSHA256, int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			graph := &v4.TraceIndex{Executions: map[string]v2.Execution{"exec/0": {CapabilityRef: capability.ParseOfficeID, State: v2.Partial}, "exec/1": {CapabilityRef: capability.OfficeRelationshipsID, State: v2.Partial}}, Diagnostics: map[string]v2.Diagnostic{}}
			for _, d := range result.Diagnostics {
				graph.Diagnostics[d.Ref] = d
			}
			base.Evidence.Packages[0].Outcomes = append(base.Evidence.Packages[0].Outcomes, result.Outcomes...)
			if err := validatePartialOutcomes(index, base.Evidence, graph); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Structural, owner and truncation failures can coexist. None may overwrite
// another when projecting a partial native inventory into report coverage.
func TestRelationshipCoveragePreservesIndependentGapCodes(t *testing.T) {
	parts := budgetBase(t, "office-minimal.docx")
	var xml strings.Builder
	xml.WriteString(`<Relationships xmlns="` + opcrels.Namespace + `" unexpected="yes">`)
	for i := 0; i < 10001; i++ {
		fmt.Fprintf(&xml, `<Relationship Id="r%d" Type="example" Target="absent.xml"/>`, i)
	}
	xml.WriteString(`</Relationships>`)
	parts["orphan/_rels/missing.xml.rels"] = xml.String()
	raw := budgetArchive(t, parts, false)
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	base, err := BuildPackageRecords(context.Background(), raw, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CollectRelationshipCoverage(context.Background(), base, p.DOCX.OPC, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"opc.relationship_structure_unknown", "opc.source_missing", "opc.resource_limit"} {
		if !slices.ContainsFunc(result.Diagnostics, func(d v2.Diagnostic) bool { return string(d.Code) == code }) {
			t.Fatalf("lost independent gap %s", code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CollectRelationshipCoverage(ctx, base, p.DOCX.OPC, 1, 0); err != context.Canceled {
		t.Fatalf("cancellation = %v", err)
	}
}
