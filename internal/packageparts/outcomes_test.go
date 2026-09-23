package packageparts

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func openOutcomes(t *testing.T, raw []byte, limits Limits) *OutcomeReader {
	t.Helper()
	r, err := OpenOutcomes(context.Background(), raw, evidence.Hash(raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func corruptPayload(t *testing.T, raw []byte, name string) []byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Clone(raw)
	for _, f := range z.File {
		if f.Name == name {
			off, err := f.DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			changed[off] ^= 1
			return changed
		}
	}
	t.Fatal("missing corruption target")
	return nil
}
func TestOutcomeReaderIsolatesCRCAndChargesFailure(t *testing.T) {
	raw := archive(t, entry{"bad.bin", "broken", zip.Store}, entry{"good.xml", "<a/>", zip.Store})
	raw = corruptPayload(t, raw, "bad.bin")
	r := openOutcomes(t, raw, DefaultLimits())
	if err := r.Admit(context.Background(), []string{"bad.bin", "good.xml"}); err != nil {
		t.Fatal(err)
	}
	v := r.View()
	if v.State != "partial" || v.ReservedBytes != 10 || v.Parts[0].Code != "office.part_crc_failed" || v.Parts[0].Part.Bytes != nil || v.Parts[0].Part.SHA256 != "" || string(v.Parts[1].Part.Bytes) != "<a/>" {
		t.Fatal("lost partial evidence", v)
	}
	if err := r.Admit(context.Background(), []string{"bad.bin", "good.xml"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v, r.View()) {
		t.Fatal("retried or charged twice")
	}
	if _, err := Read(context.Background(), raw, evidence.Hash(raw), DefaultLimits()); !errors.Is(err, ErrFormat) {
		t.Fatal("strict API changed", err)
	}
}
func TestOutcomeAdmissionBoundedFitIndependentOfStorageOrder(t *testing.T) {
	entries := []entry{{"decoy", "123456789", zip.Store}, {"large", "1234567", zip.Store}, {"critical", "1234", zip.Store}, {"small", "12", zip.Store}}
	for _, reverse := range []bool{false, true} {
		ordered := append([]entry(nil), entries...)
		if reverse {
			for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
		raw := archive(t, ordered...)
		limits := DefaultLimits()
		limits.PartBytes = 8
		limits.TotalBytes = 6
		r := openOutcomes(t, raw, limits)
		if err := r.Admit(context.Background(), []string{"decoy", "large", "critical", "small"}); err != nil {
			t.Fatal(err)
		}
		v := r.View()
		if v.ReservedBytes != 6 {
			t.Fatal("incorrect reservation", v.ReservedBytes)
		}
		want := map[string]string{"decoy": "office.part_limit", "large": "office.aggregate_limit", "critical": "", "small": ""}
		for _, o := range v.Parts {
			if o.Code != want[o.Part.Name] {
				t.Fatal("order-dependent admission", o)
			}
		}
	}
}
func TestFailedReservationCannotBeReused(t *testing.T) {
	raw := corruptPayload(t, archive(t, entry{"bad", "1234", zip.Store}, entry{"small", "12", zip.Store}), "bad")
	limits := DefaultLimits()
	limits.TotalBytes = 4
	r := openOutcomes(t, raw, limits)
	if err := r.Admit(context.Background(), []string{"bad", "small"}); err != nil {
		t.Fatal(err)
	}
	if v := r.View(); v.ReservedBytes != 4 || v.Parts[1].Code != "office.aggregate_limit" {
		t.Fatal("refunded failed decompression", v)
	}
}
func TestOutcomeSnapshotAndViewsAreOwned(t *testing.T) {
	raw := archive(t, entry{"a", "retained", zip.Deflate})
	r := openOutcomes(t, raw, DefaultLimits())
	clear(raw)
	if err := r.Admit(context.Background(), []string{"a"}); err != nil {
		t.Fatal(err)
	}
	v := r.View()
	if string(v.Parts[0].Part.Bytes) != "retained" {
		t.Fatal("source mutation affected snapshot")
	}
	v.Parts[0].Part.Bytes[0] = 'X'
	if string(r.View().Parts[0].Part.Bytes) != "retained" {
		t.Fatal("view aliases reader")
	}
}
func TestOutcomeUnknownNameAndCancellation(t *testing.T) {
	raw := archive(t, entry{"a", "a", zip.Store}, entry{"b", "b", zip.Store})
	r := openOutcomes(t, raw, DefaultLimits())
	before := r.View()
	if err := r.Admit(context.Background(), []string{"a", "absent"}); !errors.Is(err, ErrName) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, r.View()) {
		t.Fatal("unknown name partially mutated phase")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Admit(ctx, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	for _, o := range r.View().Parts {
		if o.State != "canceled" || o.ReservedBytes != 0 || o.Part.Bytes != nil {
			t.Fatal("lost cancellation outcome")
		}
	}
}
func TestOutcomeReaderKeepsGlobalIdentityChecks(t *testing.T) {
	for _, raw := range [][]byte{archive(t, entry{"a", "1", zip.Store}, entry{"a", "2", zip.Store}), archive(t, entry{"../bad", "1", zip.Store})} {
		if _, err := OpenOutcomes(context.Background(), raw, evidence.Hash(raw), DefaultLimits()); !errors.Is(err, ErrName) {
			t.Fatal("admitted ambiguous identities", err)
		}
	}
	raw := archive(t, entry{"a", "1", zip.Store})
	if _, err := OpenOutcomes(context.Background(), raw, "wrong", DefaultLimits()); !errors.Is(err, ErrIdentity) {
		t.Fatal(err)
	}
}

func TestAdmissionLimitsPrecedeCancellation(t *testing.T) {
	raw := archive(t, entry{"big", "12345", zip.Store}, entry{"small", "12", zip.Store})
	for _, aggregate := range []bool{false, true} {
		limits := DefaultLimits()
		want := "office.part_limit"
		if aggregate {
			limits.TotalBytes = 3
			want = "office.aggregate_limit"
		} else {
			limits.PartBytes = 3
		}
		reader := openOutcomes(t, raw, limits)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := reader.Admit(ctx, []string{"big", "small"}); err != nil {
			t.Fatal(err)
		}
		v := reader.View()
		if v.Parts[0].Code != want || v.Parts[0].ReservedBytes != 0 || v.Parts[1].State != "canceled" {
			t.Fatalf("%+v", v)
		}
	}
}

func TestValidatedEmptyDirectoryRequiresNoAdmission(t *testing.T) {
	raw := archive(t, entry{"word/", "", zip.Store}, entry{"word/a", "a", zip.Store})
	reader := openOutcomes(t, raw, DefaultLimits())
	if err := reader.Admit(context.Background(), []string{"word/a"}); err != nil {
		t.Fatal(err)
	}
	v := reader.View()
	if v.State != "completed" || v.Parts[0].State != "completed" || v.Parts[0].SHA256 != evidence.Hash(nil) || v.ReservedBytes != 1 {
		t.Fatalf("%+v", v)
	}
}

type cancelOnRead struct {
	context.Context
	calls int
}

func (c *cancelOnRead) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}
func TestCancellationAfterReservationDoesNotRefundAttemptedWork(t *testing.T) {
	reader := openOutcomes(t, archive(t, entry{"a", "12", zip.Store}), DefaultLimits())
	ctx := &cancelOnRead{Context: context.Background()}
	if err := reader.Admit(ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	view := reader.View()
	if view.ReservedBytes != 2 || view.Parts[0].ReservedBytes != 2 || view.Parts[0].State != "canceled" || view.Parts[0].Bytes != nil || view.Parts[0].SHA256 != "" {
		t.Fatalf("%+v", view)
	}
}

// Mutate at the inspection boundary without introducing a test data race.
type snapshotContext struct {
	context.Context
	calls  int
	mutate func()
}

func (c *snapshotContext) Err() error {
	c.calls++
	if c.calls == 2 {
		c.mutate()
	}
	return nil
}
func TestSnapshotIsTakenBeforeIdentityAndContainerValidation(t *testing.T) {
	raw := archive(t, entry{"payload.bin", "verified", zip.Store})
	digest := evidence.Hash(raw)
	ctx := &snapshotContext{Context: context.Background(), mutate: func() { clear(raw) }}
	r, err := OpenOutcomes(ctx, raw, digest, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Hash(r.source) != digest {
		t.Fatal("retained bytes do not match source identity")
	}
	if err := r.Admit(context.Background(), []string{"payload.bin"}); err != nil {
		t.Fatal(err)
	}
	if p := r.ReadOnlyView().Parts[0]; p.State != "completed" || string(p.Bytes) != "verified" {
		t.Fatal("caller mutation affected verified snapshot", p)
	}
}
