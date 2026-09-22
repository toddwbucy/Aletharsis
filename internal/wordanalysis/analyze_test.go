package wordanalysis

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

func document(body string) string {
	return `<w:document xmlns:w="` + docxidentify.TransitionalWord + `"><w:body>` + body + `</w:body></w:document>`
}
func run(text string) string { return `<w:r><w:t>` + text + `</w:t></w:r>` }
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
func TestPatternAcrossFormattingRuns(t *testing.T) {
	source := document(`<w:p>` + run(strings.Repeat("\u200b\u200c", 8)) + `<w:r><w:rPr><w:vanish/><w:b/></w:rPr><w:t>` + strings.Repeat("&#x200B;&#x200C;", 8) + `</w:t></w:r>` + run(strings.Repeat("\u200b\u200c", 8)) + `</w:p>`)
	r := analyze(t, source)
	if len(r.Scopes) != 1 || patterns(r) != 1 {
		t.Fatal("missed split sequence", len(r.Scopes), patterns(r))
	}
	s := r.Scopes[0]
	if len(s.Origins) != 48 || s.Text != strings.Repeat("\u200b\u200c", 24) || s.SHA256 != evidence.Hash([]byte(s.Text)) || s.Hashes["raw_text_sha256"] != s.SHA256 {
		t.Fatal("scope identity")
	}
	for _, f := range s.Findings {
		if f.ID != "pattern.zero_width_binary" {
			continue
		}
		positions := f.Location["character_offsets"].([]int)
		offsets := f.Location["byte_offsets"].([]int)
		if len(positions) != 48 || f.Location["source"] != s.ID {
			t.Fatal("finding scope")
		}
		for i, p := range positions {
			m := s.Origins[p]
			if offsets[i] != int(m.UTF8.Start) {
				t.Fatal("UTF-8 coordinates")
			}
			lexical := source[m.Source.Start:m.Source.End]
			if p >= 16 && p < 32 {
				if lexical != "&#x200B;" && lexical != "&#x200C;" {
					t.Fatal("entity source")
				}
			} else if lexical != "\u200b" && lexical != "\u200c" {
				t.Fatal("literal source")
			}
		}
	}
	if r.Extraction.Texts[s.Origins[16].Text].DirectVanish.State != "on" || r.Extraction.Texts[s.Origins[0].Text].DirectVanish.State != "unspecified" {
		t.Fatal("format context lost")
	}
}
func TestBoundariesPreventFalseAdjacency(t *testing.T) {
	half := strings.Repeat("\u200b\u200c", 8)
	for _, middle := range []string{`<w:r><w:tab/></w:r>`, `<w:r><w:br/></w:r>`, `<w:r><w:fldChar w:fldCharType="begin"/></w:r>`, `<w:r><w:drawing/></w:r>`, `<!-- comment -->`, `<?agent inert?>`, `<w:unknown/>`, `<w:unknown>unselected</w:unknown>`, `</w:p><w:p>`} {
		t.Run(middle, func(t *testing.T) {
			r := analyze(t, document(`<w:p>`+run(half)+middle+run(half)+`</w:p>`))
			if patterns(r) != 0 || len(r.Scopes) != 2 || len(r.Boundaries) < 1 {
				t.Fatal("crossed boundary", r)
			}
		})
	}
	r := analyze(t, document(`<w:p><w:r><w:t>`+half+`</w:t><w:instrText>`+half+`</w:instrText></w:r></w:p>`))
	if len(r.Scopes) != 2 || patterns(r) != 0 || r.Scopes[1].Role != "field_instruction" {
		t.Fatal("mixed field and plain text")
	}
	r = analyze(t, document(`<w:p><w:del w:id="1">`+run(half)+`</w:del><w:del w:id="2">`+run(half)+`</w:del></w:p>`))
	if len(r.Scopes) != 2 || patterns(r) != 0 {
		t.Fatal("mixed revisions")
	}
}
func TestCDATAUnicodeNormalizationAndLeadingFEFF(t *testing.T) {
	r := analyze(t, document(`<w:p><w:r><w:t>&#xFEFF;e<![CDATA[́😀]]></w:t></w:r></w:p>`))
	if len(r.Scopes) != 1 || len(r.Scopes[0].Origins) != 4 || r.Scopes[0].Text != "\ufeffé😀" {
		t.Fatal("CDATA join")
	}
	s := r.Scopes[0]
	bom, emoji, normalization := false, false, false
	for _, f := range s.Findings {
		if f.ID == "unicode.zero_width" {
			if f.Evidence["leading_bom"] != false {
				t.Fatal("XML character treated as transport BOM")
			}
			bom = true
		}
		if f.ID == "unicode.emoji" {
			emoji = true
		}
		if f.ID == "unicode.normalization" {
			normalization = true
		}
	}
	if !bom || !emoji || !normalization || s.Normalization["nfc_changed"] != true {
		t.Fatal("native analyzer results")
	}
}
func TestUnselectedWhitespaceIsNotInventedText(t *testing.T) {
	r := analyze(t, document("<w:p>\n"+run("a")+"\n"+run("b")+"\n</w:p>"))
	if len(r.Scopes) != 1 || r.Scopes[0].Text != "ab" || len(r.Scopes[0].Origins) != 2 {
		t.Fatal("syntax whitespace became content")
	}
}
func TestIntegrityDeterminismFailures(t *testing.T) {
	source := []byte(document(`<w:p>` + run("é\u200b") + `</w:p>`))
	before := bytes.Clone(source)
	a, err := Analyze(context.Background(), source, evidence.Hash(source))
	if err != nil {
		t.Fatal(err)
	}
	b := analyze(t, string(source))
	if !reflect.DeepEqual(a, b) || !bytes.Equal(source, before) {
		t.Fatal("determinism/integrity")
	}
	source[0] = 'x'
	if a.Extraction.XML.Document.PartSHA256 != evidence.Hash(before) {
		t.Fatal("source ownership")
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
	f.Add(document(`<w:p>` + run("A&#x200B;😀") + `</w:p>`))
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
		for _, s := range r.Scopes {
			rs := []rune(s.Text)
			if len(rs) != len(s.Origins) {
				t.Fatal("map cardinality")
			}
			for i, m := range s.Origins {
				if m.Source.Start < 0 || m.Source.End > int64(len(source)) || s.Text[m.UTF8.Start:m.UTF8.End] != string(rs[i]) {
					t.Fatal("mapping bounds")
				}
			}
			for _, finding := range s.Findings {
				if positions, ok := finding.Location["character_offsets"].([]int); ok {
					for _, p := range positions {
						if p < 0 || p >= len(rs) {
							t.Fatal("finding bounds")
						}
					}
				}
			}
		}
	})
}

func TestFormattingSubtreeTextDoesNotEnterAnalysis(t *testing.T) {
	half := strings.Repeat("\u200b\u200c", 8)
	for _, hidden := range []string{"EVIL", strings.Repeat("\u200b\u200c", 32), "   "} {
		r := analyze(t, document(`<w:p><w:r><w:t>`+half+`</w:t><w:rPr>`+run(hidden)+`</w:rPr><w:t>`+half+`</w:t></w:r></w:p>`))
		if len(r.Scopes) != 2 || patterns(r) != 0 || len(r.Extraction.Texts) != 3 {
			t.Fatal("formatting text changed scope", r)
		}
		for _, scope := range r.Scopes {
			if scope.Text != half {
				t.Fatal("formatting content entered analysis", scope.Text)
			}
			for _, origin := range scope.Origins {
				if origin.Text == 1 {
					t.Fatal("excluded text mapped into scope")
				}
			}
		}
		if len(r.Boundaries) == 0 || r.Boundaries[0].Reason != "formatting_text_not_analyzed" {
			t.Fatal("missing formatting-data boundary")
		}
	}
}

func TestPropertyContainersCannotManufactureFindings(t *testing.T) {
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, property := range []string{"rPr", "pPr", "sectPr", "tblPr", "tblPrEx", "trPr", "tcPr", "sdtPr", "tblGrid"} {
			t.Run(ns+"/"+property, func(t *testing.T) {
				hidden := run(strings.Repeat("\u200b\u200c", 24))
				body := `<w:p>` + run("before") + `<w:` + property + `><w:rPr>` + hidden + `</w:rPr></w:` + property + `>` + run("after") + `</w:p>`
				r := analyze(t, strings.ReplaceAll(document(body), docxidentify.TransitionalWord, ns))
				if len(r.Scopes) != 2 || patterns(r) != 0 || len(r.Extraction.Texts) != 3 {
					t.Fatal("formatting admitted", r)
				}
				if r.Scopes[0].Text != "before" || r.Scopes[1].Text != "after" {
					t.Fatal("incorrect scopes", r.Scopes)
				}
			})
		}
	}
}

func TestOrdinaryLargePartFailsExplicitlyAtMappingBudget(t *testing.T) {
	source := []byte(document(strings.Repeat(`<w:p>`+run(strings.Repeat("a", 1000))+`</w:p>`, 300)))
	// This is well below the byte/token/element caps: scalar mappings are the
	// independent limiting resource, not a malformed or enormous XML part.
	if _, err := xmlparts.Parse(context.Background(), source, evidence.Hash(source), xmlparts.DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if r, err := Analyze(context.Background(), source, evidence.Hash(source)); r != nil || !errors.Is(err, xmlparts.ErrLimit) {
		t.Fatalf("result %v, error %v", r, err)
	}
}

func TestFormattingOnlyStoryDeclaresEveryExcludedSegment(t *testing.T) {
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, property := range []string{"rPr", "pPr", "sectPr", "tblPr", "tblPrEx", "trPr", "tcPr", "sdtPr", "tblGrid"} {
			t.Run(ns+"/"+property, func(t *testing.T) {
				body := `<w:p><w:` + property + `>` + run(strings.Repeat("\u200b\u200c", 24)) + run(" ") + `</w:` + property + `></w:p>`
				r := analyze(t, strings.ReplaceAll(document(body), docxidentify.TransitionalWord, ns))
				if r.Extraction.State != "partial" || len(r.Extraction.Texts) != 2 || len(r.Extraction.Issues) != 2 || len(r.Scopes) != 0 || len(r.Boundaries) != 2 {
					t.Fatal("undeclared exclusion", r)
				}
				for i, b := range r.Boundaries {
					text := r.Extraction.Texts[i]
					if b.Reason != "formatting_text_not_analyzed" || b.Token != r.Extraction.XML.Segments[text.Segment].Token || r.Extraction.Issues[i].Code != "word.formatting_text_not_analyzed" || r.Extraction.Issues[i].Element != text.Element {
						t.Fatal("lost exclusion anchor", r)
					}
				}
			})
		}
	}
}
