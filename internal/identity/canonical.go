// Package identity implements bounded, versioned evidence identity operations.
// Canonical JSON is used for composite identities, never to replace the digest
// of an imported report's original bytes. It is not a report/schema validator.
package identity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Limits bounds parsing and serialization separately. Callers must select a
// budget for their operation; zero values are not interpreted as unlimited.
type Limits struct {
	InputBytes  int
	OutputBytes int
	Nodes       int
	Depth       int
}

// ErrLimit identifies an exhausted or invalid resource budget.
var ErrLimit = errors.New("identity resource limit")

func (l Limits) validate() error {
	if l.InputBytes <= 0 || l.OutputBytes <= 0 || l.Nodes <= 0 || l.Depth <= 0 || l.Depth > 256 {
		return fmt.Errorf("%w: positive budgets and depth <= 256 required", ErrLimit)
	}
	return nil
}

// Canonicalize returns RFC 8785 JSON using binary64 number semantics. It rejects
// duplicate decoded keys, invalid UTF-8, unpaired surrogate escapes and overflow.
// Schema-specific safe-integer limits must be checked before using coordinates.
// No source normalization, file access, network access or execution occurs.
func Canonicalize(raw []byte, limits Limits) ([]byte, error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}
	if len(raw) > limits.InputBytes {
		return nil, fmt.Errorf("%w: input bytes", ErrLimit)
	}
	if err := validStrings(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	remaining := limits.Nodes
	value, err := readValue(d, 1, limits.Depth, &remaining)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("identity JSON must contain exactly one value")
	}
	w := writer{limit: limits.OutputBytes}
	if err := w.value(value); err != nil {
		return nil, err
	}
	return w.out, nil
}

func readValue(d *json.Decoder, depth, maxDepth int, remaining *int) (any, error) {
	if depth > maxDepth || *remaining <= 0 {
		return nil, fmt.Errorf("%w: depth or nodes", ErrLimit)
	}
	*remaining--
	token, err := d.Token()
	if err != nil {
		return nil, errors.New("invalid identity JSON")
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for d.More() {
			if *remaining <= 0 {
				return nil, fmt.Errorf("%w: object keys", ErrLimit)
			}
			*remaining--
			keyToken, err := d.Token()
			if err != nil {
				return nil, errors.New("invalid identity object")
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("invalid identity object key")
			}
			if _, exists := object[key]; exists {
				return nil, errors.New("duplicate identity object key")
			}
			v, err := readValue(d, depth+1, maxDepth, remaining)
			if err != nil {
				return nil, err
			}
			object[key] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errors.New("invalid identity object end")
		}
		return object, nil
	case json.Delim('['):
		array := []any{}
		for d.More() {
			v, err := readValue(d, depth+1, maxDepth, remaining)
			if err != nil {
				return nil, err
			}
			array = append(array, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errors.New("invalid identity array end")
		}
		return array, nil
	}
	switch v := token.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("identity number is not finite binary64")
		}
		return f, nil
	case nil, bool, string:
		return token, nil
	default:
		return nil, errors.New("invalid identity JSON token")
	}
}

// encoding/json repairs malformed Unicode; identity operations must reject it
// before decoding. JSON syntax is subsequently checked by the decoder.
func validStrings(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("identity JSON is not UTF-8")
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return errors.New("truncated identity string escape")
		}
		if raw[i] != 'u' {
			continue
		}
		unit, err := hexUnit(raw, i+1)
		if err != nil {
			return err
		}
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return errors.New("unpaired identity low surrogate")
		}
		if unit < 0xd800 || unit > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return errors.New("unpaired identity high surrogate")
		}
		low, err := hexUnit(raw, i+3)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("unpaired identity high surrogate")
		}
		i += 6
	}
	return nil
}

func hexUnit(raw []byte, start int) (uint64, error) {
	if start+4 > len(raw) {
		return 0, errors.New("truncated identity Unicode escape")
	}
	unit, err := strconv.ParseUint(string(raw[start:start+4]), 16, 16)
	if err != nil {
		return 0, errors.New("invalid identity Unicode escape")
	}
	return unit, nil
}

type writer struct {
	out   []byte
	limit int
}

func (w *writer) append(s string) error {
	if len(s) > w.limit-len(w.out) {
		return fmt.Errorf("%w: output bytes", ErrLimit)
	}
	w.out = append(w.out, s...)
	return nil
}

func (w *writer) quoted(s string) error {
	if err := w.append(`"`); err != nil {
		return err
	}
	const hex = "0123456789abcdef"
	for _, r := range s {
		var value string
		switch r {
		case '"':
			value = `\"`
		case '\\':
			value = `\\`
		case '\b':
			value = `\b`
		case '\t':
			value = `\t`
		case '\n':
			value = `\n`
		case '\f':
			value = `\f`
		case '\r':
			value = `\r`
		default:
			if r < 0x20 {
				value = string([]byte{'\\', 'u', '0', '0', hex[r>>4], hex[r&15]})
			} else {
				value = string(r)
			}
		}
		if err := w.append(value); err != nil {
			return err
		}
	}
	return w.append(`"`)
}

func (w *writer) value(v any) error {
	switch v := v.(type) {
	case nil:
		return w.append("null")
	case bool:
		return w.append(strconv.FormatBool(v))
	case string:
		return w.quoted(v)
	case float64:
		if v == 0 {
			return w.append("0")
		}
		// Go's JSON float formatter uses shortest binary64 digits and ECMAScript's
		// fixed/exponent thresholds. Negative zero is handled above. RFC Appendix B
		// and the portable vectors guard compatibility across supported Go versions.
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return w.append(string(raw))
	case []any:
		if err := w.append("["); err != nil {
			return err
		}
		for i, item := range v {
			if i > 0 {
				if err := w.append(","); err != nil {
					return err
				}
			}
			if err := w.value(item); err != nil {
				return err
			}
		}
		return w.append("]")
	case map[string]any:
		type key struct {
			text  string
			units []uint16
		}
		keys := make([]key, 0, len(v))
		for k := range v {
			keys = append(keys, key{k, utf16.Encode([]rune(k))})
		}
		sort.Slice(keys, func(i, j int) bool { return slices.Compare(keys[i].units, keys[j].units) < 0 })
		if err := w.append("{"); err != nil {
			return err
		}
		for i, k := range keys {
			if i > 0 {
				if err := w.append(","); err != nil {
					return err
				}
			}
			if err := w.quoted(k.text); err != nil {
				return err
			}
			if err := w.append(":"); err != nil {
				return err
			}
			if err := w.value(v[k.text]); err != nil {
				return err
			}
		}
		return w.append("}")
	default:
		return errors.New("unsupported identity value")
	}
}
