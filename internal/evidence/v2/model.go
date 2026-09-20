// Package v2 defines validated producer records for the report-2.0 execution
// model and evidence graph. It does not yet assemble full reports or run
// detectors. Explicit Decode functions are the bounded record-import boundary.
package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Mechanism is separate from severity, classification and reviewer judgment.
type Mechanism string

const (
	Structural    Mechanism = "structural"
	Cryptographic Mechanism = "cryptographic"
	Statistical   Mechanism = "statistical"
)

type Role string

const (
	Acquisition Role = "acquisition"
	Parser      Role = "parser"
	Analyzer    Role = "analyzer"
	Verifier    Role = "verifier"
)

type Participation string

const (
	Required Participation = "required"
	Optional Participation = "optional"
	Disabled Participation = "disabled"
)

type State string

const (
	NotRun    State = "not_run"
	Completed State = "completed"
	Partial   State = "partial"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var localRefPattern = regexp.MustCompile(`^(exec|artifact|diagnostic|anchor|result|finding)/(0|[1-9][0-9]*)$`)

// RecordLimits bound small native planning records, not whole report imports.
// Whole-report budgets remain an independent integration gate.
func RecordLimits() identity.Limits {
	return identity.Limits{InputBytes: 1 << 20, OutputBytes: 1 << 20, Nodes: 32768, Depth: 32}
}

// Limits uses null to disclose dimensions not enforced by the operation.
type Limits struct {
	InputBytes    *uint64 `json:"input_bytes"`
	ExpandedBytes *uint64 `json:"expanded_bytes"`
	Objects       *uint64 `json:"objects"`
	Depth         *uint64 `json:"depth"`
	OutputBytes   *uint64 `json:"output_bytes"`
	ExecutionMS   *uint64 `json:"execution_ms"`
}

func (l Limits) Validate() error {
	for _, n := range []*uint64{l.InputBytes, l.ExpandedBytes, l.Objects, l.Depth, l.OutputBytes, l.ExecutionMS} {
		if n != nil && *n > identity.MaxSafeInteger {
			return errors.New("limit exceeds safe integer range")
		}
	}
	return nil
}

type Availability struct {
	State      string        `json:"state"`
	ReasonCode *failure.Code `json:"reason_code"`
}
type SupportedScope struct {
	Kinds   []string `json:"kinds"`
	Formats []string `json:"formats"`
}
type Upstream struct {
	Repository string  `json:"repository"`
	Revision   string  `json:"revision"`
	SHA256     *string `json:"sha256"`
}
type DataIdentity struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type Implementation struct {
	ID       string         `json:"id"`
	Version  string         `json:"version"`
	Upstream *Upstream      `json:"upstream"`
	Data     []DataIdentity `json:"data"`
}
type Capability struct {
	ID             string          `json:"id"`
	Revision       string          `json:"revision"`
	Role           Role            `json:"role"`
	Mechanism      *Mechanism      `json:"mechanism"`
	Purpose        *string         `json:"purpose"`
	Implementation *Implementation `json:"implementation"`
	Availability   Availability    `json:"availability"`
	Participation  Participation   `json:"participation"`
	SupportedScope SupportedScope  `json:"supported_scope"`
	Limits         Limits          `json:"limits"`
}

func (c Capability) Validate() error {
	if err := validStrings(reflect.ValueOf(c)); err != nil {
		return err
	}
	if c.ID == "" || c.Revision == "" {
		return errors.New("capability identity required")
	}
	switch c.Role {
	case Acquisition, Parser:
		if c.Mechanism != nil {
			return errors.New("operational capability cannot claim a mechanism")
		}
	case Analyzer, Verifier:
		if c.Mechanism == nil || !slices.Contains([]Mechanism{Structural, Cryptographic, Statistical}, *c.Mechanism) {
			return errors.New("invalid capability mechanism")
		}
	default:
		return errors.New("invalid capability role")
	}
	if c.Mechanism != nil && *c.Mechanism == Statistical {
		if c.Purpose == nil || !slices.Contains([]string{"watermark_detection", "channel_analysis", "intrinsic_fingerprint_research"}, *c.Purpose) {
			return errors.New("statistical purpose required")
		}
	} else if c.Purpose != nil {
		return errors.New("nonstatistical purpose must be null")
	}
	if !slices.Contains([]Participation{Required, Optional, Disabled}, c.Participation) {
		return errors.New("invalid participation")
	}
	switch c.Availability.State {
	case "available":
		if c.Implementation == nil || c.Availability.ReasonCode != nil {
			return errors.New("available capability requires implementation and null reason")
		}
	case "unavailable", "unknown":
		if !validCode(c.Availability.ReasonCode) {
			return errors.New("unavailable capability requires reason")
		}
	default:
		return errors.New("invalid availability")
	}
	if i := c.Implementation; i != nil {
		if i.ID == "" || i.Version == "" || i.Data == nil {
			return errors.New("incomplete implementation identity")
		}
		if u := i.Upstream; u != nil && (u.Repository == "" || u.Revision == "" || u.SHA256 != nil && !digestPattern.MatchString(*u.SHA256)) {
			return errors.New("invalid upstream identity")
		}
		for _, d := range i.Data {
			if d.ID == "" || d.Version == "" || !digestPattern.MatchString(d.SHA256) {
				return errors.New("invalid data identity")
			}
		}
	}
	if c.SupportedScope.Kinds == nil || c.SupportedScope.Formats == nil {
		return errors.New("supported scope arrays required")
	}
	for _, list := range [][]string{c.SupportedScope.Kinds, c.SupportedScope.Formats} {
		for _, s := range list {
			if s == "" {
				return errors.New("empty supported scope name")
			}
		}
	}
	return c.Limits.Validate()
}
func (c Capability) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	type wire Capability
	return json.Marshal(wire(c))
}

// NativeSettings is deliberately closed; remote keys/tokenizers/trust policies
// require a separately reviewed configuration variant.
type NativeSettings struct {
	Limits        Limits `json:"limits"`
	Preprocessing string `json:"preprocessing"`
	DataRevision  string `json:"data_revision"`
}
type Config struct {
	Schema            *string         `json:"schema"`
	Revision          *string         `json:"revision"`
	Settings          *NativeSettings `json:"settings"`
	SHA256            *string         `json:"sha256"`
	KeyRef            *struct{}       `json:"key_ref"`
	UnavailableReason *string         `json:"unavailable_reason,omitempty"`
}

func UnknownConfig() Config { return Config{UnavailableReason: ptr("configuration.unknown")} }
func NewNativeConfig(revision, dataRevision string, limits Limits) (Config, error) {
	c := Config{Schema: ptr("aletharsis.native-text-config/1"), Revision: ptr(revision), Settings: &NativeSettings{Limits: limits, Preprocessing: "none", DataRevision: dataRevision}}
	hash, err := c.digest()
	if err != nil {
		return Config{}, err
	}
	c.SHA256 = &hash
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
func (c Config) digest() (string, error) {
	value := struct {
		Schema   *string         `json:"schema"`
		Revision *string         `json:"revision"`
		Settings *NativeSettings `json:"settings"`
		KeyRef   *struct{}       `json:"key_ref"`
	}{c.Schema, c.Revision, c.Settings, c.KeyRef}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return identity.Digest(identity.ConfigDomain, raw, RecordLimits())
}
func (c Config) Validate() error {
	if err := validStrings(reflect.ValueOf(c)); err != nil {
		return err
	}
	if c.KeyRef != nil {
		return errors.New("native configuration cannot contain key material")
	}
	if c.Schema == nil {
		if c.Revision != nil || c.Settings != nil || c.SHA256 != nil || c.UnavailableReason == nil || *c.UnavailableReason != "configuration.unknown" {
			return errors.New("invalid unknown configuration")
		}
		return nil
	}
	if *c.Schema != "aletharsis.native-text-config/1" || c.Revision == nil || *c.Revision == "" || c.Settings == nil || c.SHA256 == nil || c.UnavailableReason != nil {
		return errors.New("invalid native configuration")
	}
	if c.Settings.Preprocessing != "none" || c.Settings.DataRevision == "" {
		return errors.New("invalid native settings")
	}
	if err := c.Settings.Limits.Validate(); err != nil {
		return err
	}
	hash, err := c.digest()
	if err != nil {
		return err
	}
	if *c.SHA256 != hash {
		return errors.New("configuration digest mismatch")
	}
	return nil
}
func (c Config) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	type wire Config
	return json.Marshal(wire(c))
}

// Scope regions use their declared artifact representation; bounds against actual
// artifacts are checked by the later coordinator, not guessed by this record.
type Scope struct {
	ArtifactRef string            `json:"artifact_ref"`
	Unit        string            `json:"unit"`
	Regions     []identity.Region `json:"regions,omitempty"`
}

func (s Scope) Validate() error {
	if err := validStrings(reflect.ValueOf(s)); err != nil {
		return err
	}
	if !validRef(s.ArtifactRef, "artifact") {
		return errors.New("invalid scope artifact reference")
	}
	if s.Unit == "whole_artifact" {
		if s.Regions != nil {
			return errors.New("whole artifact scope cannot contain regions")
		}
		return nil
	}
	if !slices.Contains([]string{"byte", "scalar"}, s.Unit) || len(s.Regions) == 0 {
		return errors.New("scope regions required")
	}
	var previous uint64
	for _, r := range s.Regions {
		if r.Start < previous || r.Start >= r.End || r.End > identity.MaxSafeInteger {
			return errors.New("invalid scope region")
		}
		previous = r.End
	}
	return nil
}
func (s Scope) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	type wire Scope
	return json.Marshal(wire(s))
}
func (s Scope) within(outer Scope) bool {
	if s.ArtifactRef != outer.ArtifactRef {
		return false
	}
	if outer.Unit == "whole_artifact" {
		return s.Unit == "whole_artifact" || s.Unit == "byte"
	}
	if s.Unit != outer.Unit {
		return false
	}
	for _, r := range s.Regions {
		covered := false
		for _, o := range outer.Regions {
			if o.Start <= r.Start && r.End <= o.End {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

type Exclusion struct {
	Scope            *Scope       `json:"scope"`
	UnknownRemainder bool         `json:"unknown_remainder"`
	ReasonCode       failure.Code `json:"reason_code"`
}
type Execution struct {
	Ref            string        `json:"execution_ref"`
	CapabilityRef  string        `json:"capability_ref"`
	Config         Config        `json:"config"`
	RequestedScope Scope         `json:"requested_scope"`
	AnalyzedScope  []Scope       `json:"analyzed_scope"`
	Exclusions     []Exclusion   `json:"exclusions"`
	State          State         `json:"state"`
	ReasonCode     *failure.Code `json:"reason_code"`
	DiagnosticRefs []string      `json:"diagnostic_refs"`
}

func (e Execution) Validate() error {
	if err := validStrings(reflect.ValueOf(e)); err != nil {
		return err
	}
	if !validRef(e.Ref, "exec") || e.CapabilityRef == "" {
		return errors.New("invalid execution identity")
	}
	if err := e.Config.Validate(); err != nil {
		return err
	}
	if err := e.RequestedScope.Validate(); err != nil {
		return err
	}
	if e.AnalyzedScope == nil || e.Exclusions == nil || e.DiagnosticRefs == nil {
		return errors.New("execution arrays required")
	}
	for _, s := range e.AnalyzedScope {
		if err := s.Validate(); err != nil {
			return err
		}
		if !s.within(e.RequestedScope) {
			return errors.New("analyzed scope exceeds request")
		}
	}
	for _, x := range e.Exclusions {
		if !validCode(&x.ReasonCode) || (x.Scope == nil) != x.UnknownRemainder {
			return errors.New("invalid exclusion")
		}
		if x.Scope != nil {
			if err := x.Scope.Validate(); err != nil {
				return err
			}
			if !x.Scope.within(e.RequestedScope) {
				return errors.New("excluded scope exceeds request")
			}
		}
	}
	for i, s := range e.AnalyzedScope {
		for _, other := range e.AnalyzedScope[i+1:] {
			if scopesOverlap(s, other) {
				return errors.New("overlapping analyzed scope")
			}
		}
		for _, x := range e.Exclusions {
			if x.Scope != nil && scopesOverlap(s, *x.Scope) {
				return errors.New("analyzed/excluded overlap")
			}
		}
	}
	for i, x := range e.Exclusions {
		if x.Scope != nil {
			for _, other := range e.Exclusions[i+1:] {
				if other.Scope != nil && scopesOverlap(*x.Scope, *other.Scope) {
					return errors.New("overlapping exclusions")
				}
			}
		}
	}
	for _, ref := range e.DiagnosticRefs {
		if !validRef(ref, "diagnostic") {
			return errors.New("invalid diagnostic reference")
		}
	}
	switch e.State {
	case NotRun, Failed, Canceled:
		if len(e.AnalyzedScope) != 0 || !validCode(e.ReasonCode) {
			return errors.New("unusable execution cannot claim analyzed scope")
		}
		if e.State == Canceled && *e.ReasonCode != failure.Canceled {
			return errors.New("canceled execution requires cancellation reason")
		}
		if e.State == Failed && len(e.DiagnosticRefs) == 0 {
			return errors.New("failed execution requires diagnostic")
		}
	case Completed:
		if e.ReasonCode != nil || len(e.Exclusions) != 0 || len(e.AnalyzedScope) != 1 || !reflect.DeepEqual(e.AnalyzedScope[0], e.RequestedScope) {
			return errors.New("completed execution must cover request")
		}
	case Partial:
		if len(e.AnalyzedScope) == 0 || len(e.Exclusions) == 0 || len(e.DiagnosticRefs) == 0 || !validCode(e.ReasonCode) {
			return errors.New("partial execution requires coverage, exclusions and diagnostic")
		}
	default:
		return errors.New("invalid execution state")
	}
	return nil
}
func (e Execution) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	type wire Execution
	return json.Marshal(wire(e))
}

func ptr[T any](v T) *T              { return &v }
func validCode(c *failure.Code) bool { return c != nil && codePattern.MatchString(string(*c)) }
func validRef(r, prefix string) bool {
	return strings.HasPrefix(r, prefix+"/") && localRefPattern.MatchString(r)
}

// decode rejects duplicate keys, invalid Unicode, unknown properties, missing
// required properties, and explicit null for nonnullable Go fields before use.
func decode(raw []byte, out any) error {
	canonical, err := identity.Canonicalize(raw, RecordLimits())
	if err != nil {
		return err
	}
	if err := requiredFields(raw, reflect.TypeOf(out).Elem()); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid v2 record shape")
	}
	return nil
}
func requiredFields(raw json.RawMessage, t reflect.Type) error {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	if t.Kind() == reflect.Pointer {
		if bytes.Equal(raw, []byte("null")) {
			return nil
		}
		return requiredFields(raw, t.Elem())
	}
	if bytes.Equal(raw, []byte("null")) {
		return errors.New("null for nonnullable v2 field")
	}
	switch t.Kind() {
	case reflect.Uint64:
		// Validate coordinate/count range on the original numeric token before
		// consuming the canonical representation. Integral spellings such as 1.0
		// remain valid, but booleans and unsafe binary64 integers do not.
		n, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > float64(identity.MaxSafeInteger) || math.Trunc(n) != n {
			return errors.New("invalid safe integer in v2 record")
		}
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return errors.New("invalid v2 object")
		}
		allowed := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, options, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			allowed[name] = true
			v, present := fields[name]
			if !present {
				if options == "omitempty" {
					continue
				}
				return fmt.Errorf("missing v2 field %s", name)
			}
			if options == "omitempty" && bytes.Equal(v, []byte("null")) {
				return errors.New("null optional variant field")
			}
			if err := requiredFields(v, f.Type); err != nil {
				return err
			}
		}
		for name := range fields {
			if !allowed[name] {
				return errors.New("unknown v2 field")
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return errors.New("invalid v2 array")
		}
		for _, v := range values {
			if err := requiredFields(v, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
func DecodeCapability(raw []byte) (Capability, error) {
	var v Capability
	if err := decode(raw, &v); err != nil {
		return Capability{}, err
	}
	if err := v.Validate(); err != nil {
		return Capability{}, err
	}
	return v, nil
}
func DecodeConfig(raw []byte) (Config, error) {
	var v Config
	if err := decode(raw, &v); err != nil {
		return Config{}, err
	}
	if err := v.Validate(); err != nil {
		return Config{}, err
	}
	return v, nil
}
func DecodeExecution(raw []byte) (Execution, error) {
	var v Execution
	if err := decode(raw, &v); err != nil {
		return Execution{}, err
	}
	if err := v.Validate(); err != nil {
		return Execution{}, err
	}
	return v, nil
}

func scopesOverlap(a, b Scope) bool {
	if a.ArtifactRef != b.ArtifactRef {
		return false
	}
	if a.Unit == "whole_artifact" || b.Unit == "whole_artifact" {
		return true
	}
	if a.Unit != b.Unit {
		return true
	} // Unmapped coordinate spaces cannot establish disjointness.
	for _, x := range a.Regions {
		for _, y := range b.Regions {
			if x.Start < y.End && y.Start < x.End {
				return true
			}
		}
	}
	return false
}

// Go's JSON encoder replaces invalid UTF-8. Evidence records must reject it.
func validStrings(v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return errors.New("invalid UTF-8 in v2 record")
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			return validStrings(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := validStrings(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := validStrings(v.Index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}
