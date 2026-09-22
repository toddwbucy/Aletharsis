package reveal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

var diffLimits = DiffLimits{TextBytes: 1 << 20, OutputBytes: 2 << 20, Lines: 10000, Escapes: 10000, Context: 1}

func TestUnifiedGolden(t *testing.T) {
	remaining := diffLimits.OutputBytes
	got, err := unified("one\ntwo\nthree", "one\nTWO\nthree", "decoded_utf8", diffLimits, &remaining)
	want := "--- a/decoded.txt\n+++ b/revealed.txt\n@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n\\ No newline at end of file\n"
	if err != nil || got.Text != want {
		t.Fatalf("diff: %q; %v", got.Text, err)
	}
	if got.SHA256 != evidence.Hash([]byte(want)) || remaining != diffLimits.OutputBytes-len(want) {
		t.Fatal("digest or budget mismatch")
	}
}

// Independently apply patches only to temporary copies. This is a test oracle,
// not a production dependency or authorization to patch original evidence.
func TestUnifiedPatchOracle(t *testing.T) {
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("optional independent patch oracle unavailable")
	}
	cases := [][2]string{{"", "a\n"}, {"a", ""}, {"a\r\nb\r\n", "a\r\nB\r\n"}, {"a\rb", "A\rb"}, {"a\nb", "A\nB"}, {"a\nb\nc\nd\ne\nf\ng\n", "A\nb\nc\nd\ne\nf\nG\n"}, {"a\nb\nc\nd\n", "a\nB\nC\nd\n"}, {"a\n", "a\nb\n"}, {"x\u200by\n", "x⟦U+200B ZERO WIDTH SPACE⟧y\n"}}
	for _, pair := range cases {
		for _, n := range []int{0, 1, 3} {
			limits := diffLimits
			limits.Context = n
			remaining := limits.OutputBytes
			got, err := unified(pair[0], pair[1], "decoded_utf8", limits, &remaining)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			original := filepath.Join(dir, "original")
			derivative := filepath.Join(dir, "derivative")
			if err := os.WriteFile(original, []byte(pair[0]), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, patch, "--batch", "--binary", "--output", derivative, original)
			cmd.Stdin = strings.NewReader(got.Text)
			out, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("patch: %v %q; diff %q", err, out, got.Text)
			}
			result, err := os.ReadFile(derivative)
			if err != nil {
				t.Fatal(err)
			}
			if string(result) != pair[1] {
				t.Fatalf("patch mismatch: %q != %q", result, pair[1])
			}
			intact, err := os.ReadFile(original)
			if err != nil || string(intact) != pair[0] {
				t.Fatal("temporary original changed", err)
			}
		}
	}
}

func TestDisplayEscapesReversible(t *testing.T) {
	input := "literal \\u{200B}\t\r\n\x1b\u202eé👩\u200d💻"
	got, err := escapeDisplay(input, 10000, 100)
	if err != nil {
		t.Fatal(err)
	}
	var restored strings.Builder
	last := uint64(0)
	for _, e := range got.Escapes {
		if e.OutputBytes.Start < last || e.OutputBytes.End > uint64(len(got.Text)) {
			t.Fatal("invalid output span")
		}
		raw := input[e.InputBytes.Start:e.InputBytes.End]
		if utf8.RuneCountInString(raw) != 1 || utf8.RuneCountInString(input[:e.InputBytes.Start]) != e.InputScalar {
			t.Fatal("invalid input coordinates")
		}
		restored.WriteString(got.Text[last:e.OutputBytes.Start])
		restored.WriteString(raw)
		last = e.OutputBytes.End
	}
	restored.WriteString(got.Text[last:])
	if restored.String() != input {
		t.Fatal("escape map lost evidence")
	}
	for _, r := range got.Text {
		if r != '\n' && (r < 32 || r > 126) {
			t.Fatal("active or non-ASCII display character")
		}
	}
	if !strings.Contains(got.Text, `\\u{200B}`) || !strings.Contains(got.Text, `\u{1F469}`) {
		t.Fatal("escape collision or astral encoding")
	}
	if got.InputSHA256 != evidence.Hash([]byte(input)) || got.SHA256 != evidence.Hash([]byte(got.Text)) {
		t.Fatal("hash mismatch")
	}
}

func TestCompareIntegrity(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("clean\r\n"), []byte("a\u200b\u202e👩\u200d💻e\u0301\x1b\r\n"), {0xff, 0xfe, 'a', 0, 0x0b, 0x20}, {0, 0, 0xfe, 0xff, 0, 0, 0, 97, 0, 0, 0x20, 0x0b}} {
		before := bytes.Clone(raw)
		text, findings := native(t, raw)
		snapshot, err := json.Marshal([]any{text, findings})
		if err != nil {
			t.Fatal(err)
		}
		got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, diffLimits)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, diffLimits)
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatal("nondeterministic comparison", err)
		}
		after, err := json.Marshal([]any{text, findings})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, raw) || !bytes.Equal(snapshot, after) {
			t.Fatal("mutated source or evidence")
		}
		if got.Faithful.BeforeSHA256 != evidence.Hash([]byte(text.Text)) || got.Faithful.AfterSHA256 != got.Reveal.RenderedSHA256 || got.Reveal.SourceSHA256 != evidence.Hash(raw) {
			t.Fatal("representation identities confused")
		}
		if got.Presentation.Diff.BeforeSHA256 != got.Presentation.Before.SHA256 || got.Presentation.Diff.AfterSHA256 != got.Presentation.After.SHA256 {
			t.Fatal("display lineage mismatch")
		}
		if strings.Contains(text.Text, "\u202e") && !strings.Contains(got.Faithful.Text, "\u202e") {
			t.Fatal("faithful diff sanitized evidence")
		}
		for _, r := range got.Presentation.Diff.Text {
			if r != '\n' && (r < 32 || r > 126) {
				t.Fatal("unsafe display diff")
			}
		}
		if got.DecodedText == got.Reveal.Text && (got.Faithful.Text != "" || got.Presentation.Diff.Text != "") {
			t.Fatal("unchanged artifact has diff")
		}
	}
}

func TestCompareLimitsAndValidation(t *testing.T) {
	raw := []byte("a\u200bb\n")
	text, findings := native(t, raw)
	for _, change := range []func(*DiffLimits){func(l *DiffLimits) { l.TextBytes = 1 }, func(l *DiffLimits) { l.OutputBytes = 1 }, func(l *DiffLimits) { l.Lines = 1 }, func(l *DiffLimits) { l.Escapes = 1 }, func(l *DiffLimits) { l.Context = -1 }, func(l *DiffLimits) { l.Context = 1001 }, func(l *DiffLimits) { l.Escapes = 0 }} {
		limits := diffLimits
		change(&limits)
		got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, limits)
		if !errors.Is(err, ErrDiffLimit) || got != nil {
			t.Fatalf("limit returned partial or successful result: %v", err)
		}
	}
	if got, err := Compare(raw, strings.Repeat("0", 64), text, findings, testLimits, diffLimits); err == nil || got != nil {
		t.Fatal("accepted changed source")
	}
	text.ByteOffsets[1]++
	if got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, diffLimits); err == nil || got != nil {
		t.Fatal("accepted invalid source map")
	}
}

func TestEscapedRepresentationPairSharesBudget(t *testing.T) {
	raw := []byte("日")
	text, findings := native(t, raw)
	limits := diffLimits
	// Each escaped side is eight ASCII bytes; the pair requires sixteen.
	limits.TextBytes = 15
	if got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, limits); got != nil || !errors.Is(err, ErrDiffLimit) {
		t.Fatal("combined escaped byte budget bypassed", err)
	}
	limits.TextBytes = 16
	limits.Escapes = 1
	if got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, limits); got != nil || !errors.Is(err, ErrDiffLimit) {
		t.Fatal("combined escape map budget bypassed", err)
	}
	limits.Escapes = 2
	got, err := Compare(raw, evidence.Hash(raw), text, findings, testLimits, limits)
	if err != nil || len(got.Presentation.Before.Text)+len(got.Presentation.After.Text) != 16 {
		t.Fatal("exact shared budget", err)
	}
}
