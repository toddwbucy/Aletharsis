package corpusv2

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func streamBytes(t *testing.T, d map[string]any) []byte {
	t.Helper()
	out := []byte{}
	values := []any{d["header"]}
	values = append(values, d["entries"].([]any)...)
	values = append(values, d["summary"])
	for _, v := range values {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, raw...)
		out = append(out, '\n')
	}
	return out
}
func revealCase(t *testing.T, version string, observerFailure bool) ([]byte, map[string]any) {
	d := sample(t)
	entry := d["entries"].([]any)[0].(map[string]any)
	report := entry["report"].(map[string]any)
	if version == "2.0" {
		d["header"].(map[string]any)["report_schema"] = version
		entry["report_schema"] = version
		report["schema_version"] = version
		delete(report, "adapter_runs")
		delete(report["evidence"].(map[string]any), "office")
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := identity.Canonicalize(raw, limits())
	if err != nil {
		t.Fatal(err)
	}
	hash := identity.ExactBytes(canonical)
	entry["report_canonical_sha256"] = hash
	outcome, reason := "unsupported", "presentation.unsupported"
	if observerFailure {
		entry["state"] = "failed"
		entry["reason"] = "execution.resource_limit"
		counts := d["summary"].(map[string]any)["counts"].(map[string]int)
		counts["partial"] = 0
		counts["failed"] = 1
		outcome, reason = "failed", "execution.resource_limit"
	}
	stream := streamBytes(t, d)
	tree := map[string]any{"contract": "aletharsis.reveal-tree/2", "corpus": map[string]any{"name": "corpus.jsonl", "size": len(stream), "sha256": identity.ExactBytes(stream)},
		"sources": []any{map[string]any{"relative_path": "a.txt", "detection_state": "partial", "presentation_outcome": outcome, "reason": reason, "report_artifact_sha256": hash,
			"artifacts": []any{map[string]any{"name": "reports/a.txt.json", "size": len(canonical), "sha256": hash}}}}}
	return stream, tree
}
func TestRevealRetainsPartialDetectionAfterObserverFailure(t *testing.T) {
	for _, version := range []string{"2.0", "4.0"} {
		for _, failed := range []bool{false, true} {
			stream, tree := revealCase(t, version, failed)
			raw, err := json.Marshal(tree)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeRevealTree(raw, stream, limits(), limits()); err != nil {
				t.Fatal(version, failed, err)
			}
			for _, mutate := range []func(map[string]any){
				func(t map[string]any) { t["corpus"].(map[string]any)["sha256"] = identity.ExactBytes([]byte("wrong")) },
				func(t map[string]any) { t["sources"].([]any)[0].(map[string]any)["detection_state"] = "failed" },
				func(t map[string]any) { t["sources"].([]any)[0].(map[string]any)["relative_path"] = "other.txt" },
				func(t map[string]any) {
					t["sources"].([]any)[0].(map[string]any)["report_artifact_sha256"] = identity.ExactBytes([]byte("wrong"))
				},
			} {
				_, bad := revealCase(t, version, failed)
				mutate(bad)
				raw, err := json.Marshal(bad)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRevealTree(raw, stream, limits(), limits()); err == nil {
					t.Fatal("accepted disconnected reveal receipt")
				}
			}
		}
	}
}
func TestStreamOrderAndStrictRecords(t *testing.T) {
	stream, _ := revealCase(t, "4.0", false)
	if _, err := DecodeStream(stream, limits(), limits()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append([]byte{'\n'}, stream...), append(append([]byte{}, stream...), stream...), []byte("{}\n"), nil} {
		if _, err := DecodeStream(bad, limits(), limits()); err == nil {
			t.Fatal("accepted malformed stream")
		}
	}
}
