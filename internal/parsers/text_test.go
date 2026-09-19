package parsers_test

import (
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

func bytesFromHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLiteralEncodingCoordinates(t *testing.T) {
	// Bytes and scalar boundaries are written explicitly, not encoded by the
	// implementation under test. BOMs and CR/LF each retain their own positions.
	for _, tc := range []struct {
		name, raw, encoding, bom, text string
		offsets                        []int
	}{
		{"empty", "", "utf-8", "", "", []int{0}},
		{"utf8", "41c3a9f09f988065cc81e2808d0d0a", "utf-8", "", "Aé😀e\u0301\u200d\r\n", []int{0, 1, 3, 7, 8, 10, 13, 14, 15}},
		{"utf8_bom", "efbbbf41", "utf-8", "efbbbf", "\ufeffA", []int{0, 3, 4}},
		{"utf16le", "fffe4100e9003dd800de650001030d200d000a00", "utf-16-le", "fffe", "\ufeffAé😀e\u0301\u200d\r\n", []int{0, 2, 4, 6, 10, 12, 14, 16, 18, 20}},
		{"utf16be", "feff004100e9d83dde0000650301200d000d000a", "utf-16-be", "feff", "\ufeffAé😀e\u0301\u200d\r\n", []int{0, 2, 4, 6, 10, 12, 14, 16, 18, 20}},
		{"utf32le", "fffe000041000000e900000000f6010065000000010300000d2000000d0000000a000000", "utf-32-le", "fffe0000", "\ufeffAé😀e\u0301\u200d\r\n", []int{0, 4, 8, 12, 16, 20, 24, 28, 32, 36}},
		{"utf32be", "0000feff00000041000000e90001f60000000065000003010000200d0000000d0000000a", "utf-32-be", "0000feff", "\ufeffAé😀e\u0301\u200d\r\n", []int{0, 4, 8, 12, 16, 20, 24, 28, 32, 36}},
		{"replacement_is_valid", "efbfbd", "utf-8", "", "\ufffd", []int{0, 3}},
		{"maximum_scalar", "f48fbfbf", "utf-8", "", "\U0010ffff", []int{0, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := bytesFromHex(t, tc.raw)
			d, err := (parsers.TextParser{}).Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Texts) != 1 {
				t.Fatalf("texts: %d", len(d.Texts))
			}
			got := d.Texts[0]
			if got.Text != tc.text || got.Encoding != tc.encoding || !reflect.DeepEqual(got.ByteOffsets, tc.offsets) {
				t.Fatalf("text/encoding/coordinates: %+v", got)
			}
			bom := ""
			if got.BOM != nil {
				bom = *got.BOM
			}
			if bom != tc.bom {
				t.Fatalf("BOM %q, want %q", bom, tc.bom)
			}
			if d.Structure["byte_length"] != len(raw) {
				t.Fatal("wrong source byte length")
			}
		})
	}
}

func TestStrictDecodeFailures(t *testing.T) {
	for _, tc := range []struct {
		name, encoding, raw, reason string
		start, end                  int
	}{
		{"utf8_start", "utf-8", "4180", "invalid start byte", 1, 2},
		{"utf8_overlong_start", "utf-8", "c080", "invalid start byte", 0, 1},
		{"utf8_overlong_three", "utf-8", "e08080", "invalid continuation byte", 0, 1},
		{"utf8_overlong_four", "utf-8", "f0808080", "invalid continuation byte", 0, 1},
		{"utf8_surrogate", "utf-8", "eda080", "invalid continuation byte", 0, 1},
		{"utf8_out_of_range", "utf-8", "f4908080", "invalid continuation byte", 0, 1},
		{"utf8_bad_second", "utf-8", "41e24180", "invalid continuation byte", 1, 2},
		{"utf8_bad_third", "utf-8", "41e28241", "invalid continuation byte", 1, 3},
		{"utf8_bad_fourth", "utf-8", "41f09f9841", "invalid continuation byte", 1, 4},
		{"utf8_truncated_two", "utf-8", "41c2", "unexpected end of data", 1, 2},
		{"utf8_truncated_three", "utf-8", "41e282", "unexpected end of data", 1, 3},
		{"utf8_truncated_four", "utf-8", "41f09f98", "unexpected end of data", 1, 4},
		{"utf16le_odd", "utf-16-le", "410042", "truncated data", 2, 3},
		{"utf16be_odd", "utf-16-be", "004100", "truncated data", 2, 3},
		{"utf16le_high_end", "utf-16-le", "410000d8", "unexpected end of data", 2, 4},
		{"utf16be_high_end", "utf-16-be", "0041d800", "unexpected end of data", 2, 4},
		{"utf16le_high_partial", "utf-16-le", "410000d800", "unexpected end of data", 2, 5},
		{"utf16be_high_partial", "utf-16-be", "0041d800dc", "unexpected end of data", 2, 5},
		{"utf16le_unpaired_high", "utf-16-le", "410000d84200", "illegal UTF-16 surrogate", 2, 4},
		{"utf16be_unpaired_high", "utf-16-be", "0041d8000042", "illegal UTF-16 surrogate", 2, 4},
		{"utf16le_lone_low", "utf-16-le", "410000dc", "illegal encoding", 2, 4},
		{"utf16be_lone_low", "utf-16-be", "0041dc00", "illegal encoding", 2, 4},
		{"utf32le_partial", "utf-32-le", "410000004200", "truncated data", 4, 6},
		{"utf32be_partial", "utf-32-be", "000000410000", "truncated data", 4, 6},
		{"utf32le_surrogate", "utf-32-le", "4100000000d80000", "code point in surrogate code point range(0xd800, 0xe000)", 4, 8},
		{"utf32be_surrogate", "utf-32-be", "000000410000dfff", "code point in surrogate code point range(0xd800, 0xe000)", 4, 8},
		{"utf32le_range", "utf-32-le", "4100000000001100", "code point not in range(0x110000)", 4, 8},
		{"utf32be_range", "utf-32-be", "0000004100110000", "code point not in range(0x110000)", 4, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, offsets, err := parsers.Decode(bytesFromHex(t, tc.raw), tc.encoding)
			var failure *parsers.DecodeError
			if !errors.As(err, &failure) {
				t.Fatalf("expected DecodeError, got %v", err)
			}
			if failure.Encoding != tc.encoding || failure.Start != tc.start || failure.End != tc.end || failure.Reason != tc.reason {
				t.Fatalf("failure: %+v", failure)
			}
			if text != "" || offsets != nil {
				t.Fatal("malformed input exposed a partial decoded result")
			}
		})
	}
}

func TestParsePreservesLiteralSource(t *testing.T) {
	raw := []byte("\ufeff<html>&#8203;</html>\\u200b\r\nA\rB\n\ufeff")
	d, err := (parsers.TextParser{}).Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Texts[0].Text != string(raw) {
		t.Fatal("source was rendered, normalized, or stripped")
	}
	if !reflect.DeepEqual(d.Texts[0].LineEndings, map[string]int{"crlf": 1, "cr": 1, "lf": 1}) {
		t.Fatal(d.Texts[0].LineEndings)
	}
	failed, err := (parsers.TextParser{}).Parse([]byte{'A', 0xff})
	if err == nil || len(failed.Texts) != 0 {
		t.Fatal("parse failure exposed partial evidence")
	}
}
