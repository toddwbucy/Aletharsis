// Package xmlparts preserves bounded XML part structure with part-byte locations.
// It does not resolve resources or interpret document-format semantics.
package xmlparts

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

const Version = "xml-parts/1"
const xmlURI = "http://www.w3.org/XML/1998/namespace"
const xmlnsURI = "http://www.w3.org/2000/xmlns/"

var (
	ErrXML         = errors.New("invalid XML part")
	ErrUnsupported = errors.New("unsupported XML feature")
	ErrLimit       = errors.New("XML resource limit")
	ErrIdentity    = errors.New("XML part identity mismatch")
)

type Limits struct{ Bytes, Tokens, Elements, Depth, Attributes, RetainedBytes int }

func DefaultLimits() Limits { return Limits{4 << 20, 100000, 50000, 128, 128, 8 << 20} }

type Span struct{ Start, End int64 }
type Name struct{ Prefix, Local, Namespace string }
type Attribute struct {
	Name                 Name
	Value                string
	NamespaceDeclaration bool
}
type Element struct {
	Name             Name
	Attributes       []Attribute
	Parent           int
	Start, End, Full Span
}

// Token byte spans refer to original part bytes, never to the ZIP container.
// Exact UTF-8 mapping is asserted only for unchanged literal character data.
// A synthetic end has a zero-width span after its self-closing start tag.
type Token struct {
	Kind           string
	Span           Span
	Element        int
	Value, Mapping string
}
type Document struct {
	PartSHA256, Parser, Encoding string
	BOM                          bool
	Elements                     []Element
	Tokens                       []Token
}
type frame struct {
	index      int
	raw        xml.Name
	namespaces map[string]string
}

func validLimits(l Limits) bool {
	h := DefaultLimits()
	return l.Bytes > 0 && l.Bytes <= h.Bytes && l.Tokens > 0 && l.Tokens <= h.Tokens && l.Elements > 0 && l.Elements <= h.Elements && l.Depth > 0 && l.Depth <= h.Depth && l.Attributes > 0 && l.Attributes <= h.Attributes && l.RetainedBytes > 0 && l.RetainedBytes <= h.RetainedBytes
}
func lookup(prefix string, local map[string]string, stack []frame) (string, bool) {
	if prefix == "xml" {
		return xmlURI, true
	}
	if v, ok := local[prefix]; ok {
		return v, true
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if v, ok := stack[i].namespaces[prefix]; ok {
			return v, true
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}
func expanded(n xml.Name, attr bool, local map[string]string, stack []frame) (Name, error) {
	if !ncname(n.Local) || n.Space != "" && !ncname(n.Space) || n.Space == "xmlns" {
		return Name{}, ErrXML
	}
	uri := ""
	if !attr || n.Space != "" {
		var ok bool
		uri, ok = lookup(n.Space, local, stack)
		if !ok {
			return Name{}, ErrXML
		}
	}
	return Name{n.Space, n.Local, uri}, nil
}
func declarations(attrs []xml.Attr) (map[string]string, error) {
	result := map[string]string{}
	for _, a := range attrs {
		if a.Name.Space != "xmlns" && (a.Name.Space != "" || a.Name.Local != "xmlns") {
			continue
		}
		prefix := a.Name.Local
		if a.Name.Space == "" {
			prefix = ""
		}
		if _, exists := result[prefix]; exists {
			return nil, ErrXML
		}
		if prefix != "" && !ncname(prefix) || prefix == "xmlns" || a.Value == xmlnsURI || prefix == "xml" && a.Value != xmlURI || prefix != "xml" && a.Value == xmlURI || prefix != "" && a.Value == "" || strings.ContainsAny(a.Value, " \t\r\n") {
			return nil, ErrXML
		}
		result[prefix] = a.Value
	}
	return result, nil
}
func declaration(data []byte) bool {
	// Reuse lexical attribute parsing, with declaration-specific ordering and values.
	// References and markup are not legal in declaration pseudo-attributes.
	if bytes.ContainsAny(data, "&<>") || !attributeSpacing([]byte("<declaration "+string(data)+"/>")) {
		return false
	}
	d := xml.NewDecoder(strings.NewReader("<declaration " + string(data) + "/>"))
	t, err := d.Token()
	if err != nil {
		return false
	}
	e, ok := t.(xml.StartElement)
	if !ok || len(e.Attr) < 1 || len(e.Attr) > 3 {
		return false
	}
	last := -1
	for _, a := range e.Attr {
		if a.Name.Space != "" {
			return false
		}
		order := -1
		switch a.Name.Local {
		case "version":
			order = 0
			if a.Value != "1.0" {
				return false
			}
		case "encoding":
			order = 1
			if !strings.EqualFold(a.Value, "utf-8") {
				return false
			}
		case "standalone":
			order = 2
			if a.Value != "yes" && a.Value != "no" {
				return false
			}
		default:
			return false
		}
		if order <= last || last == -1 && order != 0 {
			return false
		}
		last = order
	}
	return true
}

// Parse owns its output; callers must not mutate input during the call. A complete
// result means this XML subset parsed, not that an Office document is supported.
// Errors expose no partial semantic document. Original bytes remain the evidence.
// Usage records bounded work even when no semantic document can be returned.
type Usage struct{ Tokens, RetainedBytes int }

func Parse(ctx context.Context, source []byte, expectedSHA256 string, l Limits) (*Document, error) {
	doc, _, err := ParseMeasured(ctx, source, expectedSHA256, l)
	return doc, err
}

// ParseMeasured preserves Parse's fail-closed document contract and separately
// reports consumed token/value allowance. Reservations are not consumption.
func ParseMeasured(ctx context.Context, source []byte, expectedSHA256 string, l Limits) (*Document, Usage, error) {
	var used Usage
	doc, err := parseMeasured(ctx, source, expectedSHA256, l, &used)
	return doc, used, err
}
func parseMeasured(ctx context.Context, source []byte, expectedSHA256 string, l Limits, used *Usage) (*Document, error) {
	if ctx == nil || !validLimits(l) {
		return nil, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source) > l.Bytes {
		return nil, ErrLimit
	}
	if evidence.Hash(source) != expectedSHA256 {
		return nil, ErrIdentity
	}
	if !utf8.Valid(source) {
		return nil, ErrUnsupported
	}
	for _, r := range string(source) {
		if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			return nil, ErrXML
		}
	}
	base := 0
	if bytes.HasPrefix(source, []byte{0xef, 0xbb, 0xbf}) {
		base = 3
	}
	d := xml.NewDecoder(bytes.NewReader(source[base:]))
	unsupported := false
	d.CharsetReader = func(string, io.Reader) (io.Reader, error) { unsupported = true; return nil, ErrUnsupported }
	result := &Document{PartSHA256: expectedSHA256, Parser: Version, Encoding: "utf-8", BOM: base == 3, Elements: []Element{}, Tokens: []Token{}}
	stack := []frame{}
	roots, retained, tokensUsed := 0, 0, 0
	defer func() { used.Tokens = min(tokensUsed, l.Tokens); used.RetainedBytes = retained }()
	charge := func(n int) bool {
		if n > l.RetainedBytes-retained {
			return false
		}
		retained += n
		return true
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := d.InputOffset() + int64(base)
		value, err := d.RawToken()
		end := d.InputOffset() + int64(base)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if unsupported {
				return nil, ErrUnsupported
			}
			return nil, ErrXML
		}
		tokensUsed++
		if len(result.Tokens) >= l.Tokens {
			return nil, ErrLimit
		}
		span := Span{start, end}
		if start < 0 || end < start || end > int64(len(source)) {
			return nil, ErrXML
		}
		parent := -1
		if len(stack) > 0 {
			parent = stack[len(stack)-1].index
		}
		event := Token{Span: span, Element: parent, Mapping: "token_only"}
		switch t := value.(type) {
		case xml.StartElement:
			if !attributeSpacing(source[start:end]) {
				return nil, ErrXML
			}
			if len(stack) >= l.Depth || len(result.Elements) >= l.Elements || len(t.Attr) > l.Attributes {
				return nil, ErrLimit
			}
			if len(stack) == 0 {
				roots++
				if roots != 1 {
					return nil, ErrXML
				}
			}
			// XML attribute normalization replaces literal whitespace, but keeps
			// whitespace introduced by character references distinct.
			attrsValue, err := normalizedAttributes(source[start:end], t.Attr)
			if err != nil {
				return nil, err
			}
			t.Attr = attrsValue
			ns, err := declarations(t.Attr)
			if err != nil {
				return nil, err
			}
			name, err := expanded(t.Name, false, ns, stack)
			if err != nil {
				return nil, err
			}
			if !charge(len(name.Prefix) + len(name.Local) + len(name.Namespace)) {
				return nil, ErrLimit
			}
			attrs := make([]Attribute, 0, len(t.Attr))
			seen := map[xml.Name]bool{}
			for _, a := range t.Attr {
				declaration := a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns"
				var n Name
				if declaration {
					n = Name{a.Name.Space, a.Name.Local, xmlnsURI}
				} else {
					n, err = expanded(a.Name, true, ns, stack)
					if err != nil {
						return nil, err
					}
				}
				key := xml.Name{Space: n.Namespace, Local: n.Local}
				if seen[key] {
					return nil, ErrXML
				}
				seen[key] = true
				if !charge(len(n.Prefix) + len(n.Local) + len(n.Namespace) + len(a.Value)) {
					return nil, ErrLimit
				}
				attrs = append(attrs, Attribute{n, a.Value, declaration})
			}
			index := len(result.Elements)
			result.Elements = append(result.Elements, Element{Name: name, Attributes: attrs, Parent: parent, Start: span, Full: Span{Start: start}})
			stack = append(stack, frame{index, t.Name, ns})
			event.Kind = "start"
			event.Element = index
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, ErrXML
			}
			top := stack[len(stack)-1]
			if top.raw != t.Name {
				return nil, ErrXML
			}
			result.Elements[top.index].End = span
			result.Elements[top.index].Full.End = end
			event.Kind = "end"
			event.Element = top.index
			if start == end {
				event.Mapping = "synthetic_end"
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 && len(bytes.Trim(source[start:end], " \t\r\n")) != 0 {
				return nil, ErrXML
			}
			event.Kind = "text"
			event.Value = string(t)
			if bytes.Equal(source[start:end], t) {
				event.Mapping = "exact_utf8"
			}
		case xml.Comment:
			event.Kind = "comment"
			event.Value = string(t)
		case xml.ProcInst:
			rawPI := source[start:end]
			separator := 2 + len(t.Target)
			if separator < len(rawPI)-2 && !space(rawPI[separator]) {
				return nil, ErrXML
			}
			if strings.EqualFold(t.Target, "xml") {
				if t.Target != "xml" || start != int64(base) || !declaration(t.Inst) {
					return nil, ErrXML
				}
				event.Kind = "declaration"
			} else {
				event.Kind = "processing_instruction"
			}
			event.Value = t.Target + " " + string(t.Inst)
		case xml.Directive:
			return nil, ErrUnsupported
		default:
			return nil, ErrUnsupported
		}
		if !charge(len(event.Value)) {
			return nil, ErrLimit
		}
		result.Tokens = append(result.Tokens, event)
	}
	if len(stack) != 0 || roots != 1 {
		return nil, ErrXML
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// RawToken checks XML name characters as a whole; QName components must also
// begin with a name-start character and must not contain another colon.
func ncname(s string) bool {
	if s == "" || strings.Contains(s, ":") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= 0xC0 && r <= 0xD6 || r >= 0xD8 && r <= 0xF6 || r >= 0xF8 && r <= 0x2FF || r >= 0x370 && r <= 0x37D || r >= 0x37F && r <= 0x1FFF || r >= 0x200C && r <= 0x200D || r >= 0x2070 && r <= 0x218F || r >= 0x2C00 && r <= 0x2FEF || r >= 0x3001 && r <= 0xD7FF || r >= 0xF900 && r <= 0xFDCF || r >= 0xFDF0 && r <= 0xFFFD || r >= 0x10000 && r <= 0xEFFFF
}
func space(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// encoding/xml accepts adjacent attributes without required whitespace. Enforce
// the lexical boundary while leaving names, references and values to the decoder.
func attributeSpacing(raw []byte) bool {
	if len(raw) < 3 || raw[0] != '<' {
		return false
	}
	i := 1
	for i < len(raw) && !space(raw[i]) && raw[i] != '/' && raw[i] != '>' {
		i++
	}
	for i < len(raw) {
		before := i
		for i < len(raw) && space(raw[i]) {
			i++
		}
		if i == len(raw) {
			return false
		}
		if raw[i] == '>' || raw[i] == '/' {
			return true
		}
		if i == before {
			return false
		}
		for i < len(raw) && !space(raw[i]) && raw[i] != '=' {
			i++
		}
		for i < len(raw) && space(raw[i]) {
			i++
		}
		if i == len(raw) || raw[i] != '=' {
			return false
		}
		i++
		for i < len(raw) && space(raw[i]) {
			i++
		}
		if i == len(raw) || (raw[i] != '\'' && raw[i] != '"') {
			return false
		}
		quote := raw[i]
		i++
		for i < len(raw) && raw[i] != quote {
			if raw[i] == '<' {
				return false
			}
			i++
		}
		if i == len(raw) {
			return false
		}
		i++
	}
	return false
}

func normalizedAttributes(raw []byte, original []xml.Attr) ([]xml.Attr, error) {
	if !bytes.ContainsAny(raw, "\t\r\n") {
		return original, nil
	}
	normalized := make([]byte, 0, len(raw))
	var quote byte
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if quote == 0 {
			if b == '\'' || b == '"' {
				quote = b
			}
			normalized = append(normalized, b)
			continue
		}
		if b == quote {
			quote = 0
			normalized = append(normalized, b)
			continue
		}
		if b == '\t' || b == '\r' || b == '\n' {
			normalized = append(normalized, ' ')
			if b == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
			continue
		}
		normalized = append(normalized, b)
	}
	if bytes.Equal(raw, normalized) {
		return original, nil
	}
	d := xml.NewDecoder(bytes.NewReader(normalized))
	token, err := d.RawToken()
	if err != nil {
		return nil, ErrXML
	}
	element, ok := token.(xml.StartElement)
	if !ok || len(element.Attr) != len(original) {
		return nil, ErrXML
	}
	return element.Attr, nil
}
