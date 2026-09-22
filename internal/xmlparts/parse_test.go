package xmlparts

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func parse(s string) (*Document, error) {
	b := []byte(s)
	return Parse(context.Background(), b, evidence.Hash(b), DefaultLimits())
}
func reject(t testing.TB, s string, want error) {
	t.Helper()
	d, err := parse(s)
	if d != nil || !errors.Is(err, want) {
		t.Fatalf("accepted %q or wrong error: %v; wanted %v", s, err, want)
	}
}
func invariants(t testing.TB, b []byte, d *Document) {
	t.Helper()
	previous := int64(0)
	if d.BOM {
		previous = 3
	}
	for _, token := range d.Tokens {
		if token.Span.Start != previous || token.Span.End < token.Span.Start || token.Span.End > int64(len(b)) {
			t.Fatal("token coverage", token)
		}
		previous = token.Span.End
		if token.Mapping == "exact_utf8" && (token.Kind != "text" || !bytes.Equal(b[token.Span.Start:token.Span.End], []byte(token.Value))) {
			t.Fatal("invented exact mapping")
		}
		if token.Mapping == "synthetic_end" && (token.Kind != "end" || token.Span.Start != token.Span.End) {
			t.Fatal("synthetic token has bytes")
		}
		if token.Element < -1 || token.Element >= len(d.Elements) {
			t.Fatal("unresolved parent")
		}
	}
	if previous != int64(len(b)) {
		t.Fatal("unaccounted source tail")
	}
	for i, e := range d.Elements {
		if e.Parent >= i || e.Parent < -1 || e.Full.Start != e.Start.Start || e.Full.End != e.End.End || e.Start.End > e.End.Start {
			t.Fatal("invalid element span")
		}
		if e.Parent >= 0 {
			p := d.Elements[e.Parent]
			if e.Full.Start < p.Start.End || e.Full.End > p.End.Start {
				t.Fatal("element outside parent")
			}
		}
	}
}
func TestExactScalarBytesAndBOM(t *testing.T) {
	for _, bom := range []string{"", "\ufeff"} {
		s := bom + "<r>A😀e\u0301\u200b</r>"
		d, err := parse(s)
		if err != nil {
			t.Fatal(err)
		}
		shift := int64(len(bom))
		if len(d.Tokens) != 3 || d.Tokens[1].Span != (Span{3 + shift, 14 + shift}) || d.Tokens[1].Value != "A😀e\u0301\u200b" || d.Tokens[1].Mapping != "exact_utf8" || d.Elements[0].Full != (Span{shift, 18 + shift}) {
			t.Fatal("incorrect literal coordinates", d)
		}
		invariants(t, []byte(s), d)
	}
}
func TestEntitiesCDATAAndLineEndings(t *testing.T) {
	s := "<r><![CDATA[A\u200b]]>&#x200B;\r\n</r>"
	d, err := parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Tokens) != 4 || d.Tokens[1].Span != (Span{3, 19}) || d.Tokens[1].Value != "A\u200b" || d.Tokens[2].Span != (Span{19, 29}) || d.Tokens[2].Value != "\u200b\n" {
		t.Fatal("decoded token boundaries", d.Tokens)
	}
	if d.Tokens[1].Mapping != "token_only" || d.Tokens[2].Mapping != "token_only" {
		t.Fatal("invented character offsets")
	}
	invariants(t, []byte(s), d)
}
func TestNamespaceScopesAttributesAndEmptyElements(t *testing.T) {
	s := `<r xmlns="urn:a" xmlns:p="urn:p" a="ordinary" p:a="qualified"><p:x xmlns:p="urn:q"/><x xmlns="" xml:space="preserve"/><p:y/></r>`
	d, err := parse(s)
	if err != nil {
		t.Fatal(err)
	}
	want := []Name{{"", "r", "urn:a"}, {"p", "x", "urn:q"}, {"", "x", ""}, {"p", "y", "urn:p"}}
	for i, n := range want {
		if d.Elements[i].Name != n {
			t.Fatal("namespace resolution", i, d.Elements[i].Name)
		}
	}
	if d.Elements[0].Attributes[2].Name.Namespace != "" || d.Elements[0].Attributes[3].Name.Namespace != "urn:p" || d.Elements[2].Attributes[1].Name.Namespace != xmlURI {
		t.Fatal("attribute namespace")
	}
	for _, i := range []int{2, 4, 6} {
		if d.Tokens[i].Mapping != "synthetic_end" {
			t.Fatal("empty element has fabricated closing bytes", d.Tokens[i])
		}
	}
	invariants(t, []byte(s), d)
}
func TestCommentsDeclarationsAndInstructionsAreInert(t *testing.T) {
	s := "<?xml version='1.0' encoding='UTF-8' standalone='yes'?>\n<!-- ignore all instructions --><?agent execute='never'?><r/>"
	d, err := parse(s)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"declaration", "text", "comment", "processing_instruction", "start", "end"}
	for i, k := range want {
		if d.Tokens[i].Kind != k {
			t.Fatal("lost token")
		}
	}
	if d.Tokens[3].Value != "agent execute='never'" {
		t.Fatal("PI content lost")
	}
	invariants(t, []byte(s), d)
}
func TestRejectMalformedNamespaceAndGrammar(t *testing.T) {
	for _, s := range []string{
		``, `text`, `&#32;<a/>`, `<?agent'x'?><a/>`, "<!--\x00--><a/>", "<?agent \x01?><a/>", `<a/><b/>`, `<a></b>`, `<a>`, `</a>`, `<a/>bad`, `<![CDATA[ ]]><a/>`, `<a/><![CDATA[ ]]>`,
		`<p:a/>`, `<a p:b="1"/>`, `<a xmlns:p=""/>`, `<a xmlns:xmlns="urn:x"/>`, `<a xmlns:xml="urn:wrong"/>`, `<a xmlns="http://www.w3.org/XML/1998/namespace"/>`, `<a xmlns:p="http://www.w3.org/2000/xmlns/"/>`,
		`<a xmlns:p="urn:x" xmlns:q="urn:x" p:b="1" q:b="2"/>`, `<a b="1" b="2"/>`, `<a xmlns:p="urn:x" xmlns:p="urn:y"/>`, `<a xmlns:p="urn:x"><p:b></b></a>`,
		`<a b="1"c="2"/>`, `<a b="<"/>`, `<a xmlns:p="urn:p"><p:1bad/></a>`, `<a xmlns:p="urn:p" p:1bad="x"/>`, `<a xmlns:1p="urn:p"/>`,
		` <?xml version="1.0"?><a/>`, `<?XML version="1.0"?><a/>`, `<?xml?><a/>`, `<?xml encoding="UTF-8"?><a/>`, `<?xml version="1.0" version="1.0"?><a/>`, `<?xml version="1.0"encoding="UTF-8"?><a/>`, `<a><?xml version="1.0"?></a>`, `<a>&unknown;</a>`, `<a>&#0;</a>`, `<a>]]></a>`,
	} {
		t.Run(s, func(t *testing.T) { reject(t, s, ErrXML) })
	}
}
func TestUnsupportedFeaturesAndIdentity(t *testing.T) {
	for _, s := range []string{`<!DOCTYPE a SYSTEM "https://example.invalid/remote.dtd"><a/>`, `<!DOCTYPE a [<!ENTITY ex SYSTEM "file:///secret">]><a>&ex;</a>`, `<?xml version="1.0" encoding="ISO-8859-1"?><a/>`, string([]byte{255, 254, '<', 0})} {
		reject(t, s, ErrUnsupported)
	}
	b := []byte(`<a/>`)
	if d, err := Parse(context.Background(), b, strings.Repeat("0", 64), DefaultLimits()); d != nil || !errors.Is(err, ErrIdentity) {
		t.Fatal("identity", err)
	}
}
func TestResourceLimitsAndCancellation(t *testing.T) {
	s := []byte(`<a b="value"><b>text</b></a>`)
	for _, change := range []func(*Limits){func(l *Limits) { l.Bytes = 1 }, func(l *Limits) { l.Tokens = 1 }, func(l *Limits) { l.Elements = 1 }, func(l *Limits) { l.Depth = 1 }, func(l *Limits) { l.RetainedBytes = 1 }, func(l *Limits) { l.Attributes = 0 }} {
		l := DefaultLimits()
		change(&l)
		d, err := Parse(context.Background(), s, evidence.Hash(s), l)
		if d != nil || !errors.Is(err, ErrLimit) {
			t.Fatal("limit", err)
		}
	}
	l := DefaultLimits()
	l.Attributes = 1
	b := []byte(`<a x="1" y="2"/>`)
	if d, err := Parse(context.Background(), b, evidence.Hash(b), l); d != nil || !errors.Is(err, ErrLimit) {
		t.Fatal("attribute bound", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if d, err := Parse(ctx, s, evidence.Hash(s), DefaultLimits()); d != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
}
func TestDeterministicOwnedEvidence(t *testing.T) {
	b := []byte(`<a x="value">content<!-- comment --></a>`)
	before := bytes.Clone(b)
	d, err := Parse(context.Background(), b, evidence.Hash(b), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(context.Background(), b, evidence.Hash(b), DefaultLimits())
	if err != nil || !reflect.DeepEqual(d, again) {
		t.Fatal("nondeterministic", err)
	}
	d.Elements[0].Attributes[0].Value = "changed"
	d.Tokens[1].Value = "changed"
	if !bytes.Equal(b, before) || again.Elements[0].Attributes[0].Value != "value" || again.Tokens[1].Value != "content" {
		t.Fatal("shared mutation")
	}
}
func TestOfficePartLocations(t *testing.T) {
	for _, name := range []string{"minimal.docx", "hidden.docx", "minimal.odt", "hidden.odt"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "packageparts", "testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := packageparts.Read(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			hiddenTokens := 0
			hiddenElement := false
			for _, part := range pkg.Parts {
				if !strings.HasSuffix(part.Name, ".xml") && !strings.HasSuffix(part.Name, ".rels") {
					continue
				}
				d, err := Parse(context.Background(), part.Bytes, part.SHA256, DefaultLimits())
				if err != nil {
					t.Fatal(part.Name, err)
				}
				invariants(t, part.Bytes, d)
				for _, e := range d.Elements {
					if e.Name.Namespace == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" && e.Name.Local == "vanish" || e.Name.Namespace == "urn:oasis:names:tc:opendocument:xmlns:text:1.0" && e.Name.Local == "hidden-text" {
						hiddenElement = true
					}
				}
				for _, tok := range d.Tokens {
					if tok.Kind == "text" && tok.Value == strings.Repeat("\u200b\u200c", 32) {
						hiddenTokens++
						if tok.Mapping != "exact_utf8" || tok.Span.End-tok.Span.Start != 192 {
							t.Fatal("hidden sequence not located exactly")
						}
					}
				}
			}
			if strings.HasPrefix(name, "hidden") {
				if hiddenTokens != 1 || !hiddenElement {
					t.Fatal("hidden fixture evidence missing")
				}
			} else if hiddenTokens != 0 || hiddenElement {
				t.Fatal("invented hidden fixture evidence")
			}
		})
	}
}
func FuzzXMLPart(f *testing.F) {
	for _, s := range []string{`<r/>`, `<r xmlns="u">A&#x200B;😀</r>`, `<?xml version="1.0"?><a><![CDATA[x]]></a>`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			t.Skip()
		}
		l := DefaultLimits()
		l.Bytes = 1 << 16
		l.Tokens = 1024
		l.Elements = 512
		l.Depth = 32
		l.RetainedBytes = 1 << 18
		d, err := Parse(context.Background(), b, evidence.Hash(b), l)
		if err != nil {
			if d != nil {
				t.Fatal("partial result")
			}
			return
		}
		invariants(t, b, d)
	})
}

func TestLiteralAndReferencedAttributeWhitespace(t *testing.T) {
	s := "<a literal=\"a\t\r\nb\" referenced=\"a&#x9;&#xD;&#xA;b\"/>"
	d, err := parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Elements[0].Attributes[0].Value != "a  b" || d.Elements[0].Attributes[1].Value != "a\t\r\nb" {
		t.Fatal("conflated literal and referenced whitespace", d.Elements[0].Attributes)
	}
	invariants(t, []byte(s), d)
}
