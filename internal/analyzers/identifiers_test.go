package analyzers

import (
	"github.com/toddwbucy/Aletharsis/internal/parsers"
	"testing"
)

func TestReferenceProvenanceWhitespace(t *testing.T) {
	for _, tc := range []struct{ input, marker string }{{"author: \n", "author: "}, {"author:\t\r\n", "author:\\t"}, {"author:\n", ""}, {"generated\u3000by: Tool", "generated\\u3000by: Tool"}, {"éauthor: no", ""}, {"author: yes", "author: yes"}} {
		d, err := (parsers.TextParser{}).Parse([]byte(tc.input))
		if err != nil {
			t.Fatal(err)
		}
		fs := (Identifiers{}).Analyze(&d)
		if tc.marker == "" {
			if len(fs) != 0 {
				t.Fatalf("unexpected marker for %q", tc.input)
			}
		} else if len(fs) != 1 || fs[0].Evidence["escaped_marker"] != tc.marker {
			t.Fatalf("incorrect marker for %q: %+v", tc.input, fs)
		}
	}
}
