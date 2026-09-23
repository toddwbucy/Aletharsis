package v4

// ValidateXML checks the retained structural indexes and lexical bounds before
// any origin may use them. Callers must first perform closed wire validation.
func ValidateXML(x XML, part Part) error {
	if x.PartRef != part.PartRef || part.SHA256 == nil || part.ByteLength == nil ||
		x.Tokens == nil || len(x.Elements) == 0 || x.Segments == nil || x.Controls == nil {
		return ErrCoordinates
	}
	size := *part.ByteLength
	for i, e := range x.Elements {
		if e.Index != int64(i) || !e.Span.Within(size) || e.Span.Start >= e.Span.End || (i == 0) != (e.Parent == nil) {
			return ErrCoordinates
		}
		if e.Parent != nil {
			if *e.Parent < 0 || *e.Parent >= int64(i) {
				return ErrCoordinates
			}
			p := x.Elements[*e.Parent]
			if e.Span.Start < p.Span.Start || e.Span.End > p.Span.End {
				return ErrCoordinates
			}
		}
	}
	stack := []int64{}
	started := int64(0)
	for i, t := range x.Tokens {
		if t.Index != int64(i) || !t.Span.Within(size) {
			return ErrCoordinates
		}
		if t.Element != nil {
			if *t.Element < 0 || *t.Element >= int64(len(x.Elements)) {
				return ErrCoordinates
			}
			e := x.Elements[*t.Element]
			if t.Span.Start < e.Span.Start || t.Span.End > e.Span.End {
				return ErrCoordinates
			}
		}
		// The lexical token stream must describe the same tree as Elements.
		// Containment alone permits reparenting to an ancestor, missing tags,
		// or two coincident sibling extents.
		switch t.Kind {
		case "start":
			if t.Element == nil || *t.Element != started || t.Span.Start >= t.Span.End {
				return ErrCoordinates
			}
			e := x.Elements[*t.Element]
			if t.Span.Start != e.Span.Start || (len(stack) == 0) != (e.Parent == nil) {
				return ErrCoordinates
			}
			if len(stack) > 0 && *e.Parent != stack[len(stack)-1] {
				return ErrCoordinates
			}
			stack = append(stack, *t.Element)
			started++
		case "end":
			if t.Element == nil || len(stack) == 0 || *t.Element != stack[len(stack)-1] ||
				t.Span.End != x.Elements[*t.Element].Span.End {
				return ErrCoordinates
			}
			// The parser emits a zero-width end token only for a self-closing
			// start token, at that token's end.
			if t.Span.Start == t.Span.End && (i == 0 || x.Tokens[i-1].Kind != "start" ||
				x.Tokens[i-1].Element == nil || *x.Tokens[i-1].Element != *t.Element ||
				x.Tokens[i-1].Span.End != t.Span.Start) {
				return ErrCoordinates
			}
			stack = stack[:len(stack)-1]
		default:
			if t.Span.Start >= t.Span.End || (len(stack) == 0) != (t.Element == nil) {
				return ErrCoordinates
			}
			if len(stack) > 0 && *t.Element != stack[len(stack)-1] {
				return ErrCoordinates
			}
		}
		if i > 0 && t.Span.Start < x.Tokens[i-1].Span.End {
			return ErrCoordinates
		}
	}
	if len(stack) != 0 || started != int64(len(x.Elements)) {
		return ErrCoordinates
	}
	seen := map[int]bool{}
	for _, s := range x.Segments {
		if s.Token < 0 || s.Token >= len(x.Tokens) || seen[s.Token] {
			return ErrCoordinates
		}
		seen[s.Token] = true
		t := x.Tokens[s.Token]
		if t.Kind != "text" || t.Span != s.TokenSpan || (t.Element == nil) != (s.Element == nil) {
			return ErrCoordinates
		}
		if t.Element != nil && *t.Element != int64(*s.Element) {
			return ErrCoordinates
		}
		if err := ValidateSegment(s, size); err != nil {
			return err
		}
	}
	// A map cannot omit a text token yet claim complete retained XML mapping.
	for i, t := range x.Tokens {
		if t.Kind == "text" && !seen[i] {
			return ErrCoordinates
		}
	}
	elements := map[int]bool{}
	for i, c := range x.Controls {
		if c.Index != i || c.Element < 0 || c.Element >= len(x.Elements) || c.Count < 0 ||
			!c.Source.Within(size) || c.Source.Start >= c.Source.End || elements[c.Element] {
			return ErrCoordinates
		}
		elements[c.Element] = true
		e := x.Elements[c.Element]
		if c.Source != e.Span || e.Namespace != "urn:oasis:names:tc:opendocument:xmlns:text:1.0" {
			return ErrCoordinates
		}
		local, ok := map[string]string{"space": "s", "tab": "tab", "line_break": "line-break"}[c.Kind]
		if !ok || e.LocalName != local || (c.Kind != "space" && c.Count != 1) {
			return ErrCoordinates
		}
	}
	return nil
}
