// Package wordtext records WordprocessingML text and direct context facts.
// It is not a layout engine, style resolver or watermark classifier.
package wordtext

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "word-text/1"
const maxContextLinks = 200000

var ErrStructure = errors.New("unsupported Word story structure")

type Toggle struct {
	State    string
	Elements []int
}
type Text struct {
	AnalysisContext                  AnalysisContext
	Segment, Element, Run, Paragraph int
	Role                             string
	DirectVanish, DirectWebHidden    Toggle
	Revisions, UnresolvedAncestors   []int
	TextBox                          bool
	SpaceValue                       string
	SpaceDeclared                    bool
}
type Control struct {
	Element, Run, Paragraph int
	Kind                    string
	Span                    xmlparts.Span
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
	Parser, State, Story, Namespace string
	XML                             *xmlparts.MappedDocument
	Texts                           []Text
	Controls                        []Control
	Unselected                      []Unselected
	Issues                          []Issue
	Limitations                     []string
}
type runInfo struct{ vanish, web Toggle }

func (r *Result) issue(code string, element int) {
	r.State = "partial"
	r.Issues = append(r.Issues, Issue{code, element})
}
func role(local string) string {
	switch local {
	case "t":
		return "text"
	case "delText":
		return "deleted_text"
	case "instrText":
		return "field_instruction"
	case "delInstrText":
		return "deleted_field_instruction"
	}
	return ""
}

// Extract handles one verified XML story part. The host must separately bind it
// to a package and relationship; a namespace/root alone does not identify DOCX.
func Extract(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	mapped, err := xmlparts.ParseWithTextMaps(ctx, source, expectedSHA256, xmlparts.DefaultLimits(), xmlparts.MaxScalarMappings)
	if err != nil {
		return nil, err
	}
	d := mapped.Document
	root := d.Elements[0]
	ns := root.Name.Namespace
	if ns != docxidentify.TransitionalWord && ns != docxidentify.StrictWord {
		return nil, ErrStructure
	}
	story := ""
	scope := 0
	switch root.Name.Local {
	case "document":
		story = "main"
		count := 0
		for i, e := range d.Elements {
			if e.Parent == 0 && e.Name.Namespace == ns && e.Name.Local == "body" {
				scope = i
				count++
			}
		}
		if count != 1 {
			return nil, ErrStructure
		}
	case "hdr":
		story = "header"
	case "ftr":
		story = "footer"
	case "comments":
		story = "comments"
	case "footnotes":
		story = "footnotes"
	case "endnotes":
		story = "endnotes"
	default:
		return nil, ErrStructure
	}
	r := &Result{Parser: Version, State: "completed", Story: story, Namespace: ns, XML: mapped, Texts: []Text{}, Controls: []Control{}, Unselected: []Unselected{}, Issues: []Issue{}, Limitations: []string{"word.style_inheritance_unresolved", "word.rendering_not_performed", "word.property_revision_effects_unresolved", "word.character_data_only", "word.cross_run_assembly_not_performed"}}
	contexts := analysisContexts(d, ns, scope)
	children := make([][]int, len(d.Elements))
	for i, e := range d.Elements {
		if e.Parent >= 0 {
			children[e.Parent] = append(children[e.Parent], i)
		}
	}
	nonempty := map[int]bool{}
	for _, s := range mapped.Segments {
		if strings.Trim(s.Text, " \t\r\n") != "" {
			nonempty[s.Element] = true
		}
	}
	runs := map[int]runInfo{}
	links := 0
	// Context traversal is bounded by XP-001 depth. Only references needed to
	// explain revision/foreign-wrapper context are retained, under a shared budget.
	contextFor := func(element int) (int, []int, []int, bool, bool) {
		paragraph := -1
		revisions, unknown := []int{}, []int{}
		textbox, inScope := false, false
		for p := d.Elements[element].Parent; p >= 0; p = d.Elements[p].Parent {
			e := d.Elements[p]
			if p == scope {
				inScope = true
			}
			if e.Name.Namespace != ns {
				unknown = append(unknown, p)
				continue
			}
			switch e.Name.Local {
			case "p":
				if paragraph < 0 {
					paragraph = p
				}
			case "ins", "del", "moveFrom", "moveTo":
				revisions = append(revisions, p)
			case "txbxContent":
				textbox = true
			}
		}
		return paragraph, revisions, unknown, textbox, inScope
	}
	for i, s := range mapped.Segments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reason := "syntax_whitespace"
		if strings.Trim(s.Text, " \t\r\n") != "" {
			reason = "unclassified_character_data"
		}
		if s.Element < 0 {
			r.Unselected = append(r.Unselected, Unselected{i, reason})
			continue
		}
		e := d.Elements[s.Element]
		kind := role(e.Name.Local)
		if e.Name.Namespace != ns || kind == "" {
			r.Unselected = append(r.Unselected, Unselected{i, reason})
			if reason != "syntax_whitespace" {
				r.issue("word.unclassified_character_data", s.Element)
			}
			continue
		}
		run := e.Parent
		paragraph, revisions, unknown, textbox, inScope := contextFor(s.Element)
		if run < 0 || d.Elements[run].Name.Namespace != ns || d.Elements[run].Name.Local != "r" || paragraph < 0 || !inScope || len(children[s.Element]) != 0 {
			r.Unselected = append(r.Unselected, Unselected{i, "unsupported_text_structure"})
			r.issue("word.text_structure_unsupported", s.Element)
			continue
		}
		info, ok := runs[run]
		if !ok {
			properties := []int{}
			for _, child := range children[run] {
				e := d.Elements[child]
				if e.Name.Namespace == ns && e.Name.Local == "rPr" {
					properties = append(properties, child)
				}
			}
			info = runInfo{readToggle(d, children, nonempty, properties, ns, "vanish"), readToggle(d, children, nonempty, properties, ns, "webHidden")}
			runs[run] = info
			if info.vanish.State == "unknown" || info.web.State == "unknown" {
				r.issue("word.direct_visibility_unknown", run)
			}
		}
		links += len(revisions) + len(unknown) + len(info.vanish.Elements) + len(info.web.Elements)
		if links > maxContextLinks {
			return nil, xmlparts.ErrLimit
		}
		info.vanish.Elements = slices.Clone(info.vanish.Elements)
		info.web.Elements = slices.Clone(info.web.Elements)
		item := Text{AnalysisContext: contexts[s.Element], Segment: i, Element: s.Element, Run: run, Paragraph: paragraph, Role: kind, DirectVanish: info.vanish, DirectWebHidden: info.web, Revisions: revisions, UnresolvedAncestors: unknown, TextBox: textbox}
		for _, a := range e.Attributes {
			if a.Name.Namespace == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" {
				item.SpaceDeclared = true
				item.SpaceValue = a.Value
				if a.Value != "default" && a.Value != "preserve" {
					r.issue("word.xml_space_unknown", s.Element)
				}
			}
		}
		if len(unknown) > 0 {
			r.issue("word.wrapper_context_unresolved", s.Element)
		}
		if item.AnalysisContext.BlockedElement >= 0 {
			r.issue("word.text_context_not_analyzed", s.Element)
		}
		r.Texts = append(r.Texts, item)
	}
	for i, e := range d.Elements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.Name.Namespace != ns {
			continue
		}
		kind := ""
		switch e.Name.Local {
		case "tab":
			kind = "tab"
		case "br":
			kind = "break"
		case "cr":
			kind = "carriage_return"
		}
		if kind == "" {
			continue
		}
		// A tab-stop declaration is formatting, not a run control.
		if e.Name.Local == "tab" && e.Parent >= 0 && d.Elements[e.Parent].Name.Namespace == ns && d.Elements[e.Parent].Name.Local == "tabs" {
			continue
		}
		p, _, unknown, _, inside := contextFor(i)
		run := e.Parent
		if contexts[i].BlockedElement >= 0 || run < 0 || d.Elements[run].Name.Namespace != ns || d.Elements[run].Name.Local != "r" || p < 0 || !inside || len(children[i]) != 0 || nonempty[i] {
			r.issue("word.control_structure_unsupported", i)
			continue
		}
		if len(unknown) > 0 {
			r.issue("word.wrapper_context_unresolved", i)
		}
		r.Controls = append(r.Controls, Control{i, run, p, kind, e.Full})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
func readToggle(d *xmlparts.Document, children [][]int, nonempty map[int]bool, properties []int, ns, name string) Toggle {
	result := Toggle{State: "unspecified", Elements: []int{}}
	if len(properties) > 1 {
		return Toggle{State: "unknown", Elements: properties}
	}
	if len(properties) == 0 {
		return result
	}
	for _, i := range children[properties[0]] {
		e := d.Elements[i]
		if e.Name.Namespace == ns && e.Name.Local == name {
			result.Elements = append(result.Elements, i)
		}
	}
	if len(result.Elements) == 0 {
		return result
	}
	if len(result.Elements) > 1 {
		result.State = "unknown"
		return result
	}
	i := result.Elements[0]
	e := d.Elements[i]
	value := "true"
	if len(children[i]) != 0 || nonempty[i] {
		result.State = "unknown"
		return result
	}
	for _, a := range e.Attributes {
		if a.NamespaceDeclaration {
			continue
		}
		if a.Name.Namespace != ns || a.Name.Local != "val" {
			result.State = "unknown"
			return result
		}
		value = a.Value
	}
	switch value {
	case "true", "1":
		result.State = "on"
	case "false", "0":
		result.State = "off"
	case "on", "off":
		if ns == docxidentify.TransitionalWord {
			result.State = value
		} else {
			result.State = "unknown"
		}
	default:
		result.State = "unknown"
	}
	return result
}
