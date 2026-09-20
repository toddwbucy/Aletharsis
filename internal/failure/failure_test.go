package failure_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestCauseAndMessagePreserved(t *testing.T) {
	cause := &os.PathError{Op: "open", Path: "untrusted.txt", Err: os.ErrPermission}
	err := failure.Wrap(failure.Acquisition, failure.PermissionDenied, cause)
	var path *os.PathError
	if !errors.Is(err, os.ErrPermission) || !errors.As(err, &path) || path != cause || err.Error() != cause.Error() {
		t.Fatal("error chain changed")
	}
	if err.Stage() != failure.Acquisition || err.Code() != failure.PermissionDenied {
		t.Fatal("identity missing")
	}
	if failure.Wrap(failure.Acquisition, failure.IOFailed, nil) != nil || failure.AcquisitionError(nil) != nil {
		t.Fatal("nil cause acquired a failure")
	}
}
func TestClassificationDoesNotReadMessages(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want failure.Code
	}{
		{fmt.Errorf("wrapped: %w", os.ErrNotExist), failure.FileNotFound},
		{fmt.Errorf("wrapped: %w", os.ErrPermission), failure.PermissionDenied},
		{errors.New("permission denied; file.not_found; no-atime unavailable"), failure.IOFailed},
	} {
		if got := failure.AcquisitionError(tc.err); got.Code() != tc.want {
			t.Fatal(got.Code(), tc.want)
		}
	}
	typed := failure.Wrap(failure.Acquisition, failure.TooLarge, errors.New("opaque"))
	wrapped := fmt.Errorf("wrapped: %w", typed)
	got := failure.AcquisitionError(wrapped)
	if got.Code() != typed.Code() || got.Stage() != typed.Stage() || got.Error() != wrapped.Error() || !errors.Is(got, wrapped) {
		t.Fatal("typed condition or error context discarded")
	}
	if failure.AcquisitionError(typed) != typed {
		t.Fatal("typed error rewrapped unnecessarily")
	}
}

func TestZeroFailureRemainsConservative(t *testing.T) {
	for _, err := range []*failure.Error{nil, {}} {
		if err.Code() != failure.AuditFailed || err.Stage() != failure.Audit || err.Error() == "" || err.Unwrap() != nil {
			t.Fatal("invalid empty failure behavior")
		}
	}
}
