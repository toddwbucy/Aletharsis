// Package officemetadata preserves namespace-qualified core and app properties.
// The caller binds the part to package declarations; this is not DOCX identification.
package officemetadata

import (
	"context"
	"errors"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "office-metadata/1"
const CoreNamespace = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
const AppNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
const dcNamespace = "http://purl.org/dc/elements/1.1/"
const termsNamespace = "http://purl.org/dc/terms/"

var ErrStructure = errors.New("unsupported Office metadata root")

type Property struct {
	Element    int
	Name       xmlparts.Name
	Key, Value string
	// Occurrence is zero-based among properties with the same expanded name.
	Occurrence int
	// Segments index XML.Segments; their scalar maps retain exact lexical bytes.
	Segments []int
}
type Issue struct {
	Code    string
	Element int
}
type Result struct {
	Parser, State, Kind string
	XML                 *xmlparts.MappedDocument
	Properties          []Property
	Issues              []Issue
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
		return nil, ErrStructure
	}
	r := &Result{Parser: Version, State: "completed", Kind: kind, XML: mapped, Properties: []Property{}, Issues: []Issue{}}
	issue := func(code string, element int) { r.State = "partial"; r.Issues = append(r.Issues, Issue{code, element}) }
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
		if segment.Element == 0 && strings.Trim(segment.Text, " \t\r\n") != "" {
			issue("metadata.root_text_unassessed", 0)
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
			issue("metadata.property_unassessed", i)
			continue
		}
		if children[i] {
			issue("metadata.structured_value_unassessed", i)
			continue
		}
		var value strings.Builder
		segments := append([]int{}, byElement[i]...)
		for _, si := range segments {
			value.WriteString(mapped.Segments[si].Text)
		}
		r.Properties = append(r.Properties, Property{Element: i, Name: e.Name, Key: normalized, Value: value.String(), Occurrence: ordinal, Segments: segments})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
