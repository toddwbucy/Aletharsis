package officeplan

import (
	"fmt"
	"sort"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/odttext"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
	"github.com/toddwbucy/Aletharsis/internal/wordtext"
)

// WordScopeRecord converts retained scope coordinates only. The coordinator
// separately retains extraction boundaries, omissions, and operation outcomes.
func WordScopeRecord(s wordanalysis.Scope, part v4.Part, sourceHash string, x v4.XML, ordinal int) (v4.Scope, error) {
	if err := v4.ValidateXML(x, part); err != nil {
		return v4.Scope{}, err
	}
	return wordScopeRecord(s, part, sourceHash, x, ordinal)
}

func wordScopeRecord(s wordanalysis.Scope, part v4.Part, sourceHash string, x v4.XML, ordinal int) (v4.Scope, error) {
	r, err := scopeRecord(s.ID, s.Text, s.SHA256, s.Role, s.Hashes, wordtext.Version, wordanalysis.Version, part, sourceHash, ordinal)
	if err != nil {
		return v4.Scope{}, err
	}
	for _, o := range s.Origins {
		text, segment, scalar := o.Text, o.Segment, o.Scalar
		r.Origins = append(r.Origins, v4.Origin{Kind: "stored", XMLRef: x.XMLRef, TextIndex: &text, Segment: &segment, Scalar: &scalar, Source: wireSpan(o.Source), UTF8: wireSpan(o.UTF8), Transformation: o.Transformation})
	}
	return checkedScope(r, part, sourceHash, x)
}

// ODTControlsForScopes emits only controls used by retained analysis scopes.
// Unknown/unexpanded/omitted controls remain extraction evidence and coverage
// gaps; they must not acquire invented counts or dependent scope coordinates.
func ODTControlsForScopes(a *odtanalysis.Result) ([]v4.Control, map[int]int, error) {
	if a == nil || a.Extraction == nil {
		return nil, nil, v4.ErrCoordinates
	}
	used := map[int]bool{}
	for _, s := range a.Scopes {
		for _, o := range s.Origins {
			if o.Kind == "control_expansion" {
				used[o.Control] = true
			}
		}
	}
	indices := []int{}
	for i := range used {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	records := []v4.Control{}
	refs := map[int]int{}
	for _, i := range indices {
		if i < 0 || i >= len(a.Extraction.Controls) {
			return nil, nil, v4.ErrCoordinates
		}
		c := a.Extraction.Controls[i]
		if c.CountState != "known" || c.Count > odtanalysis.MaxScalars {
			return nil, nil, v4.ErrCoordinates
		}
		refs[i] = len(records)
		records = append(records, v4.Control{Index: len(records), Element: c.Element, Kind: controlKind(c.Kind), Count: int(c.Count), Source: wireSpan(c.Span)})
	}
	return records, refs, nil
}

func ODTScopeRecord(s odtanalysis.Scope, part v4.Part, sourceHash string, x v4.XML, controls map[int]int, ordinal int) (v4.Scope, error) {
	if err := v4.ValidateXML(x, part); err != nil {
		return v4.Scope{}, err
	}
	return odtScopeRecord(s, part, sourceHash, x, controls, ordinal)
}

func odtScopeRecord(s odtanalysis.Scope, part v4.Part, sourceHash string, x v4.XML, controls map[int]int, ordinal int) (v4.Scope, error) {
	r, err := scopeRecord(s.ID, s.Text, s.SHA256, "text", s.Hashes, odttext.Version, odtanalysis.Version, part, sourceHash, ordinal)
	if err != nil {
		return v4.Scope{}, err
	}
	for _, o := range s.Origins {
		origin := v4.Origin{Kind: o.Kind, XMLRef: x.XMLRef, Source: wireSpan(o.Source), UTF8: wireSpan(o.UTF8), Transformation: o.Transformation}
		switch o.Kind {
		case "stored":
			text, segment, scalar := o.Text, o.Segment, o.Scalar
			origin.TextIndex = &text
			origin.Segment = &segment
			origin.Scalar = &scalar
		case "control_expansion":
			index, ok := controls[o.Control]
			if !ok || index < 0 || index >= len(x.Controls) {
				return v4.Scope{}, v4.ErrCoordinates
			}
			element, repetition := x.Controls[index].Element, o.Repetition
			origin.Transformation = controlKind(o.Transformation)
			origin.Control = &index
			origin.Element = &element
			origin.Repetition = &repetition
		default:
			return v4.Scope{}, v4.ErrCoordinates
		}
		r.Origins = append(r.Origins, origin)
	}
	return checkedScope(r, part, sourceHash, x)
}

func scopeRecord(id, text, digest, role string, hashes map[string]string, extractor, assembler string, part v4.Part, sourceHash string, ordinal int) (v4.Scope, error) {
	if ordinal < 0 || part.SHA256 == nil {
		return v4.Scope{}, v4.ErrCoordinates
	}
	r := v4.Scope{ScopeRef: fmt.Sprintf("office-scope/%d", ordinal), PartRef: part.PartRef, LocalID: id, ExtractorVersion: extractor, AssemblerVersion: assembler, Role: role, Text: text, SHA256: digest, Origins: []v4.Origin{}, Boundaries: []v4.Boundary{}, Issues: []v4.Issue{},
		Hashes: v4.TextHashes{RawTextSHA256: hashes["raw_text_sha256"], NfcTextSHA256: hashes["nfc_text_sha256"], NfkcTextSHA256: hashes["nfkc_text_sha256"], FormattingRemovedSHA256: hashes["formatting_removed_sha256"]}}
	r.IdentitySHA256 = v4.ScopeIdentity(sourceHash, part.Name, *part.SHA256, extractor, assembler, id)
	return r, nil
}
func checkedScope(s v4.Scope, part v4.Part, sourceHash string, x v4.XML) (v4.Scope, error) {
	if err := v4.ValidateScope(s, part, sourceHash, map[string]v4.XML{x.XMLRef: x}); err != nil {
		return v4.Scope{}, err
	}
	return s, nil
}

// The native ODF producer calls a text:s expansion "spaces"; wire 4.0 uses
// "space" for the scalar-generating operation. Other control names are shared.
func controlKind(kind string) string {
	if kind == "spaces" {
		return "space"
	}
	return kind
}
