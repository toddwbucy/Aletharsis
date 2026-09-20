package v2_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func graphFixture(t *testing.T, name string) (v2.Graph, evidence.File, evidence.Document, v2.State) {
	t.Helper()
	raw := fixture(t, name)
	var file evidence.File
	var doc evidence.Document
	var state v2.State
	for key, target := range map[string]any{"file": &file, "evidence": &doc, "status": &state} {
		if err := json.Unmarshal(raw[key], target); err != nil {
			t.Fatal(err)
		}
	}
	g := v2.Graph{Capabilities: []v2.Capability{}, Executions: []v2.Execution{}, Artifacts: []v2.Artifact{}, Anchors: []v2.Anchor{}, Results: []v2.Result{}, Diagnostics: []v2.Diagnostic{}}
	for _, key := range []string{"capabilities", "executions", "artifacts", "anchors", "results", "diagnostics"} {
		for _, record := range records(t, name, key) {
			var err error
			switch key {
			case "capabilities":
				var v v2.Capability
				v, err = v2.DecodeCapability(record)
				g.Capabilities = append(g.Capabilities, v)
			case "executions":
				var v v2.Execution
				v, err = v2.DecodeExecution(record)
				g.Executions = append(g.Executions, v)
			case "artifacts":
				var v v2.Artifact
				v, err = v2.DecodeArtifact(record)
				g.Artifacts = append(g.Artifacts, v)
			case "anchors":
				var v v2.Anchor
				v, err = v2.DecodeAnchor(record)
				g.Anchors = append(g.Anchors, v)
			case "results":
				var v v2.Result
				v, err = v2.DecodeResult(record)
				g.Results = append(g.Results, v)
			case "diagnostics":
				var v v2.Diagnostic
				v, err = v2.DecodeDiagnostic(record)
				g.Diagnostics = append(g.Diagnostics, v)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return g, file, doc, state
}
func TestAcceptedGraphsAndAggregates(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts/fixtures/*.json")
	if err != nil || len(paths) != 8 {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			g, file, doc, want := graphFixture(t, strings.TrimSuffix(filepath.Base(path), ".json"))
			if err := g.Validate(file, doc); err != nil {
				t.Fatal(err)
			}
			got, err := g.AggregateStatus(file, doc)
			if err != nil || got != want {
				t.Fatalf("status %s want %s: %v", got, want, err)
			}
		})
	}
}
func TestGraphRejectsFalseEvidence(t *testing.T) {
	for name, change := range map[string]func(*v2.Graph, *evidence.File, *evidence.Document){
		"source mismatch": func(g *v2.Graph, f *evidence.File, d *evidence.Document) { h := strings.Repeat("0", 64); f.SHA256 = &h },
		"source absent":   func(g *v2.Graph, f *evidence.File, d *evidence.Document) { g.Artifacts = g.Artifacts[1:] },
		"parent order": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			g.Artifacts[0], g.Artifacts[1] = g.Artifacts[1], g.Artifacts[0]
		},
		"duplicate capability": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			g.Capabilities = append(g.Capabilities, g.Capabilities[0])
		},
		"unplanned capability": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			g.Executions = g.Executions[:len(g.Executions)-1]
		},
		"dangling execution": func(g *v2.Graph, f *evidence.File, d *evidence.Document) { g.Results[0].ExecutionRef = "exec/99" },
		"dangling mapping": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			g.Artifacts[1].Mapping.ToArtifactRef = "artifact/99"
		},
		"missing results":        func(g *v2.Graph, f *evidence.File, d *evidence.Document) { g.Results = []v2.Result{} },
		"modified retained text": func(g *v2.Graph, f *evidence.File, d *evidence.Document) { d.Texts[0].Text += "changed" },
		"modified boundary":      func(g *v2.Graph, f *evidence.File, d *evidence.Document) { d.Texts[0].ByteOffsets[1]++ },
		"fabricated selection": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			l := g.Anchors[0].Locator.(v2.TextLocator)
			l.SelectedTextSHA256 = strings.Repeat("0", 64)
			g.Anchors[0].Locator = l
		},
		"wrong anchor byte": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			l := g.Anchors[0].Locator.(v2.TextLocator)
			l.Spans[0].Byte.End++
			g.Anchors[0].Locator = l
		},
		"out of bounds scope": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			for i, e := range g.Executions {
				if e.CapabilityRef == "aletharsis.unicode.inventory" {
					s := v2.Scope{ArtifactRef: "artifact/1", Unit: "byte", Regions: []identity.Region{{Start: 0, End: 999999}}}
					g.Executions[i].RequestedScope = s
					g.Executions[i].AnalyzedScope = []v2.Scope{s}
				}
			}
		},
		"result operation mismatch": func(g *v2.Graph, f *evidence.File, d *evidence.Document) {
			g.Results[0].Payload.Operation = "another.detector"
		},
	} {
		t.Run(name, func(t *testing.T) {
			g, f, d, _ := graphFixture(t, "structural-observation")
			change(&g, &f, &d)
			if err := g.Validate(f, d); err == nil {
				t.Fatal("false evidence accepted")
			}
		})
	}
}
func TestPartialCoverageRequiresAccounting(t *testing.T) {
	g, f, d, _ := graphFixture(t, "partial-timeout")
	for i, e := range g.Executions {
		if e.State == v2.Partial {
			scope := *g.Executions[i].Exclusions[0].Scope
			scope.Regions = append([]identity.Region{}, scope.Regions...)
			scope.Regions[0].Start++
			g.Executions[i].Exclusions[0].Scope = &scope
		}
	}
	if err := g.Validate(f, d); err == nil {
		t.Fatal("unaccounted coverage accepted")
	}
}

func TestTextAnchorCannotClaimUnanalyzedRegion(t *testing.T) {
	g, file, doc, _ := graphFixture(t, "structural-observation")
	anchor := g.Anchors[0]
	locator := anchor.Locator.(v2.TextLocator)
	end := locator.Spans[0].Scalar.Start
	if end == 0 {
		t.Fatal("fixture needs prefix before selection")
	}
	scope := v2.Scope{ArtifactRef: *anchor.ArtifactRef, Unit: "scalar", Regions: []identity.Region{{Start: 0, End: end}}}
	for i, e := range g.Executions {
		if e.Ref == *anchor.ExecutionRef {
			g.Executions[i].RequestedScope = scope
			g.Executions[i].AnalyzedScope = []v2.Scope{scope}
		}
	}
	for i, r := range g.Results {
		if r.ExecutionRef == *anchor.ExecutionRef {
			g.Results[i].Payload.Scope = scope
		}
	}
	if err := g.Validate(file, doc); err == nil {
		t.Fatal("anchor outside analyzed region accepted")
	}
	if _, err := g.AggregateStatus(file, doc); err == nil {
		t.Fatal("invalid graph produced a status")
	}
}
func TestNoGraphCannotClaimSuccess(t *testing.T) {
	if _, err := (v2.Graph{}).AggregateStatus(evidence.File{}, evidence.EmptyDocument()); err == nil {
		t.Fatal("empty graph completed")
	}
}
