package odtanalysis

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/odttext"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func document(body string) string {
	return `<office:document-content xmlns:office="` + odtidentify.OfficeNS + `" xmlns:text="` + odttext.TextNS + `" office:version="1.3"><office:body><office:text>` + body + `</office:text></office:body></office:document-content>`
}
func analyze(t testing.TB, s string) *Result {
	t.Helper()
	b := []byte(s)
	r, err := Analyze(context.Background(), b, evidence.Hash(b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func patterns(r *Result) int {
	n := 0
	for _, s := range r.Scopes {
		for _, f := range s.Findings {
			if f.ID == "pattern.zero_width_binary" {
				n++
			}
		}
	}
	return n
}
func TestSplitPatternAndMapping(t *testing.T) {
	source := document(`<text:p>` + strings.Repeat("\u200b\u200c", 8) + `<text:span text:style-name="style">` + strings.Repeat("&#x200B;&#x200C;", 8) + `</text:span><text:a>` + strings.Repeat("\u200b\u200c", 8) + `</text:a></text:p>`)
	r := analyze(t, source)
	if len(r.Scopes) != 1 || patterns(r) != 1 || len(r.Scopes[0].Origins) != 48 {
		t.Fatal("split sequence")
	}
	s := r.Scopes[0]
	for _, f := range s.Findings {
		if f.ID != "pattern.zero_width_binary" {
			continue
		}
		positions := f.Location["character_offsets"].([]int)
		offsets := f.Location["byte_offsets"].([]int)
		for i, p := range positions {
			m := s.Origins[p]
			lexical := source[m.Source.Start:m.Source.End]
			if m.Kind != "stored" || m.Control != -1 || offsets[i] != int(m.UTF8.Start) {
				t.Fatal("coordinate class")
			}
			if p >= 16 && p < 32 {
				if lexical != "&#x200B;" && lexical != "&#x200C;" {
					t.Fatal("entity mapping")
				}
			} else if lexical != "\u200b" && lexical != "\u200c" {
				t.Fatal("literal mapping")
			}
		}
	}
	if s.Hashes["raw_text_sha256"] != s.SHA256 || s.SHA256 != evidence.Hash([]byte(s.Text)) {
		t.Fatal("identity")
	}
}
func TestExplicitWhitespaceOrigins(t *testing.T) {
	source := document(`<text:p>A<text:s text:c="3"/><text:span>&#x200B;</text:span><text:tab/><text:line-break/>😀</text:p>`)
	r := analyze(t, source)
	if len(r.Scopes) != 1 || r.Scopes[0].Text != "A   \u200b\t\n😀" {
		t.Fatal("projection", r.Scopes)
	}
	s := r.Scopes[0]
	if len(s.Origins) != 8 {
		t.Fatal("scalar cardinality")
	}
	for _, i := range []int{1, 2, 3, 5, 6} {
		m := s.Origins[i]
		if m.Kind != "control_expansion" || m.Text != -1 || m.Segment != -1 || m.Scalar != -1 || m.Source != r.Extraction.Controls[m.Control].Span {
			t.Fatal("fabricated source character", m)
		}
		if i <= 3 && (m.Repetition != i-1 || source[m.Source.Start:m.Source.End] != `<text:s text:c="3"/>`) {
			t.Fatal("space repetition anchor")
		}
	}
	if s.Origins[4].Kind != "stored" || source[s.Origins[4].Source.Start:s.Origins[4].Source.End] != "&#x200B;" {
		t.Fatal("stored versus generated")
	}
	for _, f := range s.Findings {
		if f.ID == "unicode.zero_width" {
			if !reflect.DeepEqual(f.Location["character_offsets"], []int{4}) || !reflect.DeepEqual(f.Location["byte_offsets"], []int{4}) {
				t.Fatal("expanded coordinate")
			}
		}
	}
}
func TestBoundariesAndPartialCoverage(t *testing.T) {
	half := strings.Repeat("\u200b\u200c", 8)
	for _, middle := range []string{`</text:p><text:p>`, `<text:hidden-paragraph/>`, `<!-- inert -->`, `<?agent inert?>`, `<text:unknown/>`, `<text:s text:c="invalid"/>`} {
		r := analyze(t, document(`<text:p>`+half+middle+half+`</text:p>`))
		if len(r.Scopes) != 2 || patterns(r) != 0 || len(r.Boundaries) == 0 {
			t.Fatal("crossed boundary", middle)
		}
	}
	r := analyze(t, document(`<text:p><text:hidden-text>`+half+`</text:hidden-text><text:conditional-text>`+half+`</text:conditional-text></text:p>`))
	if len(r.Scopes) != 2 || patterns(r) != 0 {
		t.Fatal("mixed conditional fields")
	}
	r = analyze(t, document(`<text:p>A<text:s text:c="0"/>B</text:p>`))
	if r.Extraction.State != "partial" || len(r.Scopes) != 2 || r.Scopes[0].Text != "A" || r.Scopes[1].Text != "B" {
		t.Fatal("unresolved count omitted without boundary")
	}
}
func TestGlobalExpansionBudget(t *testing.T) {
	for _, body := range []string{`<text:p><text:s text:c="200001"/></text:p>`, `<text:p><text:s text:c="18446744073709551615"/></text:p>`, `<text:p><text:s text:c="100000"/></text:p><text:p><text:s text:c="100001"/></text:p>`, `<text:p>A<text:s text:c="200000"/></text:p>`} {
		b := []byte(document(body))
		r, err := Analyze(context.Background(), b, evidence.Hash(b))
		if err != nil || r.State != "partial" || r.Extraction == nil {
			t.Fatal("expansion coverage", err)
		}
		limited := false
		for _, boundary := range r.Boundaries {
			limited = limited || boundary.Reason == "control_expansion_limit"
		}
		if !limited {
			t.Fatal("missing limit anchor")
		}
	}
	r := analyze(t, document(`<text:p><text:s text:c="200000"/></text:p>`))
	if len(r.Scopes) != 1 || len(r.Scopes[0].Origins) != MaxScalars || len(r.Scopes[0].Text) != MaxScalars {
		t.Fatal("exact budget")
	}
}
func TestCommittedHiddenODT(t *testing.T) {
	source, err := os.ReadFile("../packageparts/testdata/hidden.odt")
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(source)
	identified, err := odtidentify.Inspect(context.Background(), source, evidence.Hash(source))
	if err != nil || identified.Format != "odt" {
		t.Fatal("package identity", err)
	}
	found := false
	for _, part := range identified.Package.Parts {
		if part.Name != "content.xml" {
			continue
		}
		r, err := Analyze(context.Background(), part.Bytes, part.SHA256)
		if err != nil {
			t.Fatal(err)
		}
		if patterns(r) != 1 {
			t.Fatal("hidden pattern missing")
		}
		for _, s := range r.Scopes {
			if len(s.Origins) != 64 {
				continue
			}
			for _, m := range s.Origins {
				if m.Kind != "stored" || len(r.Extraction.Texts[m.Text].HiddenDeclarations) != 1 {
					t.Fatal("hidden context lost")
				}
			}
			found = true
		}
	}
	if !found || !bytes.Equal(source, before) {
		t.Fatal("fixture evidence/integrity")
	}
}
func TestUnicodeCDATAAndIntegrity(t *testing.T) {
	source := []byte(document(`<text:p>&#xFEFF;e<text:span><![CDATA[́😀]]></text:span></text:p>`))
	before := bytes.Clone(source)
	a, err := Analyze(context.Background(), source, evidence.Hash(source))
	if err != nil {
		t.Fatal(err)
	}
	b := analyze(t, string(source))
	if !reflect.DeepEqual(a, b) || !bytes.Equal(source, before) {
		t.Fatal("determinism/integrity")
	}
	s := a.Scopes[0]
	if len(a.Scopes) != 1 || s.Text != "\ufeffé😀" || len(s.Origins) != 4 || s.Normalization["nfc_changed"] != true {
		t.Fatal("normalization")
	}
	emoji := false
	for _, f := range s.Findings {
		if f.ID == "unicode.emoji" {
			emoji = true
		}
		if f.ID == "unicode.zero_width" && f.Evidence["leading_bom"] != false {
			t.Fatal("false transport BOM")
		}
	}
	if !emoji {
		t.Fatal("emoji lost")
	}
	source[0] = 'x'
	if a.Extraction.XML.Document.PartSHA256 != evidence.Hash(before) {
		t.Fatal("source alias")
	}
	if r, err := Analyze(context.Background(), before, strings.Repeat("0", 64)); r != nil || !errors.Is(err, xmlparts.ErrIdentity) {
		t.Fatal("identity", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Analyze(ctx, before, evidence.Hash(before)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancel", err)
	}
}
func FuzzAnalyze(f *testing.F) {
	f.Add(document(`<text:p>A<text:s text:c="3"/>&#x200B;😀</text:p>`))
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 32768 {
			t.Skip()
		}
		b := []byte(source)
		r, err := Analyze(context.Background(), b, evidence.Hash(b))
		if err != nil {
			if r != nil {
				t.Fatal("partial result")
			}
			return
		}
		total := 0
		for _, s := range r.Scopes {
			rs := []rune(s.Text)
			total += len(rs)
			if len(rs) != len(s.Origins) {
				t.Fatal("map cardinality")
			}
			for i, m := range s.Origins {
				if m.Source.Start < 0 || m.Source.End > int64(len(source)) || s.Text[m.UTF8.Start:m.UTF8.End] != string(rs[i]) {
					t.Fatal("mapping")
				}
				if m.Kind == "control_expansion" && m.Source != r.Extraction.Controls[m.Control].Span {
					t.Fatal("control anchor")
				}
			}
		}
		if total > MaxScalars {
			t.Fatal("unbounded expansion")
		}
	})
}

func TestExpansionLimitPreservesStoredEvidence(t *testing.T) {
	half := strings.Repeat("\u200b\u200c", 8)
	source := document(`<text:p>` + half + `<text:s text:c="200001"/>` + half + `😀</text:p>`)
	r := analyze(t, source)
	if r.State != "partial" || r.Extraction.State != "completed" || len(r.Scopes) != 2 || patterns(r) != 0 || r.Extraction.Controls[0].Count != 200001 {
		t.Fatal("lost extraction or crossed limit boundary")
	}
	for _, s := range r.Scopes {
		if len(s.Findings) == 0 {
			t.Fatal("lost Unicode findings")
		}
	}
	// A preceding expansion cannot consume capacity reserved for later stored text.
	r = analyze(t, document(`<text:p><text:s text:c="200000"/></text:p><text:p>😀&#x200B;</text:p>`))
	if r.State != "partial" || len(r.Scopes) != 1 || r.Scopes[0].Text != "😀\u200b" {
		t.Fatal("stored text crowded out")
	}
	b := []byte(document(`<text:p>` + strings.Repeat("a", MaxScalars+1) + `</text:p>`))
	if r, err := Analyze(context.Background(), b, evidence.Hash(b)); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
		t.Fatal("stored text budget", err)
	}
}

func TestLegacyContentStillAnalyzed(t *testing.T) {
	source := strings.Replace(document(`<text:p>😀&#x200B;</text:p>`), `office:version="1.3"`, `office:version="1.1"`, 1)
	r := analyze(t, source)
	if r.State != "partial" || r.Extraction.DocumentVersion != "1.1" || len(r.Scopes) != 1 || len(r.Scopes[0].Findings) < 2 {
		t.Fatal("legacy findings unavailable")
	}
}

func TestEachSkippedExpansionRetainsAnchorWithoutActiveScope(t *testing.T) {
	r := analyze(t, document(`<text:p><text:s text:c="200001"/><text:s text:c="200002"/></text:p>`))
	if r.State != "partial" || len(r.Scopes) != 0 || len(r.Boundaries) != 2 || len(r.Extraction.Controls) != 2 {
		t.Fatal("lost skipped control evidence")
	}
	if r.Boundaries[0].Token == r.Boundaries[1].Token {
		t.Fatal("duplicate rather than distinct anchor")
	}
	for _, b := range r.Boundaries {
		if b.Reason != "control_expansion_limit" {
			t.Fatal("wrong omission reason")
		}
	}
}
