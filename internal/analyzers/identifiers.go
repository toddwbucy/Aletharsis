package analyzers

import (
	"encoding/base64"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type Identifiers struct{}

var uuid = regexp.MustCompile(`(?i)[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}`)

// Match labels separately so Unicode whitespace/case rules and character limits
// use reference semantics instead of RE2's ASCII shorthand classes.
var label = regexp.MustCompile(`(?:generated|watermark|provenance|tracking[-_ ]id|document[-_ ]id|author)`)
var encoded = regexp.MustCompile(`[A-Za-z0-9+/]{32,}={0,2}`)

func (Identifiers) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	for _, t := range d.Texts {
		rs := []rune(t.Text)
		utfOffsets := map[int]int{}
		index := 0
		for p := range t.Text {
			utfOffsets[p] = index
			index++
		}
		utfOffsets[len(t.Text)] = len(rs)
		identifiers := map[string][]int{}
		for _, m := range uuid.FindAllStringIndex(t.Text, -1) {
			start, end := utfOffsets[m[0]], utfOffsets[m[1]]
			if start > 0 && (u.Word(rs[start-1]) || rs[start-1] == '-') || end < len(rs) && (u.Word(rs[end]) || rs[end] == '-') {
				continue
			}
			v := t.Text[m[0]:m[1]]
			identifiers[v] = append(identifiers[v], start)
		}
		keys := []string{}
		for k := range identifiers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, v := range keys {
			positions := identifiers[v]
			result = append(result, finding("identifier.uuid", evidence.Object{"value": v, "count": len(positions), "repeated": len(positions) > 1}, evidence.Location(t, positions)))
		}
		folded := []rune(strings.ToLower(t.Text))
		for i, r := range folded {
			switch r {
			case 'İ', 'ı':
				folded[i] = 'i'
			case 'ſ':
				folded[i] = 's'
			case 'K':
				folded[i] = 'k'
			}
		}
		foldedText := string(folded)
		foldOffsets := map[int]int{}
		i := 0
		for p := range foldedText {
			foldOffsets[p] = i
			i++
		}
		foldOffsets[len(foldedText)] = len(folded)
		consumed := 0
		for _, m := range label.FindAllStringIndex(foldedText, -1) {
			start, end := foldOffsets[m[0]], foldOffsets[m[1]]
			if start < consumed || start > 0 && u.Word(rs[start-1]) {
				continue
			}
			if foldedText[m[0]:m[1]] == "generated" {
				j := end
				for j < len(rs) && u.Space(rs[j]) {
					j++
				}
				if j == end || j+2 > len(rs) || string(folded[j:j+2]) != "by" {
					continue
				}
				end = j + 2
			}
			for end < len(rs) && u.Space(rs[end]) {
				end++
			}
			if end >= len(rs) || (rs[end] != ':' && rs[end] != '=') {
				continue
			}
			end++
			// Greedy whitespace can backtrack by one character when it is itself the value.
			valueStart := end
			for end < len(rs) && u.Space(rs[end]) {
				end++
			}
			allowed := func(r rune) bool { return !strings.ContainsRune("\r\n<>", r) }
			if end >= len(rs) || !allowed(rs[end]) {
				for end > valueStart {
					end--
					if allowed(rs[end]) {
						break
					}
				}
				if end >= len(rs) || !allowed(rs[end]) {
					continue
				}
			}
			stop := end
			for stop < len(rs) && stop-end < 160 && allowed(rs[stop]) {
				stop++
			}
			if stop == end {
				continue
			}
			consumed = stop
			result = append(result, finding("provenance.text_marker", evidence.Object{"escaped_marker": u.Escaped(string(rs[start:stop]))}, evidence.Location(t, []int{start})))
		}
		for _, m := range encoded.FindAllStringIndex(t.Text, -1) {
			start, end := utfOffsets[m[0]], utfOffsets[m[1]]
			if start > 0 && (u.Word(rs[start-1]) || strings.ContainsRune("+/", rs[start-1])) || end < len(rs) && (u.Word(rs[end]) || strings.ContainsRune("+/=", rs[end])) {
				continue
			}
			v := t.Text[m[0]:m[1]]
			if len(v)%4 != 0 {
				continue
			}
			hasDigit, hasAlpha := false, false
			for _, r := range v {
				hasDigit = hasDigit || r >= '0' && r <= '9'
				hasAlpha = hasAlpha || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
			}
			if !hasDigit || !hasAlpha {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				continue
			}
			result = append(result, finding("text.encoded_candidate", evidence.Object{"value": v, "character_count": utf8.RuneCountInString(v), "base64_decoded_bytes": len(decoded)}, evidence.Location(t, []int{start})))
		}
	}
	return result
}
