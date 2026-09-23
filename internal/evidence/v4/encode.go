package v4

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// MarshalJSON preserves the top-level execution graph in the wire representation.
// Use Encode for bounded, validated producer output.
func (r Report) MarshalJSON() ([]byte, error) {
	type fields Report
	return json.Marshal(struct {
		fields
		Capabilities []v2.Capability `json:"capabilities"`
		Executions   []v2.Execution  `json:"executions"`
		Diagnostics  []v2.Diagnostic `json:"diagnostics"`
		Results      []v2.Result     `json:"results"`
		Anchors      []Anchor        `json:"anchors"`
	}{fields(r), r.Trace.Capabilities, r.Trace.Executions, r.Trace.Diagnostics, r.Trace.Results, r.Trace.Anchors})
}

// Encode checks producer values before JSON can replace invalid UTF-8, then
// applies the same wire and semantic checks as native import. It returns canonical
// JSON; callers must not mutate the report concurrently.
func (r Report) Encode(limits identity.Limits) ([]byte, error) {
	nodes, stringBytes := 0, 0
	if err := checkValue(reflect.ValueOf(r), 0, &nodes, &stringBytes, limits); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeReport(raw, limits); err != nil {
		return nil, err
	}
	return identity.Canonicalize(raw, limits)
}

// This producer guard mirrors the frozen v2 guard without changing that API.
func checkValue(v reflect.Value, depth int, nodes *int, stringBytes *int, limits identity.Limits) error {
	*nodes = *nodes + 1
	if depth > limits.Depth || *nodes > limits.Nodes {
		return fmt.Errorf("%w: producer report structure", identity.ErrLimit)
	}
	if v.IsValid() && v.Type() == reflect.TypeOf(json.RawMessage{}) && !v.IsNil() {
		if _, err := identity.Canonicalize(v.Bytes(), limits); err != nil {
			return err
		}
	}
	switch v.Kind() {
	case reflect.String:
		if v.Len() > min(limits.InputBytes, limits.OutputBytes)-*stringBytes {
			return identity.ErrLimit
		}
		*stringBytes += v.Len()
		if !utf8.ValidString(v.String()) {
			return errors.New("invalid UTF-8 in report")
		}
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return checkValue(v.Elem(), depth+1, nodes, stringBytes, limits)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := checkValue(v.Field(i), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := checkValue(v.Index(i), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if err := checkValue(iter.Key(), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
			if err := checkValue(iter.Value(), depth+1, nodes, stringBytes, limits); err != nil {
				return err
			}
		}
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return errors.New("non-finite report number")
		}
	}
	return nil
}
