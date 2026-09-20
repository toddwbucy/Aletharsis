package identity_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

var budget = identity.Limits{InputBytes: 1 << 20, OutputBytes: 1 << 20, Nodes: 100000, Depth: 64}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPortableIdentityVectors(t *testing.T) {
	var vectors struct {
		Identity []struct {
			ID, Domain string
			Value      json.RawMessage
			Canonical  string `json:"canonical_utf8_hex"`
			Hash       string `json:"sha256"`
		} `json:"identity_vectors"`
		Numbers []struct{ Input, Canonical string } `json:"future_general_jcs_numbers"`
	}
	if err := json.Unmarshal(readFixture(t, "identity-vectors.json"), &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors.Identity) != 6 || len(vectors.Numbers) != 7 {
		t.Fatal("portable fixture inventory changed")
	}
	for _, v := range vectors.Identity {
		t.Run(v.ID, func(t *testing.T) {
			domain, err := json.Marshal(v.Domain)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte(fmt.Sprintf(`{"domain":%s,"value":%s}`, domain, v.Value))
			canonical, err := identity.Canonicalize(raw, budget)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := hex.DecodeString(v.Canonical)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonical, expected) {
				t.Fatalf("canonical = %s; want %s", canonical, expected)
			}
			hash, err := identity.Digest(v.Domain, v.Value, budget)
			if err != nil || hash != v.Hash {
				t.Fatalf("digest = %s, %v; want %s", hash, err, v.Hash)
			}
		})
	}
	for _, v := range vectors.Numbers {
		t.Run(v.Input, func(t *testing.T) {
			got, err := identity.Canonicalize([]byte(v.Input), budget)
			if err != nil || string(got) != v.Canonical {
				t.Fatalf("canonical = %s, %v; want %s", got, err, v.Canonical)
			}
		})
	}
}

func TestRFC8785Binary64Samples(t *testing.T) {
	// Numerical conformance data from RFC 8785 Appendix B:
	// https://www.rfc-editor.org/rfc/rfc8785#appendix-B
	cases := []struct {
		bits uint64
		want string
	}{
		{0x0000000000000000, "0"}, {0x8000000000000000, "0"},
		{0x0000000000000001, "5e-324"}, {0x8000000000000001, "-5e-324"},
		{0x7fefffffffffffff, "1.7976931348623157e+308"}, {0xffefffffffffffff, "-1.7976931348623157e+308"},
		{0x4340000000000000, "9007199254740992"}, {0xc340000000000000, "-9007199254740992"},
		{0x4430000000000000, "295147905179352830000"},
		{0x44b52d02c7e14af5, "9.999999999999997e+22"}, {0x44b52d02c7e14af6, "1e+23"}, {0x44b52d02c7e14af7, "1.0000000000000001e+23"},
		{0x444b1ae4d6e2ef4e, "999999999999999700000"}, {0x444b1ae4d6e2ef4f, "999999999999999900000"}, {0x444b1ae4d6e2ef50, "1e+21"},
		{0x3eb0c6f7a0b5ed8c, "9.999999999999997e-7"}, {0x3eb0c6f7a0b5ed8d, "0.000001"},
		{0x41b3de4355555553, "333333333.3333332"}, {0x41b3de4355555554, "333333333.33333325"}, {0x41b3de4355555555, "333333333.3333333"},
		{0x41b3de4355555556, "333333333.3333334"}, {0x41b3de4355555557, "333333333.33333343"},
		{0xbecbf647612f3696, "-0.0000033333333333333333"}, {0x43143ff3c1cb0959, "1424953923781206.2"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%016x", tc.bits), func(t *testing.T) {
			raw := []byte(strconv.FormatFloat(math.Float64frombits(tc.bits), 'g', -1, 64))
			got, err := identity.Canonicalize(raw, budget)
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestCanonicalStringsAndOrdering(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{` { "z": [ {"b":2,"a":1} ], "a":true } `, `{"a":true,"z":[{"a":1,"b":2}]}`},
		{`{"\ue000":0,"\ud83d\ude00":1,"\r":2}`, "{\"\\r\":2,\"😀\":1,\"\ue000\":0}"},
		{`"\u003c\u003e\u0026\u2028\u2029"`, "\"<>&\u2028\u2029\""},
		{`"\u0000\u0008\u0009\u000a\u000c\u000d\u001f\/"`, `"\u0000\b\t\n\f\r\u001f/"`},
		{`"\\ud800"`, `"\\ud800"`},
		{`"e\u0301"`, "\"e\u0301\""},
		{`[true,false,null,"�"]`, `[true,false,null,"�"]`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := identity.Canonicalize([]byte(tc.raw), budget)
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestInvalidJSONAndUnicode(t *testing.T) {
	for _, raw := range []string{``, `null null`, `{"a":1,"\u0061":2}`, `{"x":{"a":1,"a":2}}`,
		`"\ud800"`, `"\udc00"`, `"\ud800\u0041"`, `"\ud800\ud800"`, `"\uD83D\uDE00\udfff"`,
		`"\uZZZZ"`, `"\u123"`, `"\x00"`, `"unterminated`, "\"\xff\"", "\"\xed\xa0\x80\"", `NaN`, `Infinity`, `1e999`, `[1,]`, `{"x":}`, `01`, `+1`, `[}`, `{"x":1} trailing`,
	} {
		t.Run(fmt.Sprintf("%q", raw), func(t *testing.T) {
			got, err := identity.Canonicalize([]byte(raw), budget)
			if err == nil || got != nil {
				t.Fatalf("accepted invalid JSON: %q (%v)", got, err)
			}
		})
	}
}

func TestLimits(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		limits    identity.Limits
	}{
		{"zero", "null", identity.Limits{}},
		{"input", "null", identity.Limits{InputBytes: 3, OutputBytes: 4, Nodes: 1, Depth: 1}},
		{"output", "null", identity.Limits{InputBytes: 4, OutputBytes: 3, Nodes: 1, Depth: 1}},
		{"depth", "[[0]]", identity.Limits{InputBytes: 9, OutputBytes: 9, Nodes: 3, Depth: 2}},
		{"nodes", "[0,1]", identity.Limits{InputBytes: 9, OutputBytes: 9, Nodes: 2, Depth: 3}},
		{"keys", `{"a":0}`, identity.Limits{InputBytes: 9, OutputBytes: 9, Nodes: 2, Depth: 3}},
		{"excess-depth-budget", "0", identity.Limits{InputBytes: 9, OutputBytes: 9, Nodes: 3, Depth: 257}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := identity.Canonicalize([]byte(tc.raw), tc.limits)
			if !errors.Is(err, identity.ErrLimit) || got != nil {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
	// Exact boundary succeeds; an empty object/array is one node.
	for _, raw := range []string{"null", "{}", "[]"} {
		got, err := identity.Canonicalize([]byte(raw), identity.Limits{InputBytes: len(raw), OutputBytes: len(raw), Nodes: 1, Depth: 1})
		if err != nil || string(got) != raw {
			t.Fatalf("boundary %q: %q, %v", raw, got, err)
		}
	}
	deep := strings.Repeat("[", 300) + "0" + strings.Repeat("]", 300)
	if _, err := identity.Canonicalize([]byte(deep), budget); !errors.Is(err, identity.ErrLimit) {
		t.Fatal(err)
	}
}

func TestReportBytesAndDomainSeparation(t *testing.T) {
	first, second := []byte(`{"a":1}`), []byte("{\n\"a\": 1.0\n}")
	if identity.ExactBytes(first) == identity.ExactBytes(second) {
		t.Fatal("raw identity normalized JSON")
	}
	a, err := identity.Digest("test/1", first, budget)
	if err != nil {
		t.Fatal(err)
	}
	b, err := identity.Digest("test/1", second, budget)
	if err != nil {
		t.Fatal(err)
	}
	c, err := identity.Digest("test/2", first, budget)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == c {
		t.Fatal("incorrect canonical or domain identity")
	}
	for _, domain := range []string{"", string([]byte{0xff})} {
		if digest, err := identity.Digest(domain, first, budget); err == nil || digest != "" {
			t.Fatal("accepted invalid domain")
		}
	}
	limited := budget
	limited.InputBytes = len(first)
	if digest, err := identity.Digest("test/1", first, limited); !errors.Is(err, identity.ErrLimit) || digest != "" {
		t.Fatal("envelope exceeded input limit")
	}
}

func FuzzCanonicalize(f *testing.F) {
	for _, seed := range []string{`{"a":1}`, `"\ud800"`, `{"a":1,"a":2}`, `[null,-0.0,1e-7]`, `{"😀":0,"\ue000":1}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		limit := identity.Limits{InputBytes: 8192, OutputBytes: 8192, Nodes: 2048, Depth: 32}
		before := bytes.Clone(raw)
		got, err := identity.Canonicalize(raw, limit)
		if !bytes.Equal(raw, before) {
			t.Fatal("mutated input")
		}
		if err != nil {
			if got != nil {
				t.Fatal("partial output")
			}
			return
		}
		if !json.Valid(got) {
			t.Fatal("invalid canonical output")
		}
		again, err := identity.Canonicalize(got, limit)
		if err != nil || !bytes.Equal(got, again) {
			t.Fatalf("non-idempotent output: %v", err)
		}
	})
}

func TestDigestRejectsEnvelopeInjection(t *testing.T) {
	for _, raw := range []string{`1,"extra":true`, `1,"domain":"other/1"`, `0} {"x":1`, `null null`, ``, `"\ud800"`, `{"x":1,"x":2}`} {
		if hash, err := identity.Digest("test/1", []byte(raw), budget); err == nil || hash != "" {
			t.Fatalf("accepted value fragment %q", raw)
		}
	}
}

func TestIndependentECMAScriptNumberOracle(t *testing.T) {
	raw, err := os.ReadFile("testdata/ecmascript-numbers.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ExactBytes(raw) != "f01c53341d8b63af4566f24747a7bf6a9889200a627bd312068f2ef26fd37402" {
		t.Fatal("number oracle bytes changed")
	}
	rows := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	if len(rows) != 256 {
		t.Fatal("number oracle inventory changed")
	}
	for _, row := range rows {
		var tc struct{ Bits, Canonical string }
		if err := json.Unmarshal(row, &tc); err != nil {
			t.Fatal(err)
		}
		bits, err := strconv.ParseUint(tc.Bits, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(tc.Bits, func(t *testing.T) {
			input := strconv.FormatFloat(math.Float64frombits(bits), 'g', -1, 64)
			got, err := identity.Canonicalize([]byte(input), budget)
			if err != nil || string(got) != tc.Canonical {
				t.Fatalf("canonical = %s, %v; V8 expected %s", got, err, tc.Canonical)
			}
		})
	}
}

func TestConcurrentCanonicalization(t *testing.T) {
	for i := 0; i < 16; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			raw := []byte(`{"z":[1.0,-0.0,1e21],"😀":"e\u0301","a":true}`)
			want := "{\"a\":true,\"z\":[1,0,1e+21],\"😀\":\"é\"}"
			for n := 0; n < 20; n++ {
				got, err := identity.Canonicalize(raw, budget)
				if err != nil || string(got) != want {
					t.Fatalf("got %q, %v; want %q", got, err, want)
				}
			}
		})
	}
}
