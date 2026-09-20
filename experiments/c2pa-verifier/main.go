// Isolated byte-oriented verifier probe; not part of the production CLI.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	verifier "github.com/encypherai/encypher-c2pa/bindings/go"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func run() map[string]any {
	if len(os.Args) != 4 {
		return map[string]any{"failure": "usage"}
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		return map[string]any{"failure": "read"}
	}
	asset, err := io.ReadAll(io.LimitReader(f, 32*1024*1024+1))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return map[string]any{"failure": "read"}
	}
	if len(asset) > 32*1024*1024 {
		return map[string]any{"failure": "input_limit"}
	}
	optionsRaw, err := os.ReadFile(os.Args[3])
	if err != nil {
		return map[string]any{"failure": "options_read"}
	}
	var options verifier.Options
	if err = json.Unmarshal(optionsRaw, &options); err != nil {
		return map[string]any{"failure": "options_decode"}
	}
	disabled := false
	options.Telemetry = &verifier.TelemetryOptions{Enabled: &disabled, SDKName: "go"}
	effective, marshalErr := json.Marshal(options)
	if marshalErr != nil {
		return map[string]any{"failure": "options_encode"}
	}
	before := digest(asset)
	report, err := verifier.Verify(asset, os.Args[2], &options)
	out := map[string]any{"source_sha256": before, "source_bytes": len(asset), "source_buffer_unchanged": before == digest(asset), "options_sha256": digest(optionsRaw), "effective_options_sha256": digest(effective)}
	if err != nil {
		out["error"] = err.Error()
	} else {
		out["report"] = report
	}
	return out
}
func main() {
	if err := json.NewEncoder(os.Stdout).Encode(run()); err != nil {
		fmt.Fprintln(os.Stderr, "output failed")
		os.Exit(1)
	}
}
