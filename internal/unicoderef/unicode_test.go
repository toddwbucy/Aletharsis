package unicoderef_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

type oracle struct {
	Names      string                             `json:"names_sha256"`
	Properties string                             `json:"properties_sha256"`
	NFC        string                             `json:"all_scalars_nfc_sha256"`
	NFKC       string                             `json:"all_scalars_nfkc_sha256"`
	Vectors    []struct{ Text, NFC, NFKC string } `json:"normalization_vectors"`
}

func load(t *testing.T) oracle {
	t.Helper()
	b, err := os.ReadFile("../../reference/python-behavior/unicode.json")
	if err != nil {
		t.Fatal(err)
	}
	var o oracle
	if err = json.Unmarshal(b, &o); err != nil {
		t.Fatal(err)
	}
	return o
}
func TestAllScalarProperties(t *testing.T) {
	o := load(t)
	h := sha256.New()
	bit := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	for r := rune(0); r <= 0x10ffff; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		prefix := ""
		if u.Letter(r) {
			prefix = strings.Split(u.Name(r, ""), " ")[0]
		}
		fmt.Fprintf(h, "%X;%s;%d;%d;%d;%s\n", r, u.Category(r), bit(u.Space(r)), bit(u.Letter(r)), bit(u.Word(r)), prefix)
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != o.Properties {
		t.Fatal("Unicode properties differ from Python oracle")
	}
}
func TestNormalizationVectors(t *testing.T) {
	for i, v := range load(t).Vectors {
		if u.Normalize(v.Text, false) != v.NFC || u.Normalize(v.Text, true) != v.NFKC {
			t.Errorf("normalization vector %d differs", i)
		}
	}
}
func TestAllScalarNormalization(t *testing.T) {
	o := load(t)
	var b strings.Builder
	for r := rune(0); r <= 0x10ffff; r++ {
		if r < 0xd800 || r > 0xdfff {
			b.WriteRune(r)
		}
	}
	if evidence.Hash([]byte(u.Normalize(b.String(), false))) != o.NFC || evidence.Hash([]byte(u.Normalize(b.String(), true))) != o.NFKC {
		t.Fatal("all-scalar normalization differs")
	}
}
func TestControlNameAndSafeContexts(t *testing.T) {
	if u.Name(0, "UNNAMED CONTROL") != "UNNAMED CONTROL" {
		t.Fatal("control alias changes reference semantics")
	}
	if u.Escaped("😀\x1b\n\u200b") != `\U0001f600\x1b\n\u200b` {
		t.Fatal("unsafe context escaping")
	}
	if analyzers.Kind('\u200d') != "zero_width" {
		t.Fatal("joiner inventory missing")
	}
}

func TestReferenceFullLowercase(t *testing.T) {
	for a, b := range map[string]string{".İ": ".i\u0307", ".ΟΣ": ".ος", ".PY": ".py"} {
		if got := u.Lower(a); got != b {
			t.Fatalf("%q lower=%q, want %q", a, got, b)
		}
	}
}

func TestSupplementaryTagsDoNotComposeAsASCII(t *testing.T) {
	for _, s := range []string{"\U000e0067\u0301", "\U00010061\u0301"} {
		if u.Normalize(s, false) != s || u.Normalize(s, true) != s {
			t.Fatalf("supplementary input changed: %q", s)
		}
	}
}

func TestAllInventoriedNames(t *testing.T) {
	raw, err := os.ReadFile("../analyzers/data/emoji-17.0.txt")
	if err != nil {
		t.Fatal(err)
	}
	emoji := map[rune]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])
		if line == "" {
			continue
		}
		bounds := strings.Split(line, "..")
		lo, _ := strconv.ParseInt(bounds[0], 16, 32)
		hi, _ := strconv.ParseInt(bounds[len(bounds)-1], 16, 32)
		for r := rune(lo); r <= rune(hi); r++ {
			emoji[r] = true
		}
	}
	h := sha256.New()
	for r := rune(0); r <= 0x10ffff; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		if analyzers.Kind(r) != "" || emoji[r] {
			fmt.Fprintf(h, "%X;%s\n", r, u.Name(r, "UNNAMED CONTROL"))
		}
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != load(t).Names {
		t.Fatal("inventoried code-point names differ from Python")
	}
}
