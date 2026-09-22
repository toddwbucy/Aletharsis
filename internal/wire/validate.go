// Package wire validates bounded report JSON against compiled bundled schemas.
// It does not fetch schemas, follow document references or authenticate evidence.
package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/schemas"
)

var ErrSchema = errors.New("report does not match the bundled schema")
var ErrVersion = errors.New("unsupported report schema version")

type denyLoader struct{}

func (denyLoader) Load(string) (any, error) {
	return nil, errors.New("external schema resources disabled")
}

var compiled = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	result := map[string]*jsonschema.Schema{}
	for _, version := range []string{"1.0", "2.0", "4.0"} {
		raw, _ := schemas.Report(version)
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		c := jsonschema.NewCompiler()
		c.UseLoader(denyLoader{})
		location := "https://aletharsis.invalid/bundled/report-" + version + ".json"
		if err := c.AddResource(location, value); err != nil {
			return nil, err
		}
		sch, err := c.Compile(location)
		if err != nil {
			return nil, err
		}
		result[version] = sch
	}
	return result, nil
})

// Validate returns canonical JSON only after strict lexical, budget and schema
// checks. Schema validation uses original json.Number tokens: canonical numeric
// rounding must not turn a fractional/unsafe coordinate into an accepted integer.
// Errors deliberately omit payloads from the upstream validator's diagnostics.
func Validate(version string, raw []byte, limits identity.Limits) ([]byte, error) {
	if version != "1.0" && version != "2.0" && version != "4.0" {
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
	contracts, err := compiled()
	if err != nil {
		return nil, errors.New("bundled schema compilation failed")
	}
	if err := contracts[version].Validate(value); err != nil {
		return nil, ErrSchema
	}
	return canonical, nil
}

// The upstream validator uses exact rationals. Limit lexical numeric complexity
// before handing it tokens such as 1e-1000000000 that fit in a tiny JSON file.
func numericBudget(value any) error {
	switch v := value.(type) {
	case json.Number:
		token := string(v)
		if len(token) > 1024 {
			return errors.New("report numeric token exceeds budget")
		}
		if i := strings.IndexAny(token, "eE"); i >= 0 {
			exponent, err := strconv.ParseInt(token[i+1:], 10, 32)
			if err != nil || exponent < -4096 || exponent > 4096 {
				return errors.New("report numeric exponent exceeds budget")
			}
		}
	case []any:
		for _, item := range v {
			if err := numericBudget(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range v {
			if err := numericBudget(item); err != nil {
				return err
			}
		}
	}
	return nil
}
