package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type deliveryWriter struct {
	short              bool
	writeErr, closeErr error
	closed             bool
}

func (w *deliveryWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if w.short {
		return len(p) - 1, nil
	}
	return len(p), nil
}
func (w *deliveryWriter) Close() error { w.closed = true; return w.closeErr }

func TestV2DeliveryFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		writer    deliveryWriter
		createErr error
		want      string
		removed   bool
	}{
		{name: "success"},
		{name: "create", createErr: errors.New("exists"), want: "output.create_failed"},
		{name: "short", writer: deliveryWriter{short: true}, want: "output.write_failed", removed: true},
		{name: "write", writer: deliveryWriter{writeErr: errors.New("full")}, want: "output.write_failed", removed: true},
		{name: "close", writer: deliveryWriter{closeErr: errors.New("close")}, want: "output.close_failed", removed: true},
		{name: "both", writer: deliveryWriter{writeErr: errors.New("full"), closeErr: errors.New("close")}, want: "output.write_failed", removed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			removed := false
			got := deliverV2("report", "destination", io.Discard, func(string) (io.WriteCloser, error) { return &tc.writer, tc.createErr }, func(path string) error {
				if path != "destination" {
					t.Fatal(path)
				}
				removed = true
				return errors.New("ignored cleanup failure")
			})
			if got != tc.want || removed != tc.removed || tc.writer.closed != (tc.createErr == nil) {
				t.Fatalf("got %s removed=%v closed=%v", got, removed, tc.writer.closed)
			}
		})
	}
	w := &deliveryWriter{short: true}
	if got := deliverV2("report", "", w, nil, nil); got != "output.write_failed" || w.closed {
		t.Fatal("stdout short write ignored or closed")
	}
}
func TestSchemaVersionArguments(t *testing.T) {
	for _, value := range []string{"", "3.0", "2", "-h"} {
		for _, args := range [][]string{{"audit", "unused", "--schema-version", value}, {"audit", "unused", "--schema-version=" + value}} {
			var out, err bytes.Buffer
			if Run(args, &out, &err) != 4 || out.Len() != 0 || !strings.Contains(err.String(), "requires 1.0 or 2.0") {
				t.Fatalf("accepted %v", args)
			}
		}
	}
	var out, err bytes.Buffer
	if Run([]string{"audit", "unused", "--schema-version"}, &out, &err) != 4 {
		t.Fatal("missing schema argument accepted")
	}
}
