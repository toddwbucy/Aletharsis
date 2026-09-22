package wordanalysis

import (
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
)

func TestContextAdmissionPaths(t *testing.T) {
	payload := strings.Repeat("\u200b\u200c", 24)
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, wrap := range []string{
			`<w:p>%s</w:p>`,
			`<w:tbl><w:tr><w:tc><w:p>%s</w:p></w:tc></w:tr></w:tbl>`,
			`<w:sdt><w:sdtContent><w:p>%s</w:p></w:sdtContent></w:sdt>`,
			`<w:p><w:sdt><w:sdtContent>%s</w:sdtContent></w:sdt></w:p>`,
			`<w:p><w:r><w:ruby><w:rubyBase>%s</w:rubyBase></w:ruby></w:r></w:p>`,
			`<w:p><w:r><w:ruby><w:rt>%s</w:rt></w:ruby></w:r></w:p>`,
		} {
			r := analyze(t, strings.ReplaceAll(document(strings.Replace(wrap, "%s", run(payload), 1)), docxidentify.TransitionalWord, ns))
			if r.Extraction.State != "completed" || patterns(r) != 1 || len(r.Scopes) != 1 || r.Extraction.Texts[0].AnalysisContext.BlockedElement != -1 {
				t.Fatalf("admission %s: %+v", wrap, r)
			}
		}
		for _, wrapper := range []string{"hyperlink", "ins", "del", "moveFrom", "moveTo", "smartTag", "customXml", "fldSimple", "bdo", "dir"} {
			for _, role := range []string{"t", "delText", "instrText", "delInstrText"} {
				body := `<w:p><w:` + wrapper + `><w:r><w:` + role + `>` + payload + `</w:` + role + `></w:r></w:` + wrapper + `></w:p>`
				r := analyze(t, strings.ReplaceAll(document(body), docxidentify.TransitionalWord, ns))
				if r.Extraction.State != "completed" || patterns(r) != 1 {
					t.Fatalf("%s/%s not analyzed", wrapper, role)
				}
			}
		}
	}
}

func TestUnknownAndMisplacedAncestorsCannotAuthorizeAnalysis(t *testing.T) {
	payload := run(strings.Repeat("\u200b\u200c", 24))
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, container := range []string{"docPartPr", "customXmlPr", "smartTagPr", "rubyPr", "framePr", "numPr", "futureContainer", "p", "r", "tbl", "tr", "tc", "sdtContent", "rubyBase", "rt"} {
			body := `<w:p><w:` + container + `>` + payload + `</w:` + container + `></w:p>`
			r := analyze(t, strings.ReplaceAll(document(body), docxidentify.TransitionalWord, ns))
			if r.Extraction.State != "partial" || len(r.Scopes) != 0 || len(r.Extraction.Texts) != 1 || len(r.Boundaries) != 1 {
				t.Fatalf("%s: undeclared context", container)
			}
			text := r.Extraction.Texts[0]
			blocked := text.AnalysisContext.BlockedElement
			if blocked < 0 || r.Extraction.XML.Document.Elements[blocked].Name.Local != container || r.Boundaries[0].Reason != "text_context_not_analyzed" {
				t.Fatalf("%s: wrong blocker", container)
			}
		}
	}
	// Known descendants cannot reset an unsupported path; namespace spoofing and
	// drawing wrappers also remain declared gaps with their exact maps retained.
	for _, body := range []string{
		`<w:future><w:p>` + payload + `</w:p></w:future>`,
		`<w:p><x:hyperlink xmlns:x="urn:foreign">` + payload + `</x:hyperlink></w:p>`,
		`<w:p><w:r><w:drawing><w:txbxContent><w:p>` + payload + `</w:p></w:txbxContent></w:drawing></w:r></w:p>`,
	} {
		r := analyze(t, document(body))
		if len(r.Scopes) != 0 || len(r.Extraction.Texts) != 1 || r.Extraction.State != "partial" || r.Extraction.Texts[0].AnalysisContext.BlockedElement < 0 {
			t.Fatal("ancestry reset", r)
		}
	}
}
