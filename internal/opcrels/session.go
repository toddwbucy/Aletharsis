package opcrels

import (
	"context"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

// Session shares bounded relationship parsing across admission barriers. Each
// successfully admitted relationship part is parsed at most once. Target names
// are resolved against the full inventory; digests become available only after
// the target has been admitted. Use serially, without mutating returned XML.
type Session struct {
	reader                                       *packageparts.OutcomeReader
	parsed                                       map[string]PartResult
	bytesLeft, tokensLeft, valuesLeft, partsLeft int
}

func NewSession(reader *packageparts.OutcomeReader) (*Session, error) {
	if reader == nil || reader.SourceSHA256() == "" {
		return nil, packageparts.ErrIdentity
	}
	return &Session{reader: reader, parsed: map[string]PartResult{}, bytesLeft: maxXMLBytes, tokensLeft: maxTokens, valuesLeft: maxValues, partsLeft: maxParts}, nil
}

// Parse admits no payloads itself. Names select the coordinator's current phase,
// preserving priority over unrelated relationship parts. Unknown names fail before
// parsing any member. Unadmitted parts can be reconsidered after later admission.
func (s *Session) Parse(ctx context.Context, names []string) error {
	if ctx == nil {
		return packageparts.ErrIdentity
	}
	view := s.reader.ReadOnlyView()
	byName := map[string]packageparts.Outcome{}
	counts := map[string]int{}
	for _, o := range view.Parts {
		byName[o.Part.Name] = o
		if !o.Part.Directory {
			counts[FoldName(o.Part.Name)]++
		}
	}
	for _, name := range names {
		if _, ok := byName[name]; !ok {
			return packageparts.ErrName
		}
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := s.parsed[name]; ok {
			continue
		}
		o := byName[name]
		p := o.Part
		if p.Directory || !strings.HasSuffix(FoldName(name), ".rels") {
			continue
		}
		source, valid := owner(name)
		part := PartResult{Part: name, PartSHA256: p.SHA256, SourcePart: source, State: "completed"}
		switch {
		case !valid:
			part.State, part.Code = "unsupported", "opc.relationship_name_unsupported"
		case counts[FoldName(name)] != 1:
			part.State, part.Code = "failed", "opc.relationship_part_ambiguous"
		case o.State != "completed":
			continue
		case s.partsLeft <= 0 || len(p.Bytes) > s.bytesLeft || s.tokensLeft <= 0 || s.valuesLeft <= 0:
			part.State, part.Code = "not_run", "opc.resource_limit"
		default:
			s.partsLeft--
			s.bytesLeft -= len(p.Bytes)
			limits := xmlparts.DefaultLimits()
			limits.Tokens = min(limits.Tokens, s.tokensLeft)
			limits.RetainedBytes = min(limits.RetainedBytes, s.valuesLeft)
			doc, used, err := xmlparts.ParseMeasured(ctx, p.Bytes, p.SHA256, limits)
			s.tokensLeft -= used.Tokens
			s.valuesLeft -= used.RetainedBytes
			if err != nil {
				part.State, part.Code = "failed", xmlCode(err)
				if ctx.Err() != nil {
					part.State, part.Code = "canceled", "execution.canceled"
					s.parsed[name] = part
					return ctx.Err()
				}
			} else {
				part.XML = doc
			}
		}
		s.parsed[name] = part
	}
	return nil
}

// Result projects the shared parsed declarations against current verified target
// identities. It performs no XML parsing or decompression. Relationship indices
// are canonical for this snapshot; emit final graph references only after admission.
// Package payload slices borrow the reader's verified bytes and must not be
// modified; outcome structs and relationship records belong to this result.
func (s *Session) Result(ctx context.Context) (*Result, error) {
	if ctx == nil {
		return nil, packageparts.ErrIdentity
	}
	view := s.reader.ReadOnlyView()
	index := map[string][]int{}
	for i, o := range view.Parts {
		if !o.Part.Directory {
			index[FoldName(o.Part.Name)] = append(index[FoldName(o.Part.Name)], i)
		}
	}
	r := &Result{Parser: Version, State: "not_applicable", Outcomes: &view, Parts: []PartResult{}, Relationships: []Relationship{}}
	for _, o := range view.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := o.Part
		if p.Directory || !strings.HasSuffix(FoldName(p.Name), ".rels") {
			continue
		}
		part, ok := s.parsed[p.Name]
		if !ok {
			source, _ := owner(p.Name)
			part = PartResult{Part: p.Name, PartSHA256: p.SHA256, SourcePart: source, State: o.State, Code: o.Code}
			if o.State == "completed" {
				part.State, part.Code = "not_run", "opc.relationship_not_parsed"
			}
		} else if part.XML != nil {
			r.readPart(&part, index)
		}
		r.Parts = append(r.Parts, part)
		if r.State == "not_applicable" {
			r.State = "completed"
		}
		if part.State != "completed" {
			r.State = "partial"
		}
	}
	return r, nil
}
