package reveal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
)

var testLimits = Limits{SourceBytes: 1 << 20, OutputBytes: 4 << 20, Occurrences: 10000}

func native(t *testing.T, raw []byte) (*evidence.Text, []evidence.Finding) {
	t.Helper()
	d, err := (parsers.TextParser{}).Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	findings := (analyzers.Unicode{}).Analyze(&d)
	findings = append(findings, (analyzers.Emoji{}).Analyze(&d)...)
	findings = append(findings, (analyzers.Patterns{}).Analyze(&d)...)
	return d.Texts[0], findings
}

// Reconstruct decoded text by replacing mapped presentation spans with original
// scalars. This proves the occurrence map is lossless without interpreting HTML
// or guessing whether marker-looking source text is a generated annotation.
func restore(t *testing.T, result *Result, original string) string {
	t.Helper()
	var b strings.Builder
	scalars := []rune(original)
	previous := uint64(0)
	for _, occurrence := range result.Occurrences {
		span := occurrence.RenderedBytes
		if span.Start < previous || span.End > uint64(len(result.Text)) {
			t.Fatal("invalid rendered map")
		}
		b.WriteString(result.Text[previous:span.Start])
		b.WriteRune(scalars[occurrence.Scalar.Start])
		previous = span.End
	}
	b.WriteString(result.Text[previous:])
	return b.String()
}

func TestRenderEvidence(t *testing.T) {
	for _, s := range []string{"", "clean ASCII\r\nnext\rend\n", "日本語 café", "a\u200bb", "a\u202eb", "a\ufe0fb", "e\u0301", "👩\u200d💻", "\U000e0061\U000e007f", "\ufeffa\ufeffb", "literal ⟦U+200B ZERO WIDTH SPACE⟧ ⟦⟦", "<script>literal</script>\x1b[31m", strings.Repeat("\u200b\u200c", 32)} {
		t.Run(s, func(t *testing.T) {
			raw := []byte(s)
			before := bytes.Clone(raw)
			text, findings := native(t, raw)
			beforeEvidence, err := json.Marshal(findings)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Render(raw, evidence.Hash(raw), text, findings, testLimits)
			if err != nil {
				t.Fatal(err)
			}
			if restore(t, got, s) != s {
				t.Fatal("representation did not round trip")
			}
			again, err := Render(raw, evidence.Hash(raw), text, findings, testLimits)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("nondeterministic rendering", err)
			}
			afterEvidence, err := json.Marshal(findings)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(raw, before) || !bytes.Equal(beforeEvidence, afterEvidence) {
				t.Fatal("input mutated")
			}
			if got.RenderedSHA256 != evidence.Hash([]byte(got.Text)) {
				t.Fatal("wrong derivative digest")
			}
			if strings.Contains(s, "\u200b") && !strings.Contains(got.Text, "⟦U+200B ZERO WIDTH SPACE⟧") {
				t.Fatal("missing zero-width marker")
			}
			if strings.ContainsRune(got.Text, '\x1b') || strings.ContainsRune(got.Text, '\u202e') {
				t.Fatal("unsafe control survived")
			}
			if strings.Contains(s, "clean ASCII") && got.Text != s {
				t.Fatal("line endings changed")
			}
		})
	}
}

func TestMultipleFindingsOneMarker(t *testing.T) {
	raw := []byte(strings.Repeat("\u200b\u200c", 32))
	text, findings := native(t, raw)
	got, err := Render(raw, evidence.Hash(raw), text, findings, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Occurrences) != 64 {
		t.Fatal("multiple markers per source scalar")
	}
	foundPattern := false
	for _, f := range got.Occurrences[0].Findings {
		if f.ID == "pattern.zero_width_binary" {
			foundPattern = true
		}
	}
	if !foundPattern || len(got.Occurrences[0].Findings) < 2 {
		t.Fatal("lost overlapping evidence")
	}
}

func TestOriginalEncodingCoordinates(t *testing.T) {
	for _, encoding := range []string{"utf-16-le", "utf-16-be", "utf-32-le", "utf-32-be"} {
		t.Run(encoding, func(t *testing.T) {
			s := "\ufeff😀a\u200b\r\n"
			var raw []byte
			var order binary.AppendByteOrder = binary.LittleEndian
			if strings.HasSuffix(encoding, "be") {
				order = binary.BigEndian
			}
			if strings.HasPrefix(encoding, "utf-16") {
				for _, r := range utf16.Encode([]rune(s)) {
					raw = order.AppendUint16(raw, r)
				}
			} else {
				for _, r := range s {
					raw = order.AppendUint32(raw, uint32(r))
				}
			}
			text, findings := native(t, raw)
			got, err := Render(raw, evidence.Hash(raw), text, findings, testLimits)
			if err != nil {
				t.Fatal(err)
			}
			if restore(t, got, s) != s {
				t.Fatal("encoding round trip failed")
			}
			expectedByte := uint64(8)
			if strings.HasPrefix(encoding, "utf-32") {
				expectedByte = 12
			}
			found := false
			for _, o := range got.Occurrences {
				if o.CodePoint == "U+200B" {
					found = true
					if o.Scalar.Start != 3 || o.SourceBytes.Start != expectedByte {
						t.Fatalf("wrong original coordinates: %+v", o)
					}
				}
			}
			if !found {
				t.Fatal("missing marker")
			}
		})
	}
}

func TestRejectInvalidEvidenceAndBounds(t *testing.T) {
	for _, mutation := range []string{"hash", "text", "map", "coordinates", "codepoint", "source_limit", "output_limit", "occurrence_limit", "zero_limit"} {
		t.Run(mutation, func(t *testing.T) {
			raw := []byte("a\u200b\u200b")
			text, findings := native(t, raw)
			hash, limits := evidence.Hash(raw), testLimits
			switch mutation {
			case "hash":
				hash = "bad"
			case "text":
				text.Text = "other"
			case "map":
				text.ByteOffsets[1] = 0
			case "coordinates":
				findings[0].Location["character_offsets"] = []int{-1}
			case "codepoint":
				findings[0].Evidence["code_point"] = "U+0041"
			case "source_limit":
				limits.SourceBytes = 1
			case "output_limit":
				limits.OutputBytes = 1
			case "occurrence_limit":
				limits.Occurrences = 1
			case "zero_limit":
				limits.OutputBytes = 0
			}
			got, err := Render(raw, hash, text, findings, limits)
			if err == nil || got != nil {
				t.Fatal("invalid input returned a derivative")
			}
		})
	}
}

func TestDisplayEscapesAreNotFindings(t *testing.T) {
	raw := []byte("⟦\x1b\u202e\r\n")
	text, _ := native(t, raw)
	got, err := Render(raw, evidence.Hash(raw), text, nil, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Occurrences) != 3 || !strings.HasPrefix(got.Text, "⟦⟦") {
		t.Fatal("missing unambiguous escapes")
	}
	for _, o := range got.Occurrences {
		if len(o.Findings) != 0 || o.Reason == "finding" {
			t.Fatal("invented evidence")
		}
	}
}

func TestOrdinaryLocatedFindingsDoNotReplaceProse(t *testing.T) {
	raw := []byte("ordinary")
	text, _ := native(t, raw)
	f := evidence.Finding{ID: "identifier.example", Location: evidence.Location(text, []int{0})}
	got, err := Render(raw, evidence.Hash(raw), text, []evidence.Finding{f}, testLimits)
	if err != nil || got.Text != string(raw) || len(got.Occurrences) != 0 {
		t.Fatal("ordinary prose replaced", err)
	}
}

func TestGoldenMarkerAndOffsets(t *testing.T) {
	raw := []byte("é\u200b\r\n")
	text, findings := native(t, raw)
	got, err := Render(raw, evidence.Hash(raw), text, findings, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "⟦U+200B ZERO WIDTH SPACE⟧"
	if got.Text != "é"+marker+"\r\n" || len(got.Occurrences) != 1 {
		t.Fatalf("unexpected rendering: %#v", got)
	}
	o := got.Occurrences[0]
	if o.Scalar.Start != 1 || o.Scalar.End != 2 || o.SourceBytes.Start != 2 || o.SourceBytes.End != 5 || o.RenderedBytes.Start != 2 || o.RenderedBytes.End != uint64(2+len(marker)) {
		t.Fatalf("incorrect half-open coordinates: %+v", o)
	}
	if len(o.Findings) != 1 || o.Findings[0].ID != "unicode.zero_width" || o.Reason != "finding" {
		t.Fatalf("lost finding: %+v", o)
	}
}
