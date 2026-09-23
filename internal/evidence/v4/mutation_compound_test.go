package v4

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

func officeMutationValue(v any) (Evidence, bool) {
	root, ok := v.(map[string]any)
	if !ok {
		return Evidence{}, false
	}
	evidence, ok := root["evidence"].(map[string]any)
	if !ok {
		return Evidence{}, false
	}
	var e Evidence
	if err := json.Unmarshal(mutationJSON(evidence["office"]), &e); err != nil {
		return e, false
	}
	return e, len(e.Packages) > 0
}

// Derive uncovered byte probes from retained metadata elements, relationship
// locations, and text origins, not hard-coded part offsets. XML maps themselves
// are lexical context and do not attest analytical coverage.
func generateOfficeMutations(value any) []mutation {
	e, ok := officeMutationValue(value)
	if !ok {
		return nil
	}
	occupied := map[string][]Span{}
	docs := map[string]XML{}
	for _, doc := range e.XML {
		docs[doc.XMLRef] = doc
	}
	for _, m := range e.Metadata {
		occupied[m.PartRef] = append(occupied[m.PartRef], docs[m.XMLRef].Elements[m.Element].Span)
	}
	for _, r := range e.Relationships {
		if r.Location.Span != nil {
			occupied[r.Location.PartRef] = append(occupied[r.Location.PartRef], *r.Location.Span)
		}
	}
	for _, s := range e.Scopes {
		for _, o := range s.Origins {
			occupied[s.PartRef] = append(occupied[s.PartRef], o.Source)
		}
	}
	for _, o := range e.Objects {
		occupied[o.PartRef] = append(occupied[o.PartRef], o.Inspection.Assessed...)
	}
	sizes := map[string]int64{}
	for _, pkg := range e.Packages {
		for _, part := range pkg.Parts {
			if part.ByteLength != nil {
				sizes[part.PartRef] = *part.ByteLength
			}
		}
	}
	var result []mutation
	for pi, pkg := range e.Packages {
		for oi, o := range pkg.Outcomes {
			if o.PartRef == nil {
				continue
			}
			ref := *o.PartRef
			spans := append([]Span{}, occupied[ref]...)
			sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
			pos := int64(0)
			for _, span := range spans {
				if span.Start > pos {
					break
				}
				if span.End > pos {
					pos = span.End
				}
			}
			if pos >= sizes[ref] {
				continue
			}
			for si := range o.Assessed {
				p := fmt.Sprintf("/evidence/office/packages/%d/outcomes/%d/assessed/%d", pi, oi, si)
				before := mutationGet(value, p)
				after := map[string]any{"start": float64(pos), "end": float64(pos + 1)}
				if !reflect.DeepEqual(before, after) {
					result = append(result, mutation{"assess-outside-retained", p, before, after})
				}
			}
		}
	}
	// Coordinated empty values plus restricted authority would otherwise be
	// masked by origin/value mismatches from individual field mutations.
	if len(e.Metadata) > 0 {
		var changed any
		_ = json.Unmarshal(mutationJSON(value), &changed)
		for i := range e.Metadata {
			prefix := fmt.Sprintf("/evidence/office/metadata/%d", i)
			mutationSet(changed, prefix+"/lexical_value", "")
			mutationSet(changed, prefix+"/value_origins", []any{})
		}
		for pi, pkg := range e.Packages {
			for oi, o := range pkg.Outcomes {
				if o.Operation == "aletharsis.office.metadata" {
					mutationSet(changed, fmt.Sprintf("/evidence/office/packages/%d/outcomes/%d/assessed", pi, oi), []any{map[string]any{"start": float64(0), "end": float64(1)}})
				}
			}
		}
		result = append(result, mutation{"compound-empty-metadata", "", value, changed})
	}
	return result
}

// Generate all four historical classes for every direct-property pair in the
// shipped inventory. This isolates the authority invariant from independent
// full-import guards, so fault injection cannot pass through incidental rejection.
func TestGeneratedProjectionAuthority(t *testing.T) {
	raw := readMutationFixture(t, "metadata-inventory.json")
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	e, ok := officeMutationValue(value)
	if !ok {
		t.Fatal("Office inventory required")
	}
	counts := map[string]int{}
	for _, doc := range e.XML {
		if len(doc.Elements) == 0 || officemetadata.ProjectionKind(doc.Elements[0].Namespace, doc.Elements[0].LocalName) == "" {
			continue
		}
		for _, a := range doc.Elements {
			for _, b := range doc.Elements {
				if a.Parent == nil || b.Parent == nil || *a.Parent != 0 || *b.Parent != 0 || a.Index == b.Index {
					continue
				}
				for _, mode := range []string{"baseline", "pooled-exclusion", "coincident-sibling", "zero-property", "detached-property"} {
					counts[mode]++
					t.Run(fmt.Sprintf("%s/%d/%d/%s", doc.XMLRef, a.Index, b.Index, mode), func(t *testing.T) {
						xml := doc
						xml.Elements = append([]Element{}, doc.Elements...)
						part := Part{PartRef: doc.PartRef, ArtifactRef: "artifact/1"}
						operation := "aletharsis.office.metadata"
						omitted := a.Span
						first := Outcome{Operation: operation, ExecutionRef: "exec/0", PartRef: &part.PartRef, State: "partial", Excluded: []Span{omitted}, DiagnosticRefs: []string{"diagnostic/0"}}
						second := Outcome{Operation: operation, ExecutionRef: "exec/1", PartRef: &part.PartRef, State: "partial", Assessed: []Span{}}
						retained := []Metadata{}
						for _, el := range doc.Elements {
							if el.Parent != nil && *el.Parent == 0 && el.Index != a.Index {
								retained = append(retained, Metadata{PartRef: part.PartRef, Element: el.Index})
								second.Assessed = append(second.Assessed, el.Span)
							}
						}
						d := v2.Diagnostic{Ref: "diagnostic/0", ExecutionRef: &first.ExecutionRef, Code: "metadata.property_unassessed", Scope: &v2.Scope{ArtifactRef: part.ArtifactRef, Unit: "byte", Regions: []identity.Region{{Start: uint64(omitted.Start), End: uint64(omitted.End)}}}}
						switch mode {
						case "pooled-exclusion":
							first.Excluded = nil
							second.Excluded = []Span{omitted}
						case "coincident-sibling":
							xml.Elements[a.Index].Span = b.Span
							first.Excluded = nil
							first.DiagnosticRefs = nil
						case "zero-property":
							xml.Elements[a.Index].Span = Span{a.Span.Start, a.Span.Start}
							first.Excluded = nil
							d.Scope.Regions = []identity.Region{{Start: uint64(a.Span.Start), End: uint64(a.Span.Start)}}
						case "detached-property":
							xml.Elements[a.Index].Parent = nil
							first.Excluded = nil
							first.DiagnosticRefs = nil
						}
						trace := TraceIndex{Executions: map[string]v2.Execution{"exec/0": {CapabilityRef: operation, State: v2.Partial}, "exec/1": {CapabilityRef: operation, State: v2.Partial}}, Diagnostics: map[string]v2.Diagnostic{d.Ref: d}}
						inventory := Evidence{XML: []XML{xml}, Metadata: retained, Packages: []Package{{Outcomes: []Outcome{first, second}}}}
						index := Index{Parts: map[string]Part{part.PartRef: part}}
						err := index.validateMetadataProjection(inventory, &trace)
						if (err == nil) != (mode == "baseline") {
							t.Fatalf("%s: %v", mode, err)
						}
					})
				}
			}
		}
	}
	for _, mode := range []string{"baseline", "pooled-exclusion", "coincident-sibling", "zero-property", "detached-property"} {
		if counts[mode] == 0 {
			t.Fatal("missing generated authority class", mode)
		}
	}
	t.Logf("generated authority coverage: %s", mutationJSON(counts))
}
