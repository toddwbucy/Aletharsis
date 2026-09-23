package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"hash/crc32"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

func TestAnalysisCoverageRetainsProducerGaps(t *testing.T) {
	cases := []struct {
		name, fixture, code string
		limit               int
		mutate              func(string, string) string
	}{
		{name: "clean Word", fixture: "office-minimal.docx"},
		{name: "clean ODT", fixture: "office-odt-minimal.odt"},
		{name: "Word scope omission", fixture: "office-minimal.docx", limit: 1, code: "execution.resource_limit"},
		{name: "ODT scope omission", fixture: "office-odt-minimal.odt", limit: 1, code: "execution.resource_limit"},
		{name: "overlapping Word gaps", fixture: "office-minimal.docx", limit: 1, code: "execution.resource_limit", mutate: func(n, s string) string {
			if n == "word/document.xml" {
				return strings.Replace(s, "<w:r>", `<w:r><w:rPr><w:vanish w:val="unknown"/></w:rPr>`, 1)
			}
			return s
		}},
		{name: "Word unknown markup", fixture: "office-minimal.docx", code: "word.unclassified_character_data", mutate: func(n, s string) string {
			if n == "word/document.xml" {
				return strings.Replace(s, "</w:body>", `<unknown>unclassified</unknown></w:body>`, 1)
			}
			return s
		}},
		{name: "ODT expansion limit", fixture: "office-odt-minimal.odt", code: "execution.resource_limit", mutate: func(n, s string) string {
			if n == "content.xml" {
				return strings.Replace(s, "</text:p>", `<text:s text:c="200001"/></text:p>`, 1)
			}
			return s
		}},
		{name: "ODT unknown control", fixture: "office-odt-minimal.odt", code: "odt.control_unresolved", mutate: func(n, s string) string {
			if n == "content.xml" {
				return strings.Replace(s, "</text:p>", `<text:s text:c="unknown"/></text:p>`, 1)
			}
			return s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := coverageFixture(t, tc.fixture, tc.mutate)
			limits := DefaultLimits()
			if tc.limit > 0 {
				limits.ScopeScalarOrigins = tc.limit
			}
			checkAnalysisCoverage(t, raw, limits, tc.code)
		})
	}
}

func coverageFixture(t *testing.T, name string, mutate func(string, string) string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if mutate == nil {
		return raw
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	for _, f := range reader.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		updated := []byte(mutate(f.Name, string(data)))
		header := zip.FileHeader{Name: f.Name, Method: zip.Store, CRC32: crc32.ChecksumIEEE(updated), CompressedSize64: uint64(len(updated)), UncompressedSize64: uint64(len(updated))}
		w, err := writer.CreateRaw(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(updated); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func checkAnalysisCoverage(t *testing.T, raw []byte, limits Limits, wantCode string) {
	t.Helper()
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	base, err := BuildPackageRecords(context.Background(), raw, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	allocations := map[string]int{capability.OfficeMetadataID: 3, capability.OfficeTextID: 4}
	before, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CollectAnalysisCoverage(context.Background(), base, a, allocations, 5)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CollectAnalysisCoverage(context.Background(), base, a, allocations, 5)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("nondeterministic coverage", err)
	}
	after, err := json.Marshal(base)
	if err != nil || string(before) != string(after) {
		t.Fatal("base mutated", err)
	}
	if len(got.Outcomes) != len(a.Parts) || len(got.Limitations) == 0 {
		t.Fatal("producer records lost")
	}
	found := false
	for _, o := range got.Outcomes {
		if slices.Contains(o.Codes, wantCode) {
			found = true
		}
		if len(o.Codes) != 0 && len(o.DiagnosticRefs) == 0 {
			t.Fatal("gap lacks diagnostic")
		}
		for _, span := range o.Assessed {
			for _, gap := range o.Excluded {
				if span.Start < gap.End && gap.Start < span.End {
					t.Fatal("assessed and excluded overlap")
				}
			}
		}
	}
	expectedDiagnostics := 0
	for _, p := range a.Parts {
		if p.Error != nil {
			expectedDiagnostics++
			continue
		}
		if p.Word != nil {
			expectedDiagnostics += len(p.Word.Extraction.Issues) + len(p.Word.Omitted)
		}
		if p.ODT != nil {
			expectedDiagnostics += len(p.ODT.Extraction.Issues) + len(p.ODT.Omitted)
			for _, b := range p.ODT.Boundaries {
				if b.Reason == "control_expansion_limit" {
					expectedDiagnostics++
				}
			}
		}
		if p.Metadata != nil {
			expectedDiagnostics += len(p.Metadata.Issues)
		}
	}
	if len(got.Diagnostics) != expectedDiagnostics {
		t.Fatal("producer gaps lost or fabricated")
	}
	if wantCode != "" && !found {
		t.Fatalf("missing %s: %+v", wantCode, got)
	}
	for _, d := range got.Diagnostics {
		if d.Scope == nil || d.Scope.ArtifactRef == "artifact/0" {
			t.Fatal("part gap mapped to source bytes")
		}
	}
	if _, err := CollectAnalysisCoverage(context.Background(), base, a, map[string]int{}, 0); err == nil {
		t.Fatal("missing allocation accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CollectAnalysisCoverage(ctx, base, a, allocations, 0); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestAnalysisCoverageMetadataAndFailureIsolation(t *testing.T) {
	for _, body := range []string{
		`<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Example</dc:creator><cp:unknown><cp:nested>Unassessed</cp:nested></cp:unknown></cp:coreProperties>`,
		`<broken>`,
	} {
		raw := analysisFixture(t, func(parts map[string]string) {
			parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`, 1)
			parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="core.xml"/></Relationships>`, 1)
			parts["core.xml"] = body
		})
		code := "metadata.property_unassessed"
		if body == "<broken>" {
			code = "execution.failed"
		}
		checkAnalysisCoverage(t, raw, DefaultLimits(), code)
	}
}
