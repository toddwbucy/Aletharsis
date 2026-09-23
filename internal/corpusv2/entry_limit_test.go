package corpusv2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/toddwbucy/Aletharsis/internal/wire"
	"testing"
)

func TestBothCorpusFormsEnforceEntryLimits(t *testing.T) {
	for _, tc := range []struct {
		count, declared int
		valid           bool
	}{{10000, 10000, true}, {10001, 10001, false}, {3, 2, false}, {2, 2, true}} {
		d := sample(t)
		d["header"].(map[string]any)["limits"].(map[string]any)["entries"] = tc.declared
		entries := []any{}
		for i := 0; i < tc.count; i++ {
			entries = append(entries, map[string]any{"type": "entry", "relative_path": fmt.Sprintf("%d.txt", i), "state": "skipped", "reason": "discovery.unsupported_extension"})
		}
		d["entries"] = entries
		summary := d["summary"].(map[string]any)
		summary["entries"] = tc.count
		summary["state"] = "completed"
		summary["exit_code"] = 0
		counts := summary["counts"].(map[string]int)
		counts["partial"] = 0
		counts["skipped"] = tc.count
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var stream bytes.Buffer
		for _, v := range append(append([]any{d["header"]}, entries...), d["summary"]) {
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			stream.Write(b)
			stream.WriteByte('\n')
		}
		for _, decode := range []func() error{func() error { _, e := Decode(raw, limits(), limits()); return e }, func() error { _, e := DecodeStream(stream.Bytes(), limits(), limits()); return e }} {
			err := decode()
			if tc.count > maxEntries && !errors.Is(err, wire.ErrSchema) {
				t.Fatalf("fixed entry cap must be schema error: %v", err)
			}
			if tc.count <= maxEntries && !tc.valid && !errors.Is(err, ErrLinkage) {
				t.Fatalf("declared entry cap must be linkage error: %v", err)
			}
			if (err == nil) != tc.valid {
				t.Fatalf("count=%d declared=%d: %v", tc.count, tc.declared, err)
			}
		}
	}
}
