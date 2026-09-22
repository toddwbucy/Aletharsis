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

func (e *UnsupportedRootError) Error() string { return e.Code }
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
	Code               string
	Element            int
	Segment, Attribute int // -1 when this issue does not select one.
	Span               xmlparts.Span
}
type Result struct {
	Parser, State, Kind string
	XML                 *xmlparts.MappedDocument
	Properties          []Property
	Issues              []Issue
	Limitations         []string
}

func key(kind string, name xmlparts.Name) string {
	if kind == "core" {
		switch name.Namespace {
		case dcNamespace:
			if name.Local == "creator" {
				return "creator"
			}
			if name.Local == "identifier" {
				return "document_id"
			}
		case termsNamespace:
			if name.Local == "created" || name.Local == "modified" {
				return name.Local
			}
		case CoreNamespace:
			if name.Local == "lastModifiedBy" {
				return "last_modified_by"
			}
			if name.Local == "revision" {
				return "revision"
			}
		}
	}
	if kind == "app" && name.Namespace == AppNamespace {
		switch name.Local {
		case "Application":
			return "application"
		case "AppVersion":
			return "application_version"
		case "Company":
			return "company"
		case "Template":
			return "template"
		}
	}
	return ""
}

// This is an explicit vocabulary subset, not a trust or expectedness profile.
func standardUnselected(kind string, name xmlparts.Name) bool {
	if kind == "core" {
		if name.Namespace == dcNamespace {
			switch name.Local {
			case "title", "subject", "description", "language":
				return true
			}
		}
		if name.Namespace == CoreNamespace {
			switch name.Local {
			case "keywords", "category", "contentStatus", "lastPrinted", "version":
				return true
			}
		}
	}
	if kind == "app" && name.Namespace == AppNamespace {
		switch name.Local {
		case "TotalTime", "Pages", "Words", "Characters", "DocSecurity", "Lines", "Paragraphs", "ScaleCrop", "HeadingPairs", "TitlesOfParts", "Manager", "LinksUpToDate", "CharactersWithSpaces", "SharedDoc", "HyperlinkBase", "HLinks", "HyperlinksChanged", "DigSig", "PresentationFormat", "Slides", "Notes", "HiddenSlides", "MMClips":
			return true
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
	r := &Result{Parser: Version, State: "completed", Kind: kind, XML: mapped, Properties: []Property{}, Issues: []Issue{}, Limitations: []string{"metadata.values_not_validated", "metadata.identity_not_verified", "metadata.attribute_semantics_unresolved", "metadata.selected_properties_only", "metadata.package_binding_not_verified"}}
	issue := func(code string, element, segment, attribute int, span xmlparts.Span) {
		r.State = "partial"
		r.Issues = append(r.Issues, Issue{Code: code, Element: element, Segment: segment, Attribute: attribute, Span: span})
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
		normalized := key(kind, e.Name)
		if normalized == "" {
			code := "metadata.property_unassessed"
			if standardUnselected(kind, e.Name) {
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
				issue("metadata.attribute_semantics_unassessed", i, -1, ai, e.Start)
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
