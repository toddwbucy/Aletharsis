package audit

import (
	"errors"
	"os"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

// Diagnostic creates a native failure record for a future report coordinator.
// References and scope come from that coordinator; this does not add v2 fields
// to the legacy report or reconstruct failures from its display message.
func (o Outcome) Diagnostic(ref, executionRef string, scope *v2.Scope) (*v2.Diagnostic, error) {
	if o.Failure == nil {
		return nil, nil
	}
	kind := "ValueError"
	var details v2.Details
	var decode *parsers.DecodeError
	if errors.As(o.Failure, &decode) {
		kind = "UnicodeDecodeError"
		if decode.Start < 0 || decode.End < decode.Start {
			return nil, errors.New("invalid native decode range")
		}
		details = v2.DecodeDetails{ErrorType: kind, ByteStart: uint64(decode.Start), ByteEnd: uint64(decode.End), Reason: decode.Reason}
	} else {
		var path *os.PathError
		switch {
		case errors.Is(o.Failure, os.ErrNotExist):
			kind = "FileNotFoundError"
		case errors.Is(o.Failure, os.ErrPermission):
			kind = "PermissionError"
		case errors.As(o.Failure, &path):
			kind = "OSError"
		}
		details = v2.ErrorDetails{ErrorType: kind}
	}
	d := &v2.Diagnostic{Ref: ref, ExecutionRef: &executionRef, Stage: o.Failure.Stage(), Code: o.Failure.Code(), Message: o.Failure.Error(), Scope: scope, Details: details}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}
