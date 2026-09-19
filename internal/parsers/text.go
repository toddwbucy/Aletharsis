// Package parsers converts bounded byte snapshots to normalized evidence.
package parsers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

type Parser interface {
	Parse([]byte) (evidence.Document, error)
	Name() string
}
type TextParser struct{}

func (TextParser) Name() string { return "text-source-v1" }

type DecodeError struct {
	Encoding   string
	Start, End int
	Reason     string
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s decoding failed at bytes %d:%d: %s", e.Encoding, e.Start, e.End, e.Reason)
}
func Encoding(data []byte) (string, *string) {
	for _, v := range []struct{ bom, encoding string }{{"\xff\xfe\x00\x00", "utf-32-le"}, {"\x00\x00\xfe\xff", "utf-32-be"}, {"\xef\xbb\xbf", "utf-8"}, {"\xff\xfe", "utf-16-le"}, {"\xfe\xff", "utf-16-be"}} {
		if strings.HasPrefix(string(data), v.bom) {
			s := hex.EncodeToString([]byte(v.bom))
			return v.encoding, &s
		}
	}
	return "utf-8", nil
}
func Decode(data []byte, encoding string) (string, []int, error) {
	var b strings.Builder
	offsets := []int{}
	bad := func(start, end int, reason string) (string, []int, error) {
		return "", nil, &DecodeError{encoding, start, end, reason}
	}
	for i := 0; i < len(data); {
		start := i
		var r rune
		if encoding == "utf-8" {
			var size int
			r, size = utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				c := data[i]
				expected := 0
				if c >= 0xc2 && c <= 0xdf {
					expected = 2
				} else if c >= 0xe0 && c <= 0xef {
					expected = 3
				} else if c >= 0xf0 && c <= 0xf4 {
					expected = 4
				}
				if expected == 0 {
					return bad(i, i+1, "invalid start byte")
				}
				for j := 1; j < expected; j++ {
					if i+j >= len(data) {
						return bad(i, len(data), "unexpected end of data")
					}
					v := data[i+j]
					invalid := v < 0x80 || v > 0xbf
					if j == 1 {
						invalid = invalid || c == 0xe0 && v < 0xa0 || c == 0xed && v >= 0xa0 || c == 0xf0 && v < 0x90 || c == 0xf4 && v >= 0x90
					}
					if invalid {
						return bad(i, i+j, "invalid continuation byte")
					}
				}
				return bad(i, i+1, "invalid start byte")
			}
			i += size
		} else {
			var order binary.ByteOrder = binary.LittleEndian
			if strings.HasSuffix(encoding, "be") {
				order = binary.BigEndian
			}
			if strings.HasPrefix(encoding, "utf-16") {
				if i+2 > len(data) {
					return bad(i, len(data), "truncated data")
				}
				v := order.Uint16(data[i:])
				i += 2
				r = rune(v)
				if v >= 0xd800 && v <= 0xdbff {
					if i+2 > len(data) {
						return bad(start, len(data), "unexpected end of data")
					}
					next := order.Uint16(data[i:])
					if next < 0xdc00 || next > 0xdfff {
						return bad(start, i, "illegal UTF-16 surrogate")
					}
					r = 0x10000 + (rune(v)-0xd800)*1024 + (rune(next) - 0xdc00)
					i += 2
				} else if v >= 0xdc00 && v <= 0xdfff {
					return bad(start, i, "illegal encoding")
				}
			} else {
				if i+4 > len(data) {
					return bad(i, len(data), "truncated data")
				}
				v := order.Uint32(data[i:])
				i += 4
				if v > utf8.MaxRune {
					return bad(start, i, "code point not in range(0x110000)")
				}
				if v >= 0xd800 && v <= 0xdfff {
					return bad(start, i, "code point in surrogate code point range(0xd800, 0xe000)")
				}
				r = rune(v)
			}
		}
		offsets = append(offsets, start)
		b.WriteRune(r)
	}
	offsets = append(offsets, len(data))
	return b.String(), offsets, nil
}
func (TextParser) Parse(data []byte) (evidence.Document, error) {
	encoding, bom := Encoding(data)
	text, offsets, err := Decode(data, encoding)
	if err != nil {
		return evidence.EmptyDocument(), err
	}
	crlf := strings.Count(text, "\r\n")
	d := evidence.EmptyDocument()
	d.Texts = append(d.Texts, &evidence.Text{Source: "file", Text: text, Encoding: encoding, ByteOffsets: offsets, BOM: bom, LineEndings: map[string]int{"crlf": crlf, "cr": strings.Count(text, "\r") - crlf, "lf": strings.Count(text, "\n") - crlf}, Hashes: map[string]string{}, Normalization: evidence.Object{}})
	d.Structure = evidence.Object{"inspection": "literal source text", "byte_length": len(data)}
	return d, nil
}
