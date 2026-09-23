package corpusv2

import (
	"bytes"
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

// DecodeStream accepts a header, entries, then one summary. Blank records and
// records after the summary are rejected. All JSON parsing is bounded and strict.
func DecodeStream(raw []byte, envelopeLimits, reportLimits identity.Limits) (Document, error) {
	if envelopeLimits.InputBytes <= 0 || len(raw) > envelopeLimits.InputBytes {
		return Document{}, identity.ErrLimit
	}
	lines := bytes.Split(raw, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	var header, summary json.RawMessage
	entries := []json.RawMessage{}
	for i, line := range lines {
		canonical, err := wire.ValidateEnvelope("corpus-v2", line, envelopeLimits)
		if err != nil {
			return Document{}, err
		}
		var record struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(canonical, &record); err != nil {
			return Document{}, err
		}
		switch record.Type {
		case "header":
			if i != 0 || header != nil {
				return Document{}, ErrLinkage
			}
			header = canonical
		case "entry":
			if header == nil || summary != nil || len(entries) >= maxEntries {
				return Document{}, ErrLinkage
			}
			entries = append(entries, canonical)
		case "summary":
			if header == nil || summary != nil || i != len(lines)-1 {
				return Document{}, ErrLinkage
			}
			summary = canonical
		default:
			return Document{}, ErrLinkage
		}
	}
	if header == nil || summary == nil {
		return Document{}, ErrLinkage
	}
	wrapped, err := json.Marshal(struct {
		Header  json.RawMessage   `json:"header"`
		Entries []json.RawMessage `json:"entries"`
		Summary json.RawMessage   `json:"summary"`
	}{header, entries, summary})
	if err != nil {
		return Document{}, err
	}
	// The input budget applies to the submitted JSONL, not synthetic framing.
	// Retain aggregate node/depth/output bounds, without repeating wire validation.
	framedLimits := envelopeLimits
	framedLimits.InputBytes = len(wrapped)
	canonical, err := identity.Canonicalize(wrapped, framedLimits)
	if err != nil {
		return Document{}, err
	}
	var document Document
	if err := json.Unmarshal(canonical, &document); err != nil {
		return Document{}, err
	}
	return validateDocument(document, reportLimits)
}
