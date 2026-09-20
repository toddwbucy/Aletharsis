package identity_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

func TestAcceptedReportIdentityContracts(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "tests", "contracts", "fixtures", "*.json"))
	if err != nil || len(paths) != 8 {
		t.Fatalf("expected eight fixtures: %v, %v", paths, err)
	}
	configurations, selections := 0, 0
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var r struct {
				File       evidence.File     `json:"file"`
				Evidence   evidence.Document `json:"evidence"`
				Executions []struct {
					Config map[string]json.RawMessage `json:"config"`
				} `json:"executions"`
				Anchors []struct {
					Kind    string `json:"kind"`
					Locator struct {
						Spans []identity.TextSpan `json:"spans"`
						Hash  string              `json:"selected_text_sha256"`
					} `json:"locator"`
				} `json:"anchors"`
			}
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			for _, e := range r.Executions {
				if bytes.Equal(e.Config["sha256"], []byte("null")) {
					continue
				}
				var want string
				if err := json.Unmarshal(e.Config["sha256"], &want); err != nil {
					t.Fatal(err)
				}
				delete(e.Config, "sha256")
				config, err := json.Marshal(e.Config)
				if err != nil {
					t.Fatal(err)
				}
				hash, err := identity.Digest(identity.ConfigDomain, config, budget)
				if err != nil || hash != want {
					t.Fatalf("config %s: %s, %v; want %s", config, hash, err, want)
				}
				configurations++
			}
			for _, a := range r.Anchors {
				if a.Kind != "text" {
					t.Fatalf("unexpected anchor kind: %s", a.Kind)
				}
				// These committed public fixtures explicitly reference isolated_zwsp.txt;
				// production identity code never reopens a path from an imported report.
				if r.File.Path != "tests/fixtures/isolated_zwsp.txt" {
					t.Fatal("unexpected test source")
				}
				source, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "isolated_zwsp.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if len(r.Evidence.Texts) != 1 {
					t.Fatal("unexpected fixture text count")
				}
				hash, err := identity.TextSelectionDigest(source, r.Evidence.Texts[0], a.Locator.Spans, len(source), budget)
				if err != nil || hash != a.Locator.Hash {
					t.Fatalf("selection = %s, %v; want %s", hash, err, a.Locator.Hash)
				}
				selections++
			}
		})
	}
	if configurations != 24 || selections != 2 {
		t.Fatalf("coverage changed: configs=%d selections=%d", configurations, selections)
	}
}

func TestSelectionSourceEncodings(t *testing.T) {
	for _, tc := range []struct {
		name, hex, text string
		offsets         []int
		span            identity.TextSpan
	}{
		{"utf8", "41f09f988065cc81e2808b5a", "A😀e\u0301\u200bZ", []int{0, 1, 5, 6, 8, 11, 12}, identity.TextSpan{Scalar: identity.Region{Start: 4, End: 5}, Byte: identity.Region{Start: 8, End: 11}}},
		{"utf8-bom", "efbbbf41f09f988065cc81e2808b5a", "\ufeffA😀e\u0301\u200bZ", []int{0, 3, 4, 8, 9, 11, 14, 15}, identity.TextSpan{Scalar: identity.Region{Start: 5, End: 6}, Byte: identity.Region{Start: 11, End: 14}}},
		{"utf16-le", "fffe41003dd800de650001030b205a00", "\ufeffA😀e\u0301\u200bZ", []int{0, 2, 4, 8, 10, 12, 14, 16}, identity.TextSpan{Scalar: identity.Region{Start: 5, End: 6}, Byte: identity.Region{Start: 12, End: 14}}},
		{"utf16-be", "feff0041d83dde0000650301200b005a", "\ufeffA😀e\u0301\u200bZ", []int{0, 2, 4, 8, 10, 12, 14, 16}, identity.TextSpan{Scalar: identity.Region{Start: 5, End: 6}, Byte: identity.Region{Start: 12, End: 14}}},
		{"utf32-le", "fffe00004100000000f6010065000000010300000b2000005a000000", "\ufeffA😀e\u0301\u200bZ", []int{0, 4, 8, 12, 16, 20, 24, 28}, identity.TextSpan{Scalar: identity.Region{Start: 5, End: 6}, Byte: identity.Region{Start: 20, End: 24}}},
		{"utf32-be", "0000feff000000410001f60000000065000003010000200b0000005a", "\ufeffA😀e\u0301\u200bZ", []int{0, 4, 8, 12, 16, 20, 24, 28}, identity.TextSpan{Scalar: identity.Region{Start: 5, End: 6}, Byte: identity.Region{Start: 20, End: 24}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := hex.DecodeString(tc.hex)
			if err != nil {
				t.Fatal(err)
			}
			encoding, _ := parsers.Encoding(source)
			segment := &evidence.Text{Text: tc.text, Encoding: encoding, ByteOffsets: tc.offsets}
			spans := []identity.TextSpan{tc.span}
			before := bytes.Clone(source)
			offsets := append([]int(nil), segment.ByteOffsets...)
			hash, err := identity.TextSelectionDigest(source, segment, spans, len(source), budget)
			if err != nil {
				t.Fatal(err)
			}
			// Build the expected selection from literal coordinates and known selected
			// scalar, independently of the decoder and substring extraction under test.
			expected, err := json.Marshal([]map[string]any{{"span": tc.span, "text": "\u200b"}})
			if err != nil {
				t.Fatal(err)
			}
			want, err := identity.Digest(identity.SelectionDomain, expected, budget)
			if err != nil || hash != want {
				t.Fatalf("hash = %s, want %s, %v", hash, want, err)
			}
			if !bytes.Equal(source, before) || !reflect.DeepEqual(segment.ByteOffsets, offsets) || segment.Text != tc.text || spans[0] != tc.span {
				t.Fatal("mutated evidence")
			}
			bad := tc.span
			bad.Byte.Start++
			if hash, err := identity.TextSelectionDigest(source, segment, []identity.TextSpan{bad}, len(source), budget); err == nil || hash != "" {
				t.Fatal("accepted UTF-8/viewer bytes as source coordinates")
			}
		})
	}
}

func parsedText(t *testing.T, raw []byte) *evidence.Text {
	t.Helper()
	d, err := (parsers.TextParser{}).Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d.Texts[0]
}

func TestSelectionRejectsMismatchedEvidence(t *testing.T) {
	source := []byte("a😀e\u0301\u200bb")
	valid := identity.TextSpan{Scalar: identity.Region{Start: 4, End: 5}, Byte: identity.Region{Start: 8, End: 11}}
	for _, tc := range []struct {
		name  string
		edit  func(*evidence.Text)
		spans []identity.TextSpan
	}{
		{"missing", func(*evidence.Text) {}, nil},
		{"normalized", func(t *evidence.Text) { t.Text = "a😀é\u200bb" }, []identity.TextSpan{valid}},
		{"map", func(t *evidence.Text) { t.ByteOffsets[1] = 2 }, []identity.TextSpan{valid}},
		{"eof", func(t *evidence.Text) { t.ByteOffsets[len(t.ByteOffsets)-1]-- }, []identity.TextSpan{valid}},
		{"missing-boundary", func(t *evidence.Text) { t.ByteOffsets = t.ByteOffsets[:2] }, []identity.TextSpan{valid}},
		{"encoding", func(t *evidence.Text) { t.Encoding = "latin-1" }, []identity.TextSpan{valid}},
		{"empty-span", func(*evidence.Text) {}, []identity.TextSpan{{Scalar: identity.Region{Start: 4, End: 4}, Byte: valid.Byte}}},
		{"overlap", func(*evidence.Text) {}, []identity.TextSpan{valid, valid}},
		{"unsafe", func(*evidence.Text) {}, []identity.TextSpan{{Scalar: identity.Region{Start: 4, End: identity.MaxSafeInteger + 1}, Byte: valid.Byte}}},
		{"reversed", func(*evidence.Text) {}, []identity.TextSpan{{Scalar: identity.Region{Start: 5, End: 4}, Byte: valid.Byte}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			segment := parsedText(t, source)
			tc.edit(segment)
			hash, err := identity.TextSelectionDigest(source, segment, tc.spans, len(source), budget)
			if err == nil || hash != "" {
				t.Fatal("accepted mismatched selection")
			}
		})
	}
	segment := parsedText(t, source)
	for _, limit := range []int{0, len(source) - 1} {
		if _, err := identity.TextSelectionDigest(source, segment, []identity.TextSpan{valid}, limit, budget); !errors.Is(err, identity.ErrLimit) {
			t.Fatal(err)
		}
	}
	limited := budget
	limited.InputBytes = 16
	if _, err := identity.TextSelectionDigest(source, segment, []identity.TextSpan{valid}, len(source), limited); !errors.Is(err, identity.ErrLimit) {
		t.Fatal(err)
	}
	invalidSource := bytes.Clone(source)
	invalidSource[0] = 0xff
	if hash, err := identity.TextSelectionDigest(invalidSource, segment, []identity.TextSpan{valid}, len(source), budget); err == nil || hash != "" {
		t.Fatal("accepted invalid source")
	}
}

func TestSelectionPreservesDisjointBoundaries(t *testing.T) {
	source := []byte("abcd")
	segment := parsedText(t, source)
	pairs := [][]identity.TextSpan{
		{{Scalar: identity.Region{Start: 0, End: 1}, Byte: identity.Region{Start: 0, End: 1}}, {Scalar: identity.Region{Start: 3, End: 4}, Byte: identity.Region{Start: 3, End: 4}}},
		{{Scalar: identity.Region{Start: 0, End: 4}, Byte: identity.Region{Start: 0, End: 4}}},
		{{Scalar: identity.Region{Start: 0, End: 1}, Byte: identity.Region{Start: 0, End: 1}}, {Scalar: identity.Region{Start: 1, End: 4}, Byte: identity.Region{Start: 1, End: 4}}},
	}
	seen := map[string]bool{}
	for _, spans := range pairs {
		hash, err := identity.TextSelectionDigest(source, segment, spans, len(source), budget)
		if err != nil {
			t.Fatal(err)
		}
		if seen[hash] {
			t.Fatal("gap or selection boundaries collapsed")
		}
		seen[hash] = true
	}
}
