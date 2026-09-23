// Package packageparts reads bounded ZIP document containers from acquired bytes.
// It performs no filesystem access, XML interpretation, or format/trust inference.
package packageparts

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

var (
	ErrFormat      = errors.New("invalid document package")
	ErrUnsupported = errors.New("unsupported package feature")
	ErrLimit       = errors.New("package resource limit")
	ErrName        = errors.New("ambiguous or invalid package part name")
	ErrIdentity    = errors.New("package source identity mismatch")
)

const Version = "zip-parts/1"

type Limits struct{ SourceBytes, Entries, PartBytes, TotalBytes, NameBytes int }

func DefaultLimits() Limits { return Limits{8 << 20, 4096, 32 << 20, 64 << 20, 1 << 20} }

// CompressedSpan identifies the exact compressed payload in the original ZIP.
// It is NOT a byte mapping from decompressed XML or text to source bytes.
type Span struct{ Start, End int64 }
type Part struct {
	Name             string
	Directory        bool
	Method           uint16
	CRC32            uint32
	CompressedSpan   Span
	CompressedSHA256 string
	SHA256           string // Exact decompressed bytes, including BOMs and line endings.
	Bytes            []byte
}
type Package struct {
	SourceSHA256 string
	Parser       string
	Parts        []Part
}

func validLimits(l Limits) bool {
	h := DefaultLimits()
	return l.SourceBytes > 0 && l.SourceBytes <= h.SourceBytes && l.Entries > 0 && l.Entries <= h.Entries && l.PartBytes > 0 && l.PartBytes <= h.PartBytes && l.TotalBytes > 0 && l.TotalBytes <= h.TotalBytes && l.NameBytes > 0 && l.NameBytes <= h.NameBytes
}
func u16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func u32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

// directory bounds archive/zip's header allocation before invoking it. Version 1
// deliberately admits only single-disk ZIP32 with no prefix/trailer. It counts
// actual central records rather than trusting the advertised entry count.
func directory(b []byte, l Limits) (int, []int, error) {
	end := -1
	for i := len(b) - 22; i >= 0 && i >= len(b)-22-65535; i-- {
		if u32(b[i:]) == 0x06054b50 && i+22+int(u16(b[i+20:])) == len(b) {
			end = i
			break
		}
	}
	if end < 0 {
		return 0, nil, ErrFormat
	}
	e := b[end:]
	if u16(e[4:]) != 0 || u16(e[6:]) != 0 || u16(e[8:]) != u16(e[10:]) || u16(e[10:]) == 65535 || u32(e[12:]) == 0xffffffff || u32(e[16:]) == 0xffffffff {
		return 0, nil, ErrUnsupported
	}
	count := int(u16(e[10:]))
	size, start := uint64(u32(e[12:])), uint64(u32(e[16:]))
	if count > l.Entries {
		return 0, nil, ErrLimit
	}
	if start+size != uint64(end) {
		return 0, nil, ErrFormat
	}
	pos := int(start)
	offsets := make([]int, 0, count)
	names := 0
	for pos < end {
		if len(offsets) >= l.Entries {
			return 0, nil, ErrLimit
		}
		if end-pos < 46 || u32(b[pos:]) != 0x02014b50 {
			return 0, nil, ErrFormat
		}
		h := b[pos:]
		n, x, c := int(u16(h[28:])), int(u16(h[30:])), int(u16(h[32:]))
		length := 46 + n + x + c
		if length > end-pos {
			return 0, nil, ErrFormat
		}
		names += n
		if names > l.NameBytes || n > 4096 {
			return 0, nil, ErrLimit
		}
		if u16(h[34:]) != 0 || u32(h[20:]) == 0xffffffff || u32(h[24:]) == 0xffffffff || u32(h[42:]) == 0xffffffff {
			return 0, nil, ErrUnsupported
		}
		offsets = append(offsets, int(u32(h[42:])))
		pos += length
	}
	if len(offsets) != count {
		return 0, nil, ErrFormat
	}
	return int(start), offsets, nil
}

func validName(name string) bool {
	if !utf8.ValidString(name) || name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return false
		}
	}
	for _, p := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}

// inspectContainer validates every physical header and span before any part is read.
func inspectContainer(ctx context.Context, source []byte, expectedSHA256 string, l Limits) (*zip.Reader, error) {
	if ctx == nil || !validLimits(l) {
		return nil, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source) > l.SourceBytes {
		return nil, ErrLimit
	}
	if evidence.Hash(source) != expectedSHA256 {
		return nil, ErrIdentity
	}
	central, offsets, err := directory(source, l)
	if err != nil {
		return nil, err
	}
	z, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	// Name validation is independent of the process-wide zipinsecurepath setting.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, ErrFormat
	}
	if z == nil || len(z.File) != len(offsets) {
		return nil, ErrFormat
	}
	// Keep supported methods independent of application-global registrations.
	z.RegisterDecompressor(zip.Store, func(r io.Reader) io.ReadCloser { return io.NopCloser(r) })
	z.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser { return flate.NewReader(r) })
	names := map[string]bool{}
	spans := make([]Span, 0, len(z.File))
	for i, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !validName(f.Name) {
			return nil, ErrName
		}
		name := strings.TrimSuffix(f.Name, "/")
		if _, ok := names[name]; ok {
			return nil, ErrName
		}
		names[name] = strings.HasSuffix(f.Name, "/")
		if f.Flags & ^uint16(0x080e) != 0 || f.Method != zip.Store && f.Method != zip.Deflate {
			return nil, ErrUnsupported
		}
		if f.Mode()&(fs.ModeType & ^fs.ModeDir) != 0 {
			return nil, ErrUnsupported
		}
		if f.Mode().IsDir() != strings.HasSuffix(f.Name, "/") {
			return nil, ErrFormat
		}
		if strings.HasSuffix(f.Name, "/") && (f.UncompressedSize64 != 0 || f.CompressedSize64 != 0 || f.Method != zip.Store || f.CRC32 != 0) {
			return nil, ErrFormat
		}
		off, e := f.DataOffset()
		if e != nil || off < 0 || off > int64(central) || f.CompressedSize64 > uint64(int64(central)-off) {
			return nil, ErrFormat
		}
		// Require the local and central identities to agree. archive/zip normally
		// trusts the central name; forensic callers must not conceal a second name.
		local := offsets[i]
		if local < 0 || local > central-30 || u32(source[local:]) != 0x04034b50 {
			return nil, ErrFormat
		}
		h := source[local:]
		n, x := int(u16(h[26:])), int(u16(h[28:]))
		if int64(local+30+n+x) != off || u16(h[6:]) != f.Flags || u16(h[8:]) != f.Method || string(h[30:30+n]) != f.Name {
			return nil, ErrFormat
		}
		end := off + int64(f.CompressedSize64)
		if f.Flags&8 == 0 {
			if u32(h[14:]) != f.CRC32 || uint64(u32(h[18:])) != f.CompressedSize64 || uint64(u32(h[22:])) != f.UncompressedSize64 {
				return nil, ErrFormat
			}
		} else {
			if crc := u32(h[14:]); crc != 0 && crc != f.CRC32 {
				return nil, ErrFormat
			}
			if size := u32(h[18:]); size != 0 && uint64(size) != f.CompressedSize64 {
				return nil, ErrFormat
			}
			if size := u32(h[22:]); size != 0 && uint64(size) != f.UncompressedSize64 {
				return nil, ErrFormat
			}
			// ZIP32 descriptor with optional signature. Retain it in the occupancy
			// interval even though only compressed payload bytes receive the span.
			p := int(end)
			if central-p >= 4 && u32(source[p:]) == 0x08074b50 {
				p += 4
			}
			if central-p < 12 || u32(source[p:]) != f.CRC32 || uint64(u32(source[p+4:])) != f.CompressedSize64 || uint64(u32(source[p+8:])) != f.UncompressedSize64 {
				return nil, ErrFormat
			}
			end = int64(p + 12)
		}
		spans = append(spans, Span{int64(local), end})
	}
	for name := range names {
		parent := name
		for {
			i := strings.LastIndexByte(parent, '/')
			if i < 0 {
				break
			}
			parent = parent[:i]
			if dir, ok := names[parent]; ok && !dir {
				return nil, ErrName
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	previous := int64(0)
	for _, s := range spans {
		if s.Start != previous {
			return nil, ErrFormat
		}
		previous = s.End
	}
	if previous != int64(central) {
		return nil, ErrFormat
	}
	return z, nil
}

// Read requires an already acquired snapshot and its expected digest. The caller
// must not mutate the snapshot during the call. Returned byte slices are owned by
// the result. Errors return no partial package; cancellation is cooperative.
func Read(ctx context.Context, source []byte, expectedSHA256 string, l Limits) (*Package, error) {
	z, err := inspectContainer(ctx, source, expectedSHA256, l)
	if err != nil {
		return nil, err
	}

	// Strict reads reserve every declared payload before opening any member.
	// Staged readers apply their independent per-admission policy in Admit.
	total := uint64(0)
	for _, f := range z.File {
		if f.UncompressedSize64 > uint64(l.PartBytes) || f.UncompressedSize64 > uint64(l.TotalBytes)-total {
			return nil, ErrLimit
		}
		total += f.UncompressedSize64
	}
	result := &Package{SourceSHA256: expectedSHA256, Parser: Version, Parts: make([]Part, 0, len(z.File))}
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r, e := f.Open()
		if e != nil {
			return nil, ErrFormat
		}
		data, e := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, r: r}, int64(f.UncompressedSize64)+1))
		closeErr := r.Close()
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, ErrFormat
		}
		if closeErr != nil || uint64(len(data)) != f.UncompressedSize64 {
			return nil, ErrFormat
		}
		off, e := f.DataOffset()
		if e != nil {
			return nil, ErrFormat
		}
		end := off + int64(f.CompressedSize64)
		result.Parts = append(result.Parts, Part{Name: f.Name, Directory: strings.HasSuffix(f.Name, "/"), Method: f.Method, CRC32: f.CRC32, CompressedSpan: Span{off, end}, CompressedSHA256: evidence.Hash(source[off:end]), SHA256: evidence.Hash(data), Bytes: data})
	}
	sort.Slice(result.Parts, func(i, j int) bool { return result.Parts[i].Name < result.Parts[j].Name })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > 32<<10 {
		p = p[:32<<10]
	}
	return r.r.Read(p)
}
