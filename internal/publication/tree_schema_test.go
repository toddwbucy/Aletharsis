package publication

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestTreeClosedSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "reveal-tree-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://aletharsis.local/schemas/reveal-tree-v1.schema.json", value); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://aletharsis.local/schemas/reveal-tree-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	r, dir := root(t)
	tree, err := NewTree(r, []TreeSource{{"a", true}, {"skip", false}}, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("a", "revealed", "", digest, treeArtifacts()); err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("skip", "skipped", "symlink_not_followed", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Commit([]byte("stream")); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	value, err = jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	object["unknown"] = true
	if schema.Validate(object) == nil {
		t.Fatal("unknown manifest key accepted")
	}
	delete(object, "unknown")
	source := object["sources"].([]any)[0].(map[string]any)
	delete(source, "source_sha256")
	if schema.Validate(object) == nil {
		t.Fatal("revealed source without identity accepted")
	}
}
