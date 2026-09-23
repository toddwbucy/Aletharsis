package v4

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/schemas"
)

func TestMutationSchemaTraversalAndDeterminism(t *testing.T) {
	var value, schema any
	if err := json.Unmarshal([]byte(`{"a/b~c":"x","direct":"x","nested":{"child":"x"},"items":["x"],"plain":"x"}`), &value); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{
  "$defs":{"optional":{"anyOf":[{"type":"null"},{"type":"string"}]}},
  "type":"object","properties":{
   "a/b~c":{"$ref":"#/$defs/optional"},
   "direct":{"type":["string","null"]},
   "nested":{"allOf":[{"type":"object","properties":{"child":{"oneOf":[{"type":"string"},{"type":"null"}]}}}]},
   "items":{"type":"array","items":{"$ref":"#/$defs/optional"}},
   "plain":{"type":"string"}
  }}`), &schema); err != nil {
		t.Fatal(err)
	}
	first := generateMutations(value, schema)
	second := generateMutations(value, schema)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("nondeterministic generator")
	}
	actual := map[string]bool{}
	for _, m := range first {
		if m.op == "null" {
			actual[m.pointer] = true
		}
	}
	want := map[string]bool{"/a~1b~0c": true, "/direct": true, "/nested/child": true, "/items/0": true}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("nullable traversal: %v", actual)
	}
	mutationSet(value, "/a~1b~0c", "changed")
	if mutationGet(value, "/a~1b~0c") != "changed" {
		t.Fatal("JSON pointer escaping")
	}
}

func TestCompoundAndOutsideMutationCoverage(t *testing.T) {
	raw := readMutationFixture(t, "metadata-inventory.json")
	var value, schema any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	rawSchema, _ := schemas.Report("4.0")
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		t.Fatal(err)
	}
	generated := generateMutations(value, schema)
	counts := map[string]int{}
	for _, m := range generated {
		counts[m.op]++
	}
	for _, op := range []string{"empty-string", "empty-array", "null", "delete-record", "collapse-start", "collapse-end", "assess-first", "assess-last", "assess-outside-retained", "compound-empty-metadata", "reparent-root", "reparent-zero", "reparent-previous", "count-minus", "count-plus"} {
		if counts[op] == 0 {
			t.Errorf("missing %s", op)
		}
	}
	if len(generated) > 12000 {
		t.Fatal("fixture mutation budget exceeded")
	}
	// Emptying is generated from the original value without mutating it.
	if !reflect.DeepEqual(mutationJSON(value), mutationJSON(mustMutationValue(t, raw))) {
		t.Fatal("generator changed its source")
	}
}
func mustMutationValue(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
