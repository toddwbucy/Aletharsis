// Package opcrels inventories OPC relationship declarations without following them.
package opcrels

import (
	"context"
	"errors"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "opc-relationships/1"
const Namespace = "http://schemas.openxmlformats.org/package/2006/relationships"
const maxParts = 256
const maxXMLBytes = 8 << 20
const maxTokens = 100000
const maxValues = 16 << 20
const maxRelationships = 10000

type Anchor struct {
	Part, PartSHA256 string
	Element          int
	Span             xmlparts.Span
}
type Relationship struct {
	ID, Type, Target, TargetMode           string
	SourcePart, ResolvedPart, TargetSHA256 string
	State, Code                            string
	Anchor                                 Anchor
}
type PartResult struct {
	Part, PartSHA256, SourcePart, State, Code string
	XML                                       *xmlparts.Document
}
type Result struct {
	Parser, State string
	Package       *packageparts.Package
	Parts         []PartResult
	Relationships []Relationship
}

func fold(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)
}
func owner(name string) (string, bool) {
	if fold(name) == "_rels/.rels" {
		return "", true
	}
	split := strings.Split(name, "/")
	n := len(split)
	if n < 2 || fold(split[n-2]) != "_rels" || !strings.HasSuffix(fold(split[n-1]), ".rels") || len(split[n-1]) <= 5 {
		return "", false
	}
	return strings.Join(append(split[:n-2], split[n-1][:len(split[n-1])-5]), "/"), true
}

// ResolveInternalTarget admits a conservative ASCII URI-path subset. Unsupported escaping,
// fragments, queries and authority/scheme forms are preserved, never guessed.
func ResolveInternalTarget(source, target string) (string, string) {
	if target == "" {
		return "", "opc.target_empty"
	}
	if strings.ContainsAny(target, "%?#\\:") || strings.HasPrefix(target, "//") {
		return "", "opc.target_syntax_unsupported"
	}
	for _, r := range target {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("/-._~!$&'()*+,;=@", r)) {
			return "", "opc.target_syntax_unsupported"
		}
	}
	stack := []string{}
	if !strings.HasPrefix(target, "/") && source != "" {
		pieces := strings.Split(source, "/")
		stack = append(stack, pieces[:len(pieces)-1]...)
	}
	for _, p := range strings.Split(strings.TrimPrefix(target, "/"), "/") {
		switch p {
		case "":
			return "", "opc.target_syntax_unsupported"
		case ".":
			continue
		case "..":
			if len(stack) == 0 {
				return "", "opc.target_outside_package"
			}
			stack = stack[:len(stack)-1]
		default:
			stack = append(stack, p)
		}
	}
	if len(stack) == 0 || target == "." || target == ".." || strings.HasSuffix(target, "/.") || strings.HasSuffix(target, "/..") {
		return "", "opc.target_syntax_unsupported"
	}
	return strings.Join(stack, "/"), ""
}
func xmlCode(err error) string {
	switch {
	case errors.Is(err, xmlparts.ErrLimit):
		return "xml.resource_limit"
	case errors.Is(err, xmlparts.ErrUnsupported):
		return "xml.unsupported"
	default:
		return "xml.invalid"
	}
}
func valueBytes(d *xmlparts.Document) int {
	n := 0
	for _, e := range d.Elements {
		n += len(e.Name.Prefix) + len(e.Name.Local) + len(e.Name.Namespace)
		for _, a := range e.Attributes {
			n += len(a.Name.Prefix) + len(a.Name.Local) + len(a.Name.Namespace) + len(a.Value)
		}
	}
	for _, t := range d.Tokens {
		n += len(t.Value)
	}
	return n
}
func attrs(e xmlparts.Element) (map[string]string, bool) {
	result := map[string]string{}
	unknown := false
	for _, a := range e.Attributes {
		if a.NamespaceDeclaration {
			continue
		}
		if a.Name.Namespace != "" {
			unknown = true
			continue
		}
		switch a.Name.Local {
		case "Id", "Type", "Target", "TargetMode":
			result[a.Name.Local] = a.Value
		default:
			unknown = true
		}
	}
	return result, unknown
}

// Inspect reuses one acquired snapshot. Package-reader errors return no result;
// failed or unsupported relationship parts remain explicit inside a partial result.
// A completed result covers only relationship candidates, not Office conformance.
func Inspect(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	pkg, err := packageparts.Read(ctx, source, expectedSHA256, packageparts.DefaultLimits())
	if err != nil {
		return nil, err
	}
	result := &Result{Parser: Version, State: "not_applicable", Package: pkg, Parts: []PartResult{}, Relationships: []Relationship{}}
	index := map[string][]int{}
	for i, p := range pkg.Parts {
		if !p.Directory {
			index[fold(p.Name)] = append(index[fold(p.Name)], i)
		}
	}
	bytesLeft, tokensLeft, valuesLeft, partsLeft := maxXMLBytes, maxTokens, maxValues, maxParts
	for _, p := range pkg.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if p.Directory || !strings.HasSuffix(fold(p.Name), ".rels") {
			continue
		}
		part := PartResult{Part: p.Name, PartSHA256: p.SHA256, State: "completed"}
		sourceName, valid := owner(p.Name)
		part.SourcePart = sourceName
		switch {
		case !valid:
			part.State, part.Code = "unsupported", "opc.relationship_name_unsupported"
		case len(index[fold(p.Name)]) != 1:
			part.State, part.Code = "failed", "opc.relationship_part_ambiguous"
		case partsLeft <= 0 || len(p.Bytes) > bytesLeft || tokensLeft <= 0 || valuesLeft <= 0:
			part.State, part.Code = "not_run", "opc.resource_limit"
		default:
			partsLeft--
			bytesLeft -= len(p.Bytes)
			limits := xmlparts.DefaultLimits()
			limits.Tokens = min(limits.Tokens, tokensLeft)
			limits.RetainedBytes = min(limits.RetainedBytes, valuesLeft)
			doc, err := xmlparts.Parse(ctx, p.Bytes, p.SHA256, limits)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				// Failed parses do not expose actual token use. Charge their full allowance.
				tokensLeft -= limits.Tokens
				valuesLeft -= limits.RetainedBytes
				part.State, part.Code = "failed", xmlCode(err)
			} else {
				tokensLeft -= len(doc.Tokens)
				valuesLeft -= valueBytes(doc)
				part.XML = doc
				result.readPart(&part, index)
			}
		}
		result.Parts = append(result.Parts, part)
		if result.State == "not_applicable" {
			result.State = "completed"
		}
		if part.State != "completed" {
			result.State = "partial"
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func (r *Result) readPart(part *PartResult, index map[string][]int) {
	d := part.XML
	if len(d.Elements) == 0 || d.Elements[0].Name.Namespace != Namespace || d.Elements[0].Name.Local != "Relationships" {
		part.State, part.Code = "failed", "opc.relationship_root_invalid"
		return
	}
	rootUnknown := false
	for _, a := range d.Elements[0].Attributes {
		if !a.NamespaceDeclaration {
			rootUnknown = true
		}
	}
	sourceCode := ""
	if part.SourcePart != "" {
		if _, code := ResolveInternalTarget("", part.SourcePart); code != "" {
			sourceCode = "opc.source_syntax_unsupported"
		}
		found := index[fold(part.SourcePart)]
		if sourceCode != "" {
			// Preserve the source name without inventing unsupported URI resolution.
		} else if len(found) == 0 {
			sourceCode = "opc.source_missing"
		} else if len(found) > 1 {
			sourceCode = "opc.source_ambiguous"
		} else {
			part.SourcePart = r.Package.Parts[found[0]].Name
		}
	}
	counts := map[string]int{}
	invalidContent := map[int]bool{}
	for _, t := range d.Tokens {
		if t.Kind == "text" && strings.Trim(t.Value, " \t\r\n") != "" {
			invalidContent[t.Element] = true
		}
	}
	for _, e := range d.Elements {
		if e.Parent == 0 && e.Name.Namespace == Namespace && e.Name.Local == "Relationship" {
			a, _ := attrs(e)
			counts[a["Id"]]++
		} else if e.Parent >= 0 {
			invalidContent[e.Parent] = true
		}
	}
	if invalidContent[0] || rootUnknown {
		part.State, part.Code = "partial", "opc.relationship_structure_unknown"
	}
	if sourceCode != "" {
		part.State, part.Code = "partial", sourceCode
	}
	for i, e := range d.Elements {
		if e.Parent != 0 || e.Name.Namespace != Namespace || e.Name.Local != "Relationship" {
			continue
		}
		if len(r.Relationships) >= maxRelationships {
			part.State, part.Code = "partial", "opc.resource_limit"
			return
		}
		a, unknown := attrs(e)
		rel := Relationship{ID: a["Id"], Type: a["Type"], Target: a["Target"], TargetMode: a["TargetMode"], SourcePart: part.SourcePart, Anchor: Anchor{part.Part, part.PartSHA256, i, e.Full}}
		if _, declared := a["TargetMode"]; !declared {
			rel.TargetMode = "Internal"
		}
		switch {
		case rel.ID == "" || rel.Type == "" || rel.Target == "":
			rel.State, rel.Code = "invalid", "opc.relationship_required_attribute"
		case counts[rel.ID] != 1:
			rel.State, rel.Code = "invalid", "opc.relationship_id_ambiguous"
		case unknown || rootUnknown || invalidContent[i]:
			rel.State, rel.Code = "unsupported", "opc.relationship_structure_unknown"
		case sourceCode != "":
			rel.State, rel.Code = "unresolved", sourceCode
		case rel.TargetMode == "External":
			rel.State = "external"
		case rel.TargetMode != "Internal":
			rel.State, rel.Code = "invalid", "opc.target_mode_invalid"
		default:
			target, code := ResolveInternalTarget(part.SourcePart, rel.Target)
			if code != "" {
				rel.State, rel.Code = "unresolved", code
			} else {
				matches := index[fold(target)]
				switch len(matches) {
				case 0:
					rel.State, rel.Code = "missing", "opc.target_missing"
				case 1:
					found := r.Package.Parts[matches[0]]
					rel.ResolvedPart, rel.TargetSHA256, rel.State = found.Name, found.SHA256, "resolved"
				default:
					rel.State, rel.Code = "ambiguous", "opc.target_ambiguous"
				}
			}
		}
		if rel.State != "resolved" && rel.State != "external" {
			part.State = "partial"
			if part.Code == "" {
				part.Code = "opc.relationship_unresolved"
			}
		}
		r.Relationships = append(r.Relationships, rel)
	}
}
