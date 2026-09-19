package analyzers_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func textDocument(s string) evidence.Document {
	// Go range supplies UTF-8 byte positions independently of Aletharsis decoding.
	offsets := []int{}
	for offset := range s {
		offsets = append(offsets, offset)
	}
	offsets = append(offsets, len(s))
	return evidence.Document{Texts: []*evidence.Text{{Source: "test", Text: s, ByteOffsets: offsets}}}
}

func findingByID(fs []evidence.Finding, id string) []evidence.Finding {
	result := []evidence.Finding{}
	for _, f := range fs {
		if f.ID == id {
			result = append(result, f)
		}
	}
	return result
}

func binarySymbols(n int) string {
	return strings.Repeat("\u200b\u200c", n/2) + strings.Repeat("\u200b", n%2)
}

func TestBinaryPatternCountAndMinorityBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"31", binarySymbols(31), false}, {"32", binarySymbols(32), true}, {"33", binarySymbols(33), true},
		{"minority_below", strings.Repeat("\u200b", 37) + strings.Repeat("\u200c", 3), false},
		{"minority_at", strings.Repeat("\u200b", 36) + strings.Repeat("\u200c", 4), true},
		{"minority_above", strings.Repeat("\u200b", 35) + strings.Repeat("\u200c", 5), true},
		{"single_symbol", strings.Repeat("\u200b", 40), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := textDocument(tc.text)
			fs := findingByID((analyzers.Patterns{}).Analyze(&d), "pattern.zero_width_binary")
			wantCount := 0
			if tc.want {
				wantCount = 1
			}
			if len(fs) != wantCount {
				t.Fatalf("findings: %+v", fs)
			}
			if tc.want {
				f := fs[0]
				if f.Evidence["count"] != len([]rune(tc.text)) || f.Classification != "suspicious_pattern" || f.Severity != "HIGH" {
					t.Fatalf("incorrect evidence/classification: %+v", f)
				}
			}
		})
	}
}

func TestBinaryAdjacencyAndRegularSpacing(t *testing.T) {
	// 33 symbols have 32 gaps. 23/32 is below .75, 24/32 is exactly .75.
	for _, adjacent := range []int{23, 24, 25} {
		t.Run(fmt.Sprint(adjacent), func(t *testing.T) {
			var b strings.Builder
			for i, r := range []rune(binarySymbols(33)) {
				if i > adjacent {
					b.WriteByte('x')
				}
				b.WriteRune(r)
			}
			d := textDocument(b.String())
			fs := findingByID((analyzers.Patterns{}).Analyze(&d), "pattern.zero_width_binary")
			wantCount := 0
			if adjacent >= 24 {
				wantCount = 1
			}
			if len(fs) != wantCount {
				t.Fatalf("adjacent gaps %d: %+v", adjacent, fs)
			}
			if len(fs) == 1 && (fs[0].Evidence["adjacent_fraction"] != float64(adjacent)/32 || fs[0].Evidence["constant_spacing"] != false) {
				t.Fatal(fs[0].Evidence)
			}
		})
	}
	d := textDocument(strings.Repeat("\u200bx\u200cx", 16))
	fs := findingByID((analyzers.Patterns{}).Analyze(&d), "pattern.zero_width_binary")
	if len(fs) != 1 || fs[0].Evidence["adjacent_fraction"] != float64(0) || fs[0].Evidence["constant_spacing"] != true || fs[0].Evidence["sequence_period"] != 2 {
		t.Fatalf("regular spacing: %+v", fs)
	}
}

func TestRunBoundariesAndSeparators(t *testing.T) {
	for _, tc := range []struct {
		id, symbol string
		threshold  int
	}{
		{"pattern.variation_selector_run", "\U000e0100", 8},
		{"pattern.tag_run", "\U000e0061", 16},
	} {
		for _, n := range []int{tc.threshold - 1, tc.threshold, tc.threshold + 1} {
			t.Run(fmt.Sprintf("%s/%d", tc.id, n), func(t *testing.T) {
				d := textDocument("é" + strings.Repeat(tc.symbol, n) + "x")
				fs := findingByID((analyzers.Patterns{}).Analyze(&d), tc.id)
				wantCount := 0
				if n >= tc.threshold {
					wantCount = 1
				}
				if len(fs) != wantCount {
					t.Fatalf("findings: %+v", fs)
				}
				if len(fs) == 1 {
					chars, bytes := []int{}, []int{}
					for i := 0; i < n; i++ {
						chars = append(chars, i+1)
						bytes = append(bytes, 2+4*i)
					}
					if fs[0].Evidence["count"] != n || !reflect.DeepEqual(fs[0].Location["character_offsets"], chars) || !reflect.DeepEqual(fs[0].Location["byte_offsets"], bytes) {
						t.Fatalf("count/coordinates: %+v", fs[0])
					}
					if tc.id == "pattern.tag_run" && fs[0].Evidence["ascii_projection"] != strings.Repeat("a", n) {
						t.Fatal(fs[0].Evidence)
					}
				}
			})
		}
		d := textDocument(strings.Repeat(tc.symbol, tc.threshold-1) + "x" + strings.Repeat(tc.symbol, tc.threshold-1))
		if fs := findingByID((analyzers.Patterns{}).Analyze(&d), tc.id); len(fs) != 0 {
			t.Fatal("separated short runs combined", fs)
		}
	}
}

func TestPeriodicInsertionBoundaries(t *testing.T) {
	for _, n := range []int{11, 12, 13} {
		d := textDocument(strings.Repeat("é\u2060", n))
		fs := findingByID((analyzers.Patterns{}).Analyze(&d), "pattern.periodic_insertion")
		wantCount := 0
		if n >= 12 {
			wantCount = 1
		}
		if len(fs) != wantCount {
			t.Fatalf("count %d: %+v", n, fs)
		}
		if len(fs) == 1 && (fs[0].Evidence["character_interval"] != 2 || fs[0].Evidence["count"] != n) {
			t.Fatal(fs[0].Evidence)
		}
	}
	for _, text := range []string{strings.Repeat("\u2060", 12), strings.Repeat("a\u2060", 11) + "aa\u2060"} {
		d := textDocument(text)
		if fs := findingByID((analyzers.Patterns{}).Analyze(&d), "pattern.periodic_insertion"); len(fs) != 0 {
			t.Fatal("contiguous or irregular insertion flagged", fs)
		}
	}
}

func TestLegitimateUnicodeDoesNotImplyEncoding(t *testing.T) {
	for _, text := range []string{
		"می\u200cروم و خانه\u200cها", "مرحبا بالعالم", "עברית רגילה", "normal café prose",
		strings.Repeat("👩\u200d💻 ", 40), strings.Repeat("❤️ ", 20), "1\ufe0f\u20e3 2\u20e3",
		"🏴\U000e0067\U000e0062\U000e0065\U000e006e\U000e0067\U000e007f",
		"\ufeff" + strings.Repeat("a\u2060", 11),
	} {
		d := textDocument(text)
		if fs := (analyzers.Patterns{}).Analyze(&d); len(fs) != 0 {
			t.Fatalf("encoding warning for %q: %+v", text, fs)
		}
	}
	// Emoji remain inventoried even when no encoding pattern is inferred.
	d := textDocument("👩\u200d💻")
	if fs := (analyzers.Emoji{}).Analyze(&d); len(fs) != 2 {
		t.Fatalf("emoji inventory missing: %+v", fs)
	}
	if fs := findingByID((analyzers.Unicode{}).Analyze(&d), "unicode.zero_width"); len(fs) != 1 {
		t.Fatal("ZWJ inventory missing")
	}
}
