// Package unicoderef pins Python-reference Unicode semantics across Go versions.
package unicoderef

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/runenames"
)

const Version = "15.0.0"

// Lower uses pinned full Unicode casing for descriptive extension/MIME hints.
func Lower(s string) string { return cases.Lower(language.Und).String(s) }

//go:embed categories15.txt
var categoryData string

type entry struct {
	start, end rune
	category   string
}

var categories = func() []entry {
	var rows []entry
	for _, line := range strings.Split(categoryData, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		p := strings.Split(line, ";")
		a, _ := strconv.ParseInt(p[0], 16, 32)
		b, _ := strconv.ParseInt(p[1], 16, 32)
		rows = append(rows, entry{rune(a), rune(b), p[2]})
	}
	return rows
}()

func Category(r rune) string {
	i := sort.Search(len(categories), func(i int) bool { return categories[i].end >= r })
	if i == len(categories) || r < 0 {
		return "Cn"
	}
	return categories[i].category
}
func Space(r rune) bool {
	return r >= 0x1c && r <= 0x20 || r >= 9 && r <= 13 || r == 0x85 || r == 0xa0 || r == 0x1680 || r >= 0x2000 && r <= 0x200a || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
}
func Letter(r rune) bool { return strings.HasPrefix(Category(r), "L") }
func Word(r rune) bool {
	c := Category(r)
	return r == '_' || strings.HasPrefix(c, "L") || strings.HasPrefix(c, "N")
}
func Name(r rune, fallback string) string {
	if Category(r) == "Cc" || Category(r) == "Cn" || Category(r) == "Co" {
		return fallback
	}
	// x/text omits algorithmic UCD names; Python expands these two families.
	if r >= 0xac00 && r <= 0xd7a3 {
		leading := []string{"G", "GG", "N", "D", "DD", "R", "M", "B", "BB", "S", "SS", "", "J", "JJ", "C", "K", "T", "P", "H"}
		vowels := []string{"A", "AE", "YA", "YAE", "EO", "E", "YEO", "YE", "O", "WA", "WAE", "OE", "YO", "U", "WEO", "WE", "WI", "YU", "EU", "YI", "I"}
		trailing := []string{"", "G", "GG", "GS", "N", "NJ", "NH", "D", "L", "LG", "LM", "LB", "LS", "LT", "LP", "LH", "M", "B", "BS", "S", "SS", "NG", "J", "C", "K", "T", "P", "H"}
		i := int(r - 0xac00)
		return "HANGUL SYLLABLE " + leading[i/588] + vowels[(i%588)/28] + trailing[i%28]
	}
	if Category(r) == "Lo" && (r >= 0x3400 && r <= 0x4dbf || r >= 0x4e00 && r <= 0x9fff || r >= 0x20000 && r <= 0x3ffff) {
		return fmt.Sprintf("CJK UNIFIED IDEOGRAPH-%X", r)
	}
	name := runenames.Name(r)
	if name == "" || strings.HasPrefix(name, "<") {
		return fallback
	}
	return name
}

// Escaped mirrors Python's unicode_escape representation for safe context display.
func Escaped(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r >= 32 && r <= 126 {
				b.WriteRune(r)
			} else if r <= 255 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else if r <= 65535 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		}
	}
	return b.String()
}

// Normalize preserves Python normalization, including non-stream-safe long runs.
// x/text inserts CGJ after 30 nonstarters; Python does not. Use its pinned UCD
// decomposition/composition primitives to normalize those uncommon inputs fully.
func Normalize(s string, compatibility bool) string {
	form, decomp := norm.NFC, norm.NFD
	if compatibility {
		form, decomp = norm.NFKC, norm.NFKD
	}
	fast := form.String(s)
	if fast == s {
		return fast
	}
	if strings.Count(fast, "\u034f") == strings.Count(s, "\u034f") && decomp.String(fast) == decomp.String(s) {
		return fast
	}
	var runes []rune
	for _, r := range s {
		runes = append(runes, []rune(decomp.String(string(r)))...)
	}
	ccc := func(r rune) uint8 { return norm.NFD.PropertiesString(string(r)).CCC() }
	// Canonical ordering, bounded by starters. Stable sort avoids quadratic runs.
	for start := 0; start < len(runes); {
		end := start
		if ccc(runes[start]) == 0 {
			end++
		}
		for end < len(runes) && ccc(runes[end]) != 0 {
			end++
		}
		lo := start
		if ccc(runes[start]) == 0 {
			lo++
		}
		sort.SliceStable(runes[lo:end], func(i, j int) bool { return ccc(runes[lo+i]) < ccc(runes[lo+j]) })
		start = end
	}
	var result []rune
	starter := -1
	var previous uint8
	for _, r := range runes {
		class := ccc(r)
		if starter >= 0 && (previous < class || previous == 0) {
			original := string([]rune{result[starter], r})
			pair := norm.NFC.String(original)
			// Reject supplementary-plane composition-key collisions: tag 'g'
			// plus acute must never become the unrelated Latin character 'ǵ'.
			if utf8.RuneCountInString(pair) == 1 && norm.NFD.String(pair) == norm.NFD.String(original) {
				result[starter], _ = utf8.DecodeRuneInString(pair)
				continue
			}
		}
		if class == 0 {
			starter = len(result)
		}
		result = append(result, r)
		previous = class
	}
	return string(result)
}
