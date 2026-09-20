package audit

import (
	"bytes"
	"context"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func FuzzNativeV2(f *testing.F) {
	for _, source := range [][]byte{[]byte("ordinary"), []byte("a\u200b\u200cb\u202e🧪\n"), {0xff, 0xfe, 0x00, 0xd8}, []byte("Author: Example\n12345678-1234-1234-1234-123456789abc\n")} {
		f.Add(source)
	}
	catalog, err := capability.Native(Version, MaxBytes)
	if err != nil {
		f.Fatal(err)
	}
	for i := range catalog {
		if catalog[i].ID == capability.AcquireID {
			catalog[i].Availability = v2.Availability{State: "available"}
		}
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		if len(source) > 4096 {
			t.Skip()
		}
		before := bytes.Clone(source)
		out, err := runV2WithReader(context.Background(), "fixture.txt", DefaultV2Options(), func(string, int) ([]byte, error) { return bytes.Clone(source), nil }, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if out.Report.File.SHA256 == nil || *out.Report.File.SHA256 != identity.ExactBytes(source) {
			t.Fatal("source identity lost")
		}
		if !bytes.Equal(source, before) {
			t.Fatal("source mutated")
		}
		if _, err := v2.DecodeReport(out.JSON, DefaultV2Options().ReportLimits); err != nil {
			t.Fatal(err)
		}
	})
}
