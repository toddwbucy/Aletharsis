package corpus

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type noNetwork struct{}

func (noNetwork) Load(string) (any, error) {
	return nil, errors.New("external schema loading disabled")
}
func TestStreamClosedSchema(t *testing.T) {
	linux(t)
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(noNetwork{})
	for _, name := range []string{"corpus-v1.schema.json", "report.schema.json", "report-v2.schema.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", name))
		if err != nil {
			t.Fatal(err)
		}
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource("https://aletharsis.local/schemas/"+name, value); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := compiler.Compile("https://aletharsis.local/schemas/corpus-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	put(t, filepath.Join(dir, "clean"), []byte("text"))
	put(t, filepath.Join(dir, "bad"), []byte{0xff})
	for _, version := range []string{"1.0", "2.0"} {
		options := DefaultOptions()
		options.Schema = version
		var out bytes.Buffer
		if _, err := Run(context.Background(), dir, options, &out); err != nil {
			t.Fatal(err)
		}
		for _, raw := range records(t, out.Bytes()) {
			value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(value); err != nil {
				t.Fatal(err)
			}
			object := value.(map[string]any)
			object["unknown"] = true
			if schema.Validate(object) == nil {
				t.Fatal("unknown key accepted")
			}
			delete(object, "unknown")
			if object["type"] == "entry" {
				delete(object, "report_canonical_sha256")
				if schema.Validate(object) == nil {
					t.Fatal("unbound report accepted")
				}
			}
		}
	}
}
