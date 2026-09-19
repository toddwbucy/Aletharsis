package analyzers_test

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
)

func TestUnicodeHashesDoNotTransformSource(t *testing.T) {
	raw := "e\u0301ﬃ\u200b\u00a0"
	d := textDocument(raw)
	offsets := append([]int(nil), d.Texts[0].ByteOffsets...)
	fs := (analyzers.Unicode{}).Analyze(&d)
	// Expected normalization strings are explicit Unicode equivalences; hashing
	// uses the standard library rather than the application's hash helper.
	for key, text := range map[string]string{
		"raw_text_sha256":           raw,
		"nfc_text_sha256":           "éﬃ\u200b\u00a0",
		"nfkc_text_sha256":          "éffi\u200b ",
		"formatting_removed_sha256": "e\u0301ﬃ\u00a0",
	} {
		want := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		if d.Texts[0].Hashes[key] != want {
			t.Fatalf("%s hash mismatch", key)
		}
	}
	if d.Texts[0].Text != raw || !reflect.DeepEqual(d.Texts[0].ByteOffsets, offsets) {
		t.Fatal("source text/coordinates mutated")
	}
	if d.Texts[0].Normalization["formatting_removed_count"] != 1 || d.Texts[0].Normalization["nfc_changed"] != true || d.Texts[0].Normalization["nfkc_changed"] != true {
		t.Fatal(d.Texts[0].Normalization)
	}
	normalization := findingByID(fs, "unicode.normalization")
	if len(normalization) != 1 || normalization[0].Classification != "observed_fact" || normalization[0].Severity != "INFO" {
		t.Fatal(normalization)
	}
	zw := findingByID(fs, "unicode.zero_width")
	if len(zw) != 1 || !reflect.DeepEqual(zw[0].Location["character_offsets"], []int{3}) || !reflect.DeepEqual(zw[0].Location["byte_offsets"], []int{6}) {
		t.Fatal(zw)
	}
}

func TestUUIDBoundariesAndRepeatedCoordinates(t *testing.T) {
	const id = "12345678-1234-1234-1234-123456789abc"
	for _, affix := range []string{"é", "α", "_", "1", "-"} {
		for _, text := range []string{affix + id, id + affix} {
			d := textDocument(text)
			if fs := findingByID((analyzers.Identifiers{}).Analyze(&d), "identifier.uuid"); len(fs) != 0 {
				t.Fatalf("embedded token accepted: %q", text)
			}
		}
	}
	d := textDocument("é!" + id + " " + id)
	fs := findingByID((analyzers.Identifiers{}).Analyze(&d), "identifier.uuid")
	if len(fs) != 1 {
		t.Fatal(fs)
	}
	f := fs[0]
	if f.Evidence["value"] != id || f.Evidence["count"] != 2 || f.Evidence["repeated"] != true || f.Classification != "observed_fact" {
		t.Fatal(f)
	}
	if !reflect.DeepEqual(f.Location["character_offsets"], []int{2, 39}) || !reflect.DeepEqual(f.Location["byte_offsets"], []int{3, 40}) {
		t.Fatal(f.Location)
	}
}

func TestBase64CandidateBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{strings.Repeat("A", 30) + "0", false},
		{strings.Repeat("A", 31) + "0", true},
		{strings.Repeat("A", 32) + "0", false}, // threshold met but invalid quartet length
		{strings.Repeat("A", 35) + "0", true},
		{strings.Repeat("A", 32), false}, // no digit: ordinary text-like alphabet
		{"é" + strings.Repeat("A", 31) + "0", false},
		{strings.Repeat("A", 31) + "0_", false},
	} {
		d := textDocument(tc.value)
		fs := findingByID((analyzers.Identifiers{}).Analyze(&d), "text.encoded_candidate")
		wantCount := 0
		if tc.want {
			wantCount = 1
		}
		if len(fs) != wantCount {
			t.Fatalf("%q: %+v", tc.value, fs)
		}
		if tc.want && (fs[0].Evidence["base64_decoded_bytes"] != len(tc.value)/4*3 || fs[0].Classification != "suspicious_pattern") {
			t.Fatal(fs[0])
		}
	}
}

func TestEmojiKeycapsAndOrdinarySource(t *testing.T) {
	d := textDocument("x = 123 # ordinary * source")
	if fs := (analyzers.Emoji{}).Analyze(&d); len(fs) != 0 {
		t.Fatal("ASCII source falsely inventoried as emoji", fs)
	}
	for _, text := range []string{"# 😀", "\"\"\"😀\"\"\"", "value = '😀'"} {
		d := textDocument(text)
		fs := (analyzers.Emoji{}).Analyze(&d)
		if len(fs) != 1 || fs[0].Evidence["code_point"] != "U+1F600" || fs[0].Evidence["requires_context_review"] != true || fs[0].Classification != "observed_fact" {
			t.Fatal("source emoji omitted or overclaimed", fs)
		}
	}
	d = textDocument("1\ufe0f\u20e3 2\u20e3")
	fs := (analyzers.Emoji{}).Analyze(&d)
	if len(fs) != 2 {
		t.Fatal(fs)
	}
	for _, f := range fs {
		if f.Evidence["match_basis"] != "keycap_base" {
			t.Fatal(f)
		}
	}
}
