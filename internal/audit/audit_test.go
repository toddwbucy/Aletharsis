package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
)

func canonical(b []byte) map[string]any {
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		panic(err)
	}
	delete(v, "aletharsis_version")
	findings := v["findings"].([]any)
	for _, item := range findings {
		f := item.(map[string]any)
		if f["id"] == "parser.failure" {
			delete(f["evidence"].(map[string]any), "message")
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		a, _ := json.Marshal(findings[i])
		b, _ := json.Marshal(findings[j])
		return string(a) < string(b)
	})
	return v
}
func TestFrozenPythonParity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("forensic acquisition is Linux-only")
	}
	root, _ := filepath.Abs("../..")
	base := filepath.Join(root, "reference/python-behavior")
	manifest, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Cases []struct {
			Input, Report string
			Exit          int `json:"exit_code"`
		}
	}
	if err = json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Cases {
		t.Run(c.Input, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(base, c.Report))
			if err != nil {
				t.Fatal(err)
			}
			expected := canonical(raw)
			path := filepath.Join(root, c.Input)
			expected["file"].(map[string]any)["path"] = path
			r := audit.Run(path)
			out, err := reporters.JSON(r, true)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected, canonical([]byte(out))) {
				t.Fatalf("report differs from frozen Python reference: %s", c.Report)
			}
			if r.Summary["exit_code"] != c.Exit {
				t.Fatalf("exit %d != %d", r.Summary["exit_code"], c.Exit)
			}
			again, _ := reporters.JSON(audit.Run(path), true)
			if out != again {
				t.Fatal("nondeterministic report")
			}
		})
	}
}
func TestConcurrentAudits(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only reader")
	}
	path := filepath.Join(t.TempDir(), "source.py")
	if err := os.WriteFile(path, []byte("# 😀 👩\u200d💻\nA\u200b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	want, _ := reporters.JSON(audit.Run(path), true)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := reporters.JSON(audit.Run(path), true)
			if err != nil || got != want {
				t.Error("concurrent audit differs")
			}
		}()
	}
	wg.Wait()
}
