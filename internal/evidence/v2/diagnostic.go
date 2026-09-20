package v2

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"

	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Details is sealed to the stage-specific contracts in EC-002. There is no raw
// upstream payload or arbitrary map alternative.
type Details interface{ diagnosticDetails() }
type ErrorDetails struct {
	ErrorType string `json:"error_type"`
}
type DecodeDetails struct {
	ErrorType string `json:"error_type"`
	ByteStart uint64 `json:"byte_start"`
	ByteEnd   uint64 `json:"byte_end"`
	Reason    string `json:"reason"`
}
type ExecutionDetails struct {
	Limit      *string `json:"limit"`
	LimitValue *uint64 `json:"limit_value"`
}
type ImportDetails struct {
	ReportPointer *string `json:"report_pointer"`
}
type OutputDetails struct {
	Operation string `json:"operation"`
}

func (ErrorDetails) diagnosticDetails()     {}
func (DecodeDetails) diagnosticDetails()    {}
func (ExecutionDetails) diagnosticDetails() {}
func (ImportDetails) diagnosticDetails()    {}
func (OutputDetails) diagnosticDetails()    {}

type Diagnostic struct {
	Ref          string        `json:"diagnostic_ref"`
	ExecutionRef *string       `json:"execution_ref"`
	Stage        failure.Stage `json:"stage"`
	Code         failure.Code  `json:"code"`
	Message      string        `json:"message"`
	Scope        *Scope        `json:"scope"`
	Details      Details       `json:"details"`
}

func (d Diagnostic) Validate() error {
	if !validRef(d.Ref, "diagnostic") || d.ExecutionRef != nil && !validRef(*d.ExecutionRef, "exec") || !validCode(&d.Code) {
		return errors.New("invalid diagnostic identity")
	}
	if d.Scope != nil {
		if err := d.Scope.Validate(); err != nil {
			return err
		}
	}
	switch v := d.Details.(type) {
	case ErrorDetails:
		if !slices.Contains([]failure.Stage{failure.Acquisition, failure.Parsing, failure.Audit}, d.Stage) || v.ErrorType == "" || d.Stage == failure.Parsing && v.ErrorType == "UnicodeDecodeError" {
			return errors.New("invalid error details")
		}
	case DecodeDetails:
		if d.Stage != failure.Parsing || v.ErrorType != "UnicodeDecodeError" || v.ByteStart > v.ByteEnd || v.ByteEnd > identity.MaxSafeInteger {
			return errors.New("invalid decode details")
		}
	case ExecutionDetails:
		if d.Stage != failure.Execution || v.Limit != nil && !slices.Contains([]string{"input_bytes", "expanded_bytes", "objects", "depth", "output_bytes", "execution_ms"}, *v.Limit) || v.LimitValue != nil && *v.LimitValue > identity.MaxSafeInteger {
			return errors.New("invalid execution details")
		}
	case ImportDetails:
		if d.Stage != "import" {
			return errors.New("invalid import details")
		}
	case OutputDetails:
		if d.Stage != "output" || !slices.Contains([]string{"create", "write", "close"}, v.Operation) {
			return errors.New("invalid output details")
		}
	default:
		return errors.New("unsupported diagnostic details")
	}
	if err := validStrings(reflect.ValueOf(d)); err != nil {
		return err
	}
	return nil
}
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire Diagnostic
	return json.Marshal(wire(d))
}

// DecodeDiagnostic validates the discriminator before decoding its closed details.
func DecodeDiagnostic(raw []byte) (Diagnostic, error) {
	var wire struct {
		Ref          string          `json:"diagnostic_ref"`
		ExecutionRef *string         `json:"execution_ref"`
		Stage        failure.Stage   `json:"stage"`
		Code         failure.Code    `json:"code"`
		Message      string          `json:"message"`
		Scope        *Scope          `json:"scope"`
		Details      json.RawMessage `json:"details"`
	}
	if err := decode(raw, &wire); err != nil {
		return Diagnostic{}, err
	}
	d := Diagnostic{Ref: wire.Ref, ExecutionRef: wire.ExecutionRef, Stage: wire.Stage, Code: wire.Code, Message: wire.Message, Scope: wire.Scope}
	switch d.Stage {
	case failure.Acquisition, failure.Audit:
		var v ErrorDetails
		if err := decode(wire.Details, &v); err != nil {
			return Diagnostic{}, err
		}
		d.Details = v
	case failure.Parsing:
		var kind struct {
			ErrorType string `json:"error_type"`
		}
		if err := json.Unmarshal(wire.Details, &kind); err != nil {
			return Diagnostic{}, err
		}
		if kind.ErrorType == "UnicodeDecodeError" {
			var v DecodeDetails
			if err := decode(wire.Details, &v); err != nil {
				return Diagnostic{}, err
			}
			d.Details = v
		} else {
			var v ErrorDetails
			if err := decode(wire.Details, &v); err != nil {
				return Diagnostic{}, err
			}
			d.Details = v
		}
	case failure.Execution:
		var v ExecutionDetails
		if err := decode(wire.Details, &v); err != nil {
			return Diagnostic{}, err
		}
		d.Details = v
	case "import":
		var v ImportDetails
		if err := decode(wire.Details, &v); err != nil {
			return Diagnostic{}, err
		}
		d.Details = v
	case "output":
		var v OutputDetails
		if err := decode(wire.Details, &v); err != nil {
			return Diagnostic{}, err
		}
		d.Details = v
	default:
		return Diagnostic{}, errors.New("invalid diagnostic stage")
	}
	if err := d.Validate(); err != nil {
		return Diagnostic{}, err
	}
	return d, nil
}
