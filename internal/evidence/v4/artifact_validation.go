package v4

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// nativeArtifact validates common record fields using the frozen v2 validator.
// After Office content is independently checked, a temporary blob reference
// permits reuse of those common checks. It is never emitted or exposed as data.
func nativeArtifact(a Artifact) (v2.Artifact, error) {
	checked := a
	if a.ContentRef != nil && a.ContentRef.Kind == "office_scope" {
		checked.ContentRef = &ContentRef{Kind: "retained_blob", SHA256: a.SHA256}
	}
	raw, err := json.Marshal(checked)
	if err != nil {
		return v2.Artifact{}, err
	}
	return v2.DecodeArtifact(raw)
}
func validateArtifacts(r Report, office *Index) (map[string]Artifact, error) {
	if err := office.ValidateScopeArtifacts(r.Artifacts); err != nil {
		return nil, err
	}
	byRef := map[string]Artifact{}
	for _, a := range r.Artifacts {
		if _, ok := byRef[a.Ref]; ok {
			return nil, ErrLinkage
		}
		for _, p := range a.Parents {
			if _, ok := byRef[p]; !ok {
				return nil, ErrLinkage
			}
		}
		byRef[a.Ref] = a
	}
	coverage := coverageIndex{artifacts: byRef, office: office, flat: r.Evidence.Document}
	sources := 0
	for _, a := range r.Artifacts {
		native, err := nativeArtifact(a)
		if err != nil {
			return nil, err
		}
		if a.Kind == "source" {
			sources++
			if !reflect.DeepEqual(a.SHA256, r.File.SHA256) || (a.ByteLength == nil) != (r.File.Size == nil) ||
				r.File.Size != nil && (*r.File.Size < 0 || *a.ByteLength != uint64(*r.File.Size)) {
				return nil, ErrLinkage
			}
		}
		if err := coverage.mapping(native.Mapping); err != nil {
			return nil, err
		}
		if native.Transform != nil {
			for _, ex := range native.Transform.Exclusions {
				if ex.Scope != nil {
					if _, err := coverage.intervals(*ex.Scope, false); err != nil {
						return nil, err
					}
				}
			}
		}
		if a.ContentRef != nil && (a.ContentRef.Kind == "report_pointer" || a.ContentRef.Kind == "office_scope") {
			text, err := coverage.text(a)
			if err != nil || !utf8.ValidString(text) || a.SHA256 == nil || a.ByteLength == nil ||
				identity.ExactBytes([]byte(text)) != *a.SHA256 || uint64(len(text)) != *a.ByteLength ||
				native.Representation.Encoding == nil || *native.Representation.Encoding != "utf-8" {
				return nil, ErrLinkage
			}
		}
	}
	if sources != 1 {
		return nil, ErrLinkage
	}
	if err := office.ValidateArtifactBindings(r.Artifacts); err != nil {
		return nil, err
	}
	return byRef, nil
}
func (x coverageIndex) mapping(m v2.Mapping) error {
	if m.Quality == "unavailable" {
		return nil
	}
	if _, ok := x.artifacts[m.FromArtifactRef]; !ok {
		return ErrLinkage
	}
	if _, ok := x.artifacts[m.ToArtifactRef]; !ok {
		return ErrLinkage
	}
	if m.DataRef != nil {
		pointer := strings.TrimSuffix(*m.DataRef, "/byte_offsets") + "/text"
		_, err := x.text(Artifact{ContentRef: &ContentRef{Kind: "report_pointer", Pointer: &pointer}})
		if err != nil {
			return err
		}
	}
	return nil
}

// validateFlatCoordinates preserves the boundary-map invariants for native flat
// evidence; Office scopes never enter this collection.
func validateFlatCoordinates(file evidence.File, d evidence.Document) error {
	for _, t := range d.Texts {
		if t == nil || file.Size == nil || !utf8.ValidString(t.Text) || len(t.ByteOffsets) != utf8.RuneCountInString(t.Text)+1 {
			return ErrCoordinates
		}
		for _, b := range t.ByteOffsets {
			if b < 0 || b > *file.Size {
				return ErrCoordinates
			}
		}
		i := 0
		for _, r := range t.Text {
			width := 0
			switch t.Encoding {
			case "utf-8":
				width = utf8.RuneLen(r)
			case "utf-16-le", "utf-16-be":
				width = 2
				if r > 0xffff {
					width = 4
				}
			case "utf-32-le", "utf-32-be":
				width = 4
			default:
				return ErrCoordinates
			}
			if t.ByteOffsets[i+1]-t.ByteOffsets[i] != width {
				return ErrCoordinates
			}
			i++
		}
		if hash, ok := t.Hashes["raw_text_sha256"]; ok && hash != identity.ExactBytes([]byte(t.Text)) {
			return ErrCoordinates
		}
	}
	return nil
}
