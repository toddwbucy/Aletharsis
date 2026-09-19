package unicoderef_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
	"golang.org/x/text/unicode/norm"
)

func FuzzNormalization(f *testing.F) {
	for _, s := range []string{"", "plain", "e\u0301ﬃ", "\u1100\u1161\u11a8", "👩\u200d💻\u202e", "\U000e0067\u0301", "A" + strings.Repeat("\u0305", 64) + "\u0301\u0323", "\u034f" + strings.Repeat("\u0300", 32)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 || !utf8.ValidString(s) {
			t.Skip("valid UTF-8 domain, at most 4 KiB")
		}
		for _, compatibility := range []bool{false, true} {
			got := u.Normalize(s, compatibility)
			if !utf8.ValidString(got) {
				t.Fatal("normalization produced invalid UTF-8")
			}
			if u.Normalize(got, compatibility) != got {
				t.Fatal("normalization is not idempotent")
			}
			if strings.Count(got, "\u034f") != strings.Count(s, "\u034f") {
				t.Fatal("normalization inserted/removed original CGJ")
			}
			form := norm.NFD
			if compatibility {
				form = norm.NFKD
			}
			// x/text inserts stream-safety CGJs in long nonstarter runs. Its
			// decomposition is an equivalence oracle only when neither side
			// required that transformation. Long runs still check idempotence
			// and CGJ preservation, alongside the existing frozen vectors.
			before, after := form.String(s), form.String(got)
			streamSafe := strings.Count(before, "\u034f") == strings.Count(s, "\u034f") && strings.Count(after, "\u034f") == strings.Count(got, "\u034f")
			if streamSafe && before != after {
				t.Fatal("canonical/compatibility equivalence lost")
			}
		}
	})
}
