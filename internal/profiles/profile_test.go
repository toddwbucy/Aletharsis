package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

var budget = Limits{Observations: 1000, Signals: 100, Comparisons: 100000, Links: 10000}
var artifact = strings.Repeat("a", 64)
var contextConfig = "c06d8ade015f699ec652c31a3b70b7c2c7956030f4d6e1f2eeef2433edf80420"

func fixture(t *testing.T, name string) ([]byte, definition) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var d definition
	if err = json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return raw, d
}
func encode(t *testing.T, d definition) []byte {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func observation(d definition) Observation {
	facts := map[string]Fact{}
	for k, v := range d.Rules[0].Facts {
		facts[k] = v
	}
	return Observation{ID: "obs.a", ArtifactSHA256: artifact, Scope: d.Scope, Kind: d.Rules[0].Kind, Facts: facts, Coverage: "complete", Exact: true}
}
func registeredFixture(t *testing.T, name string) (*Registry, definition) {
	t.Helper()
	raw, d := fixture(t, name)
	r := &Registry{}
	if _, err := r.Register(raw); err != nil {
		t.Fatal(err)
	}
	return r, d
}
func assess(t *testing.T, r *Registry, d definition, obs []Observation, signals []Signal) []Assessment {
	t.Helper()
	a, err := r.Assess(d.ID, d.Version, obs, signals, budget)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestIndependentFormatProfiles(t *testing.T) {
	for _, name := range []string{"text", "docx", "odt", "pdf-born", "pdf-ocr"} {
		t.Run(name, func(t *testing.T) {
			r, d := registeredFixture(t, name)
			o := observation(d)
			got := assess(t, r, d, []Observation{o}, nil)[0]
			if got.Expectedness != "expected" || !got.OmitDefault || len(got.Matches) != 1 || got.Matches[0].Rationale != d.Rules[0].Rationale || len(got.Profile.ArtifactSHA256) != 64 {
				t.Fatalf("missing assessment: %+v", got)
			}
			o.Scope.Variant = "other"
			got = assess(t, r, d, []Observation{o}, nil)[0]
			if got.Expectedness != "unknown" || got.OmitDefault || got.Reason != "profile_scope_mismatch" {
				t.Fatal("cross-variant suppression")
			}
			o = observation(d)
			o.Facts = map[string]Fact{}
			if assess(t, r, d, []Observation{o}, nil)[0].OmitDefault {
				t.Fatal("missing context matched")
			}
		})
	}
}

func TestImmutableRevisionAndCopies(t *testing.T) {
	raw, d := fixture(t, "text")
	r := &Registry{}
	id, err := r.Register(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := r.Register(raw); err != nil || again != id {
		t.Fatal("identical registration failed")
	}
	reformatted := encode(t, d)
	if _, err = r.Register(reformatted); err == nil {
		t.Fatal("published bytes changed under same revision")
	}
	other := &Registry{}
	formatted, err := other.Register(reformatted)
	if err != nil || formatted.SemanticSHA256 != id.SemanticSHA256 || formatted.ArtifactSHA256 == id.ArtifactSHA256 {
		t.Fatal("raw versus semantic identity conflated")
	}
	old := assess(t, r, d, []Observation{observation(d)}, nil)
	old[0].Matches[0].Rationale = "mutated caller copy"
	retained, err := r.Artifact(d.ID, d.Version)
	if err != nil || string(retained) != string(raw) {
		t.Fatal("profile bytes not retained", err)
	}
	retained[0] = 'x'
	again, err := r.Artifact(d.ID, d.Version)
	if err != nil || string(again) != string(raw) {
		t.Fatal("artifact copy changed registry", err)
	}
	raw[0] = 'x'
	if got := assess(t, r, d, []Observation{observation(d)}, nil)[0]; got.Matches[0].Rationale == old[0].Matches[0].Rationale {
		t.Fatal("caller changed registry")
	}
	revised := d
	revised.Version = "1.1.0"
	revised.Rules = append([]Rule{}, d.Rules...)
	revised.Rules[0].Queue = "retain"
	if _, err := r.Register(encode(t, revised)); !errors.Is(err, ErrConflict) {
		t.Fatal("rule revision changed meaning", err)
	}
	revised.Rules[0].Revision = "1.1.0"
	if _, err = r.Register(encode(t, revised)); err != nil {
		t.Fatal(err)
	}
	if !assess(t, r, d, []Observation{observation(d)}, nil)[0].OmitDefault || assess(t, r, revised, []Observation{observation(d)}, nil)[0].OmitDefault {
		t.Fatal("historical revision changed")
	}
}

func TestContextIdentityCoverageAndUnknownStructure(t *testing.T) {
	for _, mutation := range []string{"partial", "failed", "unsupported", "unavailable", "unknown", "inexact", "producer", "version", "configuration", "unknown_structure"} {
		t.Run(mutation, func(t *testing.T) {
			r, d := registeredFixture(t, "text")
			o := observation(d)
			switch mutation {
			case "inexact":
				o.Exact = false
			case "producer":
				f := o.Facts["context"]
				f.Producer = "untrusted.document"
				o.Facts["context"] = f
			case "version":
				f := o.Facts["context"]
				f.Version = "2.0.0"
				o.Facts["context"] = f
			case "configuration":
				f := o.Facts["context"]
				f.ConfigurationSHA256 = strings.Repeat("b", 64)
				o.Facts["context"] = f
			case "unknown_structure":
				o.Kind = "unknown_structure"
			default:
				o.Coverage = mutation
			}
			a := assess(t, r, d, []Observation{o}, nil)[0]
			if a.Expectedness != "unknown" || a.OmitDefault {
				t.Fatalf("unsupported context hidden: %+v", a)
			}
		})
	}
}

func TestConflictsRetainAllExplanations(t *testing.T) {
	for _, state := range []string{"expected", "noteworthy", "unknown", "suspicious"} {
		t.Run(state, func(t *testing.T) {
			_, d := fixture(t, "text")
			second := d.Rules[0]
			second.ID = "fixture.retain"
			second.Assessment = state
			second.Queue = "retain"
			d.Rules = append(d.Rules, second)
			r := &Registry{}
			if _, err := r.Register(encode(t, d)); err != nil {
				t.Fatal(err)
			}
			a := assess(t, r, d, []Observation{observation(d)}, nil)[0]
			if a.Expectedness != state || a.OmitDefault || len(a.Matches) != 2 {
				t.Fatalf("conflict erased: %+v", a)
			}
		})
	}
}

func TestPatternOverrideAndIncompleteMapping(t *testing.T) {
	r, d := registeredFixture(t, "text")
	a := observation(d)
	b := observation(d)
	b.ID = "obs.b"
	c := observation(d)
	c.ID = "obs.c"
	c.ArtifactSHA256 = strings.Repeat("b", 64)
	observations := []Observation{c, b, a}
	signals := []Signal{{ID: "signal.z", ArtifactSHA256: artifact, Targets: []string{a.ID}, MappingComplete: true}}
	original, _ := json.Marshal(observations)
	got := assess(t, r, d, observations, signals)
	if got[0].ObservationID != a.ID || got[0].OmitDefault || got[0].Expectedness != "suspicious" || !got[1].OmitDefault {
		t.Fatalf("incorrect scoped override: %+v", got)
	}
	signals[0].MappingComplete = false
	signals[0].Targets = nil
	got = assess(t, r, d, observations, signals)
	if got[0].OmitDefault || got[1].OmitDefault || !got[2].OmitDefault {
		t.Fatal("incomplete map did not retain artifact scope")
	}
	if got[0].Expectedness != "unknown" || len(got[0].Overrides) != 0 || len(got[0].UnmappedSignals) != 1 {
		t.Fatal("uncertain membership presented as exact suspicious evidence")
	}
	after, _ := json.Marshal(observations)
	if string(original) != string(after) {
		t.Fatal("observations mutated")
	}
	if !reflect.DeepEqual(got, assess(t, r, d, []Observation{a, b, c}, signals)) {
		t.Fatal("input order changes assessment")
	}
	signals[0].Targets = []string{a.ID}
	got = assess(t, r, d, observations, signals)
	if got[0].Expectedness != "suspicious" || len(got[0].Overrides) != 1 || len(got[0].UnmappedSignals) != 0 || got[1].Expectedness != "unknown" {
		t.Fatal("partial mapping conflated known and unknown members")
	}
}

func TestNativePatternSurvivesPermissiveContext(t *testing.T) {
	// Adversarial normalized-context fixture: even an overly permissive expected
	// rule cannot hide an independently detected native binary sequence.
	raw := []byte(strings.Repeat("\u200b\u200c", 32))
	doc, err := (parsers.TextParser{}).Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	findings := (analyzers.Patterns{}).Analyze(&doc)
	original, _ := json.Marshal(findings)
	_, d := fixture(t, "text")
	d.Rules[0].Facts = map[string]Fact{
		"context":    {Value: "permissive_test", Producer: "fixture.context", Version: "0.0.0", ConfigurationSHA256: contextConfig},
		"code_point": {Value: "U+200B", Producer: "fixture.context", Version: "0.0.0", ConfigurationSHA256: contextConfig},
	}
	second := d.Rules[0]
	second.ID = "fixture.other_state"
	second.Facts = map[string]Fact{
		"context":    d.Rules[0].Facts["context"],
		"code_point": {Value: "U+200C", Producer: "fixture.context", Version: "0.0.0", ConfigurationSHA256: contextConfig},
	}
	d.Rules = append(d.Rules, second)
	r := &Registry{}
	if _, err = r.Register(encode(t, d)); err != nil {
		t.Fatal(err)
	}
	observations := []Observation{}
	targets := []string{}
	for i, cp := range []rune(doc.Texts[0].Text) {
		o := observation(d)
		o.ID = fmt.Sprintf("obs.%03d", i)
		o.Facts["code_point"] = Fact{Value: fmt.Sprintf("U+%04X", cp), Producer: "fixture.context", Version: "0.0.0", ConfigurationSHA256: contextConfig}
		o.ArtifactSHA256 = evidence.Hash(raw)
		observations = append(observations, o)
		targets = append(targets, o.ID)
	}
	if len(findings) != 1 || findings[0].ID != "pattern.zero_width_binary" {
		t.Fatal("native pattern absent")
	}
	got := assess(t, r, d, observations, []Signal{{ID: "finding.binary.1", ArtifactSHA256: evidence.Hash(raw), Targets: targets, MappingComplete: true}})
	for _, a := range got {
		if a.OmitDefault || a.Expectedness != "suspicious" || len(a.Matches) != 1 || len(a.Overrides) != 1 {
			t.Fatal("expected rule suppressed pattern")
		}
	}
	after, _ := json.Marshal(findings)
	if string(original) != string(after) {
		t.Fatal("native finding altered")
	}
}

func TestClosedInertDefinitions(t *testing.T) {
	raw, _ := fixture(t, "text")
	for _, bad := range [][]byte{
		[]byte(strings.Replace(string(raw), `"contract":`, `"ID":"unexpected", "contract":`, 1)),
		[]byte(strings.Replace(string(raw), `"contract":`, `"id":"duplicate", "contract":`, 1)),
		[]byte(strings.Replace(string(raw), `"facts":`, `"command":"touch source", "facts":`, 1)),
		[]byte(strings.Replace(string(raw), `"assessment": "expected"`, `"assessment": "suspicious"`, 1)),
		[]byte(strings.Replace(string(raw), `"unicode_character"`, `"credential_verification"`, 1)),
		[]byte(strings.Replace(string(raw), `"1.0.0"`, `"01.0.0"`, 1)),
		[]byte(strings.Repeat(" ", 256<<10) + string(raw)),
	} {
		r := &Registry{}
		if _, err := r.Register(bad); err == nil {
			t.Fatal("invalid/inert contract accepted")
		}
	}
	_, d := fixture(t, "text")
	d.Rules = append(d.Rules, d.Rules[0])
	if _, err := (&Registry{}).Register(encode(t, d)); err == nil {
		t.Fatal("duplicate rule ID accepted")
	}
}

func TestLimitsAndInvalidReferences(t *testing.T) {
	for _, mode := range []string{"zero", "observations", "signals", "comparisons", "links", "duplicate_obs", "unknown_target", "wrong_artifact", "duplicate_signal", "empty_signal", "unknown_artifact", "credential"} {
		t.Run(mode, func(t *testing.T) {
			r, d := registeredFixture(t, "text")
			a := observation(d)
			b := observation(d)
			b.ID = "obs.b"
			obs := []Observation{a, b}
			signals := []Signal{{ID: "signal.a", ArtifactSHA256: artifact, Targets: []string{a.ID}, MappingComplete: true}}
			limit := budget
			switch mode {
			case "zero":
				limit.Links = 0
			case "observations":
				limit.Observations = 1
			case "signals":
				limit.Signals = 1
				signals = append(signals, Signal{ID: "signal.b", ArtifactSHA256: artifact, Targets: []string{b.ID}, MappingComplete: true})
			case "comparisons":
				limit.Comparisons = 1
			case "links":
				limit.Links = 1
			case "duplicate_obs":
				obs[1].ID = a.ID
			case "unknown_target":
				signals[0].Targets = []string{"missing"}
			case "wrong_artifact":
				obs[0].ArtifactSHA256 = strings.Repeat("c", 64)
			case "duplicate_signal":
				signals = append(signals, signals[0])
			case "empty_signal":
				signals[0].Targets = nil
			case "unknown_artifact":
				signals[0].Targets = nil
				signals[0].MappingComplete = false
				signals[0].ArtifactSHA256 = strings.Repeat("c", 64)
			case "credential":
				obs[0].Kind = "credential_verification"
			}
			got, err := r.Assess(d.ID, d.Version, obs, signals, limit)
			if err == nil || got != nil {
				t.Fatal("invalid evaluation produced annotations")
			}
		})
	}
}

func TestUnicodeRulesRequireContext(t *testing.T) {
	_, d := fixture(t, "text")
	delete(d.Rules[0].Facts, "context")
	if _, err := (&Registry{}).Register(encode(t, d)); err == nil {
		t.Fatal("bare character whitelist accepted")
	}
}

func TestRegistryCapacity(t *testing.T) {
	_, d := fixture(t, "text")
	r := &Registry{}
	for i := 0; i < 128; i++ {
		d.ID = fmt.Sprintf("fixture.profile.%d", i)
		if _, err := r.Register(encode(t, d)); err != nil {
			t.Fatal(err)
		}
	}
	d.ID = "fixture.overflow"
	if _, err := r.Register(encode(t, d)); err == nil {
		t.Fatal("unbounded registry")
	}
}

func TestErrorSentinelsAndCanonicalCodePoints(t *testing.T) {
	_, d := fixture(t, "text")
	for _, cp := range []string{"U+D800", "U+110000", "U+000041"} {
		f := d.Rules[0].Facts["code_point"]
		f.Value = cp
		d.Rules[0].Facts["code_point"] = f
		if _, err := (&Registry{}).Register(encode(t, d)); !errors.Is(err, ErrDefinition) {
			t.Fatalf("noncanonical code point: %v", err)
		}
	}
	if _, err := (&Registry{}).Assess("missing", "1.0.0", nil, nil, budget); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	r, d := registeredFixture(t, "text")
	if _, err := r.Assess(d.ID, d.Version, nil, nil, Limits{}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestConcurrentRegistrationAndAssessment(t *testing.T) {
	raw, d := fixture(t, "text")
	r := &Registry{}
	if _, err := r.Register(raw); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.Register(raw); err != nil {
				t.Error(err)
				return
			}
			got, err := r.Assess(d.ID, d.Version, []Observation{observation(d)}, nil, budget)
			if err != nil || len(got) != 1 || !got[0].OmitDefault {
				t.Error("concurrent assessment failed", err)
			}
		}()
	}
	wg.Wait()
}
