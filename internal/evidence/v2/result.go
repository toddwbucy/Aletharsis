package v2

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
)

// StructuralPayload is an observation outcome, not a probability or an assertion
// about authorship. Statistical/cryptographic payload contracts remain deferred.
type StructuralPayload struct {
	Operation string `json:"operation"`
	Scope     Scope  `json:"scope"`
	Outcome   string `json:"outcome"`
}
type Result struct {
	Ref             string            `json:"result_ref"`
	ExecutionRef    string            `json:"execution_ref"`
	Kind            string            `json:"kind"`
	ContractVersion string            `json:"contract_version"`
	AnchorRefs      []string          `json:"anchor_refs"`
	Payload         StructuralPayload `json:"payload"`
	Limitations     []string          `json:"limitations"`
}

func (r Result) Validate() error {
	if err := validStrings(reflect.ValueOf(r)); err != nil {
		return err
	}
	if !validRef(r.Ref, "result") || !validRef(r.ExecutionRef, "exec") || r.Kind != "structural_scan" || r.ContractVersion != "1" || r.AnchorRefs == nil || r.Limitations == nil {
		return errors.New("invalid result record")
	}
	seen := map[string]bool{}
	for _, a := range r.AnchorRefs {
		if !validRef(a, "anchor") || seen[a] {
			return errors.New("invalid result anchor reference")
		}
		seen[a] = true
	}
	if r.Payload.Operation == "" || !slices.Contains([]string{"observations_present", "no_observations"}, r.Payload.Outcome) {
		return errors.New("invalid structural result payload")
	}
	return r.Payload.Scope.Validate()
}
func (r Result) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire Result
	return json.Marshal(wire(r))
}
func DecodeResult(raw []byte) (Result, error) {
	var r Result
	if err := decode(raw, &r); err != nil {
		return Result{}, err
	}
	return r, r.Validate()
}
