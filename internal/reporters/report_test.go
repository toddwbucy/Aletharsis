package reporters_test

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestJSONEscapingAndDeterminism(t *testing.T) {
	value := map[string]string{"z": "é😀\u202e\x1b\n", "a": "<literal>"}
	for _, pretty := range []bool{false, true} {
		first, err := reporters.JSON(value, pretty)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			next, err := reporters.JSON(value, pretty)
			if err != nil || next != first {
				t.Fatal("nondeterministic JSON")
			}
		}
		for _, r := range first {
			if r > 127 || r == 0x1b {
				t.Fatal("unsafe raw Unicode/control")
			}
		}
		if !strings.Contains(first, `\ud83d\ude00`) || strings.Index(first, `"a"`) > strings.Index(first, `"z"`) {
			t.Fatal(first)
		}
		var decoded map[string]string
		if err := json.Unmarshal([]byte(first), &decoded); err != nil || !reflect.DeepEqual(decoded, value) {
			t.Fatalf("evidence changed in JSON: %v", err)
		}
	}
	for _, value := range []any{math.Inf(1), make(chan int)} {
		out, err := reporters.JSON(value, false)
		if err == nil || out != "" {
			t.Fatal("invalid JSON value accepted or partial output returned")
		}
	}
}

func TestConsoleEscapesEvidenceAndPaths(t *testing.T) {
	source := "file\x1b[31m\u202e.txt"
	r := &evidence.Report{Status: "completed", File: evidence.File{Path: source}, Findings: []evidence.Finding{{ID: "test.observation", Severity: "LOW", Title: "text\x1b", Description: "value\u202e", Evidence: evidence.Object{"value": "\x1b\u200b"}, Location: evidence.Object{"source": source}}}}
	r.Summarize()
	for _, verbose := range []bool{false, true} {
		out := reporters.Console(r, verbose)
		if strings.ContainsAny(out, "\x1b\u202e\u200b") {
			t.Fatal("raw control or invisible evidence in console")
		}
		if !strings.Contains(out, `\x1b`) || !strings.Contains(out, `\u202e`) {
			t.Fatal("escaped evidence lost")
		}
	}
}
