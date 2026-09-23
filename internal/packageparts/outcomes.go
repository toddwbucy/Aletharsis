package packageparts

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

// Outcome preserves compressed identity even when decompressed bytes cannot be
// verified. Failed/not-run parts never expose partially decoded bytes or hashes.
type Outcome struct {
	Part
	State         string
	Code          string
	DeclaredBytes uint64
	ReservedBytes uint64
}

type OutcomeView struct {
	SourceSHA256  string
	Parser        string
	State         string
	Parts         []Outcome // Canonical ordinal name order, independent of admission order.
	ReservedBytes uint64
}

// OutcomeReader admits caller-selected phases serially. It is not safe for
// concurrent use. The coordinator selects dependency-aware priority tiers;
// this reader owns reservation, one-time decompression, and verified bytes.
type OutcomeReader struct {
	source   []byte
	hash     string
	limits   Limits
	entries  map[string]*zip.File
	outcomes map[string]Outcome
	names    []string
	reserved uint64
}

// OpenOutcomes checks the entire container before admitting any payload. Unlike
// Read, local expansion limits are deferred to admission and do not invalidate
// unrelated parts. The acquired source is copied once and never exposed.
func OpenOutcomes(ctx context.Context, source []byte, expectedSHA256 string, limits Limits) (*OutcomeReader, error) {
	if ctx == nil || !validLimits(limits) || len(source) > limits.SourceBytes {
		return nil, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Own the bytes before hashing or parsing. The retained ZIP reader, all
	// validated offsets, and the advertised digest refer to this same snapshot.
	snapshot := bytes.Clone(source)
	z, err := inspectContainer(ctx, snapshot, expectedSHA256, limits, false)
	if err != nil {
		return nil, err
	}
	r := &OutcomeReader{source: snapshot, hash: expectedSHA256, limits: limits, entries: map[string]*zip.File{}, outcomes: map[string]Outcome{}}
	for _, f := range z.File {
		off, err := f.DataOffset()
		if err != nil {
			return nil, ErrFormat
		}
		end := off + int64(f.CompressedSize64)
		part := Part{Name: f.Name, Directory: strings.HasSuffix(f.Name, "/"), Method: f.Method, CRC32: f.CRC32,
			CompressedSpan: Span{off, end}, CompressedSHA256: evidence.Hash(snapshot[off:end])}
		r.entries[f.Name] = f
		r.outcomes[f.Name] = Outcome{Part: part, State: "not_run", Code: "office.part_not_admitted", DeclaredBytes: f.UncompressedSize64}
		// Container validation proves directories are stored, empty and CRC=0.
		// Their empty payload needs no later decompression or priority admission.
		if part.Directory {
			part.SHA256, part.Bytes = evidence.Hash(nil), []byte{}
			r.outcomes[f.Name] = Outcome{Part: part, State: "completed"}
		}
		r.names = append(r.names, f.Name)
	}
	sort.Strings(r.names)
	return r, nil
}

// Admit applies one caller-ordered priority phase. Already considered names are
// not retried or charged twice. Oversized parts consume no reservation; a failed
// decompression retains its reservation. Cancellation before reservation charges
// nothing; once decompression starts its reservation remains charged, even on
// cancellation, so discarded attempted work cannot replenish the budget.
// Unknown names fail before any mutation.
func (r *OutcomeReader) Admit(ctx context.Context, names []string) error {
	if ctx == nil {
		return ErrLimit
	}
	for _, name := range names {
		if _, ok := r.entries[name]; !ok {
			return ErrName
		}
	}
	for _, name := range names {
		o := r.outcomes[name]
		if o.Code != "office.part_not_admitted" {
			continue
		}
		if o.DeclaredBytes > uint64(r.limits.PartBytes) {
			o.State, o.Code = "not_run", "office.part_limit"
			r.outcomes[name] = o
			continue
		}
		if o.DeclaredBytes > uint64(r.limits.TotalBytes)-r.reserved {
			o.State, o.Code = "not_run", "office.aggregate_limit"
			r.outcomes[name] = o
			continue
		}
		if err := ctx.Err(); err != nil {
			o.State, o.Code = "canceled", "execution.canceled"
			r.outcomes[name] = o
			continue
		}
		o.ReservedBytes = o.DeclaredBytes
		r.reserved += o.ReservedBytes
		reader, err := r.entries[name].Open()
		if err != nil {
			o.State, o.Code = "failed", "office.part_decompression_failed"
			r.outcomes[name] = o
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, r: reader}, int64(o.ReservedBytes)+1))
		closeErr := reader.Close()
		switch {
		case ctx.Err() != nil:
			o.State, o.Code = "canceled", "execution.canceled"
		case errors.Is(readErr, zip.ErrChecksum):
			o.State, o.Code = "failed", "office.part_crc_failed"
		case readErr != nil || closeErr != nil || uint64(len(data)) != o.DeclaredBytes:
			o.State, o.Code = "failed", "office.part_decompression_failed"
		default:
			o.State, o.Code = "completed", ""
			o.Part.Bytes, o.Part.SHA256 = data, evidence.Hash(data)
		}
		r.outcomes[name] = o
	}
	return nil
}

// View returns independent values, including owned copies of verified bytes.
// A partial state means at least one enumerated part has not been verified.
func (r *OutcomeReader) View() OutcomeView { return r.view(true) }

// ReadOnlyView borrows immutable admitted payloads for trusted in-process
// inspectors. Callers must not modify bytes. Outcome records are a snapshot.
func (r *OutcomeReader) ReadOnlyView() OutcomeView { return r.view(false) }

// CompletedView exposes a successfully verified strict package through the same
// explicit admission-state seam used by staged readers; it does not copy bytes.
func CompletedView(pkg *Package) OutcomeView {
	v := OutcomeView{SourceSHA256: pkg.SourceSHA256, Parser: pkg.Parser, State: "completed"}
	for _, p := range pkg.Parts {
		v.Parts = append(v.Parts, Outcome{Part: p, State: "completed", DeclaredBytes: uint64(len(p.Bytes))})
	}
	return v
}

func (r *OutcomeReader) view(owned bool) OutcomeView {
	v := OutcomeView{SourceSHA256: r.hash, Parser: Version, State: "completed", Parts: make([]Outcome, 0, len(r.names)), ReservedBytes: r.reserved}
	for _, name := range r.names {
		o := r.outcomes[name]
		if owned {
			o.Part.Bytes = bytes.Clone(o.Part.Bytes)
		}
		if o.State != "completed" {
			v.State = "partial"
		}
		v.Parts = append(v.Parts, o)
	}
	return v
}

// InspectionView checks the common inspector preconditions and returns a
// borrowed read-only snapshot. The caller must not mutate retained payloads.
func (r *OutcomeReader) InspectionView(ctx context.Context) (OutcomeView, error) {
	if ctx == nil || r == nil || r.hash == "" {
		return OutcomeView{}, ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return OutcomeView{}, err
	}
	return r.ReadOnlyView(), nil
}
