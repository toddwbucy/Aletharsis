// Package odtanalysis joins stored ODF text and explicitly mapped text controls.
// The result is a bounded analysis projection, not a rendered document.
package odtanalysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/odttext"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "odt-analysis/1"
const MaxScalars = 200000

// Origin distinguishes lexical character mappings from generated analysis
// whitespace. Control Source spans anchor an element, not a removable character.
type Origin struct {
	Kind, Transformation                       string
	Text, Segment, Scalar, Control, Repetition int
	Source, UTF8                               xmlparts.Span
}
type Scope struct {
	ID, Text, SHA256 string
	Paragraph        int
	Origins          []Origin
	Hashes           map[string]string
	Normalization    evidence.Object
	Findings         []evidence.Finding
}
type Boundary struct {
	Token  int
	Reason string
}
type Result struct {
	Version     string
	State       string
	Extraction  *odttext.Result
	Scopes      []Scope
	Boundaries  []Boundary
	Limitations []string
}

func Analyze(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	extracted, err := odttext.Extract(ctx, source, expectedSHA256)
	if err != nil {
		return nil, err
	}
	r := &Result{Version: Version, State: extracted.State, Extraction: extracted, Scopes: []Scope{}, Boundaries: []Boundary{}, Limitations: []string{"odt.analysis_not_rendered_text", "odt.whitespace_not_collapsed", "odt.cross_paragraph_patterns_not_assessed", "odt.structural_boundaries_split_analysis", "odt.only_unicode_emoji_and_patterns_analyzed"}}
	d := extracted.XML.Document
	texts := map[int]int{}
	for i, t := range extracted.Texts {
		texts[extracted.XML.Segments[t.Segment].Token] = i
	}
	controls := map[int]int{}
	for i, c := range extracted.Controls {
		controls[c.Element] = i
	}
	var builder strings.Builder
	origins := []Origin{}
	active := false
	paragraph := -1
	total := 0
	// Reserve capacity for stored text so control expansion cannot crowd out
	// observable characters later in the document.
	remainingStored := 0
	for _, t := range extracted.Texts {
		remainingStored += len(extracted.XML.Segments[t.Segment].Scalars)
		if remainingStored > MaxScalars {
			return nil, xmlparts.ErrLimit
		}
	}
	flush := func() {
		if !active {
			return
		}
		text := builder.String()
		r.Scopes = append(r.Scopes, Scope{ID: fmt.Sprintf("odt-scope/%d", len(r.Scopes)), Text: text, SHA256: evidence.Hash([]byte(text)), Paragraph: paragraph, Origins: origins, Findings: []evidence.Finding{}})
		builder = strings.Builder{}
		origins = []Origin{}
		active = false
		paragraph = -1
	}
	boundary := func(token int, reason string) {
		if active {
			r.Boundaries = append(r.Boundaries, Boundary{token, reason})
			flush()
		}
	}
	begin := func(token, p int) {
		if active && paragraph != p {
			boundary(token, "paragraph_changed")
		}
		active = true
		paragraph = p
	}
	for ti, token := range d.Tokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if idx, ok := texts[ti]; ok {
			t := extracted.Texts[idx]
			s := extracted.XML.Segments[t.Segment]
			if len(s.Scalars) > MaxScalars-total {
				return nil, xmlparts.ErrLimit
			}
			begin(ti, t.Paragraph)
			start := int64(builder.Len())
			for _, m := range s.Scalars {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				origins = append(origins, Origin{Kind: "stored", Transformation: m.Transformation, Text: idx, Segment: t.Segment, Scalar: m.Scalar, Control: -1, Repetition: -1, Source: m.Source, UTF8: xmlparts.Span{Start: start + m.UTF8.Start, End: start + m.UTF8.End}})
			}
			total += len(s.Scalars)
			remainingStored -= len(s.Scalars)
			builder.WriteString(s.Text)
			continue
		}
		if token.Kind == "start" || token.Kind == "end" {
			if idx, ok := controls[token.Element]; ok {
				c := extracted.Controls[idx]
				if c.CountState != "known" {
					boundary(ti, "unresolved_control")
					continue
				}
				if token.Kind == "end" {
					continue
				}
				if c.Count > uint64(MaxScalars-total-remainingStored) {
					r.State = "partial"
					r.Boundaries = append(r.Boundaries, Boundary{ti, "control_expansion_limit"})
					flush()
					continue
				}
				begin(ti, c.Paragraph)
				char := byte(' ')
				switch c.Kind {
				case "tab":
					char = '\t'
				case "line_break":
					char = '\n'
				}
				for j := uint64(0); j < c.Count; j++ {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					start := int64(builder.Len())
					builder.WriteByte(char)
					origins = append(origins, Origin{Kind: "control_expansion", Transformation: c.Kind, Text: -1, Segment: -1, Scalar: -1, Control: idx, Repetition: int(j), Source: c.Span, UTF8: xmlparts.Span{Start: start, End: start + 1}})
				}
				total += int(c.Count)
				continue
			}
			e := d.Elements[token.Element]
			if e.Name.Namespace == odttext.TextNS && (e.Name.Local == "span" || e.Name.Local == "a") {
				continue
			}
			boundary(ti, "structural_boundary")
			continue
		}
		if token.Kind == "text" {
			if strings.Trim(token.Value, " \t\r\n") != "" {
				boundary(ti, "unselected_character_data")
			}
		} else if token.Kind == "comment" || token.Kind == "processing_instruction" {
			boundary(ti, "non_text_xml")
		}
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s.Findings = append(s.Findings, (analyzers.Patterns{}).Analyze(&doc)...)
		s.Hashes = doc.Texts[0].Hashes
		s.Normalization = doc.Texts[0].Normalization
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
