// Package docxidentify identifies a Word document main part from package evidence.
// It does not render content, evaluate profiles, or claim Office conformance.
package docxidentify

import (
	"context"
	"errors"
	"mime"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "docx-identification/1"
const TypesNamespace = "http://schemas.openxmlformats.org/package/2006/content-types"
const MainContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
const TransitionalRelationship = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
const StrictRelationship = "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument"
const TransitionalWord = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
const StrictWord = "http://purl.oclc.org/ooxml/wordprocessingml/main"
const maxDeclarations = 4096

type Declaration struct {
	Namespace, Kind, Key, ContentType, State, Code string
	Anchor                                         opcrels.Anchor
}
type Assignment struct {
	// Type assignment and payload admission are independent facts.
	AdmissionState, AdmissionCode              string
	Part, PartSHA256, ContentType, State, Code string
	Declaration                                int
}
type Issue struct {
	Code, Part string
	Element    int
}
type Result struct {
	Parser, State, Format, Variant   string
	OPC                              *opcrels.Result
	TypesPart, TypesSHA256           string
	TypesXML                         *xmlparts.Document
	Declarations                     []Declaration
	Assignments                      []Assignment
	Issues                           []Issue
	MainPart, MainSHA256             string
	MainRelationship, MainAssignment int
	MainXML                          *xmlparts.Document
}

func (r *Result) issue(code, part string, element int) {
	r.State = "partial"
	r.Issues = append(r.Issues, Issue{code, part, element})
}
func parseCode(err error) string {
	switch {
	case errors.Is(err, xmlparts.ErrLimit):
		return "xml.resource_limit"
	case errors.Is(err, xmlparts.ErrUnsupported):
		return "xml.unsupported"
	default:
		return "xml.invalid"
	}
}
func find(parts []packageparts.Outcome, name string) []int {
	result := []int{}
	for i, p := range parts {
		if !p.Directory && opcrels.FoldName(p.Name) == opcrels.FoldName(name) {
			result = append(result, i)
		}
	}
	return result
}

// Inspect uses package declarations and namespace-qualified XML, never a filename
// extension or assumed word/document.xml path. Acquisition errors return nil;
// incomplete identification retains the package and explicit issues.
func Inspect(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	opc, err := opcrels.Inspect(ctx, source, expectedSHA256)
	if err != nil {
		return nil, err
	}
	return inspectOPC(ctx, opc)
}

// InspectVerified reuses the coordinator's single OPC inventory. It never reads
// or decompresses the container. The caller must not mutate OPC during inspection.
func InspectVerified(ctx context.Context, opc *opcrels.Result) (*Result, error) {
	if ctx == nil || opc == nil || opc.Outcomes == nil || opc.Outcomes.SourceSHA256 == "" {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return inspectOPC(ctx, opc)
}

func inspectOPC(ctx context.Context, opc *opcrels.Result) (*Result, error) {
	r := &Result{Parser: Version, State: "not_applicable", OPC: opc, Declarations: []Declaration{}, Assignments: []Assignment{}, Issues: []Issue{}, MainRelationship: -1, MainAssignment: -1}
	parts := opc.Outcomes.Parts
	matches := find(parts, "[Content_Types].xml")
	if len(matches) == 0 {
		if opc.State != "not_applicable" {
			r.issue("opc.content_types_missing", "", -1)
		}
		return r, nil
	}
	r.State = "completed"
	if len(matches) != 1 {
		r.issue("opc.content_types_ambiguous", "", -1)
		return r, nil
	}
	p := parts[matches[0]]
	r.TypesPart, r.TypesSHA256 = p.Name, p.SHA256
	if p.State != "completed" {
		r.issue("opc.content_types_unavailable", p.Name, -1)
		if p.Code != "" {
			r.issue(p.Code, p.Name, -1)
		}
		return r, nil
	}
	if p.Name != "[Content_Types].xml" {
		r.issue("opc.content_types_name_unsupported", p.Name, -1)
		return r, nil
	}
	d, err := xmlparts.Parse(ctx, p.Bytes, p.SHA256, xmlparts.DefaultLimits())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r.issue(parseCode(err), p.Name, -1)
		return r, nil
	}
	r.TypesXML = d
	if !r.readTypes() {
		return r, nil
	}
	r.assign(parts)
	rootOK := false
	for _, part := range opc.Parts {
		if opcrels.FoldName(part.Part) == "_rels/.rels" {
			rootOK = part.State == "completed"
		}
	}
	rootTypeOK := false
	for _, a := range r.Assignments {
		if opcrels.FoldName(a.Part) == "_rels/.rels" && a.State == "assigned" && a.ContentType == "application/vnd.openxmlformats-package.relationships+xml" {
			rootTypeOK = true
		}
	}
	if !rootTypeOK {
		r.issue("docx.root_relationship_type_unknown", "_rels/.rels", -1)
		return r, nil
	}
	if !rootOK {
		// Non-deferral admission codes were already emitted by assign().
		for _, p := range parts {
			if opcrels.FoldName(p.Name) == "_rels/.rels" && p.Code == "office.part_not_admitted" {
				r.issue(p.Code, p.Name, -1)
			}
		}
		r.issue("docx.root_relationships_incomplete", "_rels/.rels", -1)
		return r, nil
	}
	candidates := []int{}
	for i, rel := range opc.Relationships {
		if opcrels.FoldName(rel.Anchor.Part) == "_rels/.rels" && (rel.Type == TransitionalRelationship || rel.Type == StrictRelationship) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) != 1 {
		if len(candidates) == 0 {
			r.issue("docx.main_relationship_missing", "_rels/.rels", -1)
		} else {
			r.issue("docx.main_relationship_ambiguous", "_rels/.rels", -1)
		}
		return r, nil
	}
	i := candidates[0]
	rel := opc.Relationships[i]
	r.MainRelationship = i
	if rel.State != "resolved" {
		r.issue("docx.main_relationship_unresolved", rel.Anchor.Part, rel.Anchor.Element)
		return r, nil
	}
	for j, a := range r.Assignments {
		if a.Part == rel.ResolvedPart {
			r.MainAssignment = j
			break
		}
	}
	if r.MainAssignment < 0 || r.Assignments[r.MainAssignment].State != "assigned" {
		r.issue("docx.main_type_unknown", rel.ResolvedPart, -1)
		return r, nil
	}
	a := r.Assignments[r.MainAssignment]
	if a.ContentType != MainContentType {
		r.issue("docx.main_type_unsupported", a.Part, -1)
		return r, nil
	}
	r.MainPart, r.MainSHA256 = a.Part, a.PartSHA256
	matches = find(parts, a.Part)
	if len(matches) != 1 {
		r.issue("docx.main_part_ambiguous", a.Part, -1)
		return r, nil
	}
	main := parts[matches[0]]
	if main.State != "completed" {
		r.issue("docx.main_part_unavailable", main.Name, -1)
		// Non-deferral admission codes were already emitted by assign().
		if main.Code == "office.part_not_admitted" {
			r.issue(main.Code, main.Name, -1)
		}
		return r, nil
	}
	doc, err := xmlparts.Parse(ctx, main.Bytes, main.SHA256, xmlparts.DefaultLimits())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r.issue(parseCode(err), main.Name, -1)
		return r, nil
	}
	r.MainXML = doc
	wantNS := TransitionalWord
	variant := "transitional"
	if rel.Type == StrictRelationship {
		wantNS = StrictWord
		variant = "strict"
	}
	if len(doc.Elements) == 0 || doc.Elements[0].Name.Namespace != wantNS || doc.Elements[0].Name.Local != "document" {
		r.issue("docx.main_root_mismatch", main.Name, 0)
		return r, nil
	}
	bodies := 0
	for _, e := range doc.Elements {
		if e.Parent == 0 && e.Name.Namespace == wantNS && e.Name.Local == "body" {
			bodies++
		}
	}
	if bodies != 1 {
		r.issue("docx.main_body_ambiguous", main.Name, 0)
		return r, nil
	}
	r.Format, r.Variant = "docx", variant
	if opc.State == "partial" {
		r.issue("opc.relationship_inventory_partial", "", -1)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Result) readTypes() bool {
	d := r.TypesXML
	if len(d.Elements) == 0 || d.Elements[0].Name.Namespace != TypesNamespace || d.Elements[0].Name.Local != "Types" {
		r.issue("opc.content_types_root_invalid", r.TypesPart, 0)
		return false
	}
	bad := map[int]bool{}
	for _, e := range d.Elements {
		if e.Parent > 0 {
			bad[e.Parent] = true
		}
	}
	for _, t := range d.Tokens {
		if t.Kind == "text" && strings.Trim(t.Value, " \t\r\n") != "" {
			bad[t.Element] = true
		}
	}
	for _, a := range d.Elements[0].Attributes {
		if !a.NamespaceDeclaration {
			bad[0] = true
		}
	}
	if bad[0] {
		r.issue("opc.content_types_structure_unknown", r.TypesPart, 0)
		return false
	}
	keys := map[string][]int{}
	for i, e := range d.Elements {
		if e.Parent != 0 {
			continue
		}
		if len(r.Declarations) >= maxDeclarations {
			r.issue("opc.content_types_limit", r.TypesPart, i)
			return false
		}
		dec := Declaration{Namespace: e.Name.Namespace, Kind: e.Name.Local, State: "accepted", Anchor: opcrels.Anchor{Part: r.TypesPart, PartSHA256: r.TypesSHA256, Element: i, Span: e.Full}}
		attrs := map[string]string{}
		unknown := false
		for _, a := range e.Attributes {
			if a.NamespaceDeclaration {
				continue
			}
			if a.Name.Namespace != "" {
				unknown = true
				continue
			}
			attrs[a.Name.Local] = a.Value
		}
		if e.Name.Namespace != TypesNamespace || (dec.Kind != "Default" && dec.Kind != "Override") {
			unknown = true
		}
		keyAttr := "Extension"
		if dec.Kind == "Override" {
			keyAttr = "PartName"
		}
		for name := range attrs {
			if name != keyAttr && name != "ContentType" {
				unknown = true
			}
		}
		dec.Key, dec.ContentType = attrs[keyAttr], attrs["ContentType"]
		switch {
		case unknown || bad[i]:
			dec.State, dec.Code = "unsupported", "opc.content_types_structure_unknown"
		case dec.Key == "" || dec.ContentType == "":
			dec.State, dec.Code = "invalid", "opc.content_type_required_attribute"
		default:
			typ, params, err := mime.ParseMediaType(dec.ContentType)
			if err != nil || !strings.Contains(typ, "/") || len(params) != 0 {
				dec.State, dec.Code = "unsupported", "opc.media_type_unsupported"
			} else {
				dec.ContentType = typ
			}
			if dec.Kind == "Default" {
				for _, c := range dec.Key {
					if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
						if dec.Code == "" {
							dec.State, dec.Code = "unsupported", "opc.extension_unsupported"
						} else {
							r.issue("opc.extension_unsupported", r.TypesPart, i)
						}
						break
					}
				}
			} else {
				target, code := opcrels.ResolveInternalTarget("", dec.Key)
				if !strings.HasPrefix(dec.Key, "/") || code != "" || target != strings.TrimPrefix(dec.Key, "/") {
					if dec.Code == "" {
						dec.State, dec.Code = "unsupported", "opc.override_name_unsupported"
					} else {
						r.issue("opc.override_name_unsupported", r.TypesPart, i)
					}
				}
			}
		}
		if dec.State != "accepted" {
			r.issue(dec.Code, r.TypesPart, i)
		}
		key := declarationKey(dec)
		if dec.typeDeclaration() {
			keys[key] = append(keys[key], len(r.Declarations))
		}
		r.Declarations = append(r.Declarations, dec)
	}
	// Iterate declaration order for deterministic issue order, not map order.
	for i := range r.Declarations {
		dec := &r.Declarations[i]
		if dec.typeDeclaration() && dec.Key != "" && len(keys[declarationKey(*dec)]) > 1 {
			dec.State, dec.Code = "ambiguous", "opc.content_type_ambiguous"
			r.issue(dec.Code, r.TypesPart, dec.Anchor.Element)
		}
	}
	return true
}

// Only expanded OPC element names can claim type lookup keys.
func (d Declaration) typeDeclaration() bool {
	return d.Namespace == TypesNamespace && (d.Kind == "Default" || d.Kind == "Override")
}

// Use the same lookup equivalence for duplicate detection and assignment,
// including malformed overrides that omit the required leading slash.
func declarationKey(d Declaration) string {
	key := d.Key
	if d.Kind == "Override" {
		key = strings.TrimPrefix(key, "/")
	}
	return d.Kind + ":" + opcrels.FoldName(key)
}
func (r *Result) assign(parts []packageparts.Outcome) {
	defaults, overrides := map[string]int{}, map[string]int{}
	for i, d := range r.Declarations {
		if !d.typeDeclaration() {
			continue
		}
		if d.Kind == "Default" {
			defaults[opcrels.FoldName(d.Key)] = i
		} else if d.Kind == "Override" {
			overrides[opcrels.FoldName(strings.TrimPrefix(d.Key, "/"))] = i
		}
	}
	counts := map[string]int{}
	for _, p := range parts {
		if !p.Directory {
			counts[opcrels.FoldName(p.Name)]++
		}
	}
	for _, p := range parts {
		if p.Directory || p.Name == r.TypesPart {
			continue
		}
		a := Assignment{Part: p.Name, PartSHA256: p.SHA256, AdmissionState: p.State, AdmissionCode: p.Code, State: "unknown", Declaration: -1}
		if counts[opcrels.FoldName(p.Name)] > 1 {
			a.Code = "opc.part_name_ambiguous"
		} else {
			if i, ok := overrides[opcrels.FoldName(p.Name)]; ok {
				a.Declaration = i
			} else {
				base := p.Name
				if n := strings.LastIndexByte(base, '/'); n >= 0 {
					base = base[n+1:]
				}
				if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
					if i, ok := defaults[opcrels.FoldName(base[dot+1:])]; ok {
						a.Declaration = i
					}
				}
			}
			if a.Declaration >= 0 {
				d := r.Declarations[a.Declaration]
				if d.State == "accepted" {
					a.State = "assigned"
					a.ContentType = d.ContentType
				} else {
					a.Code = d.Code
				}
			} else {
				a.Code = "opc.part_type_missing"
			}
		}
		if a.Code != "" {
			r.issue(a.Code, p.Name, -1)
		}
		if p.State != "completed" && p.Code != "" && p.Code != "office.part_not_admitted" {
			r.issue(p.Code, p.Name, -1)
		}
		r.Assignments = append(r.Assignments, a)
	}
	// Overrides for absent targets remain observable package inconsistencies.
	for _, d := range r.Declarations {
		if d.typeDeclaration() && d.Kind == "Override" && d.State == "accepted" && counts[opcrels.FoldName(strings.TrimPrefix(d.Key, "/"))] == 0 {
			r.issue("opc.override_target_missing", r.TypesPart, d.Anchor.Element)
		}
	}
}
