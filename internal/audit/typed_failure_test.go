package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

func TestTypedNativeOutcome(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  []byte
		err   error
		code  failure.Code
		stage failure.Stage
	}{
		{"missing", nil, os.ErrNotExist, failure.FileNotFound, failure.Acquisition},
		{"denied", nil, os.ErrPermission, failure.PermissionDenied, failure.Acquisition},
		{"opaque", nil, errors.New("permission denied"), failure.IOFailed, failure.Acquisition},
		{"format", []byte("%PDF-1.7\n"), nil, failure.UnsupportedFormat, failure.Parsing},
		{"decode", []byte{0xff, 0xfe, 0x00}, nil, failure.DecodeFailed, failure.Parsing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := inspectWithReader("source.txt", 32, func(string, int) ([]byte, error) { return tc.data, tc.err })
			if outcome.Failure == nil || outcome.Failure.Code() != tc.code || outcome.Failure.Stage() != tc.stage {
				t.Fatal(outcome.Failure)
			}
			if outcome.Report.Status != "failed" || outcome.Report.Summary["exit_code"] != 4 {
				t.Fatal("failed outcome reported success")
			}
			if tc.stage == failure.Acquisition && (outcome.Report.File.SHA256 != nil || outcome.Report.File.Size != nil) {
				t.Fatal("unacquired identity published")
			}
			if tc.code == failure.DecodeFailed {
				var decode *parsers.DecodeError
				if !errors.As(outcome.Failure, &decode) {
					t.Fatal("decoder identity lost")
				}
			}
			scope := v2.Scope{ArtifactRef: "artifact/0", Unit: "whole_artifact"}
			d, err := outcome.Diagnostic("diagnostic/0", "exec/0", &scope)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := v2.DecodeDiagnostic(raw)
			if err != nil || parsed.Code != tc.code {
				t.Fatal(parsed, err)
			}
			// A machine code must not leak into the frozen schema-1 finding payload.
			if _, ok := outcome.Report.Findings[0].Evidence["failure_code"]; ok {
				t.Fatal("schema 1.0 changed")
			}
		})
	}
	outcome := inspectWithReader("source.txt", 32, func(string, int) ([]byte, error) { return []byte("plain"), nil })
	if outcome.Failure != nil || outcome.Report.Status != "completed" {
		t.Fatal(outcome.Failure)
	}
	if d, err := outcome.Diagnostic("diagnostic/0", "exec/0", nil); d != nil || err != nil {
		t.Fatal("success acquired diagnostic")
	}
}
func TestNativeInspectPlatformBoundary(t *testing.T) {
	outcome := Inspect(filepath.Join(t.TempDir(), "missing.txt"), 32)
	want := failure.FileNotFound
	if runtime.GOOS != "linux" {
		want = failure.NoAtimeUnavailable
	}
	if outcome.Failure == nil || outcome.Failure.Code() != want {
		t.Fatal(outcome.Failure, want)
	}
}

func TestRegistryMatchesExecutedNativeChecks(t *testing.T) {
	catalog, err := capability.Native(Version, MaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, c := range catalog {
		if c.Role == v2.Analyzer && c.Implementation != nil {
			expected[c.ID] = true
		}
	}
	for _, check := range checks {
		if !expected[check.id] {
			t.Fatal("unregistered or duplicate native check", check.id)
		}
		delete(expected, check.id)
	}
	if len(expected) != 0 {
		t.Fatal("declared analyzer is not executed", expected)
	}
}
