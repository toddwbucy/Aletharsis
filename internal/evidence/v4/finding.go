package v4

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

// OfficeFinding translates a WA/OA local finding without changing evidence or
// claiming that scope byte offsets address the source ZIP. Graph linkage is
// assigned by the report assembler after this translation.
func OfficeFinding(f evidence.Finding, scope Scope) (evidence.Finding, error) {
	if !strings.HasPrefix(f.ID, "unicode.") && !strings.HasPrefix(f.ID, "pattern.") {
		return evidence.Finding{}, ErrCoordinates
	}
	source, ok := f.Location["source"].(string)
	if !ok || source != scope.LocalID {
		return evidence.Finding{}, ErrCoordinates
	}
	if _, ok := f.Location["character_offsets"]; ok {
		if len(f.Location) != 3 {
			return evidence.Finding{}, ErrCoordinates
		}
		f.Location = evidence.Object{"kind": "office_offsets", "scope_ref": scope.ScopeRef,
			"scope_character_offsets": f.Location["character_offsets"], "scope_byte_offsets": f.Location["byte_offsets"]}
	} else {
		if f.ID != "unicode.normalization" || len(f.Location) != 1 {
			return evidence.Finding{}, ErrCoordinates
		}
		f.Location = evidence.Object{"kind": "office_scope", "scope_ref": scope.ScopeRef}
	}
	if err := ValidateFindingCoordinates(f, map[string]Scope{scope.ScopeRef: scope}); err != nil {
		return evidence.Finding{}, err
	}
	return f, nil
}

// ValidateFindingCoordinates uses Office scopes exclusively. Legacy flat-file
// coordinates are the responsibility of the unchanged v2 validator.
func ValidateFindingCoordinates(f evidence.Finding, scopes map[string]Scope) error {
	ref, ok := f.Location["scope_ref"].(string)
	if !ok {
		return ErrCoordinates
	}
	s, ok := scopes[ref]
	if !ok || !utf8.ValidString(s.Text) {
		return ErrCoordinates
	}
	switch f.Location["kind"] {
	case "office_scope":
		if f.ID != "unicode.normalization" || len(f.Location) != 2 {
			return ErrCoordinates
		}
	case "office_offsets":
		if len(f.Location) != 4 || (!strings.HasPrefix(f.ID, "unicode.") && !strings.HasPrefix(f.ID, "pattern.")) || f.ID == "unicode.normalization" {
			return ErrCoordinates
		}
		scalars, err := offsets(f.Location["scope_character_offsets"])
		if err != nil {
			return err
		}
		bytes, err := offsets(f.Location["scope_byte_offsets"])
		if err != nil || len(scalars) == 0 || len(scalars) != len(bytes) {
			return ErrCoordinates
		}
		if count, ok := f.Evidence["count"]; ok {
			values, err := offsets([]any{count})
			if err != nil || len(values) != 1 || values[0] != int64(len(scalars)) {
				return ErrCoordinates
			}
		}
		starts := make([]int64, 0, utf8.RuneCountInString(s.Text))
		for offset := range s.Text {
			starts = append(starts, int64(offset))
		}
		for i, index := range scalars {
			if index < 0 || index >= int64(len(starts)) || bytes[i] != starts[index] || (i > 0 && index <= scalars[i-1]) {
				return ErrCoordinates
			}
		}
	default:
		return ErrCoordinates
	}
	return nil
}
func offsets(v any) ([]int64, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, ErrCoordinates
	}
	var values []int64
	if string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
		return nil, ErrCoordinates
	}
	for _, n := range values {
		if n < 0 || n > 9007199254740991 {
			return nil, ErrCoordinates
		}
	}
	return values, nil
}
