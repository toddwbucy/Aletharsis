package analyzers

import (
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type Unicode struct{}

func (Unicode) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	for _, t := range d.Texts {
		runes := []rune(t.Text)
		nfc, nfkc := u.Normalize(t.Text, false), u.Normalize(t.Text, true)
		var removed strings.Builder
		removedCount := 0
		inventory := map[rune][]int{}
		for i, r := range runes {
			if Removable(r) {
				removedCount++
			} else {
				removed.WriteRune(r)
			}
			if Kind(r) != "" {
				inventory[r] = append(inventory[r], i)
			}
		}
		t.Hashes = map[string]string{"raw_text_sha256": evidence.Hash([]byte(t.Text)), "nfc_text_sha256": evidence.Hash([]byte(nfc)), "nfkc_text_sha256": evidence.Hash([]byte(nfkc)), "formatting_removed_sha256": evidence.Hash([]byte(removed.String()))}
		t.Normalization = evidence.Object{"nfc_changed": nfc != t.Text, "nfkc_changed": nfkc != t.Text, "formatting_removed_count": removedCount, "unicode_version": u.Version, "hash_encoding": "utf-8", "removal_policy": "zero_width,bidi_control,variation_selector,tag,soft_hyphen"}
		for _, r := range sortedRunes(inventory) {
			positions := inventory[r]
			kind := Kind(r)
			bom := r == 0xfeff && len(positions) == 1 && positions[0] == 0 && t.BOM != nil
			severity := "LOW"
			if bom || kind == "combining" || kind == "unusual_whitespace" || kind == "soft_hyphen" || kind == "variation_selector" {
				severity = "INFO"
			}
			if kind == "control" {
				severity = "MEDIUM"
			}
			name := u.Name(r, "UNNAMED CONTROL")
			f := finding("unicode."+kind, evidence.Object{"code_point": Codepoint(r), "name": name, "count": len(positions), "leading_bom": bom, "contexts": contexts(runes, positions), "contexts_omitted": max(0, len(positions)-8)}, evidence.Location(t, positions))
			f.Severity = severity
			f.Title = Codepoint(r) + " " + name + " detected"
			result = append(result, f)
		}
		if nfc != t.Text || nfkc != t.Text {
			result = append(result, finding("unicode.normalization", evidence.Object{"nfc_changed": nfc != t.Text, "nfkc_changed": nfkc != t.Text}, evidence.Object{"source": t.Source}))
		}
	}
	return result
}

type Emoji struct{}

func (Emoji) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	for _, t := range d.Texts {
		runes := []rune(t.Text)
		inventory := map[rune][]int{}
		for i, r := range runes {
			if !emoji[r] {
				continue
			}
			if strings.ContainsRune("#*0123456789", r) {
				if !(i+1 < len(runes) && runes[i+1] == 0x20e3 || i+2 < len(runes) && runes[i+1] == 0xfe0f && runes[i+2] == 0x20e3) {
					continue
				}
			}
			inventory[r] = append(inventory[r], i)
		}
		for _, r := range sortedRunes(inventory) {
			positions := inventory[r]
			basis := "unicode_emoji_property"
			if strings.ContainsRune("#*0123456789", r) {
				basis = "keycap_base"
			}
			name := u.Name(r, "NAME UNAVAILABLE IN RUNTIME UNICODE DATABASE")
			f := finding("unicode.emoji", evidence.Object{"code_point": Codepoint(r), "name": name, "count": len(positions), "emoji_data_version": "17.0", "match_basis": basis, "count_unit": "code_point", "requires_context_review": true, "contexts": contexts(runes, positions), "contexts_omitted": max(0, len(positions)-8)}, evidence.Location(t, positions))
			f.Title = "Emoji-capable " + Codepoint(r) + " " + name + " detected"
			result = append(result, f)
		}
	}
	return result
}
