// Package reporters renders evidence without interpreting source content.
package reporters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

// JSON uses fixed struct ordering, sorted map keys, ASCII escaping and stable
// indentation. Encode the typed model directly to avoid duplicating large offset
// arrays into an interface-valued JSON tree.
func JSON(value any, pretty bool) (string, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, r := range b.String() {
		if r > 127 {
			if r <= 65535 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				a, b := utf16.EncodeRune(r)
				fmt.Fprintf(&out, `\u%04x\u%04x`, a, b)
			}
		} else {
			out.WriteRune(r)
		}
	}
	return out.String(), nil
}
func compact(v any) string {
	s, err := JSON(v, false)
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSuffix(s, "\n")
}
func Console(r *evidence.Report, verbose bool) string {
	var b strings.Builder
	f := r.File
	size := "None"
	if f.Size != nil {
		size = fmt.Sprint(*f.Size)
	}
	hash, parser := "unavailable", "unavailable"
	if f.SHA256 != nil {
		hash = *f.SHA256
	}
	if f.Parser != nil {
		parser = *f.Parser
	}
	fmt.Fprintf(&b, "ALETHARSIS FORENSIC AUDIT\n\nFile\n  Path: %s\n  Type: %s (%s)\n  Size: %s bytes\n  SHA256: %s\n  Parser: %s\n  Status: %s\n\nSummary: %d findings; exit code %d\n  HIGH %d  MEDIUM %d  LOW %d  INFO %d\n\n", u.Escaped(f.Path), f.Format, f.MIME, size, hash, parser, r.Status, len(r.Findings), r.Summary["exit_code"], r.Summary["high"], r.Summary["medium"], r.Summary["low"], r.Summary["info"])
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "[%s] %s\n  ID: %s | Category: %s | %s\n  Confidence: %.2f\n  %s\n", f.Severity, u.Escaped(f.Title), f.ID, f.Category, f.Classification, f.Confidence, u.Escaped(f.Description))
		if verbose {
			fmt.Fprintf(&b, "  Evidence: %s\n  Location: %s\n", compact(f.Evidence), compact(f.Location))
		} else {
			if count, ok := f.Evidence["count"]; ok {
				fmt.Fprintf(&b, "  Occurrences: %v\n", count)
			}
			if offsets, ok := f.Location["byte_offsets"].([]int); ok && len(offsets) > 0 {
				fmt.Fprintf(&b, "  Byte offsets (first 8): %v\n", offsets[:min(8, len(offsets))])
			}
			if f.ID == "parser.failure" {
				fmt.Fprintf(&b, "  Error: %s\n", u.Escaped(fmt.Sprint(f.Evidence["message"])))
			}
		}
		b.WriteByte('\n')
	}
	if verbose {
		for _, t := range r.Evidence.Texts {
			fmt.Fprintf(&b, "Text hashes: %s\n", compact(t.Hashes))
		}
	}
	b.WriteString("Limitations\n")
	for _, s := range r.Limitations {
		fmt.Fprintf(&b, "  - %s\n", s)
	}
	return b.String()
}
