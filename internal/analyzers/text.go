package analyzers

import (
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type Text struct{}

func (Text) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	for _, t := range d.Texts {
		active := 0
		for _, count := range t.LineEndings {
			if count > 0 {
				active++
			}
		}
		if active > 1 || t.LineEndings["cr"] > 0 {
			details := evidence.Object{}
			for k, v := range t.LineEndings {
				details[k] = v
			}
			result = append(result, finding("text.line_endings", details, evidence.Object{"source": t.Source}))
		}
		if t.BOM != nil {
			result = append(result, finding("text.bom", evidence.Object{"hex": *t.BOM, "encoding": t.Encoding}, evidence.Object{"source": t.Source, "byte_offset": 0}))
		}
		rs := []rune(t.Text)
		positions := []int{}
		for i, r := range rs {
			if i > 0 && r == 0xfeff {
				positions = append(positions, i)
			}
		}
		if len(positions) > 0 {
			result = append(result, finding("text.embedded_bom", evidence.Object{"count": len(positions)}, evidence.Location(t, positions)))
		}
		tokenRune := func(r rune) bool { return u.Word(r) && r != '_' && u.Category(r) != "Nd" }
		for start := 0; start < len(rs); {
			if !tokenRune(rs[start]) {
				start++
				continue
			}
			end := start + 1
			for end < len(rs) && tokenRune(rs[end]) {
				end++
			}
			scripts := map[string]bool{}
			for _, r := range rs[start:end] {
				if u.Letter(r) {
					scripts[strings.Split(u.Name(r, ""), " ")[0]] = true
				}
			}
			if scripts["LATIN"] && (scripts["GREEK"] || scripts["CYRILLIC"]) {
				keys := []string{}
				for k := range scripts {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				result = append(result, finding("text.mixed_script", evidence.Object{"escaped_token": u.Escaped(string(rs[start:end])), "scripts": keys}, evidence.Location(t, []int{start})))
			}
			start = end
		}
	}
	return result
}

type Metadata struct{}

func (Metadata) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	keys := []string{}
	for k := range d.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, evidence.Finding{ID: "metadata.present", Severity: "INFO", Confidence: 1, Category: "metadata", Classification: "observed_fact", Title: k + " metadata present", Description: "Metadata is evidence, not proof of watermarking.", Evidence: evidence.Object{"key": k, "value": d.Metadata[k]}, Location: evidence.Object{}})
	}
	return result
}
