package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type corpusSchemaLoader struct{}

func (corpusSchemaLoader) Load(string) (any, error) {
	return nil, errors.New("network schema loading disabled")
}
func TestDirectoryJSONClosedSchema(t *testing.T) {
	dir, _ := corpusFixture(t)
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(corpusSchemaLoader{})
	for _, name := range []string{"corpus-document-v1.schema.json", "corpus-v1.schema.json", "report.schema.json", "report-v2.schema.json"} {
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
	schema, err := compiler.Compile("https://aletharsis.local/schemas/corpus-document-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0", "2.0"} {
		var out, stderr bytes.Buffer
		if Run([]string{"audit", dir, "--recursive", "--json", "--schema-version", version}, &out, &stderr) != 4 || stderr.Len() != 0 {
			t.Fatal("mixed corpus failed to deliver")
		}
		value, err := jsonschema.UnmarshalJSON(&out)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(value); err != nil {
			t.Fatal(err)
		}
		object := value.(map[string]any)
		object["unrecognized"] = true
		if schema.Validate(object) == nil {
			t.Fatal("unknown document property accepted")
		}
		delete(object, "unrecognized")
		delete(object, "summary")
		if schema.Validate(object) == nil {
			t.Fatal("incomplete JSON accepted")
		}
	}
}
