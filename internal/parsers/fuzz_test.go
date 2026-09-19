package parsers_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

// encodeScalar is independent of the application decoder, including its offsets.
func encodeScalar(r rune, encoding string) []byte {
	if encoding == "utf-8" {
		return []byte(string(r))
	}
	var order binary.AppendByteOrder = binary.LittleEndian
	if encoding == "utf-16-be" || encoding == "utf-32-be" {
		order = binary.BigEndian
	}
	if encoding == "utf-16-le" || encoding == "utf-16-be" {
		var out []byte
		for _, unit := range utf16.Encode([]rune{r}) {
			out = order.AppendUint16(out, unit)
		}
		return out
	}
	return order.AppendUint32(nil, uint32(r))
}

func FuzzDecodeCoordinates(f *testing.F) {
	for _, seed := range []struct {
		data     []byte
		encoding byte
	}{
		{nil, 0}, {[]byte("é😀e\u0301\u200d\u202e\r\n"), 0},
		{[]byte{0xf0, 0x9f, 0x98}, 0}, {[]byte{0xed, 0xa0, 0x80}, 0},
		{[]byte{0xff, 0xfe, 0x3d, 0xd8, 0, 0xde}, 1},
		{[]byte{0xfe, 0xff, 0xd8, 0, 0, 0x41}, 2},
		{[]byte{0xff, 0xfe, 0, 0, 0, 0xf6, 1, 0}, 3},
		{[]byte{0, 0, 0xfe, 0xff, 0, 0x11, 0, 0}, 4},
		{[]byte("%PDF-1.7\n"), 0}, {[]byte("PK\x03\x04"), 0},
	} {
		f.Add(seed.data, seed.encoding)
	}
	f.Fuzz(func(t *testing.T, data []byte, mode byte) {
		if len(data) > 4096 {
			t.Skip("bounded to 4 KiB")
		}
		original := bytes.Clone(data)
		encoding := []string{"utf-8", "utf-16-le", "utf-16-be", "utf-32-le", "utf-32-be"}[int(mode)%5]
		text, offsets, err := parsers.Decode(data, encoding)
		if !bytes.Equal(original, data) {
			t.Fatal("decoder mutated source bytes")
		}
		if err != nil {
			var decode *parsers.DecodeError
			if !errors.As(err, &decode) || text != "" || offsets != nil {
				t.Fatalf("partial or unstructured decode failure: %v", err)
			}
			if decode.Encoding != encoding || decode.Start < 0 || decode.Start >= decode.End || decode.End > len(data) {
				t.Fatal("invalid error coordinates", decode)
			}
			if encoding == "utf-8" && utf8.Valid(data) {
				t.Fatal("valid UTF-8 rejected")
			}
		} else {
			if !utf8.ValidString(text) || len(offsets) != utf8.RuneCountInString(text)+1 || offsets[0] != 0 || offsets[len(offsets)-1] != len(data) {
				t.Fatal("invalid scalar/end coordinates")
			}
			var rebuilt []byte
			for i, r := range []rune(text) {
				encoded := encodeScalar(r, encoding)
				if offsets[i] != len(rebuilt) || offsets[i+1] != offsets[i]+len(encoded) {
					t.Fatal("coordinate does not match encoded scalar")
				}
				rebuilt = append(rebuilt, encoded...)
			}
			if !bytes.Equal(rebuilt, data) {
				t.Fatal("accepted bytes did not round trip")
			}
			again, positions, e := parsers.Decode(data, encoding)
			if e != nil || again != text || !reflect.DeepEqual(positions, offsets) {
				t.Fatal("nondeterministic decoding")
			}
		}
		// Identification always examines the original bytes and may reject a format
		// independently of whether the explicitly selected decoder accepted it.
		format, mime, basis := parsers.Identify(data, ".txt")
		format2, mime2, basis2 := parsers.Identify(data, ".txt")
		if format == "" || mime == "" || basis == "" || format != format2 || mime != mime2 || basis != basis2 {
			t.Fatal("unstable or empty identification")
		}
	})
}
