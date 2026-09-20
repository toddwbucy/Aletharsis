package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

// Finding preserves the published native payload names and adds execution linkage.
// Wire validation uses the bundled detector-specific schema, never arbitrary maps
// as an extension point. Reviewer decisions do not live in this record.
type Finding struct {
	evidence.Finding
	Ref          string     `json:"finding_ref"`
	Mechanism    *Mechanism `json:"mechanism"`
	ExecutionRef string     `json:"execution_ref"`
	AnchorRefs   []string   `json:"anchor_refs"`
}
type View struct {
	Name              string   `json:"name"`
	FindingCategories []string `json:"finding_categories"`
}
type Report struct {
	Version     string            `json:"aletharsis_version"`
	Schema      string            `json:"schema_version"`
	File        evidence.File     `json:"file"`
	Status      State             `json:"status"`
	Summary     map[string]int    `json:"summary"`
	Evidence    evidence.Document `json:"evidence"`
	Findings    []Finding         `json:"findings"`
	Limitations []string          `json:"limitations"`
	Graph
	CatalogVersion     string     `json:"catalog_version"`
	ProfileAssessments []struct{} `json:"profile_assessments"`
	View               View       `json:"view"`
}

// Encode validates producer strings, budgets, wire shape and semantic linkage
// before returning canonical JSON. Callers must not mutate the report concurrently.
func (r Report) Encode(limits identity.Limits) ([]byte, error) {
	nodes, stringBytes := 0, 0
	if err := checkValue(reflect.ValueOf(r), 0, &nodes, &stringBytes, limits); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	canonical, err := wire.Validate("2.0", raw, limits)
	if err != nil {
		return nil, err
	}
	if err := r.ValidateSemantics(); err != nil {
		return nil, err
	}
	return canonical, nil
}

// DecodeReport checks the exact import bytes before allocating typed records.
// The caller retains those bytes and their ExactBytes digest separately. Record
// decoder limits still apply to each graph record inside the caller's total budget.
func DecodeReport(raw []byte, limits identity.Limits) (Report, error) {
	canonical, err := wire.Validate("2.0", raw, limits)
	if err != nil {
		return Report{}, err
	}
	var w struct {
		Version            string            `json:"aletharsis_version"`
		Schema             string            `json:"schema_version"`
		File               evidence.File     `json:"file"`
		Status             State             `json:"status"`
		Summary            map[string]int    `json:"summary"`
		Evidence           evidence.Document `json:"evidence"`
		Findings           []Finding         `json:"findings"`
		Limitations        []string          `json:"limitations"`
		Capabilities       []json.RawMessage `json:"capabilities"`
		Executions         []json.RawMessage `json:"executions"`
		Artifacts          []json.RawMessage `json:"artifacts"`
		Anchors            []json.RawMessage `json:"anchors"`
		Results            []json.RawMessage `json:"results"`
		Diagnostics        []json.RawMessage `json:"diagnostics"`
		CatalogVersion     string            `json:"catalog_version"`
		ProfileAssessments []struct{}        `json:"profile_assessments"`
		View               View              `json:"view"`
	}
	if err := json.Unmarshal(canonical, &w); err != nil {
		return Report{}, errors.New("invalid report representation")
	}
	r := Report{Version: w.Version, Schema: w.Schema, File: w.File, Status: w.Status, Summary: w.Summary, Evidence: w.Evidence, Findings: w.Findings, Limitations: w.Limitations, CatalogVersion: w.CatalogVersion, ProfileAssessments: w.ProfileAssessments, View: w.View}
	r.Capabilities, err = decodeRecords(w.Capabilities, DecodeCapability)
	if err != nil {
		return Report{}, err
	}
	r.Executions, err = decodeRecords(w.Executions, DecodeExecution)
	if err != nil {
		return Report{}, err
	}
	r.Artifacts, err = decodeRecords(w.Artifacts, DecodeArtifact)
	if err != nil {
		return Report{}, err
	}
	r.Anchors, err = decodeRecords(w.Anchors, DecodeAnchor)
	if err != nil {
		return Report{}, err
	}
	r.Results, err = decodeRecords(w.Results, DecodeResult)
	if err != nil {
		return Report{}, err
	}
	r.Diagnostics, err = decodeRecords(w.Diagnostics, DecodeDiagnostic)
	if err != nil {
		return Report{}, err
	}
	if err := r.ValidateSemantics(); err != nil {
		return Report{}, err
	}
	return r, nil
}
func decodeRecords[T any](raw []json.RawMessage, decode func([]byte) (T, error)) ([]T, error) {
	out := make([]T, 0, len(raw))
	for _, b := range raw {
		v, err := decode(b)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ValidateSemantics assumes wire validation. Use Encode or DecodeReport at a
// service/import boundary; this method alone does not check finding wire variants.
func (r Report) ValidateSemantics() error {
	status, err := r.Graph.AggregateStatus(r.File, r.Evidence)
	if err != nil {
		return err
	}
	if status != r.Status {
		return errors.New("aggregate status mismatch")
	}
	categories, ok := viewCategories(r.View.Name)
	if !ok || !reflect.DeepEqual(categories, r.View.FindingCategories) {
		return errors.New("view category mismatch")
	}
	x, err := r.Graph.index(r.Evidence)
	if err != nil {
		return err
	}
	for _, t := range r.Evidence.Texts {
		if hash, ok := t.Hashes["raw_text_sha256"]; ok && hash != identity.ExactBytes([]byte(t.Text)) {
			return errors.New("raw text hash mismatch")
		}
	}
	positiveExecutions := map[string]bool{}
	for _, result := range r.Results {
		if result.Payload.Outcome == "observations_present" {
			positiveExecutions[result.ExecutionRef] = true
		}
	}
	refs := map[string]bool{}
	counts := map[string]int{"findings": len(r.Findings), "high": 0, "medium": 0, "low": 0, "info": 0, "exit_code": 0}
	for _, f := range r.Findings {
		if !validRef(f.Ref, "finding") || refs[f.Ref] {
			return errors.New("duplicate or invalid finding reference")
		}
		refs[f.Ref] = true
		e, ok := x.executions[f.ExecutionRef]
		if !ok {
			return errors.New("dangling finding execution")
		}
		c := x.caps[e.CapabilityRef]
		if !reflect.DeepEqual(f.Mechanism, c.Mechanism) {
			return errors.New("finding mechanism mismatch")
		}
		if r.View.Name != "audit" && f.Category != "parser" && !slices.Contains(categories, f.Category) {
			return errors.New("finding outside view")
		}
		seen := map[string]bool{}
		for _, ref := range f.AnchorRefs {
			a, ok := x.anchors[ref]
			if !ok || a.ExecutionRef == nil || *a.ExecutionRef != e.Ref || seen[ref] {
				return errors.New("finding anchor mismatch")
			}
			seen[ref] = true
		}
		if f.ID == "parser.failure" {
			code, ok := f.Evidence["failure_code"].(string)
			if !ok || e.State != Failed || (c.Role != Acquisition && c.Role != Parser) {
				return errors.New("invalid parser failure execution")
			}
			matched := false
			for _, ref := range e.DiagnosticRefs {
				if string(x.diagnostics[ref].Code) == code {
					matched = true
				}
			}
			if !matched {
				return errors.New("failure finding/diagnostic code mismatch")
			}
		} else {
			if (e.State != Completed && e.State != Partial) || c.Role != Analyzer {
				return errors.New("finding from unusable operation")
			}

			if !positiveExecutions[e.Ref] {
				return errors.New("finding without positive structural result")
			}
		}
		if err := findingCoordinates(f, r.Evidence); err != nil {
			return err
		}
		if err := findingAnchors(f, x); err != nil {
			return err
		}
		key := strings.ToLower(f.Severity)
		if _, ok := counts[key]; !ok || key == "findings" || key == "exit_code" {
			return errors.New("invalid finding severity")
		}
		counts[key]++
		counts["exit_code"] = max(counts["exit_code"], evidence.Rank(f.Severity))
	}
	if status != Completed {
		counts["exit_code"] = 4
	}
	if !reflect.DeepEqual(counts, r.Summary) {
		return errors.New("finding summary mismatch")
	}
	return nil
}
func findingCoordinates(f Finding, document evidence.Document) error {
	raw, hasScalars := f.Location["character_offsets"]
	if !hasScalars {
		return nil
	}
	scalars, err := integers(raw)
	if err != nil {
		return err
	}
	if value, ok := f.Evidence["count"]; ok {
		count, err := integers([]any{value})
		if err != nil || len(count) != 1 || count[0] != uint64(len(scalars)) {
			return errors.New("finding occurrence count mismatch")
		}
	}
	offsets, err := integers(f.Location["byte_offsets"])
	if err != nil || len(scalars) != len(offsets) {
		return errors.New("finding coordinate count mismatch")
	}
	source, ok := f.Location["source"].(string)
	if !ok {
		return errors.New("finding source missing")
	}
	var segment *evidence.Text
	for _, t := range document.Texts {
		if t.Source == source {
			if segment != nil {
				return errors.New("ambiguous finding source")
			}
			segment = t
		}
	}
	if segment == nil {
		return errors.New("finding source missing")
	}
	for i, p := range scalars {
		if p >= uint64(len(segment.ByteOffsets)-1) || uint64(segment.ByteOffsets[p]) != offsets[i] {
			return errors.New("finding coordinate mismatch")
		}
	}
	return nil
}
func integers(value any) ([]uint64, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var values []uint64
	if string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
		return nil, errors.New("invalid coordinate list")
	}
	return values, nil
}
func viewCategories(name string) ([]string, bool) {
	switch name {
	case "audit":
		return []string{}, true
	case "unicode":
		return []string{"possible_steganography", "unicode"}, true
	case "metadata":
		return []string{"identifier", "metadata", "provenance"}, true
	case "structure":
		return []string{"document_structure", "embedded_content", "hidden_content", "visual_watermark"}, true
	default:
		return nil, false
	}
}

// SelectView copies the finding slice and summary; it does not alter detector
// results, evidence, execution coverage or the original report's backing arrays.
func (r Report) SelectView(name string) (Report, error) {
	categories, ok := viewCategories(name)
	if !ok {
		return Report{}, errors.New("unknown finding view")
	}
	if r.View.Name != "audit" && name != r.View.Name {
		return Report{}, errors.New("cannot reconstruct findings from a filtered report")
	}
	if err := r.ValidateSemantics(); err != nil {
		return Report{}, err
	}
	r.View = View{name, categories}
	selected := make([]Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if name == "audit" || f.Category == "parser" || slices.Contains(categories, f.Category) {
			selected = append(selected, f)
		}
	}
	r.Findings = selected
	r.Recount()
	return r, nil
}

// Recount sets counts from the current view. The operational status must already
// have been obtained from the validated graph; Encode verifies both again.
func (r *Report) Recount() {
	s := map[string]int{"findings": len(r.Findings), "high": 0, "medium": 0, "low": 0, "info": 0, "exit_code": 0}
	for _, f := range r.Findings {
		s[strings.ToLower(f.Severity)]++
		s["exit_code"] = max(s["exit_code"], evidence.Rank(f.Severity))
	}
	if r.Status != Completed {
		s["exit_code"] = 4
	}
	r.Summary = s
}
func checkValue(v reflect.Value, depth int, nodes *int, stringBytes *int, limits identity.Limits) error {
	*nodes = *nodes + 1
	if depth > limits.Depth || *nodes > limits.Nodes {
		return fmt.Errorf("%w: producer report structure", identity.ErrLimit)
	}
	switch v.Kind() {
	case reflect.String:
		if v.Len() > min(limits.InputBytes, limits.OutputBytes)-*stringBytes {
			return identity.ErrLimit
		}
		*stringBytes += v.Len()
		if !utf8.ValidString(v.String()) {
			return errors.New("invalid UTF-8 in report")
		}
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return checkValue(v.Elem(), depth+1, nodes, stringBytes, limits)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := checkValue(v.Field(i), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := checkValue(v.Index(i), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if err := checkValue(iter.Key(), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
			if err := checkValue(iter.Value(), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return errors.New("non-finite report number")
		}
	}
	return nil
}

func findingAnchors(f Finding, x *graphIndex) error {
	source, hasSource := f.Location["source"].(string)
	if !hasSource {
		return nil
	}
	segment := ""
	for i, t := range x.document.Texts {
		if t.Source == source {
			if segment != "" {
				return errors.New("ambiguous finding source")
			}
			segment = fmt.Sprintf("/evidence/texts/%d", i)
		}
	}
	if segment == "" {
		return errors.New("finding source missing")
	}
	if f.ID == "text.bom" {
		t, err := x.text(segment, "")
		if err != nil {
			return err
		}
		offsets, err := integers([]any{f.Location["byte_offset"]})
		if err != nil || len(offsets) != 1 || t.BOM == nil || offsets[0] != uint64(t.ByteOffsets[0]) || f.Evidence["hex"] != *t.BOM || f.Evidence["encoding"] != t.Encoding {
			return errors.New("BOM finding identity mismatch")
		}
	}
	offsets, hasOffsets := f.Location["character_offsets"]
	if !hasOffsets {
		return nil
	}
	positions, err := integers(offsets)
	if err != nil {
		return err
	}
	slices.Sort(positions)
	spans := []identity.Region{}
	for _, ref := range f.AnchorRefs {
		a := x.anchors[ref]
		l, ok := a.Locator.(TextLocator)
		if !ok || l.SegmentPointer != segment {
			return errors.New("finding/text anchor source mismatch")
		}
		for _, span := range l.Spans {

			i := sort.Search(len(positions), func(i int) bool { return positions[i] >= span.Scalar.Start })
			if i == len(positions) || positions[i] >= span.Scalar.End {
				return errors.New("finding anchor has unrelated span")
			}
			spans = append(spans, span.Scalar)
		}
	}

	slices.SortFunc(spans, func(a, b identity.Region) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	cursor := 0
	var coveredEnd uint64
	for _, p := range positions {
		for cursor < len(spans) && spans[cursor].Start <= p {
			coveredEnd = max(coveredEnd, spans[cursor].End)
			cursor++
		}
		if p >= coveredEnd {
			return errors.New("finding occurrence lacks an anchor")
		}
	}
	return nil
}
