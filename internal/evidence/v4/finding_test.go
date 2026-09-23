package v4

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
	"testing"
)

func TestFindingTranslationFromWordAnalysis(t *testing.T) {
	raw := []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>😀é&#x200B;&#x200C;&#x202E;</w:t></w:r></w:p></w:body></w:document>`)
	result, err := wordanalysis.Analyze(context.Background(), raw, identity.ExactBytes(raw))
	if err != nil {
		t.Fatal(err)
	}
	tested := 0
	for _, local := range result.Scopes {
		s := Scope{ScopeRef: "office-scope/0", LocalID: local.ID, Text: local.Text}
		for _, finding := range local.Findings {
			original := finding.Location
			translated, err := OfficeFinding(finding, s)
			if err != nil {
				t.Fatalf("%s: %v", finding.ID, err)
			}
			if _, ok := translated.Location["source"]; ok {
				t.Fatal("legacy source retained")
			}
			if original["source"] != local.ID {
				t.Fatal("mutated caller location")
			}
			if err := ValidateFindingCoordinates(translated, map[string]Scope{s.ScopeRef: s}); err != nil {
				t.Fatal(err)
			}
			tested++
		}
	}
	if tested < 4 {
		t.Fatalf("only %d finding variants exercised", tested)
	}
}
func TestFindingRejectsMixedAndWrongCoordinates(t *testing.T) {
	s := Scope{ScopeRef: "office-scope/0", Text: "😀\u200bA"}
	valid := evidence.Finding{ID: "unicode.zero_width", Evidence: evidence.Object{"count": 1}, Location: evidence.Object{
		"kind": "office_offsets", "scope_ref": s.ScopeRef, "scope_character_offsets": []int{1}, "scope_byte_offsets": []int{4}}}
	scopes := map[string]Scope{s.ScopeRef: s}
	if err := ValidateFindingCoordinates(valid, scopes); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*evidence.Finding){
		func(f *evidence.Finding) { f.Location["scope_byte_offsets"] = []int{1} },
		func(f *evidence.Finding) { f.Location["scope_character_offsets"] = []int{3} },
		func(f *evidence.Finding) { f.Location["scope_character_offsets"] = []bool{true} },
		func(f *evidence.Finding) { f.Location["scope_ref"] = "office-scope/99" },
		func(f *evidence.Finding) { f.Location["byte_offsets"] = []int{4} },
		func(f *evidence.Finding) { f.Evidence["count"] = 2 },
		func(f *evidence.Finding) { f.ID = "metadata.present" },
	} {
		f := valid
		f.Location = evidence.Object{}
		f.Evidence = evidence.Object{"count": 1}
		for k, v := range valid.Location {
			f.Location[k] = v
		}
		mutate(&f)
		if ValidateFindingCoordinates(f, scopes) == nil {
			t.Fatal("accepted invalid Office finding coordinates")
		}
	}
}
