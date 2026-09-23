package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"hash/crc32"
	"io"
	"reflect"
	"sort"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestConflictingIdentityPreservesBothContentPaths(t *testing.T) {
	var previous []PartAnalysis
	for _, reverse := range []bool{false, true} {
		raw := hybridFixture(t, reverse)
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if p.Identity != IdentityConflicting || p.DOCX.Format != "docx" || p.ODT.Format != "odt" {
			t.Fatal("hybrid not recognized")
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Parts) != 2 || a.Parts[0].Part != "content.xml" || a.Parts[1].Part != "word/document.xml" {
			t.Fatal("identified content silently omitted", a.Parts)
		}
		for _, part := range a.Parts {
			if part.Operation != capability.OfficeTextID || part.State != "completed" || part.Error != nil || (part.Word == nil) == (part.ODT == nil) {
				t.Fatal("conflict analyzed as a unique format", part)
			}
		}
		if previous != nil && !reflect.DeepEqual(previous, a.Parts) {
			t.Fatal("member order chose a format")
		}
		previous = a.Parts
		base, err := BuildPackageRecords(context.Background(), raw, p, 1)
		if err != nil {
			t.Fatal("conflict discarded package", err)
		}
		pkg := base.Evidence.Packages[0]
		if pkg.Format != "unknown" || len(pkg.Parts) != len(p.Admission.Outcomes.Parts) || pkg.State != "completed" || len(pkg.Issues) != 1 || pkg.Issues[0].Code != IdentityConflictCode {
			t.Fatal("conflict/source evidence lost", pkg)
		}
		for _, part := range pkg.Parts {
			if part.SHA256 == nil || part.State != "completed" {
				t.Fatal("conflict invalidated verified part")
			}
		}
		coverage, err := CollectAnalysisCoverage(context.Background(), base, a, map[string]int{capability.OfficeTextID: 4}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(coverage.Outcomes) != 2 || len(coverage.Diagnostics) != 0 {
			t.Fatal("conflict diagnostics not retained")
		}
		for _, outcome := range coverage.Outcomes {
			if outcome.State != "completed" || len(outcome.Assessed) == 0 || len(outcome.Codes) != 0 {
				t.Fatal("conflict falsely assessed")
			}
		}
		assembled, err := Assemble(context.Background(), raw, p, a, 1, 3)
		if err != nil {
			t.Fatal("conflict blocked retained evidence assembly", err)
		}
		if len(assembled.Evidence.Scopes) != 2 || len(assembled.Findings) != 2 || len(assembled.Evidence.XML) == 0 {
			t.Fatal("wrong conflict evidence projection")
		}
	}
}

func hybridFixture(t *testing.T, reverse bool) []byte {
	t.Helper()
	parts := map[string][]byte{}
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt"} {
		raw := coverageFixture(t, name, nil)
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range z.File {
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, readErr := io.ReadAll(r)
			closeErr := r.Close()
			if readErr != nil || closeErr != nil {
				t.Fatal(readErr, closeErr)
			}
			parts[f.Name] = b
		}
	}
	names := []string{}
	for name := range parts {
		if name != "mimetype" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if reverse {
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
	}
	// ODF requires the stored mimetype member first. Permute every other member,
	// including the manifest/content pair relative to OPC's declarations/main.
	names = append([]string{"mimetype"}, names...)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		data := parts[name]
		header := zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc32.ChecksumIEEE(data), CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data))}
		w, err := writer.CreateRaw(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
