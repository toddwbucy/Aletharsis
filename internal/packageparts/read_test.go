package packageparts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

type entry struct {
	name, text string
	method     uint16
}

func archive(t testing.TB, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		f, err := w.CreateHeader(&zip.FileHeader{Name: e.name, Method: e.method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(e.text)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func read(b []byte) (*Package, error) {
	return Read(context.Background(), b, evidence.Hash(b), DefaultLimits())
}
func rejected(t testing.TB, b []byte, want error) {
	t.Helper()
	p, err := read(b)
	if p != nil || !errors.Is(err, want) {
		t.Fatalf("got package %v, error %v; want %v", p, err, want)
	}
}
func TestPartsExactIdentityAndOwnership(t *testing.T) {
	b := archive(t, entry{"word/document.xml", "\ufeff<p>A\u200b😀\r\n</p>", zip.Deflate}, entry{"custom/data.bin", "\x00\xff", zip.Store}, entry{"word/", "", zip.Store})
	original := bytes.Clone(b)
	a, err := read(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := read(b)
	if err != nil || !reflect.DeepEqual(a, again) {
		t.Fatal("nondeterministic", err)
	}
	if a.Parser != Version || a.SourceSHA256 != evidence.Hash(b) || len(a.Parts) != 3 || a.Parts[0].Name != "custom/data.bin" || a.Parts[2].Name != "word/document.xml" {
		t.Fatal("identity/order", a)
	}
	for _, p := range a.Parts {
		if p.SHA256 != evidence.Hash(p.Bytes) || p.CompressedSHA256 != evidence.Hash(b[p.CompressedSpan.Start:p.CompressedSpan.End]) {
			t.Fatal("part identity")
		}
	}
	if string(a.Parts[2].Bytes) != "\ufeff<p>A\u200b😀\r\n</p>" {
		t.Fatal("normalized content")
	}
	a.Parts[0].Bytes[0] = 42
	if !bytes.Equal(b, original) || again.Parts[0].Bytes[0] != 0 {
		t.Fatal("shared mutable source/result")
	}
}
func TestNamesAndFileTypes(t *testing.T) {
	for _, name := range []string{"../a", "/a", "a/../b", "a/./b", "a//b", "a\\b", "a:b", "a\x00b", "a\nb", string([]byte{0xff})} {
		t.Run(name, func(t *testing.T) { rejected(t, archive(t, entry{name, "x", zip.Store}), ErrName) })
	}
	for _, entries := range [][]entry{{{"a", "", 0}, {"a", "", 0}}, {{"a", "", 0}, {"a/", "", 0}}, {{"a", "", 0}, {"a/b", "x", 0}}} {
		rejected(t, archive(t, entries...), ErrName)
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(os.ModeSymlink | 0600)
	f, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	rejected(t, b.Bytes(), ErrUnsupported)
}
func TestHeaderAndPayloadCorruption(t *testing.T) {
	base := archive(t, entry{"a", "abcdef", zip.Store})
	central, _, err := directory(base, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func([]byte)
		want   error
	}{
		{"local_name", func(b []byte) { b[30] = 'b' }, ErrFormat},
		{"local_method", func(b []byte) { b[8] = 8 }, ErrFormat},
		{"payload_crc", func(b []byte) { b[31] ^= 1 }, ErrFormat},
		{"descriptor_crc", func(b []byte) { b[41] ^= 1 }, ErrFormat},
		{"encryption", func(b []byte) { b[central+8] |= 1 }, ErrUnsupported},
		{"method", func(b []byte) { b[central+10] = 99 }, ErrUnsupported},
		{"count_lie", func(b []byte) { end := len(b) - 22; b[end+8] = 0; b[end+10] = 0 }, ErrFormat},
		{"offset", func(b []byte) { binary.LittleEndian.PutUint32(b[central+42:], 0xfffffff0) }, ErrFormat},
		{"huge_expansion_inconsistent_header", func(b []byte) { binary.LittleEndian.PutUint32(b[central+24:], 1<<30) }, ErrFormat},
		{"huge_expansion", func(b []byte) {
			binary.LittleEndian.PutUint32(b[central+24:], 1<<30)
			// Keep the data descriptor consistent so this exercises the
			// expansion policy, not the earlier structural identity check.
			binary.LittleEndian.PutUint32(b[central-4:], 1<<30)
		}, ErrLimit},
		{"multidisk", func(b []byte) { b[len(b)-18] = 1 }, ErrUnsupported},
		{"zip64", func(b []byte) { binary.LittleEndian.PutUint32(b[central+24:], 0xffffffff) }, ErrUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { b := bytes.Clone(base); tc.mutate(b); rejected(t, b, tc.want) })
	}
	rejected(t, append(bytes.Clone(base), 'x'), ErrFormat)
	for i := 0; i < len(base); i++ {
		if p, err := read(base[:i]); p != nil || err == nil {
			t.Fatal("accepted truncated archive", i)
		}
	}
}
func TestBoundsAndCancellation(t *testing.T) {
	b := archive(t, entry{"first", strings.Repeat("a", 4096), zip.Deflate}, entry{"second", "hello", zip.Store})
	for _, change := range []func(*Limits){func(l *Limits) { l.SourceBytes = len(b) - 1 }, func(l *Limits) { l.Entries = 1 }, func(l *Limits) { l.PartBytes = 4095 }, func(l *Limits) { l.TotalBytes = 4096 }, func(l *Limits) { l.NameBytes = 10 }, func(l *Limits) { l.Entries = 0 }} {
		l := DefaultLimits()
		change(&l)
		p, err := Read(context.Background(), b, evidence.Hash(b), l)
		if p != nil || !errors.Is(err, ErrLimit) {
			t.Fatal("limit bypass", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p, err := Read(ctx, b, evidence.Hash(b), DefaultLimits()); p != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	if p, err := Read(context.Background(), b, strings.Repeat("0", 64), DefaultLimits()); p != nil || !errors.Is(err, ErrIdentity) {
		t.Fatal("source identity", err)
	}
}
func TestEmptyAndOverlappingRecords(t *testing.T) {
	empty := archive(t)
	p, err := read(empty)
	if err != nil || len(p.Parts) != 0 {
		t.Fatal("empty ZIP", err)
	}
	b := archive(t, entry{"a", "x", zip.Store}, entry{"b", "x", zip.Store})
	c, _, err := directory(b, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	// Second central entry aliases the first local payload while using its name.
	second := c + 47
	b[second+46] = 'a'
	binary.LittleEndian.PutUint32(b[second+42:], 0)
	rejected(t, b, ErrName)
	// Padding/prefix bytes are not silently ignored as outside-package content.
	b = archive(t, entry{"a", "x", zip.Store})
	c, _, err = directory(b, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	padded := append([]byte{0}, b...)
	binary.LittleEndian.PutUint32(padded[c+1+42:], 1)
	binary.LittleEndian.PutUint32(padded[len(padded)-6:], uint32(c+1))
	rejected(t, padded, ErrFormat)
}
func TestRetainedOfficePackages(t *testing.T) {
	for _, name := range []string{"minimal.docx", "hidden.docx", "minimal.odt", "hidden.odt"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(b)
			p, err := read(b)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, before) {
				t.Fatal("source changed")
			}
			partName := "word/document.xml"
			if strings.HasSuffix(name, ".odt") {
				partName = "content.xml"
			}
			found := false
			for _, part := range p.Parts {
				if part.Name == partName {
					found = true
					if !bytes.Contains(part.Bytes, []byte("Hello")) {
						t.Fatal("lost visible content")
					}
					if strings.HasPrefix(name, "hidden") && !bytes.Contains(part.Bytes, []byte("\u200b\u200c")) {
						t.Fatal("lost concealed sequence")
					}
				}
			}
			if !found {
				t.Fatal("missing body part")
			}
		})
	}
}
func FuzzRead(f *testing.F) {
	f.Add(archive(f, entry{"a", "hello", zip.Deflate}))
	f.Add([]byte("PK"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			t.Skip()
		}
		l := DefaultLimits()
		l.SourceBytes = 1 << 16
		l.PartBytes = 1 << 16
		l.TotalBytes = 1 << 18
		l.Entries = 64
		p, err := Read(context.Background(), b, evidence.Hash(b), l)
		if err != nil && p != nil {
			t.Fatal("partial result")
		}
		if err == nil {
			for _, part := range p.Parts {
				if part.SHA256 != evidence.Hash(part.Bytes) {
					t.Fatal("identity")
				}
			}
		}
	})
}

func TestRetainedPartManifest(t *testing.T) {
	raw, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []struct {
		Name   string
		SHA256 string
		Parts  []struct {
			Name   string
			Size   int
			SHA256 string
		}
	}
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatal("fixture count")
	}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.Name] {
			t.Fatal("duplicate fixture")
		}
		seen[record.Name] = true
		b, err := os.ReadFile(filepath.Join("testdata", record.Name))
		if err != nil {
			t.Fatal(err)
		}
		p, err := Read(context.Background(), b, record.SHA256, DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Parts) != len(record.Parts) {
			t.Fatal("part count")
		}
		for i, part := range p.Parts {
			want := record.Parts[i]
			if part.Name != want.Name || len(part.Bytes) != want.Size || part.SHA256 != want.SHA256 {
				t.Fatal("retained part identity differs", record.Name, part.Name)
			}
		}
	}
}
func TestContextReaderCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := contextReader{ctx: ctx, r: strings.NewReader("payload")}
	n, err := r.Read(make([]byte, 16))
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled read consumed data", n, err)
	}
}

func TestDescriptorFormsAndCentralOrder(t *testing.T) {
	b := archive(t, entry{"a", "payload", zip.Store})
	central, _, err := directory(b, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if u32(b[central-16:]) != 0x08074b50 {
		t.Fatal("fixture lacks signed descriptor")
	}
	unsigned := append(bytes.Clone(b[:central-16]), b[central-12:]...)
	binary.LittleEndian.PutUint32(unsigned[len(unsigned)-6:], uint32(central-4))
	p, err := read(unsigned)
	if err != nil || string(p.Parts[0].Bytes) != "payload" {
		t.Fatal("unsigned descriptor", err)
	}
	b = archive(t, entry{"a", "first", zip.Store}, entry{"b", "second", zip.Store})
	central, _, err = directory(b, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(b)
	copy(b[central:central+47], original[central+47:central+94])
	copy(b[central+47:central+94], original[central:central+47])
	p, err = read(b)
	if err != nil || p.Parts[0].Name != "a" || string(p.Parts[1].Bytes) != "second" {
		t.Fatal("central order affects part ordering", err)
	}
}
