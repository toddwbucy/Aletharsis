package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

const (
	// ConfigDomain identifies the EC-002 configuration envelope.
	ConfigDomain = "aletharsis.config/1"
	// SelectionDomain identifies an ordered list of exact text selections.
	SelectionDomain = "aletharsis.text-selection/1"
	// MaxSafeInteger is the maximum exact coordinate allowed by schema 2.0.
	MaxSafeInteger uint64 = 1<<53 - 1
)

// Digest hashes JCS UTF-8 for {domain, value}. The supplied value is JSON, not
// text to be implicitly parsed as a finding or executed. Limits apply to the
// entire envelope, including its domain, keys, and nodes. No partial digest is
// returned on failure. This validates JSON identity, not domain-specific schema.
func Digest(domain string, value json.RawMessage, limits Limits) (string, error) {
	if err := limits.validate(); err != nil {
		return "", err
	}
	if domain == "" || !utf8.ValidString(domain) {
		return "", errors.New("invalid identity domain")
	}
	if len(value) > limits.InputBytes {
		return "", fmt.Errorf("%w: identity value bytes", ErrLimit)
	}
	// Check the standalone value before embedding it: a raw fragment must not
	// be able to inject additional members into the identity envelope.
	if !json.Valid(value) {
		return "", errors.New("identity value must be one JSON value")
	}
	w := writer{limit: limits.InputBytes}
	if err := w.append(`{"domain":`); err != nil {
		return "", err
	}
	if err := w.quoted(domain); err != nil {
		return "", err
	}
	if err := w.append(`,"value":`); err != nil {
		return "", err
	}
	if err := w.append(string(value)); err != nil {
		return "", err
	}
	if err := w.append(`}`); err != nil {
		return "", err
	}
	canonical, err := Canonicalize(w.out, limits)
	if err != nil {
		return "", err
	}
	return ExactBytes(canonical), nil
}

// ExactBytes hashes a byte sequence without parsing, normalization or re-encoding.
// Callers own acquisition/retention and must enforce their input byte budget.
func ExactBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Region is a zero-based, half-open interval in its declared representation.
type Region struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

// TextSpan pairs scalar positions in extracted text with original-source bytes.
// Byte positions are not UTF-8 re-encoding offsets for UTF-16/32 source files.
type TextSpan struct {
	Scalar Region `json:"scalar"`
	Byte   Region `json:"byte"`
}

// VerifiedText owns a verified native scalar/byte map. Its fields are private so
// later caller mutations cannot change the selections it identifies.
type VerifiedText struct {
	scalars []rune
	offsets []int
}

// VerifyText compares the complete native text and boundary map with source bytes.
// It owns the decoded representation; it never retains the caller's mutable slices.
func VerifyText(source []byte, text *evidence.Text, sourceLimit int) (*VerifiedText, error) {
	if sourceLimit <= 0 || len(source) > sourceLimit {
		return nil, fmt.Errorf("%w: source bytes", ErrLimit)
	}
	if text == nil {
		return nil, errors.New("text segment required")
	}
	switch text.Encoding {
	case "utf-8", "utf-16-le", "utf-16-be", "utf-32-le", "utf-32-be":
	default:
		return nil, errors.New("unsupported text selection encoding")
	}
	decoded, offsets, err := parsers.Decode(source, text.Encoding)
	if err != nil {
		return nil, errors.New("text selection source cannot be decoded")
	}
	if decoded != text.Text || !slices.Equal(offsets, text.ByteOffsets) {
		return nil, errors.New("text selection does not match source and boundary map")
	}
	return &VerifiedText{scalars: []rune(decoded), offsets: offsets}, nil
}

// TextSelectionDigest verifies text and its full boundary map against supplied
// source bytes using the native literal-text decoder, then hashes ordered exact
// selections. It supports the existing UTF-8/16/32 encodings only. Normalized,
// extracted office, or approximate mappings require separate adapter contracts.
// sourceLimit bounds decoding before allocation; limits bounds the JSON identity.
// The caller must not mutate source, text, or spans concurrently with this call.
func TextSelectionDigest(source []byte, text *evidence.Text, spans []TextSpan, sourceLimit int, limits Limits) (string, error) {
	if err := limits.validate(); err != nil {
		return "", err
	}
	verified, err := VerifyText(source, text, sourceLimit)
	if err != nil {
		return "", err
	}
	return verified.SelectionDigest(spans, limits)
}

// SelectionDigest hashes exact spans against the privately owned verified map.
func (v *VerifiedText) SelectionDigest(spans []TextSpan, limits Limits) (string, error) {
	if err := limits.validate(); err != nil {
		return "", err
	}
	if v == nil || len(spans) == 0 {
		return "", errors.New("text selection requires a segment and spans")
	}
	if len(spans) > limits.Nodes {
		return "", fmt.Errorf("%w: selection count", ErrLimit)
	}
	scalars, offsets := v.scalars, v.offsets
	previous := uint64(0)
	// Serialize incrementally so selection JSON cannot grow past the input budget
	// before Digest checks it. json.Marshal is only an intermediate valid encoding;
	// final identity serialization always uses Canonicalize.
	w := writer{limit: limits.InputBytes}
	if err := w.append("["); err != nil {
		return "", err
	}
	for i, span := range spans {
		if span.Scalar.Start < previous || span.Scalar.Start >= span.Scalar.End || span.Scalar.End > uint64(len(scalars)) ||
			span.Scalar.End > MaxSafeInteger || span.Byte.Start >= span.Byte.End || span.Byte.End > MaxSafeInteger {
			return "", errors.New("invalid or overlapping text selection span")
		}
		if span.Byte.Start != uint64(offsets[span.Scalar.Start]) || span.Byte.End != uint64(offsets[span.Scalar.End]) {
			return "", errors.New("text selection scalar/byte mismatch")
		}
		previous = span.Scalar.End
		raw, err := json.Marshal(span)
		if err != nil {
			return "", err
		}
		if i > 0 {
			if err := w.append(","); err != nil {
				return "", err
			}
		}
		if err := w.append(`{"span":`); err != nil {
			return "", err
		}
		if err := w.append(string(raw)); err != nil {
			return "", err
		}
		if err := w.append(`,"text":`); err != nil {
			return "", err
		}
		if err := w.quoted(string(scalars[span.Scalar.Start:span.Scalar.End])); err != nil {
			return "", err
		}
		if err := w.append("}"); err != nil {
			return "", err
		}
	}
	if err := w.append("]"); err != nil {
		return "", err
	}
	return Digest(SelectionDomain, w.out, limits)
}
