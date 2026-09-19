package reporters_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
)

func FuzzJSONEvidence(f *testing.F) {
	for _, s := range []string{"", "ordinary", "\x00\x1b\n\r", "é😀\u202e\u200b", "\\\"<literal>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 || !utf8.ValidString(s) {
			t.Skip("valid extracted strings, at most 4 KiB")
		}
		original := evidence.Object{"source": s, "evidence": evidence.Object{"text": s, "present": true}, "positions": []int{0, len(s)}}
		for _, pretty := range []bool{false, true} {
			got, err := reporters.JSON(original, pretty)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := reporters.JSON(original, pretty)
			if err != nil || repeated != got {
				t.Fatal("nondeterministic serialization")
			}
			for _, r := range got {
				if r > 127 || r == 0x1b {
					t.Fatal("unsafe raw character in serialized evidence")
				}
			}
			standard, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var expected, actual any
			if err := json.Unmarshal(standard, &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(got), &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal("evidence changed during serialization")
			}
		}
	})
}
