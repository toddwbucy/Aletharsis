// Package officemetadata preserves namespace-qualified core and app properties.
// The caller binds the part to package declarations; this is not DOCX identification.
package officemetadata

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "office-metadata/1"
const CoreNamespace = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
const AppNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
const dcNamespace = "http://purl.org/dc/elements/1.1/"
const termsNamespace = "http://purl.org/dc/terms/"

var ErrStructure = errors.New("unsupported Office metadata root")

// UnsupportedRootError distinguishes recognized but unimplemented metadata forms.
type UnsupportedRootError struct{ Code string }

func (e *UnsupportedRootError) Error() string { return e.Code + ": " + ErrStructure.Error() }
func (e *UnsupportedRootError) Unwrap() error { return ErrStructure }

type Property struct {
	Element    int
	Name       xmlparts.Name
	Key, Value string
	// Occurrence is zero-based among properties with the same expanded name.
	Occurrence int
	// Segments index XML.Segments; their scalar maps retain exact lexical bytes.
	Segments []int
	// Attributes index non-declaration attributes on the property element. Their
	// semantics (including xsi:nil/type) are unresolved; Value is decoded text only.
	Attributes []int
}
type Issue struct {
	Code                  string
	Element               int
	Segment, Attribute    int // -1 when this issue does not select one.
	Token                 int // XML token index, or -1 for element/attribute-only issues.
	StandardProjectionGap bool
	Span                  xmlparts.Span
}

// Coverage counts declared projection gaps separately from other unassessed
// markup. Neither count is a safety/trust verdict or full schema validation.
type Coverage struct{ StandardProjectionGaps, OtherGaps int }

type Result struct {
	Parser, State, Kind string
	XML                 *xmlparts.MappedDocument
	Properties          []Property
	Issues              []Issue
	Limitations         []string
	Coverage            Coverage
	RootAttributes      []int
}

// Empty values name recognized but unselected properties; absent keys are unknown.
var vocabulary = map[[3]string]string{
	{"core", dcNamespace, "creator"}:              "creator",
	{"core", dcNamespace, "identifier"}:           "document_id",
	{"core", dcNamespace, "title"}:                "",
	{"core", dcNamespace, "subject"}:              "",
	{"core", dcNamespace, "description"}:          "",
	{"core", dcNamespace, "language"}:             "",
	{"core", termsNamespace, "created"}:           "created",
	{"core", termsNamespace, "modified"}:          "modified",
	{"core", CoreNamespace, "lastModifiedBy"}:     "last_modified_by",
	{"core", CoreNamespace, "revision"}:           "revision",
	{"core", CoreNamespace, "keywords"}:           "",
	{"core", CoreNamespace, "category"}:           "",
	{"core", CoreNamespace, "contentStatus"}:      "",
	{"core", CoreNamespace, "lastPrinted"}:        "",
	{"core", CoreNamespace, "version"}:            "",
	{"app", AppNamespace, "Application"}:          "application",
	{"app", AppNamespace, "AppVersion"}:           "application_version",
	{"app", AppNamespace, "Company"}:              "company",
	{"app", AppNamespace, "Template"}:             "template",
	{"app", AppNamespace, "TotalTime"}:            "",
	{"app", AppNamespace, "Pages"}:                "",
	{"app", AppNamespace, "Words"}:                "",
	{"app", AppNamespace, "Characters"}:           "",
	{"app", AppNamespace, "DocSecurity"}:          "",
	{"app", AppNamespace, "Lines"}:                "",
	{"app", AppNamespace, "Paragraphs"}:           "",
	{"app", AppNamespace, "ScaleCrop"}:            "",
	{"app", AppNamespace, "HeadingPairs"}:         "",
	{"app", AppNamespace, "TitlesOfParts"}:        "",
	{"app", AppNamespace, "Manager"}:              "",
	{"app", AppNamespace, "LinksUpToDate"}:        "",
	{"app", AppNamespace, "CharactersWithSpaces"}: "",
	{"app", AppNamespace, "SharedDoc"}:            "",
	{"app", AppNamespace, "HyperlinkBase"}:        "",
	{"app", AppNamespace, "HLinks"}:               "",
	{"app", AppNamespace, "HyperlinksChanged"}:    "",
	{"app", AppNamespace, "DigSig"}:               "",
	{"app", AppNamespace, "PresentationFormat"}:   "",
	{"app", AppNamespace, "Slides"}:               "",
	{"app", AppNamespace, "Notes"}:                "",
	{"app", AppNamespace, "HiddenSlides"}:         "",
	{"app", AppNamespace, "MMClips"}:              "",
}

// Recognize only the declared date-type QName, not the validity of its value.
func standardDateType(d *xmlparts.Document, element int, a xmlparts.Attribute) bool {
	name := d.Elements[element].Name
	if name.Namespace != termsNamespace || (name.Local != "created" && name.Local != "modified") || a.Name.Namespace != "http://www.w3.org/2001/XMLSchema-instance" || a.Name.Local != "type" {
		return false
	}
	prefix, local, qualified := strings.Cut(a.Value, ":")
	if !qualified {
		local = prefix
		prefix = ""
	}
	if local != "W3CDTF" {
		return false
	}
	for i := element; i >= 0; i = d.Elements[i].Parent {
		for _, decl := range d.Elements[i].Attributes {
			if !decl.NamespaceDeclaration {
				continue
			}
			if (prefix == "" && decl.Name.Prefix == "" && decl.Name.Local == "xmlns") || (prefix != "" && decl.Name.Prefix == "xmlns" && decl.Name.Local == prefix) {
				return decl.Value == termsNamespace
			}
		}
	}
	return false
}

func unsupportedRoot(name xmlparts.Name) string {
	if name.Namespace == "http://purl.oclc.org/ooxml/officeDocument/extendedProperties" && name.Local == "Properties" {
		return "metadata.strict_app_unsupported"
	}
	if (name.Namespace == "http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" || name.Namespace == "http://purl.oclc.org/ooxml/officeDocument/customProperties") && name.Local == "Properties" {
		return "metadata.custom_properties_unsupported"
	}
	if name.Namespace == "urn:oasis:names:tc:opendocument:xmlns:office:1.0" && name.Local == "document-meta" {
		return "metadata.odf_unsupported"
	}
	return ""
}

// Extract reads one immutable part under XP/XM bounds. All errors return nil;
// callers must retain a failed-part outcome and continue unrelated parts. Unknown
// or structured properties remain in XML, with located partial-coverage issues.
// Values are decoded observations, never validated timestamps or trusted identity.
func Extract(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	mapped, err := xmlparts.ParseWithTextMaps(ctx, source, expectedSHA256, xmlparts.DefaultLimits(), xmlparts.MaxScalarMappings)
	if err != nil {
		return nil, err
	}
	d := mapped.Document
	root := d.Elements[0].Name
	kind := ""
	if root.Namespace == CoreNamespace && root.Local == "coreProperties" {
		kind = "core"
	}
	if root.Namespace == AppNamespace && root.Local == "Properties" {
		kind = "app"
	}
	if kind == "" {
		if code := unsupportedRoot(root); code != "" {
			return nil, &UnsupportedRootError{Code: code}
		}
		return nil, ErrStructure
	}
	r := &Result{Parser: Version, State: "completed", Kind: kind, XML: mapped, Properties: []Property{}, Issues: []Issue{}, RootAttributes: []int{}, Limitations: []string{"metadata.values_not_validated", "metadata.identity_not_verified", "metadata.attribute_semantics_unresolved", "metadata.selected_properties_only", "metadata.package_binding_not_verified", "metadata.standard_subtrees_not_validated"}}
	issue := func(code string, element, segment, attribute int, span xmlparts.Span) {
		r.State = "partial"
		token := -1
		if segment >= 0 {
			token = mapped.Segments[segment].Token
		}
		standard := code == "metadata.standard_property_unassessed" || code == "metadata.standard_attribute_unassessed"
		if standard {
			r.Coverage.StandardProjectionGaps++
		} else {
			r.Coverage.OtherGaps++
		}
		r.Issues = append(r.Issues, Issue{Code: code, Element: element, Segment: segment, Attribute: attribute, Span: span, Token: token, StandardProjectionGap: standard})
	}
	for ai, a := range d.Elements[0].Attributes {
		if !a.NamespaceDeclaration {
			r.RootAttributes = append(r.RootAttributes, ai)
			issue("metadata.attribute_semantics_unassessed", 0, -1, ai, d.Elements[0].Start)
		}
	}
	for ti, token := range d.Tokens {
		if token.Element >= 0 && (token.Kind == "comment" || token.Kind == "processing_instruction") {
			issue("metadata.markup_unassessed", token.Element, -1, -1, token.Span)
			r.Issues[len(r.Issues)-1].Token = ti
		}
	}
	children := make([]bool, len(d.Elements))
	byElement := make([][]int, len(d.Elements))
	for _, e := range d.Elements {
		if e.Parent >= 0 {
			children[e.Parent] = true
		}
	}
	for i, segment := range mapped.Segments {
		if segment.Element >= 0 {
			byElement[segment.Element] = append(byElement[segment.Element], i)
		}
		if segment.Element == 0 && (segment.CDATA || len(bytes.Trim(source[segment.ContentSpan.Start:segment.ContentSpan.End], " \t\r\n")) != 0) {
			issue("metadata.root_text_unassessed", 0, i, -1, segment.TokenSpan)
		}
	}
	occurrences := map[[2]string]int{}
	for i, e := range d.Elements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.Parent != 0 {
			continue
		}
		name := [2]string{e.Name.Namespace, e.Name.Local}
		ordinal := occurrences[name]
		occurrences[name]++
		normalized, known := vocabulary[[3]string{kind, e.Name.Namespace, e.Name.Local}]
		if normalized == "" {
			code := "metadata.property_unassessed"
			if known {
				code = "metadata.standard_property_unassessed"
			}
			issue(code, i, -1, -1, e.Full)
			continue
		}
		if children[i] {
			issue("metadata.structured_value_unassessed", i, -1, -1, e.Full)
			continue
		}
		attributes := []int{}
		for ai, a := range e.Attributes {
			if !a.NamespaceDeclaration {
				attributes = append(attributes, ai)
				code := "metadata.attribute_semantics_unassessed"
				if standardDateType(d, i, a) {
					code = "metadata.standard_attribute_unassessed"
				}
				issue(code, i, -1, ai, e.Start)
			}
		}
		var value strings.Builder
		segments := append([]int{}, byElement[i]...)
		for _, si := range segments {
			value.WriteString(mapped.Segments[si].Text)
		}
		r.Properties = append(r.Properties, Property{Element: i, Name: e.Name, Key: normalized, Value: value.String(), Occurrence: ordinal, Segments: segments, Attributes: attributes})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(r.Issues, func(i, j int) bool { return r.Issues[i].Span.Start < r.Issues[j].Span.Start })
	return r, nil
}
