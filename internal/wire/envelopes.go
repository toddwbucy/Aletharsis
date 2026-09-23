package wire

import (
	"bytes"
	"errors"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/schemas"
	"sync"
)

var envelopeContracts = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(denyLoader{})
	resources := map[string][]byte{}
	for _, name := range []string{"corpus-v2", "corpus-document-v2", "reveal-tree-v2"} {
		raw, _ := schemas.Envelope(name)
		resources[name+".schema.json"] = raw
	}
	for _, v := range []string{"2.0", "4.0"} {
		raw, _ := schemas.Report(v)
		resources["report-v"+v[:1]+".schema.json"] = raw
	}
	const base = "https://aletharsis.local/schemas/"
	for name, raw := range resources {
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if err := c.AddResource(base+name, value); err != nil {
			return nil, err
		}
	}
	result := map[string]*jsonschema.Schema{}
	for _, name := range []string{"corpus-v2", "corpus-document-v2", "reveal-tree-v2"} {
		schema, err := c.Compile(base + name + ".schema.json")
		if err != nil {
			return nil, err
		}
		result[name] = schema
	}
	return result, nil
})

func ValidateEnvelope(name string, raw []byte, limits identity.Limits) ([]byte, error) {
	if _, ok := schemas.Envelope(name); !ok {
		return nil, ErrVersion
	}
	canonical, err := identity.Canonicalize(raw, limits)
	if err != nil {
		return nil, err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrSchema
	}
	if err := numericBudget(value); err != nil {
		return nil, err
	}
	contracts, err := envelopeContracts()
	if err != nil {
		return nil, errors.New("bundled envelope compilation failed")
	}
	if err := contracts[name].Validate(value); err != nil {
		return nil, ErrSchema
	}
	return canonical, nil
}
