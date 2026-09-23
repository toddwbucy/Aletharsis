// Package officeobjects inventories inert DOCX embedded-object candidates.
// It never opens nested containers, executes content, or follows external targets.
package officeobjects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

const Version = "office-objects/1"
const SignatureBytes = 8

// Anchor is an opaque part identity, never a text offset or an editable span.
type Anchor struct{ ObjectID, Part, PartSHA256 string }
type Object struct {
	ID            string
	Anchor        Anchor
	ByteLength    *uint64
	Reasons       []string
	Relationships []int // Indices in the shared OPC inventory, including repeats.
	DeclaredTypes []string
	Signature     string // Magic-byte observation only; empty means no recognized prefix.
	State, Code   string
	Assessed      []packageparts.Span // Only the bounded signature prefix is inspected.
}
type Declaration struct {
	Relationship int
	ObjectID     string // Empty for external/missing/ambiguous targets.
	State, Code  string
	Anchor       opcrels.Anchor
}
type Result struct {
	Parser, State, SourceSHA256 string
	Objects                     []Object
	Declarations                []Declaration
	Limitations                 []string
}

func recognizedType(s string) bool {
	for _, prefix := range []string{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/", "http://purl.oclc.org/ooxml/officeDocument/relationships/"} {
		if s == prefix+"oleObject" || s == prefix+"package" {
			return true
		}
	}
	return false
}
func conventional(name string) bool {
	return strings.HasPrefix(opcrels.FoldName(name), "word/embeddings/")
}
func signature(b []byte) string {
	for _, s := range []struct {
		prefix []byte
		name   string
	}{
		{[]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, "ole_compound_magic"},
		{[]byte{'P', 'K', 3, 4}, "zip_local_header_magic"},
		{[]byte("%PDF-"), "pdf_header_magic"},
		{[]byte{0x89, 'P', 'N', 'G', 13, 10, 26, 10}, "png_magic"},
	} {
		if bytes.HasPrefix(b, s.prefix) {
			return s.name
		}
	}
	return ""
}

// Inspect consumes one identified DOCX plus its shared verified OPC inventory.
// Callers must not mutate the supplied records during inspection. Ordinary
// filenames only create candidates; they never establish type or authenticity.
func Inspect(ctx context.Context, doc *docxidentify.Result) (*Result, error) {
	if ctx == nil || doc == nil || doc.OPC == nil || doc.OPC.Outcomes == nil {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if doc.Format != "docx" {
		return nil, errors.New("embedded-object inventory requires identified DOCX")
	}
	opc := doc.OPC
	if opc.Outcomes.SourceSHA256 == "" {
		return nil, packageparts.ErrIdentity
	}
	result := &Result{Parser: Version, State: "completed", SourceSHA256: opc.Outcomes.SourceSHA256, Objects: []Object{}, Declarations: []Declaration{},
		Limitations: []string{"office.objects_type_not_verified", "office.objects_recursive_ole_unavailable", "office.objects_nested_archives_unavailable", "office.objects_disk_extraction_unavailable", "office.objects_macro_analysis_unavailable"}}
	// OPC incompleteness can conceal declarations. Preserve it even with no objects.
	if opc.State == "partial" {
		result.State = "partial"
	}
	outcomes := map[string]packageparts.Outcome{}
	candidates := map[string]*Object{}
	for _, o := range opc.Outcomes.Parts {
		if _, duplicate := outcomes[o.Part.Name]; duplicate {
			return nil, packageparts.ErrIdentity
		}
		outcomes[o.Part.Name] = o
		if !o.Part.Directory && conventional(o.Part.Name) {
			candidates[o.Part.Name] = &Object{Reasons: []string{"conventional_embedding_directory"}, Relationships: []int{}, DeclaredTypes: []string{}, Assessed: []packageparts.Span{}}
		}
	}
	for i, rel := range opc.Relationships {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !recognizedType(rel.Type) {
			continue
		}
		d := Declaration{Relationship: i, State: rel.State, Code: rel.Code, Anchor: rel.Anchor}
		result.Declarations = append(result.Declarations, d)
		// Unavailable but uniquely resolved names remain candidates, with no invented
		// digest. External and otherwise unresolved declarations are retained alone.
		if rel.TargetMode != "Internal" || rel.State != "resolved" {
			continue
		}
		target, exists := outcomes[rel.ResolvedPart]
		if !exists || target.Part.Directory {
			return nil, packageparts.ErrIdentity
		}
		object, exists := candidates[rel.ResolvedPart]
		if !exists {
			object = &Object{Reasons: []string{}, Relationships: []int{}, DeclaredTypes: []string{}, Assessed: []packageparts.Span{}}
			candidates[rel.ResolvedPart] = object
		}
		if len(object.Relationships) == 0 {
			object.Reasons = append(object.Reasons, "embedded_relationship")
		}
		object.Relationships = append(object.Relationships, i)
	}
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		o := candidates[name]
		part := outcomes[name]
		o.ID = fmt.Sprintf("office-object/%d", len(result.Objects))
		o.Anchor = Anchor{o.ID, name, part.Part.SHA256}
		for _, a := range doc.Assignments {
			if a.Part == name && a.State == "assigned" {
				o.DeclaredTypes = append(o.DeclaredTypes, a.ContentType)
			}
		}
		o.State, o.Code = part.State, part.Code
		if part.State == "completed" {
			if evidence.Hash(part.Part.Bytes) != part.Part.SHA256 {
				return nil, packageparts.ErrIdentity
			}
			size := uint64(len(part.Part.Bytes))
			o.ByteLength = &size
			end := min(len(part.Part.Bytes), SignatureBytes)
			o.Signature = signature(part.Part.Bytes[:end])
			if end > 0 {
				o.Assessed = append(o.Assessed, packageparts.Span{Start: 0, End: int64(end)})
			}
		} else {
			if part.Part.SHA256 != "" || part.Part.Bytes != nil {
				return nil, packageparts.ErrIdentity
			}
			if part.Code != "office.part_not_admitted" {
				result.State = "partial"
			}
		}
		result.Objects = append(result.Objects, *o)
	}
	byPart := map[string]string{}
	for _, o := range result.Objects {
		byPart[o.Anchor.Part] = o.ID
	}
	for i := range result.Declarations {
		d := &result.Declarations[i]
		rel := opc.Relationships[d.Relationship]
		if rel.TargetMode == "Internal" && rel.State == "resolved" {
			d.ObjectID = byPart[rel.ResolvedPart]
		}
	}
	return result, nil
}
