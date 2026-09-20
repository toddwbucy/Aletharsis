// An isolated feasibility probe; not linked into the Aletharsis CLI.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	carrier "github.com/encypherai/c2pa-text/go/v3/c2pa_text"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func run() map[string]any {
	result := map[string]any{}
	if len(os.Args) != 3 {
		return map[string]any{"failure": "usage"}
	}
	f, err := os.Open(os.Args[2])
	if err != nil {
		return map[string]any{"failure": "read"}
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16*1024*1024+1))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return map[string]any{"failure": "read"}
	}
	if len(raw) > 16*1024*1024 {
		return map[string]any{"failure": "input_limit"}
	}
	result["source_sha256"] = digest(raw)
	result["source_bytes"] = len(raw)
	if !utf8.Valid(raw) {
		result["failure"] = "invalid_utf8"
		return result
	}
	text := string(raw)
	var manifest []byte
	switch os.Args[1] {
	case "vs":
		var clean string
		var start, length int
		manifest, clean, start, length, err = carrier.ExtractManifest(text)
		result["offset"] = start
		result["length"] = length
		result["clean_sha256"] = digest([]byte(clean))
		validation := carrier.ValidateText(text)
		codes := []string{}
		for _, issue := range validation.Issues {
			codes = append(codes, string(issue.Code))
		}
		result["validation_codes"] = codes
	case "structured":
		var got carrier.StructuredExtraction
		got, err = carrier.ExtractStructured(text)
		manifest = got.Manifest
		result["reference"] = got.Reference
	case "html":
		var got *carrier.HTMLExtraction
		got, err = carrier.ExtractHTML(text)
		result["association"] = got != nil
		if got != nil {
			manifest = got.Manifest
			result["reference"] = got.Reference
			result["method"] = got.Method
		}
	default:
		result["failure"] = "method"
		return result
	}
	result["error"] = ""
	if err != nil {
		result["error"] = err.Error()
	}
	result["manifest_present"] = manifest != nil
	result["manifest_bytes"] = len(manifest)
	if manifest != nil {
		result["manifest_sha256"] = digest(manifest)
	}
	return result
}
func main() {
	if err := json.NewEncoder(os.Stdout).Encode(run()); err != nil {
		fmt.Fprintln(os.Stderr, "probe output failed")
		os.Exit(1)
	}
}
