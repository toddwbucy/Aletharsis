package v2

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Mapping describes a coordinate claim. Exact navigation still requires checking
// the named method and its data against retained evidence and source bytes.
type Mapping struct {
	Quality         string
	ReasonCode      *failure.Code
	FromArtifactRef string
	ToArtifactRef   string
	Method          string
	Version         string
	DataRef         *string
}
type unavailableMapping struct {
	Quality    string       `json:"quality"`
	ReasonCode failure.Code `json:"reason_code"`
}
type availableMapping struct {
	Quality         string  `json:"quality"`
	FromArtifactRef string  `json:"from_artifact_ref"`
	ToArtifactRef   string  `json:"to_artifact_ref"`
	Method          string  `json:"method"`
	Version         string  `json:"version"`
	DataRef         *string `json:"data_ref"`
}

var textPointerPattern = regexp.MustCompile(`^/evidence/texts/(0|[1-9][0-9]*)/text$`)
var offsetsPointerPattern = regexp.MustCompile(`^/evidence/texts/(0|[1-9][0-9]*)/byte_offsets$`)

func (m Mapping) Validate() error {
	if err := validStrings(reflect.ValueOf(m)); err != nil {
		return err
	}
	if m.Quality == "unavailable" {
		if !validCode(m.ReasonCode) || m.FromArtifactRef != "" || m.ToArtifactRef != "" || m.Method != "" || m.Version != "" || m.DataRef != nil {
			return errors.New("invalid unavailable mapping")
		}
	} else if !slices.Contains([]string{"exact", "derived", "approximate"}, m.Quality) || m.ReasonCode != nil || !validRef(m.FromArtifactRef, "artifact") || !validRef(m.ToArtifactRef, "artifact") || m.Method == "" || m.Version == "" || m.DataRef != nil && !offsetsPointerPattern.MatchString(*m.DataRef) {
		return errors.New("invalid coordinate mapping")
	}
	return nil
}
func (m Mapping) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if m.Quality == "unavailable" {
		return json.Marshal(unavailableMapping{m.Quality, *m.ReasonCode})
	}
	return json.Marshal(availableMapping{m.Quality, m.FromArtifactRef, m.ToArtifactRef, m.Method, m.Version, m.DataRef})
}
func DecodeMapping(raw []byte) (Mapping, error) {
	if _, err := identity.Canonicalize(raw, RecordLimits()); err != nil {
		return Mapping{}, err
	}
	// Variant selection is inert; each selected shape is strictly decoded below.
	var discriminator struct {
		Quality string `json:"quality"`
	}
	if err := json.Unmarshal(raw, &discriminator); err != nil {
		return Mapping{}, err
	}
	var m Mapping
	if discriminator.Quality == "unavailable" {
		var w unavailableMapping
		if err := decode(raw, &w); err != nil {
			return m, err
		}
		m = Mapping{Quality: w.Quality, ReasonCode: &w.ReasonCode}
	} else {
		var w availableMapping
		if err := decode(raw, &w); err != nil {
			return m, err
		}
		m = Mapping{Quality: w.Quality, FromArtifactRef: w.FromArtifactRef, ToArtifactRef: w.ToArtifactRef, Method: w.Method, Version: w.Version, DataRef: w.DataRef}
	}
	return m, m.Validate()
}

// Representation specifies the exact byte serialization covered by the digest.
type Representation struct {
	Serialization string  `json:"serialization"`
	Encoding      *string `json:"encoding"`
	Normalization string  `json:"normalization"`
	LineEndings   string  `json:"line_endings"`
}
type ContentRef struct {
	Kind    string  `json:"kind"`
	Pointer *string `json:"pointer,omitempty"`
	SHA256  *string `json:"sha256,omitempty"`
}

func (c ContentRef) Validate() error {
	switch c.Kind {
	case "report_pointer":
		if c.Pointer == nil || !textPointerPattern.MatchString(*c.Pointer) || c.SHA256 != nil {
			return errors.New("invalid text content pointer")
		}
	case "retained_blob":
		if c.Pointer != nil || c.SHA256 == nil || !digestPattern.MatchString(*c.SHA256) {
			return errors.New("invalid retained blob identity")
		}
	default:
		return errors.New("invalid content reference kind")
	}
	return nil
}

type Transform struct {
	Operation    string      `json:"operation"`
	Version      string      `json:"version"`
	ConfigSHA256 *string     `json:"config_sha256"`
	Inputs       []string    `json:"inputs"`
	Exclusions   []Exclusion `json:"exclusions"`
}
type Artifact struct {
	Ref               string         `json:"artifact_ref"`
	Kind              string         `json:"kind"`
	Representation    Representation `json:"representation"`
	SHA256            *string        `json:"sha256"`
	ByteLength        *uint64        `json:"byte_length"`
	ContentRef        *ContentRef    `json:"content_ref"`
	UnavailableReason *string        `json:"unavailable_reason"`
	Parents           []string       `json:"parents"`
	Transform         *Transform     `json:"transform"`
	Mapping           Mapping        `json:"mapping"`
}

func (a Artifact) Validate() error {
	if err := validStrings(reflect.ValueOf(a)); err != nil {
		return err
	}
	if !validRef(a.Ref, "artifact") || !slices.Contains([]string{"source", "text", "package_part", "stream", "object", "manifest", "sample", "binding_text", "detector_response"}, a.Kind) {
		return errors.New("invalid artifact identity")
	}
	r := a.Representation
	if r.Serialization == "" || r.Encoding != nil && *r.Encoding == "" || !slices.Contains([]string{"none", "NFC", "NFD", "NFKC", "NFKD"}, r.Normalization) || !slices.Contains([]string{"preserved", "LF", "not_applicable"}, r.LineEndings) {
		return errors.New("invalid artifact representation")
	}
	if a.SHA256 == nil {
		if a.ByteLength != nil || a.ContentRef != nil || a.UnavailableReason == nil || !slices.Contains([]string{"not_acquired", "extraction_unavailable"}, *a.UnavailableReason) {
			return errors.New("unknown artifact must disclose absent identity")
		}
	} else {
		if !digestPattern.MatchString(*a.SHA256) || a.ByteLength == nil || *a.ByteLength > identity.MaxSafeInteger {
			return errors.New("invalid artifact digest or length")
		}
		if a.ContentRef == nil {
			if a.UnavailableReason == nil || !slices.Contains([]string{"not_retained", "redacted"}, *a.UnavailableReason) {
				return errors.New("unretained content requires reason")
			}
		} else {
			if a.UnavailableReason != nil {
				return errors.New("retained content cannot claim absence")
			}
			if err := a.ContentRef.Validate(); err != nil {
				return err
			}
			if a.ContentRef.Kind == "retained_blob" && *a.ContentRef.SHA256 != *a.SHA256 {
				return errors.New("blob/artifact digest mismatch")
			}
		}
	}
	if a.Parents == nil {
		return errors.New("artifact parents array required")
	}
	seen := map[string]bool{}
	for _, p := range a.Parents {
		if !validRef(p, "artifact") || p == a.Ref || seen[p] {
			return errors.New("invalid artifact parent")
		}
		seen[p] = true
	}
	if a.Kind == "source" && (len(a.Parents) > 0 || a.Transform != nil) {
		return errors.New("source cannot be transformed")
	}
	if t := a.Transform; t != nil {
		if t.Operation == "" || t.Version == "" || t.ConfigSHA256 != nil && !digestPattern.MatchString(*t.ConfigSHA256) || t.Inputs == nil || !slices.Equal(t.Inputs, a.Parents) || t.Exclusions == nil {
			return errors.New("invalid artifact transform")
		}
		for _, x := range t.Exclusions {
			if !validCode(&x.ReasonCode) || (x.Scope == nil) != x.UnknownRemainder {
				return errors.New("invalid transform exclusion")
			}
			if x.Scope != nil {
				if err := x.Scope.Validate(); err != nil {
					return err
				}
			}
		}
	}
	return a.Mapping.Validate()
}
func (a Artifact) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	type wire Artifact
	return json.Marshal(wire(a))
}
func DecodeArtifact(raw []byte) (Artifact, error) {
	var w struct {
		Ref               string          `json:"artifact_ref"`
		Kind              string          `json:"kind"`
		Representation    Representation  `json:"representation"`
		SHA256            *string         `json:"sha256"`
		ByteLength        *uint64         `json:"byte_length"`
		ContentRef        *ContentRef     `json:"content_ref"`
		UnavailableReason *string         `json:"unavailable_reason"`
		Parents           []string        `json:"parents"`
		Transform         *Transform      `json:"transform"`
		Mapping           json.RawMessage `json:"mapping"`
	}
	if err := decode(raw, &w); err != nil {
		return Artifact{}, err
	}
	m, err := DecodeMapping(w.Mapping)
	if err != nil {
		return Artifact{}, err
	}
	a := Artifact{w.Ref, w.Kind, w.Representation, w.SHA256, w.ByteLength, w.ContentRef, w.UnavailableReason, w.Parents, w.Transform, m}
	return a, a.Validate()
}
