// Package analyzers classifies normalized evidence independently of file parsing.
package analyzers

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type Analyzer interface {
	Analyze(*evidence.Document) []evidence.Finding
}

//go:embed data/*
var data embed.FS
var templates = func() map[string]evidence.Finding {
	b, err := data.ReadFile("data/messages.json")
	if err != nil {
		panic(err)
	}
	v := map[string]evidence.Finding{}
	if err = json.Unmarshal(b, &v); err != nil {
		panic(err)
	}
	return v
}()

func Limitations() []string {
	b, _ := data.ReadFile("data/limitations.json")
	var v []string
	if err := json.Unmarshal(b, &v); err != nil {
		panic(err)
	}
	return v
}
func finding(id string, details, location evidence.Object) evidence.Finding {
	f, ok := templates[id]
	if !ok {
		panic("missing finding template: " + id)
	}
	f.Evidence = details
	f.Location = location
	return f
}
func Kind(r rune) string {
	if r >= 0x200b && r <= 0x200d || r >= 0x2060 && r <= 0x2064 || r == 0xfeff {
		return "zero_width"
	}
	if r == 0x61c || r == 0x200e || r == 0x200f || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
		return "bidi_control"
	}
	if r >= 0xfe00 && r <= 0xfe0f || r >= 0xe0100 && r <= 0xe01ef {
		return "variation_selector"
	}
	if r >= 0xe0000 && r <= 0xe007f {
		return "tag"
	}
	if r == 0xad {
		return "soft_hyphen"
	}
	if u.Space(r) && !strings.ContainsRune(" \t\r\n", r) {
		return "unusual_whitespace"
	}
	c := u.Category(r)
	if (c == "Cc" || c == "Cf") && !strings.ContainsRune("\t\r\n", r) {
		return "control"
	}
	if strings.HasPrefix(c, "M") {
		return "combining"
	}
	return ""
}
func Removable(r rune) bool {
	switch Kind(r) {
	case "zero_width", "bidi_control", "variation_selector", "tag", "soft_hyphen":
		return true
	}
	return false
}
func contexts(r []rune, positions []int) []evidence.Object {
	result := []evidence.Object{}
	for _, p := range positions[:min(8, len(positions))] {
		result = append(result, evidence.Object{"character_offset": p, "escaped_text": u.Escaped(string(r[max(0, p-16):min(len(r), p+17)]))})
	}
	return result
}
func sortedRunes(inventory map[rune][]int) []rune {
	keys := make([]rune, 0, len(inventory))
	for r := range inventory {
		keys = append(keys, r)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

var emoji = func() map[rune]bool {
	b, err := data.ReadFile("data/emoji-17.0.txt")
	if err != nil {
		panic(err)
	}
	v := map[rune]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])
		if line == "" {
			continue
		}
		bounds := strings.Split(line, "..")
		lo, _ := strconv.ParseInt(bounds[0], 16, 32)
		hi, _ := strconv.ParseInt(bounds[len(bounds)-1], 16, 32)
		for r := rune(lo); r <= rune(hi); r++ {
			v[r] = true
		}
	}
	return v
}()

func Codepoint(r rune) string { return fmt.Sprintf("U+%04X", r) }
