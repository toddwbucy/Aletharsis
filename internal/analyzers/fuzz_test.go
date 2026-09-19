package analyzers_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func FuzzAnalyzerEvidence(f *testing.F) {
	for _, s := range []string{"", "ordinary text", "author: Example", "12345678-1234-1234-1234-123456789abc", "می\u200cروم", "👩\u200d💻\u202e", strings.Repeat("\u200b\u200c", 16), strings.Repeat("\U000e0100", 8), strings.Repeat("\U000e0061", 16), "A" + strings.Repeat("\u0301", 40)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 || !utf8.ValidString(s) {
			t.Skip("valid extracted text, at most 4 KiB")
		}
		checks := []analyzers.Analyzer{analyzers.Unicode{}, analyzers.Emoji{}, analyzers.Text{}, analyzers.Identifiers{}, analyzers.Patterns{}}
		analyze := func() ([]evidence.Finding, evidence.Document) {
			d := textDocument(s)
			before := append([]int(nil), d.Texts[0].ByteOffsets...)
			findings := []evidence.Finding{}
			for _, check := range checks {
				findings = append(findings, check.Analyze(&d)...)
			}
			if d.Texts[0].Text != s || !reflect.DeepEqual(before, d.Texts[0].ByteOffsets) {
				t.Fatal("analysis mutated source text/coordinates")
			}
			return findings, d
		}
		findings, d := analyze()
		repeated, again := analyze()
		if !reflect.DeepEqual(findings, repeated) || !reflect.DeepEqual(d, again) {
			t.Fatal("analysis is nondeterministic")
		}
		for _, finding := range findings {
			if finding.Confidence < 0 || finding.Confidence > 1 {
				t.Fatal("confidence out of range")
			}
			chars, ok := finding.Location["character_offsets"].([]int)
			if !ok {
				continue
			}
			offsets, ok := finding.Location["byte_offsets"].([]int)
			if !ok || len(chars) != len(offsets) {
				t.Fatal("coordinate arrays do not align")
			}
			previous := -1
			for i, p := range chars {
				if p <= previous || p < 0 || p >= utf8.RuneCountInString(s) || offsets[i] != d.Texts[0].ByteOffsets[p] {
					t.Fatal("finding does not identify exact source evidence")
				}
				previous = p
			}
			if count, ok := finding.Evidence["count"].(int); ok && count != len(chars) {
				t.Fatal("finding count/positions disagree")
			}
		}
		if _, err := json.Marshal(findings); err != nil {
			t.Fatal("non-serializable evidence", err)
		}
	})
}
