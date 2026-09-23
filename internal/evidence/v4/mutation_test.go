package v4

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
	"github.com/toddwbucy/Aletharsis/schemas"
)

// Mutation exceptions bind the complete input and changed report, not a path
// wildcard. Discovery writes only to an explicitly selected scratch destination;
// it never edits the reviewed exception ledger.
type mutationException struct {
	Fixture      string `json:"fixture"`
	Operator     string `json:"operator"`
	Pointer      string `json:"pointer"`
	Before       any    `json:"before"`
	After        any    `json:"after"`
	InputSHA256  string `json:"input_sha256"`
	MutantSHA256 string `json:"mutant_sha256"`
	Reason       string `json:"reason"`
}
type mutation struct {
	op, pointer   string
	before, after any
}

func mutationJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func mutationDigest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func mutationEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func mutationGet(v any, pointer string) any {
	if pointer == "" {
		return v
	}
	for _, s := range strings.Split(pointer[1:], "/") {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
		switch n := v.(type) {
		case map[string]any:
			v = n[s]
		case []any:
			i, _ := strconv.Atoi(s)
			v = n[i]
		default:
			panic(pointer)
		}
	}
	return v
}
func mutationSet(v any, pointer string, value any) {
	at := strings.LastIndex(pointer, "/")
	parent := mutationGet(v, pointer[:at])
	key := strings.ReplaceAll(strings.ReplaceAll(pointer[at+1:], "~1", "/"), "~0", "~")
	switch n := parent.(type) {
	case map[string]any:
		n[key] = value
	case []any:
		i, _ := strconv.Atoi(key)
		n[i] = value
	default:
		panic(pointer)
	}
}

// Expand only local schema references and applicators. Child schemas are kept
// separate; a nullable descendant cannot accidentally make its parent nullable.
func mutationSchemas(root any, nodes []any) []map[string]any {
	var out []map[string]any
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 64 {
			panic("schema recursion budget")
		}
		n, ok := v.(map[string]any)
		if !ok {
			return
		}
		out = append(out, n)
		if ref, ok := n["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/") {
				panic("nonlocal schema reference")
			}
			walk(mutationGet(root, ref[1:]), depth+1)
		}
		for _, key := range []string{"if", "then", "else"} {
			if branch, ok := n[key]; ok {
				walk(branch, depth+1)
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf"} {
			if branches, ok := n[key].([]any); ok {
				for _, branch := range branches {
					walk(branch, depth+1)
				}
			}
		}
	}
	for _, n := range nodes {
		walk(n, 0)
	}
	return out
}
func generateMutations(value, schema any) []mutation {
	var result []mutation
	add := func(op, p string, b, a any) {
		if !reflect.DeepEqual(b, a) {
			result = append(result, mutation{op, p, b, a})
		}
	}
	var walk func(any, string, []any)
	walk = func(v any, p string, nodes []any) {
		expanded := mutationSchemas(schema, nodes)
		for _, n := range expanded {
			nullable := n["type"] == "null"
			if types, ok := n["type"].([]any); ok {
				for _, typ := range types {
					nullable = nullable || typ == "null"
				}
			}
			if values, ok := n["enum"].([]any); ok {
				for _, value := range values {
					nullable = nullable || value == nil
				}
			}
			if value, ok := n["const"]; ok && value == nil {
				nullable = true
			}
			if nullable {
				add("null", p, v, nil)
				break
			}
		}
		switch n := v.(type) {
		case string:
			add("empty-string", p, v, "")
			if strings.HasSuffix(p, "/state") && (n == "partial" || n == "failed" || n == "canceled" || n == "not_run") {
				add("complete-state", p, v, "completed")
			}
		case []any:
			if len(n) > 0 {
				add("empty-array", p, v, []any{})
			}
			for i, child := range n {
				removed := append([]any{}, n[:i]...)
				removed = append(removed, n[i+1:]...)
				add("delete-record", p, v, removed)
				var next []any
				for _, s := range expanded {
					if item, ok := s["items"]; ok {
						next = append(next, item)
					}
				}
				walk(child, p+"/"+strconv.Itoa(i), next)
			}
		case map[string]any:
			start, sok := n["start"].(float64)
			end, eok := n["end"].(float64)
			if sok && eok && start < end {
				clone := func(a, b float64) map[string]any {
					r := map[string]any{}
					for k, v := range n {
						r[k] = v
					}
					r["start"] = a
					r["end"] = b
					return r
				}
				add("collapse-start", p, v, clone(start, start))
				add("collapse-end", p, v, clone(end, end))
				if strings.Contains(p, "/assessed/") {
					add("assess-first", p, v, clone(start, start+1))
					add("assess-last", p, v, clone(end-1, end))
				}
			}
			if _, has := n["parent"]; has {
				add("reparent-root", p+"/parent", n["parent"], nil)
				add("reparent-zero", p+"/parent", n["parent"], float64(0))
				if index, ok := n["index"].(float64); ok && index > 1 {
					add("reparent-previous", p+"/parent", n["parent"], index-1)
				}
			}
			keys := make([]string, 0, len(n))
			for key := range n {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				var next []any
				for _, s := range expanded {
					if props, ok := s["properties"].(map[string]any); ok {
						if child, ok := props[key]; ok {
							next = append(next, child)
						}
					}
				}
				child := n[key]
				path := p + "/" + mutationEscape(key)
				if number, ok := child.(float64); ok && (strings.Contains(p, "/summary") || strings.Contains(key, "count")) {
					add("count-minus", path, child, number-1)
					add("count-plus", path, child, number+1)
				}
				walk(child, path, next)
			}
		}
	}
	walk(value, "", []any{schema})
	result = append(result, generateOfficeMutations(value)...)
	return result
}

func TestAdversarialReportMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("full mutation gate runs separately without race instrumentation")
	}
	paths, err := filepath.Glob("../../../tests/contracts_v4/fixtures/*.json")
	if err != nil || len(paths) < 13 {
		t.Fatal("complete report corpus required", err)
	}
	rawSchema, _ := schemas.Report("4.0")
	var schema any
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	var exceptions []mutationException
	ledger := "testdata/mutation-exceptions.json"
	data, err := os.ReadFile(ledger)
	discovery := os.Getenv("ALETHARSIS_MUTATION_DISCOVERY")
	if err != nil && (discovery == "" || !errors.Is(err, fs.ErrNotExist)) {
		t.Fatal(err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Log("WARNING: discovery starts with no exception ledger")
	}
	if err == nil {
		if err = json.Unmarshal(data, &exceptions); err != nil {
			t.Fatal(err)
		}
	}
	fixtures := map[string]bool{}
	for _, path := range paths {
		fixtures[filepath.Base(path)] = true
	}
	allowed := map[string]mutationException{}
	key := func(e mutationException) string {
		return e.Fixture + "|" + e.Operator + "|" + e.Pointer + "|" + e.InputSHA256 + "|" + e.MutantSHA256
	}
	for _, e := range exceptions {
		if !fixtures[e.Fixture] {
			t.Fatal("exception references missing fixture", e.Fixture)
		}
		if strings.TrimSpace(e.Reason) == "" {
			t.Fatal("exception lacks reason", e)
		}
		if _, ok := allowed[key(e)]; ok {
			t.Fatal("duplicate exception", e)
		}
		allowed[key(e)] = e
	}
	used := map[string]bool{}
	finished := map[string]bool{}
	var survivors []mutationException
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeReport(raw, limits); err != nil {
				t.Fatal("invalid baseline", err)
			}
			var base any
			if err := json.Unmarshal(raw, &base); err != nil {
				t.Fatal(err)
			}
			stats := map[string]map[string]int{}
			seen := map[string]bool{}
			mutations := generateMutations(base, schema)
			if len(mutations) > 12000 {
				t.Fatal("fixture mutation budget exceeded; review corpus expansion")
			}
			for _, m := range mutations {
				var changed any
				if err := json.Unmarshal(raw, &changed); err != nil {
					t.Fatal(err)
				}
				if m.pointer == "" {
					changed = m.after
				} else {
					mutationSet(changed, m.pointer, m.after)
				}
				payload := mutationJSON(changed)
				e := mutationException{Fixture: filepath.Base(path), Operator: m.op, Pointer: m.pointer, Before: m.before, After: m.after, InputSHA256: mutationDigest(raw), MutantSHA256: mutationDigest(payload)}
				if seen[key(e)] {
					continue
				}
				seen[key(e)] = true
				stage := "accepted"
				if canonical, err := wire.Validate("4.0", payload, limits); err != nil {
					stage = "schema"
				} else if r, err := DecodeReport(payload, limits); err != nil {
					stage = "decode"
				} else if encoded, err := r.Encode(limits); err != nil {
					stage = "encode"
				} else if !bytes.Equal(canonical, encoded) {
					t.Fatalf("accepted mutation changed during round trip: %s", mutationJSON(e))
				}
				if stats[m.op] == nil {
					stats[m.op] = map[string]int{}
				}
				stats[m.op][stage]++
				if stage == "accepted" {
					if a, ok := allowed[key(e)]; ok && reflect.DeepEqual(a.Before, e.Before) && reflect.DeepEqual(a.After, e.After) {
						used[key(e)] = true
					} else {
						survivors = append(survivors, e)
						if discovery == "" {
							t.Errorf("untriaged mutant %s", mutationJSON(e))
						}
					}
				}
			}
			finished[filepath.Base(path)] = true
			t.Logf("mutation stages: %s", mutationJSON(stats))
		})
	}
	for k, e := range allowed {
		if finished[e.Fixture] && !used[k] {
			t.Errorf("stale exception: %s", mutationJSON(e))
		}
	}
	if discovery != "" {
		b, err := json.MarshalIndent(survivors, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(discovery, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(b)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
		t.Logf("discovery recorded %d UNTRIAGED survivors", len(survivors))
		if len(survivors) != 0 {
			t.Errorf("discovery is not acceptance: %d survivors require triage", len(survivors))
		}
	}
}

// These regression probes are selected from the broad walker; their selectors
// must keep generating at least one case. They do not replace the full ledger.
func TestGeneratedXMLAndRelationshipWeakening(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/metadata-inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var base, schema any
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	rawSchema, _ := schemas.Report("4.0")
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	if _, err := DecodeReport(raw, limits); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"compressed collapse": 0, "relationship deletion": 0, "XML token deletion": 0, "XML token null": 0, "XML reparent": 0}
	mutations := generateMutations(base, schema)
	if len(mutations) > 12000 {
		t.Fatal("fixture mutation budget exceeded; review corpus expansion")
	}
	for _, m := range mutations {
		group := ""
		if strings.HasPrefix(m.op, "collapse-") && strings.HasSuffix(m.pointer, "/compressed_span") {
			group = "compressed collapse"
		}
		if m.op == "delete-record" && m.pointer == "/evidence/office/relationships" {
			group = "relationship deletion"
		}
		if m.op == "delete-record" && strings.HasPrefix(m.pointer, "/evidence/office/xml/") && strings.HasSuffix(m.pointer, "/tokens") {
			group = "XML token deletion"
		}
		if m.op == "null" && strings.Contains(m.pointer, "/tokens/") && strings.HasSuffix(m.pointer, "/element") {
			group = "XML token null"
		}
		if strings.HasPrefix(m.op, "reparent-") {
			group = "XML reparent"
		}
		if group == "" {
			continue
		}
		counts[group]++
		var changed any
		if err := json.Unmarshal(raw, &changed); err != nil {
			t.Fatal(err)
		}
		mutationSet(changed, m.pointer, m.after)
		payload := mutationJSON(changed)
		if _, err := wire.Validate("4.0", payload, limits); err != nil {
			t.Fatalf("probe rejected by schema rather than semantic guard: %s %s", m.op, m.pointer)
		}
		if _, err := DecodeReport(payload, limits); err == nil {
			t.Errorf("accepted generated %s %s before=%s after=%s", m.op, m.pointer, mutationJSON(m.before), mutationJSON(m.after))
		}
	}
	for group, count := range counts {
		if count == 0 {
			t.Errorf("missing generated coverage: %s", group)
		}
	}
	t.Logf("generated regression coverage: %s", mutationJSON(counts))
}

func readMutationFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../../tests/contracts_v4/fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
