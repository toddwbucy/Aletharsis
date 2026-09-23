// Package odttext preserves ODF character data and explicit text controls.
// It does not render documents, resolve styles or evaluate conditions/fields.
package odttext

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "odt-text/1"
const TextNS = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
const maxContextLinks = 200000

var ErrStructure = errors.New("unsupported ODF text structure")

type Text struct {
	Segment, Element, Paragraph int
	// Context includes self, then ancestors, ending at office:text.
	Context, HiddenDeclarations []int
}
type Control struct {
	Element, Paragraph int
	Kind, CountState   string
	Count              uint64
	Span               xmlparts.Span
}
type Unselected struct {
	Segment int
	Reason  string
}
type Issue struct {
	Code    string
	Element int
}
type Result struct {
	Parser, State, DocumentVersion string
	XML                            *xmlparts.MappedDocument
	Texts                          []Text
	Controls                       []Control
	// Declarations reference XML attributes verbatim, not evaluated visibility.
	Declarations []int
	Unselected   []Unselected
	Issues       []Issue
	Limitations  []string
}

func (r *Result) issue(code string, element int) {
	r.State = "partial"
	r.Issues = append(r.Issues, Issue{code, element})
}
func attr(e xmlparts.Element, ns, name string) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name.Namespace == ns && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}
func named(e xmlparts.Element, ns, name string) bool {
	return e.Name.Namespace == ns && e.Name.Local == name
}
func control(e xmlparts.Element) string {
	if e.Name.Namespace != TextNS {
		return ""
	}
	switch e.Name.Local {
	case "s":
		return "spaces"
	case "tab":
		return "tab"
	case "line-break":
		return "line_break"
	}
	return ""
}

// knownContext limits interpretation, not preservation: unfamiliar wrappers keep
// their text and anchors but produce a partial result. This is not an ODF grammar.
func knownContext(e xmlparts.Element) bool {
	if e.Name.Namespace == odtidentify.OfficeNS {
		return e.Name.Local == "text" || e.Name.Local == "annotation"
	}
	if e.Name.Namespace != TextNS {
		return false
	}
	switch e.Name.Local {
	case "p", "h", "span", "a", "section", "list", "list-item", "list-header",
		"hidden-text", "hidden-paragraph", "conditional-text", "ruby", "ruby-base", "ruby-text",
		"note", "note-body", "note-citation", "tracked-changes", "changed-region", "deletion", "insertion", "format-change":
		return true
	}
	return false
}

// Extract accepts one identity-checked content.xml. Package/manifest identity,
// encryption admission and source-part naming remain the caller's responsibility.
func Extract(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	mapped, err := xmlparts.ParseWithTextMaps(ctx, source, expectedSHA256, xmlparts.DefaultLimits(), xmlparts.MaxScalarMappings)
	if err != nil {
		return nil, err
	}
	return extractMapped(ctx, mapped)
}

// ExtractParsed reuses a parser-produced document from these exact source bytes.
// The caller must retain the document unchanged; MapParsed verifies byte identity.
func ExtractParsed(ctx context.Context, source []byte, doc *xmlparts.Document) (*Result, error) {
	mapped, err := xmlparts.MapParsed(ctx, source, doc, xmlparts.MaxScalarMappings)
	if err != nil {
		return nil, err
	}
	return extractMapped(ctx, mapped)
}

func extractMapped(ctx context.Context, mapped *xmlparts.MappedDocument) (*Result, error) {
	d := mapped.Document
	if !named(d.Elements[0], odtidentify.OfficeNS, "document-content") {
		return nil, ErrStructure
	}
	version, _ := attr(d.Elements[0], odtidentify.OfficeNS, "version")
	body, scope := -1, -1
	for i, e := range d.Elements {
		if e.Parent == 0 && named(e, odtidentify.OfficeNS, "body") {
			if body != -1 {
				return nil, ErrStructure
			}
			body = i
		}
	}
	if body < 0 {
		return nil, ErrStructure
	}
	for i, e := range d.Elements {
		if e.Parent != body {
			continue
		}
		if scope != -1 || !named(e, odtidentify.OfficeNS, "text") {
			return nil, ErrStructure
		}
		scope = i
	}
	if scope < 0 {
		return nil, ErrStructure
	}
	r := &Result{Parser: Version, State: "completed", DocumentVersion: version, XML: mapped, Texts: []Text{}, Controls: []Control{}, Declarations: []int{}, Unselected: []Unselected{}, Issues: []Issue{}, Limitations: []string{"odt.rendering_not_performed", "odt.styles_unresolved", "odt.conditions_and_fields_not_evaluated", "odt.whitespace_not_rendered", "odt.cross_segment_assembly_not_performed", "odt.change_ranges_unresolved"}}
	if version != "1.2" && version != "1.3" {
		r.issue("odt.content_version_unsupported", 0)
	}
	n := len(d.Elements)
	inside := make([]bool, n)
	paragraphs := make([]int, n)
	children := make([]int, n)
	paragraphHidden := map[int][]int{}
	for i, e := range d.Elements {
		paragraphs[i] = -1
		if e.Parent >= 0 {
			inside[i] = inside[e.Parent]
			paragraphs[i] = paragraphs[e.Parent]
			children[e.Parent]++
		}
		if i == scope {
			inside[i] = true
		}
		if inside[i] && (named(e, TextNS, "p") || named(e, TextNS, "h")) {
			paragraphs[i] = i
		}
		if inside[i] && e.Name.Namespace == TextNS {
			switch e.Name.Local {
			case "hidden-text", "hidden-paragraph", "conditional-text":
				r.Declarations = append(r.Declarations, i)
				if e.Name.Local == "hidden-paragraph" && paragraphs[i] >= 0 {
					paragraphHidden[paragraphs[i]] = append(paragraphHidden[paragraphs[i]], i)
				}
			}
		}
	}
	nonempty := make([]bool, n)
	for _, s := range mapped.Segments {
		if s.Element >= 0 && strings.Trim(s.Text, " \t\r\n") != "" {
			nonempty[s.Element] = true
		}
	}
	links := 0
	for i, s := range mapped.Segments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e := s.Element
		if e < 0 || !inside[e] || paragraphs[e] < 0 || control(d.Elements[e]) != "" {
			reason := "syntax_whitespace"
			if strings.Trim(s.Text, " \t\r\n") != "" {
				reason = "unclassified_character_data"
				r.issue("odt.unclassified_character_data", e)
			}
			r.Unselected = append(r.Unselected, Unselected{i, reason})
			continue
		}
		item := Text{Segment: i, Element: e, Paragraph: paragraphs[e], Context: []int{}, HiddenDeclarations: []int{}}
		unresolved := false
		for a := e; a >= scope; a = d.Elements[a].Parent {
			item.Context = append(item.Context, a)
			if !knownContext(d.Elements[a]) {
				unresolved = true
			}
			if named(d.Elements[a], TextNS, "hidden-text") || named(d.Elements[a], TextNS, "conditional-text") {
				item.HiddenDeclarations = append(item.HiddenDeclarations, a)
			}
			if a == scope {
				break
			}
		}
		item.HiddenDeclarations = append(item.HiddenDeclarations, paragraphHidden[item.Paragraph]...)
		links += len(item.Context) + len(item.HiddenDeclarations)
		if links > maxContextLinks {
			return nil, xmlparts.ErrLimit
		}
		if unresolved {
			r.issue("odt.context_unresolved", e)
		}
		r.Texts = append(r.Texts, item)
	}
	for i, e := range d.Elements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := control(e)
		if kind == "" || !inside[i] {
			continue
		}
		c := Control{Element: i, Paragraph: paragraphs[i], Kind: kind, CountState: "known", Count: 1, Span: e.Full}
		if c.Paragraph < 0 || children[i] != 0 || nonempty[i] {
			c.CountState = "unknown"
		}
		if kind == "spaces" {
			if value, ok := attr(e, TextNS, "c"); ok {
				count, valid := nonNegativeCount(value)
				if !valid {
					c.CountState = "unknown"
				} else {
					c.Count = count
				}
			}
		}
		for _, a := range e.Attributes {
			if a.NamespaceDeclaration {
				continue
			}
			// text:tab-ref is retained without interpreting tab-stop layout.
			if (kind == "spaces" && a.Name.Namespace == TextNS && a.Name.Local == "c") || (kind == "tab" && a.Name.Namespace == TextNS && a.Name.Local == "tab-ref") {
				continue
			}
			c.CountState = "unknown"
		}
		if c.CountState == "unknown" {
			c.Count = 0
			r.issue("odt.control_unresolved", i)
		}
		// Control ancestry remains recoverable from its retained XML element.
		for a := e.Parent; a >= scope; a = d.Elements[a].Parent {
			if !knownContext(d.Elements[a]) {
				r.issue("odt.context_unresolved", i)
				break
			}
			if a == scope {
				break
			}
		}
		r.Controls = append(r.Controls, c)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

// No expansion is performed. Overflows stay unknown rather than allocating from
// attacker-controlled repetition counts. XML Schema whitespace and '+' accepted.
func nonNegativeCount(value string) (uint64, bool) {
	s := strings.Trim(value, " \t\r\n")
	s = strings.TrimPrefix(s, "+")
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}
