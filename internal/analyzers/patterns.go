package analyzers

import (
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

type Patterns struct{}

func period(s []rune) any {
	for p := 1; p <= min(32, len(s)/3); p++ {
		ok := true
		for i, r := range s {
			if r != s[i%p] {
				ok = false
				break
			}
		}
		if ok {
			return p
		}
	}
	return nil
}
func (Patterns) Analyze(d *evidence.Document) []evidence.Finding {
	result := []evidence.Finding{}
	for _, t := range d.Texts {
		rs := []rune(t.Text)
		states := []int{}
		sequence := []rune{}
		counts := map[rune]int{}
		for i, r := range rs {
			if r == 0x200b || r == 0x200c {
				states = append(states, i)
				sequence = append(sequence, r)
				counts[r]++
			}
		}
		if len(states) >= 32 && len(counts) == 2 && float64(min(counts[0x200b], counts[0x200c]))/float64(len(states)) >= .1 {
			adjacent := 0
			regular := true
			gap := states[1] - states[0]
			for i := 1; i < len(states); i++ {
				g := states[i] - states[i-1]
				if g == 1 {
					adjacent++
				}
				if g != gap {
					regular = false
				}
			}
			fraction := float64(adjacent) / float64(len(states)-1)
			if fraction >= .75 || regular {
				result = append(result, finding("pattern.zero_width_binary", evidence.Object{"count": len(states), "alphabet": evidence.Object{"U+200B": counts[0x200b], "U+200C": counts[0x200c]}, "adjacent_fraction": fraction, "constant_spacing": regular, "sequence_period": period(sequence), "thresholds": evidence.Object{"minimum_count": 32, "minimum_minority_fraction": .1, "minimum_adjacent_fraction_or_constant_spacing": .75}}, evidence.Location(t, states)))
			}
		}
		for _, group := range []string{"variation_selector", "tag"} {
			threshold := 8
			if group == "tag" {
				threshold = 16
			}
			for start := 0; start < len(rs); {
				if Kind(rs[start]) != group {
					start++
					continue
				}
				end := start + 1
				for end < len(rs) && Kind(rs[end]) == group {
					end++
				}
				if end-start >= threshold {
					positions := []int{}
					alphabet := map[rune]bool{}
					var projection strings.Builder
					for i := start; i < end; i++ {
						positions = append(positions, i)
						alphabet[rs[i]] = true
						if rs[i] >= 0xe0020 && rs[i] <= 0xe007e {
							projection.WriteRune(rs[i] - 0xe0000)
						}
					}
					details := evidence.Object{"count": end - start, "minimum_run": threshold, "alphabet_size": len(alphabet)}
					if group == "tag" {
						details["ascii_projection"] = projection.String()
					}
					result = append(result, finding("pattern."+group+"_run", details, evidence.Location(t, positions)))
				}
				start = end
			}
		}
		positions := []int{}
		for i, r := range rs {
			if Kind(r) == "zero_width" && r != 0x200d && !(i == 0 && r == 0xfeff) {
				positions = append(positions, i)
			}
		}
		if len(positions) >= 12 {
			gap := positions[1] - positions[0]
			regular := true
			for i := 2; i < len(positions); i++ {
				if positions[i]-positions[i-1] != gap {
					regular = false
					break
				}
			}
			if regular && gap > 1 {
				result = append(result, finding("pattern.periodic_insertion", evidence.Object{"count": len(positions), "character_interval": gap, "minimum_count": 12}, evidence.Location(t, positions)))
			}
		}
	}
	return result
}
