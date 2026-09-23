package officeplan

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func scopeGraphFixture(t *testing.T, raw []byte, limits Limits) (*Assembly, []v2.Capability, map[string]v2.Config) {
	t.Helper()
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
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
	catalog, err := limits.Catalog("test")
	if err != nil {
		t.Fatal(err)
	}
	data, err := capability.NativeDataRevision()
	if err != nil {
		t.Fatal(err)
	}
	configs := map[string]v2.Config{}
	for _, c := range catalog {
		config, err := v2.NewNativeConfig(c.Revision, data, c.Limits)
		if err != nil {
			t.Fatal(err)
		}
		configs[c.ID] = config
	}
	return assembly, catalog, configs
}

func TestOfficeScopeGraphHasResultsForEveryRetainedScope(t *testing.T) {
	for _, file := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		for _, omitted := range []bool{false, true} {
			raw := coverageFixture(t, file, nil)
			limits := DefaultLimits()
			if omitted {
				limits.ScopeScalarOrigins = 1
			}
			assembly, catalog, configs := scopeGraphFixture(t, raw, limits)
			original, err := json.Marshal(assembly)
			if err != nil {
				t.Fatal(err)
			}
			offsets := TraceOffsets{Execution: 5, Result: 2, Anchor: 3, Finding: 7}
			graph, err := BuildScopeGraph(context.Background(), assembly, catalog, configs, offsets, limits.Report)
			if err != nil {
				t.Fatal(err)
			}
			again, err := BuildScopeGraph(context.Background(), assembly, catalog, configs, offsets, limits.Report)
			if err != nil || !reflect.DeepEqual(graph, again) {
				t.Fatal("nondeterministic graph", err)
			}
			if len(graph.Trace.Executions) != 3*len(assembly.Evidence.Scopes) || len(graph.Trace.Results) != len(graph.Trace.Executions) {
				t.Fatal("analyzer coverage missing")
			}
			if omitted {
				if len(graph.Findings) != 0 || len(graph.Trace.Anchors) != 0 || len(graph.Trace.Capabilities) != 0 {
					t.Fatal("omitted scope gained a result")
				}
				continue
			}
			positives, negatives := 0, 0
			for _, r := range graph.Trace.Results {
				switch r.Payload.Outcome {
				case "observations_present":
					positives++
				case "no_observations":
					negatives++
				}
			}
			if positives != 1 || negatives != 2 || len(graph.Findings) != 1 {
				t.Fatal("zero results confused with unrun analyzers")
			}
			finding := graph.Findings[0]
			if finding.Ref != "finding/7" || finding.ExecutionRef != "exec/5" || finding.Location["kind"] != "office_offsets" || finding.Location["scope_ref"] != "office-scope/0" || finding.Evidence["code_point"] != "U+200B" {
				t.Fatal("wrong finding/coordinate binding", finding)
			}
			if _, ok := finding.Location["byte_offsets"]; ok {
				t.Fatal("scope offsets masquerade as source bytes")
			}
			finding.Evidence["count"] = 999
			after, err := json.Marshal(assembly)
			if err != nil || string(original) != string(after) {
				t.Fatal("returned graph aliases producer evidence", err)
			}
		}
	}
}

func TestScopeGraphEncodesWithOfficeReportAndDistinctIdenticalScopes(t *testing.T) {
	text := "A😀e\u0301" + strings.Repeat("\u200b\u200c", 32)
	raw := analysisFixture(t, func(parts map[string]string) {
		paragraph := `<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>`
		parts["word/document.xml"] = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + paragraph + paragraph + `</w:body></w:document>`
		budgetEmbedding(parts, "object.bin", "%PDF-1.7 payload")
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Default Extension="bin" ContentType="application/octet-stream"/></Types>`, 1)
	})
	limits := DefaultLimits()
	assembly, catalog, configs := scopeGraphFixture(t, raw, limits)
	if len(assembly.Evidence.Scopes) != 2 || assembly.Evidence.Scopes[0].SHA256 != assembly.Evidence.Scopes[1].SHA256 || assembly.Evidence.Scopes[0].IdentitySHA256 == assembly.Evidence.Scopes[1].IdentitySHA256 {
		t.Fatal("test needs distinct equal-text scopes")
	}
	graph, err := BuildScopeGraph(context.Background(), assembly, catalog, configs, TraceOffsets{Execution: 5}, limits.Report)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Trace.Executions) != 6 || len(graph.Trace.Anchors) != 6 {
		t.Fatal("scope/operation identities conflated")
	}
	seen := map[string]map[string]bool{}
	for _, f := range graph.Findings {
		ref := f.Location["scope_ref"].(string)
		if seen[ref] == nil {
			seen[ref] = map[string]bool{}
		}
		seen[ref][f.ID] = true
	}
	for _, scope := range assembly.Evidence.Scopes {
		if !seen[scope.ScopeRef]["unicode.emoji"] || !seen[scope.ScopeRef]["unicode.normalization"] {
			t.Fatal("emoji/normalization evidence lost")
		}
	}
	// Compose this native fragment into an existing complete wire fixture. This
	// exercises report Encode/Decode, not just isolated record validators.
	fixture, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := v4.DecodeReport(fixture, limits.Report)
	if err != nil {
		t.Fatal(err)
	}
	report.File.SHA256 = new(string)
	*report.File.SHA256 = evidence.Hash(raw)
	report.File.Size = new(int)
	*report.File.Size = len(raw)
	report.Evidence.Document = evidence.EmptyDocument()
	report.Evidence.Office = assembly.Evidence
	report.Evidence.Office.Packages = append([]v4.Package{}, assembly.Evidence.Packages...)
	report.Artifacts = assembly.Artifacts
	caps := []v2.Capability{}
	for _, c := range report.Trace.Capabilities {
		if c.ID != capability.UnicodeInventoryID && c.ID != capability.OfficeTextID {
			caps = append(caps, c)
		}
	}
	report.Trace.Capabilities = append(caps, graph.Trace.Capabilities...)
	report.Trace.Executions = slices.DeleteFunc(report.Trace.Executions, func(e v2.Execution) bool { return e.CapabilityRef == capability.OfficeTextID })
	executions := []v2.Execution{}
	for _, e := range report.Trace.Executions {
		if e.CapabilityRef != capability.UnicodeInventoryID {
			executions = append(executions, e)
		}
	}
	report.Trace.Executions = append(executions, graph.Trace.Executions...)
	report.Trace.Results = graph.Trace.Results
	report.Trace.Anchors = graph.Trace.Anchors
	report.Findings = graph.Findings
	prepared, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := AnalyzePrepared(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := BuildPartGraph(context.Background(), prepared, analysis, assembly, catalog, configs, 20, len(report.Trace.Diagnostics))
	if err != nil {
		t.Fatal(err)
	}
	report.Trace.Capabilities = append(report.Trace.Capabilities, parts.Trace.Capabilities...)
	report.Trace.Executions = append(report.Trace.Executions, parts.Trace.Executions...)
	report.Trace.Diagnostics = append(report.Trace.Diagnostics, parts.Trace.Diagnostics...)
	report.Evidence.Office.Packages[0].Outcomes = append(report.Evidence.Office.Packages[0].Outcomes, parts.Outcomes...)
	// The fixture reserved exec/4 for the disabled credential declaration;
	// the native assembly uses that slot for object inspection instead.
	for i := range report.Trace.Executions {
		if report.Trace.Executions[i].Ref == "exec/4" {
			report.Trace.Executions[i].Ref = "exec/40"
		}
	}
	var objectDescriptor v2.Capability
	for _, c := range catalog {
		if c.ID == capability.OfficeObjectsID {
			objectDescriptor = c
		}
	}
	objects, err := BuildObjectGraph(context.Background(), prepared, analysis, assembly, objectDescriptor, configs[capability.OfficeObjectsID], 4, len(report.Trace.Diagnostics))
	if err != nil {
		t.Fatal(err)
	}
	report.Trace.Capabilities = append(report.Trace.Capabilities, objects.Trace.Capabilities...)
	report.Trace.Executions = append(report.Trace.Executions, objects.Trace.Executions...)
	report.Trace.Diagnostics = append(report.Trace.Diagnostics, objects.Trace.Diagnostics...)
	report.Evidence.Office.Objects = objects.Objects
	var identityDescriptor v2.Capability
	for _, c := range catalog {
		if c.ID == capability.OfficeIdentifyID {
			identityDescriptor = c
		}
	}
	identification, err := BuildIdentityGraph(context.Background(), prepared, assembly, identityDescriptor, configs[capability.OfficeIdentifyID], 2, len(report.Trace.Diagnostics))
	if err != nil {
		t.Fatal(err)
	}
	report.Trace.Capabilities = append(report.Trace.Capabilities, identification.Trace.Capabilities...)
	report.Trace.Executions = append(report.Trace.Executions, identification.Trace.Executions...)
	report.Trace.Diagnostics = append(report.Trace.Diagnostics, identification.Trace.Diagnostics...)
	report.Evidence.Office.Packages[0].Issues = identification.Issues
	report.Evidence.Office.Packages[0].Outcomes = append(report.Evidence.Office.Packages[0].Outcomes, identification.Outcomes...)
	report.Summary = map[string]int{"findings": len(report.Findings), "high": 0, "medium": 0, "low": 0, "info": 0, "exit_code": 0}
	for _, f := range report.Findings {
		report.Summary[strings.ToLower(f.Severity)]++
		report.Summary["exit_code"] = max(report.Summary["exit_code"], evidence.Rank(f.Severity))
	}
	encoded, err := report.Encode(limits.Report)
	if err != nil {
		t.Fatal("graph cannot enter closed Office report", err)
	}
	decoded, err := v4.DecodeReport(encoded, limits.Report)
	if err != nil || len(decoded.Findings) != len(graph.Findings) {
		t.Fatal("graph did not round trip", err)
	}
}

func TestScopeGraphRejectsMissingOrMisboundEvidence(t *testing.T) {
	raw := coverageFixture(t, "office-minimal.docx", nil)
	for _, change := range []func(*Assembly, []v2.Capability, map[string]v2.Config){
		func(a *Assembly, _ []v2.Capability, _ map[string]v2.Config) { a.Findings = nil },
		func(a *Assembly, _ []v2.Capability, _ map[string]v2.Config) { a.Findings[0].ArtifactRef = "artifact/0" },
		func(a *Assembly, _ []v2.Capability, _ map[string]v2.Config) {
			a.Findings[0].PartRef = "office-part/999"
		},
		func(a *Assembly, _ []v2.Capability, _ map[string]v2.Config) {
			a.Findings[0].Findings[0].Location["byte_offsets"] = []int{999}
		},
		func(_ *Assembly, c []v2.Capability, _ map[string]v2.Config) {
			for i := range c {
				if c[i].ID == capability.EmojiID {
					c[i].Participation = v2.Disabled
				}
			}
		},
		func(_ *Assembly, _ []v2.Capability, c map[string]v2.Config) { delete(c, capability.PatternsID) },
	} {
		a, c, configs := scopeGraphFixture(t, raw, DefaultLimits())
		change(a, c, configs)
		if _, err := BuildScopeGraph(context.Background(), a, c, configs, TraceOffsets{}, DefaultLimits().Report); err == nil {
			t.Fatal("invalid graph inputs accepted")
		}
	}
}
