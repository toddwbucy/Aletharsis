package v2

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Locator variants preserve non-text evidence without manufacturing byte spans.
// Validate checks wire shape only; a report must resolve references and identities.
type Locator interface{ locator() }
type TextLocator struct {
	Version            string              `json:"version"`
	SegmentPointer     string              `json:"segment_pointer"`
	Spans              []identity.TextSpan `json:"spans"`
	SelectedTextSHA256 string              `json:"selected_text_sha256"`
	DigestDomain       string              `json:"digest_domain"`
}

func (TextLocator) locator() {}

type StructuralLocator struct {
	Version      string  `json:"version"`
	Format       string  `json:"format"`
	Part         *string `json:"part"`
	ObjectID     string  `json:"object_id"`
	ObjectSHA256 *string `json:"object_sha256"`
}

func (StructuralLocator) locator() {}

type CredentialLocator struct {
	Version                  string  `json:"version"`
	ManifestRef              string  `json:"manifest_ref"`
	AssertionID              *string `json:"assertion_id"`
	VerificationExecutionRef *string `json:"verification_execution_ref"`
}

func (CredentialLocator) locator() {}

type StatisticalLocator struct {
	Version   string `json:"version"`
	SampleRef string `json:"sample_ref"`
	Scope     Scope  `json:"scope"`
}

func (StatisticalLocator) locator() {}

type LegacyLocator struct {
	Version       string `json:"version"`
	ReportPointer string `json:"report_pointer"`
	DiagnosticRef string `json:"diagnostic_ref"`
}

func (LegacyLocator) locator() {}

type Anchor struct {
	Ref          string  `json:"anchor_ref"`
	Kind         string  `json:"kind"`
	ArtifactRef  *string `json:"artifact_ref"`
	ExecutionRef *string `json:"execution_ref"`
	Mapping      Mapping `json:"mapping"`
	Locator      Locator `json:"locator"`
}

var segmentPointerPattern = regexp.MustCompile(`^/evidence/texts/(0|[1-9][0-9]*)$`)

func (a Anchor) Validate() error {
	if !validRef(a.Ref, "anchor") || a.ArtifactRef != nil && !validRef(*a.ArtifactRef, "artifact") || a.ExecutionRef != nil && !validRef(*a.ExecutionRef, "exec") {
		return errors.New("invalid anchor references")
	}
	if a.Kind != "legacy_unknown" && (a.ArtifactRef == nil || a.ExecutionRef == nil) {
		return errors.New("anchor requires artifact and execution")
	}
	if err := a.Mapping.Validate(); err != nil {
		return err
	}
	version := ""
	switch l := a.Locator.(type) {
	case TextLocator:
		version = l.Version
		if a.Kind != "text" || !segmentPointerPattern.MatchString(l.SegmentPointer) || len(l.Spans) == 0 || !digestPattern.MatchString(l.SelectedTextSHA256) || l.DigestDomain != identity.SelectionDomain {
			return errors.New("invalid text locator")
		}
		var scalarEnd, byteEnd uint64
		for _, s := range l.Spans {
			if s.Scalar.Start < scalarEnd || s.Byte.Start < byteEnd || s.Scalar.Start >= s.Scalar.End || s.Byte.Start >= s.Byte.End || s.Scalar.End > identity.MaxSafeInteger || s.Byte.End > identity.MaxSafeInteger {
				return errors.New("invalid anchor spans")
			}
			scalarEnd, byteEnd = s.Scalar.End, s.Byte.End
		}
	case StructuralLocator:
		version = l.Version
		if a.Kind != "structural_object" || !slices.Contains([]string{"ooxml", "pdf", "odf"}, l.Format) || l.Part != nil && *l.Part == "" || l.ObjectID == "" || l.ObjectSHA256 != nil && !digestPattern.MatchString(*l.ObjectSHA256) {
			return errors.New("invalid structural locator")
		}
	case CredentialLocator:
		version = l.Version
		if a.Kind != "credential" || !validRef(l.ManifestRef, "artifact") || l.AssertionID != nil && *l.AssertionID == "" || l.VerificationExecutionRef != nil && !validRef(*l.VerificationExecutionRef, "exec") {
			return errors.New("invalid credential locator")
		}
	case StatisticalLocator:
		version = l.Version
		if a.Kind != "statistical_sample" || !validRef(l.SampleRef, "artifact") || l.SampleRef != *a.ArtifactRef || l.Scope.ArtifactRef != l.SampleRef {
			return errors.New("invalid statistical sample locator")
		}
		if err := l.Scope.Validate(); err != nil {
			return err
		}
	case LegacyLocator:
		version = l.Version
		if a.Kind != "legacy_unknown" || !strings.HasPrefix(l.ReportPointer, "/") || !validRef(l.DiagnosticRef, "diagnostic") {
			return errors.New("invalid legacy locator")
		}
	default:
		return errors.New("unsupported anchor locator")
	}
	if version != "1" {
		return errors.New("unsupported locator version")
	}
	return validStrings(reflect.ValueOf(a))
}
func (a Anchor) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	type wire Anchor
	return json.Marshal(wire(a))
}
func DecodeAnchor(raw []byte) (Anchor, error) {
	var w struct {
		Ref          string          `json:"anchor_ref"`
		Kind         string          `json:"kind"`
		ArtifactRef  *string         `json:"artifact_ref"`
		ExecutionRef *string         `json:"execution_ref"`
		Mapping      json.RawMessage `json:"mapping"`
		Locator      json.RawMessage `json:"locator"`
	}
	if err := decode(raw, &w); err != nil {
		return Anchor{}, err
	}
	m, err := DecodeMapping(w.Mapping)
	if err != nil {
		return Anchor{}, err
	}
	a := Anchor{Ref: w.Ref, Kind: w.Kind, ArtifactRef: w.ArtifactRef, ExecutionRef: w.ExecutionRef, Mapping: m}
	switch w.Kind {
	case "text":
		var l TextLocator
		err = decode(w.Locator, &l)
		a.Locator = l
	case "structural_object":
		var l StructuralLocator
		err = decode(w.Locator, &l)
		a.Locator = l
	case "credential":
		var l CredentialLocator
		err = decode(w.Locator, &l)
		a.Locator = l
	case "statistical_sample":
		var l StatisticalLocator
		err = decode(w.Locator, &l)
		a.Locator = l
	case "legacy_unknown":
		var l LegacyLocator
		err = decode(w.Locator, &l)
		a.Locator = l
	default:
		return Anchor{}, errors.New("unsupported anchor kind")
	}
	if err != nil {
		return Anchor{}, err
	}
	return a, a.Validate()
}
