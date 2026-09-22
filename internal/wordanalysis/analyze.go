// Package wordanalysis assembles bounded Word text scopes for native analyzers.
// Every scalar retains its original part-byte map and extraction context.
package wordanalysis

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
	"github.com/toddwbucy/Aletharsis/internal/wordtext"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "word-analysis/1"

type Origin struct {
	Text, Segment, Scalar int
	Source, UTF8          xmlparts.Span
	Transformation        string
}
type Scope struct {
	Hashes                 map[string]string
	Normalization          evidence.Object
	ID, Text, SHA256, Role string
	Paragraph              int
	Origins                []Origin
	Findings               []evidence.Finding
}
type Boundary struct {
	Token  int
	Reason string
}
type Result struct {
	Version     string
	Extraction  *wordtext.Result
	Scopes      []Scope
	Boundaries  []Boundary
	Limitations []string
}

// Analyze invokes only Unicode and pattern analyzers over stored-text scopes.
// Native finding byte offsets refer to scope UTF-8, not XML part bytes. Join each
// character offset to Scope.Origins for the source location and WT-001 context.
func Analyze(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	extracted, err := wordtext.Extract(ctx, source, expectedSHA256)
	if err != nil {
		return nil, err
	}
	r := &Result{Version: Version, Extraction: extracted, Scopes: []Scope{}, Boundaries: []Boundary{}, Limitations: []string{"word.analysis_not_rendered_text", "word.cross_paragraph_patterns_not_assessed", "word.controls_split_analysis", "word.unknown_structure_splits_analysis", "word.only_unicode_emoji_and_patterns_analyzed"}}
	d := extracted.XML.Document
	byToken := map[int]int{}
	for i, t := range extracted.Texts {
		byToken[extracted.XML.Segments[t.Segment].Token] = i
	}
	// Only direct run properties are transparent to adjacency. This mask is
	// never used for admission: extraction owns the complete ancestry decision.
	properties := make([]bool, len(d.Elements))
	for i, e := range d.Elements {
		if e.Parent >= 0 {
			p := d.Elements[e.Parent]
			properties[i] = properties[e.Parent] || (e.Name.Namespace == extracted.Namespace && e.Name.Local == "rPr" && p.Name.Namespace == extracted.Namespace && p.Name.Local == "r")
		}
	}
	var builder strings.Builder
	origins := []Origin{}
	previous := -1
	first := -1
	flush := func() {
		if first < 0 {
			return
		}
		t := extracted.Texts[first]
		text := builder.String()
		r.Scopes = append(r.Scopes, Scope{ID: fmt.Sprintf("word-scope/%d", len(r.Scopes)), Text: text, SHA256: evidence.Hash([]byte(text)), Role: t.Role, Paragraph: t.Paragraph, Origins: origins, Findings: []evidence.Finding{}})
		builder = strings.Builder{}
		origins = []Origin{}
		first = -1
		previous = -1
	}
	boundary := func(token int, reason string) {
		// Only boundaries that terminate a selected scope are retained.
		if first >= 0 {
			r.Boundaries = append(r.Boundaries, Boundary{token, reason})
			flush()
		}
	}
	for ti, token := range d.Tokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if textIndex, ok := byToken[ti]; ok {
			t := extracted.Texts[textIndex]
			if t.AnalysisContext.BlockedElement >= 0 {
				r.Boundaries = append(r.Boundaries, Boundary{ti, "text_context_not_analyzed"})
				flush()
				continue
			}
			if previous >= 0 {
				p := extracted.Texts[previous]
				if t.Paragraph != p.Paragraph || t.Role != p.Role || !slices.Equal(t.Revisions, p.Revisions) || !slices.Equal(t.UnresolvedAncestors, p.UnresolvedAncestors) {
					boundary(ti, "context_changed")
				}
			}
			if first < 0 {
				first = textIndex
			}
			s := extracted.XML.Segments[t.Segment]
			start := int64(builder.Len())
			for _, m := range s.Scalars {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				origins = append(origins, Origin{Text: textIndex, Segment: t.Segment, Scalar: m.Scalar, Source: m.Source, UTF8: xmlparts.Span{Start: start + m.UTF8.Start, End: start + m.UTF8.End}, Transformation: m.Transformation})
			}
			builder.WriteString(s.Text)
			previous = textIndex
			continue
		}
		if token.Kind == "text" {
			if strings.Trim(token.Value, " \t\r\n") != "" {
				boundary(ti, "unselected_character_data")
			}
			continue
		}
		if token.Kind == "comment" || token.Kind == "processing_instruction" {
			boundary(ti, "non_text_xml")
			continue
		}
		if token.Kind != "start" && token.Kind != "end" {
			continue
		}
		e := d.Elements[token.Element]
		if properties[token.Element] {
			// Paragraph/section/table properties remain structural boundaries.
			if e.Name.Local != "rPr" && (e.Parent < 0 || !properties[e.Parent]) {
				boundary(ti, "structural_boundary")
			}
			continue
		}
		if e.Name.Namespace == extracted.Namespace {
			switch e.Name.Local {
			case "r", "t", "delText", "instrText", "delInstrText":
				continue
			}
		}
		boundary(ti, "structural_boundary")
	}
	flush()
	for i := range r.Scopes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s := &r.Scopes[i]
		decoded, offsets, err := parsers.Decode([]byte(s.Text), "utf-8")
		if err != nil {
			return nil, err
		}
		doc := evidence.EmptyDocument()
		doc.Texts = []*evidence.Text{{Source: s.ID, Text: decoded, Encoding: "utf-8", ByteOffsets: offsets}}
		s.Findings = append(s.Findings, (analyzers.Unicode{}).Analyze(&doc)...)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s.Findings = append(s.Findings, (analyzers.Emoji{}).Analyze(&doc)...)
		s.Findings = append(s.Findings, (analyzers.Patterns{}).Analyze(&doc)...)
		s.Hashes = doc.Texts[0].Hashes
		s.Normalization = doc.Texts[0].Normalization
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
