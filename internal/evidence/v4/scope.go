package v4

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// ScopeIdentity uses a versioned, length-unambiguous JSON tuple. The namespace
// prefix separates this digest from content hashes. Exact original part names
// participate even when two parts have identical bytes.
func ScopeIdentity(sourceHash, partName, partHash, extractor, assembler, localID string) string {
	tuple, _ := json.Marshal([]string{sourceHash, partName, partHash, extractor, assembler, localID})
	return identity.ExactBytes(append([]byte("aletharsis.office-scope/1\x00"), tuple...))
}

// ValidateScope checks origins against already validated XML records for its
// part. It does not open files or promote internal consistency to authenticity.
func ValidateScope(s Scope, part Part, sourceHash string, documents map[string]XML) error {
	if part.SHA256 == nil || part.ByteLength == nil || s.PartRef != part.PartRef ||
		s.IdentitySHA256 != ScopeIdentity(sourceHash, part.Name, *part.SHA256,
			s.ExtractorVersion, s.AssemblerVersion, s.LocalID) || !utf8.ValidString(s.Text) ||
		s.SHA256 != identity.ExactBytes([]byte(s.Text)) || s.Hashes.RawTextSHA256 != s.SHA256 ||
		s.Origins == nil || len(s.Origins) != utf8.RuneCountInString(s.Text) {
		return ErrCoordinates
	}
	i := 0
	for offset, r := range s.Text {
		o := s.Origins[i]
		x, ok := documents[o.XMLRef]
		if !ok || x.PartRef != part.PartRef || o.UTF8 != (Span{int64(offset), int64(offset + utf8.RuneLen(r))}) ||
			!o.Source.Within(*part.ByteLength) {
			return ErrCoordinates
		}
		switch o.Kind {
		case "stored":
			// ValidateStoredOrigins is a whole-string primitive. Shift this one-scalar
			// view into its own coordinate domain; keep the lexical region unchanged.
			local := o
			local.UTF8 = Span{0, int64(utf8.RuneLen(r))}
			if err := ValidateStoredOrigins(string(r), identity.ExactBytes([]byte(string(r))),
				[]Origin{local}, map[string][]Segment{o.XMLRef: x.Segments}); err != nil {
				return err
			}
		case "control_expansion":
			if err := ValidateControlOrigin(o, r, map[string][]Control{o.XMLRef: x.Controls}); err != nil {
				return err
			}
		default:
			return ErrCoordinates
		}
		i++
	}
	for _, b := range s.Boundaries {
		x, ok := documents[b.XMLRef]
		if !ok || x.PartRef != part.PartRef || b.Token < 0 || b.Token >= int64(len(x.Tokens)) {
			return ErrCoordinates
		}
	}
	return nil
}
