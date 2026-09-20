//go:build linux

package reportimport_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/reportimport"
)

// This is report-to-consumer acceptance, not a viewer or permission to edit.
func TestNativeReportConsumerCoordinates(t *testing.T) {
	source := []byte("\ufeff😀e\u0301\u200bA\u202eZ\u2060B")
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	options := audit.DefaultV2Options()
	emitted, err := audit.RunV2(context.Background(), path, options)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := reportimport.Read(emitted.JSON, options.ReportLimits)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ReportArtifactSHA256 != identity.ExactBytes(emitted.JSON) || imported.Coverage != "declared" {
		t.Fatal("report identity lost")
	}
	r := imported.V2
	if r.File.SHA256 == nil || *r.File.SHA256 != identity.ExactBytes(source) {
		t.Fatal("source identity lost")
	}
	text := r.Evidence.Texts[0]
	scalars := []rune(text.Text)
	// Independent byte/UTF-16 boundary table for BOM, supplementary emoji,
	// combining marks, invisible formatting and bidi controls.
	byteBoundary := []uint64{0, 3, 7, 8, 10, 13, 14, 17, 18, 21, 22}
	utf16Boundary := []int{0, 1, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	display := utf16.Encode(scalars)
	seen := 0
	for _, a := range r.Anchors {
		if a.Kind != "text" {
			continue
		}
		loc, ok := a.Locator.(v2.TextLocator)
		if !ok {
			t.Fatal("wrong locator type")
		}
		got, err := identity.TextSelectionDigest(source, text, loc.Spans, len(source), v2.RecordLimits())
		if err != nil || got != loc.SelectedTextSHA256 {
			t.Fatal("selection digest not bound to source", err)
		}
		for _, span := range loc.Spans {
			start, end := int(span.Scalar.Start), int(span.Scalar.End)
			if span.Byte.Start != byteBoundary[start] || span.Byte.End != byteBoundary[end] {
				t.Fatal("original byte mapping changed")
			}
			selected := string(scalars[start:end])
			if string(source[span.Byte.Start:span.Byte.End]) != selected {
				t.Fatal("wrong source selection")
			}
			if string(utf16.Decode(display[utf16Boundary[start]:utf16Boundary[end]])) != selected {
				t.Fatal("UTF-16 projection changed selection")
			}
			seen++
		}
	}
	if seen < 4 {
		t.Fatalf("expected BOM/emoji/format evidence, got %d spans", seen)
	}
	// Display boundaries inside the emoji's surrogate pair are not source scalar
	// boundaries. A future UI adapter must reject/snap explicitly, never infer bytes.
	for _, boundary := range utf16Boundary {
		if boundary == 2 {
			t.Fatal("surrogate interior accepted")
		}
	}
	original := imported.Bytes()
	r.Evidence.Texts[0].Text = "consumer display state"
	if !bytes.Equal(imported.Bytes(), original) {
		t.Fatal("consumer state mutated evidence artifact")
	}
}
