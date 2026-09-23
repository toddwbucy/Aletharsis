package packageparts

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestReadOnlyOutcomeSnapshotsAndOwnedViews(t *testing.T) {
	raw := archive(t, entry{"a", "retained", zip.Deflate}, entry{"b", "later", zip.Store})
	r := openOutcomes(t, raw, DefaultLimits())
	if r.SourceSHA256() != evidence.Hash(raw) {
		t.Fatal("wrong source identity")
	}
	names := r.Names()
	names[0] = "modified"
	if !reflect.DeepEqual(r.Names(), []string{"a", "b"}) {
		t.Fatal("names alias reader")
	}
	if err := r.Admit(context.Background(), []string{"a"}); err != nil {
		t.Fatal(err)
	}
	before := r.ReadOnlyView()
	owned := r.View()
	if &before.Parts[0].Part.Bytes[0] == &owned.Parts[0].Part.Bytes[0] {
		t.Fatal("owned view borrows bytes")
	}
	if cap(before.Parts[0].Part.Bytes) != len(before.Parts[0].Part.Bytes) {
		t.Fatal("borrow exposes spare capacity")
	}
	if err := r.Admit(context.Background(), []string{"b"}); err != nil {
		t.Fatal(err)
	}
	after := r.ReadOnlyView()
	if before.Parts[1].State != "not_run" || after.Parts[1].State != "completed" {
		t.Fatal("outcomes alias later admission")
	}
	if &before.Parts[0].Part.Bytes[0] != &after.Parts[0].Part.Bytes[0] {
		t.Fatal("borrow copied payload")
	}
	before.Parts[0].State = "modified"
	owned.Parts[0].Part.Bytes[0] = 'X'
	current := r.View()
	if current.Parts[0].State != "completed" || string(current.Parts[0].Part.Bytes) != "retained" {
		t.Fatal("owned/value mutation reached reader")
	}
}

type stalledReader struct{}

func (stalledReader) Read([]byte) (int, error) { return 0, nil }
func TestReservedPayloadRetainsDecoderErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reader io.Reader
		size   int
		want   string
		err    error
	}{
		{"normal EOF", strings.NewReader("abc"), 3, "abc", nil},
		{"empty", strings.NewReader(""), 0, "", nil},
		{"overflow probe", strings.NewReader("abcdef"), 3, "abcd", nil},
		{"short", strings.NewReader("ab"), 3, "ab", nil},
		{"truncated after declared bytes", io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(io.ErrUnexpectedEOF)), 3, "abc", io.ErrUnexpectedEOF},
		{"CRC after declared bytes", io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(zip.ErrChecksum)), 3, "abc", zip.ErrChecksum},
		{"stalled", stalledReader{}, 3, "", io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := readReservedPayload(tc.reader, tc.size)
			if string(data) != tc.want || !errors.Is(err, tc.err) {
				t.Fatal(string(data), err)
			}
		})
	}
}

type observedReadSize struct {
	io.Reader
	largest int
}

func (r *observedReadSize) Read(p []byte) (int, error) {
	r.largest = max(r.largest, len(p))
	return r.Reader.Read(p)
}
func TestDeclaredCeilingDoesNotPresizePayload(t *testing.T) {
	r := &observedReadSize{Reader: strings.NewReader("tiny")}
	data, err := readReservedPayload(r, 32<<20)
	if err != nil || string(data) != "tiny" || r.largest > 1024 {
		t.Fatal("header claim sized a payload allocation", r.largest, err)
	}
}
