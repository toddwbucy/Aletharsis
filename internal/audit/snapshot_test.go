package audit

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestSnapshotSingleAcquisition(t *testing.T) {
	for _, schema := range []string{"1", "2"} {
		t.Run(schema, func(t *testing.T) {
			if schema == "2" && runtime.GOOS != "linux" {
				t.Skip("native acquisition unavailable")
			}
			calls := 0
			data := []byte("one\u200b acquisition")
			read := func(string, int) ([]byte, error) { calls++; return data, nil }
			var source []byte
			var report *evidence.Report
			if schema == "1" {
				out, raw := inspectSnapshotWithReader("input.txt", MaxBytes, read)
				source, report = raw, out.Report
			} else {
				options := DefaultV2Options()
				options.RetainSnapshot = true
				out, err := runV2WithReader(context.Background(), "input.txt", options, read, nil)
				if err != nil {
					t.Fatal(err)
				}
				source, report = out.Source, out.Native
			}
			if calls != 1 || string(source) != string(data) || report == nil || report.File.SHA256 == nil || *report.File.SHA256 != evidence.Hash(source) {
				t.Fatal("snapshot mismatch or repeated acquisition")
			}
		})
	}
}
func TestSnapshotFailureDoesNotExposeBytes(t *testing.T) {
	out, source := inspectSnapshotWithReader("input.txt", MaxBytes, func(string, int) ([]byte, error) { return []byte("partial"), errors.New("read failed") })
	if out.Failure == nil || source != nil {
		t.Fatal("partial acquisition retained")
	}
}
