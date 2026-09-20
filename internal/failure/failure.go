// Package failure preserves machine-readable causes independently of error prose.
package failure

import (
	"errors"
	"os"
)

// Code identifies a stable condition, not a message or inferred intent.
type Code string

const (
	FileNotFound              Code = "file.not_found"
	PermissionDenied          Code = "file.permission_denied"
	NotRegular                Code = "file.not_regular"
	TooLarge                  Code = "file.too_large"
	ChangedDuringRead         Code = "file.changed_during_read"
	IOFailed                  Code = "file.io_failed"
	NoAtimeUnavailable        Code = "integrity.no_atime_unavailable"
	UnsupportedFormat         Code = "format.unsupported"
	DecodeFailed              Code = "text.decode_failed"
	AuditFailed               Code = "audit.failed"
	NotImplemented            Code = "capability.not_implemented"
	DetectorAccessUnavailable Code = "capability.detector_access_unavailable"
	Disabled                  Code = "execution.disabled"
	PrerequisiteFailed        Code = "execution.prerequisite_failed"
	UnsupportedInput          Code = "execution.unsupported_input"
	PolicyDenied              Code = "execution.policy_denied"
	ResourceLimit             Code = "execution.resource_limit"
	Timeout                   Code = "execution.timeout"
	Canceled                  Code = "execution.canceled"
	ExecutionFailed           Code = "execution.failed"
)

// Stage locates the boundary that assigned a failure code.
type Stage string

const (
	Acquisition Stage = "acquisition"
	Parsing     Stage = "parsing"
	Execution   Stage = "execution"
	Audit       Stage = "audit"
)

// Error keeps the original cause for errors.Is/As and legacy error reporting.
// Its fields are private so callers cannot change a classified failure in place.
type Error struct {
	code  Code
	stage Stage
	cause error
}

func (e *Error) Error() string {
	if e == nil || e.cause == nil {
		return "unclassified audit failure"
	}
	return e.cause.Error()
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
func (e *Error) Code() Code {
	if e == nil || e.code == "" {
		return AuditFailed
	}
	return e.code
}
func (e *Error) Stage() Stage {
	if e == nil || e.stage == "" {
		return Audit
	}
	return e.stage
}

// Wrap classifies an error at a known boundary. A nil cause stays nil.
func Wrap(stage Stage, code Code, cause error) *Error {
	if cause == nil {
		return nil
	}
	return &Error{code: code, stage: stage, cause: cause}
}

// AcquisitionError preserves an existing typed condition; otherwise it uses only
// structured OS causes. ELOOP and unsupported syscalls remain generic I/O failures:
// neither proves a symlink at the final path or unavailable no-atime support.
func AcquisitionError(err error) *Error {
	if err == nil {
		return nil
	}
	var typed *Error
	if errors.As(err, &typed) && typed != nil {
		if err == typed {
			return typed
		}
		return Wrap(typed.Stage(), typed.Code(), err)
	}
	code := IOFailed
	if errors.Is(err, os.ErrNotExist) {
		code = FileNotFound
	} else if errors.Is(err, os.ErrPermission) {
		code = PermissionDenied
	}
	return Wrap(Acquisition, code, err)
}
