package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

type budgetCase struct {
	parts          map[string]string
	limits         Limits
	format         FormatIdentity
	outcomes       map[string][2]string
	findings       []string
	metadata       int
	partialObjects bool
	badCRC         string
}

func TestRealOfficeBudgetPrioritiesAndZIPOrder(t *testing.T) {
	cases := map[string]func(*testing.T) budgetCase{
		"ordinary_and_custom_relationships_follow_targets": func(t *testing.T) budgetCase {
			parts := budgetBase(t, "office-minimal.docx")
			budgetMetadata(parts)
			budgetStory(parts, "footnotes.xml", "footnotes", "footnotes", `<w:footnote><w:p><w:r><w:t>note`+"\u200b"+`here</w:t></w:r></w:p></w:footnote>`)
			budgetEmbedding(parts, "a.bin", strings.Repeat("x", 512))
			limits := DefaultLimits()
			limits.Package.TotalBytes = budgetSize(parts) + 128
			for _, name := range []string{"Pictures/large.bin", "customXml/large.xml", "customXml/_rels/large.xml.rels"} {
				parts[name] = strings.Repeat("x", 8192)
			}
			return budgetCase{parts: parts, limits: limits, format: IdentityDOCX, metadata: 2, findings: []string{"word/footnotes.xml"}, outcomes: map[string][2]string{
				"docProps/core.xml": {"completed", ""}, "docProps/app.xml": {"completed", ""}, "word/_rels/document.xml.rels": {"completed", ""}, "word/embeddings/a.bin": {"completed", ""},
				"Pictures/large.bin": {"not_run", "office.aggregate_limit"}, "customXml/large.xml": {"not_run", "office.aggregate_limit"}, "customXml/_rels/large.xml.rels": {"not_run", "office.aggregate_limit"}}}
		},
		"default_aggregate_large_embeddings_preserve_footnote": func(t *testing.T) budgetCase {
			parts := budgetBase(t, "office-minimal.docx")
			budgetStory(parts, "footnotes.xml", "footnotes", "footnotes", `<w:footnote><w:p><w:r><w:t>note`+"\u200b"+`here</w:t></w:r></w:p></w:footnote>`)
			payload := strings.Repeat("x", 24<<20)
			for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
				budgetEmbedding(parts, name, payload)
			}
			// The mirror case: after embeddings, this unrelated relationship part
			// cannot fit the remaining aggregate reservation either.
			parts["customXml/_rels/large.xml.rels"] = strings.Repeat("x", 20<<20)
			return budgetCase{parts: parts, limits: DefaultLimits(), format: IdentityDOCX, findings: []string{"word/footnotes.xml"}, partialObjects: true, outcomes: map[string][2]string{
				"word/embeddings/a.bin": {"completed", ""}, "word/embeddings/b.bin": {"completed", ""}, "word/embeddings/c.bin": {"not_run", "office.aggregate_limit"},
				"customXml/_rels/large.xml.rels": {"not_run", "office.aggregate_limit"}}}
		},
		"many_text_parts_precede_small_embedding": func(t *testing.T) budgetCase {
			parts := budgetBase(t, "office-minimal.docx")
			findings := []string{}
			for i := 0; i < 4; i++ {
				name := fmt.Sprintf("header%d.xml", i)
				budgetStory(parts, name, "header", "hdr", `<w:p><w:r><w:t>`+strings.Repeat("x", 4096)+"\u200b"+`</w:t></w:r></w:p>`)
				findings = append(findings, "word/"+name)
			}
			budgetEmbedding(parts, "small.bin", strings.Repeat("x", 64))
			limits := DefaultLimits()
			limits.Package.TotalBytes = budgetSize(parts) - len(parts["word/embeddings/small.bin"])
			return budgetCase{parts: parts, limits: limits, format: IdentityDOCX, findings: findings, partialObjects: true, outcomes: map[string][2]string{"word/embeddings/small.bin": {"not_run", "office.aggregate_limit"}}}
		},
		"CRC_reservation_is_not_refunded": func(t *testing.T) budgetCase {
			parts := budgetBase(t, "office-minimal.docx")
			budgetEmbedding(parts, "a.bin", strings.Repeat("x", 1024))
			budgetEmbedding(parts, "b.bin", strings.Repeat("x", 1024))
			limits := DefaultLimits()
			limits.Package.TotalBytes = budgetSize(parts) - 1024
			return budgetCase{parts: parts, limits: limits, format: IdentityDOCX, findings: []string{"word/document.xml"}, partialObjects: true, badCRC: "word/embeddings/a.bin", outcomes: map[string][2]string{
				"word/embeddings/a.bin": {"failed", "office.part_crc_failed"}, "word/embeddings/b.bin": {"not_run", "office.aggregate_limit"}}}
		},
	}
	for _, kind := range []string{"part", "aggregate"} {
		cases["ODT_decoy_"+kind+"_limit"] = func(t *testing.T) budgetCase {
			parts := budgetBase(t, "office-odt-minimal.odt")
			parts["[Content_Types].xml"] = strings.Repeat("x", 8192)
			limits := DefaultLimits()
			code := "office.aggregate_limit"
			if kind == "part" {
				limits.Package.PartBytes = 4096
				code = "office.part_limit"
			} else {
				limits.Package.TotalBytes = 4096
			}
			return budgetCase{parts: parts, limits: limits, format: IdentityODT, findings: []string{"content.xml"}, outcomes: map[string][2]string{"[Content_Types].xml": {"not_run", code}, "content.xml": {"completed", ""}, "META-INF/manifest.xml": {"completed", ""}}}
		}
	}
	names := []string{}
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) { runBudgetCase(t, cases[name](t)) })
	}
}

func runBudgetCase(t *testing.T, tc budgetCase) {
	t.Helper()
	var phases []Phase
	var projection map[string]string
	var findingCounts map[string]int
	for _, reverse := range []bool{false, true} {
		raw := budgetArchive(t, tc.parts, reverse)
		if tc.badCRC != "" {
			raw = wrongCRCFixture(t, raw, tc.badCRC)
		}
		if len(raw) >= tc.limits.Package.SourceBytes {
			t.Fatal("fixture never reaches decompression")
		}
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), tc.limits)
		if err != nil {
			t.Fatal(err)
		}
		if p.Identity != tc.format || p.Admission.Outcomes.State != "partial" {
			t.Fatal("budget gap became fatal or disappeared", p.Identity, p.Admission.Outcomes.State)
		}
		actual := map[string]string{}
		var reserved uint64
		remaining := map[string][2]string{}
		for n, want := range tc.outcomes {
			remaining[n] = want
		}
		for _, o := range p.Admission.Outcomes.Parts {
			actual[o.Part.Name] = fmt.Sprintf("%s/%s/%d/%s", o.State, o.Code, o.ReservedBytes, o.Part.SHA256)
			reserved += o.ReservedBytes
			if want, ok := remaining[o.Part.Name]; ok {
				if o.State != want[0] || o.Code != want[1] {
					t.Fatal("wrong local outcome", o.Part.Name, o.State, o.Code, want)
				}
				delete(remaining, o.Part.Name)
			}
			if o.Code == "office.aggregate_limit" || o.Code == "office.part_limit" {
				if o.ReservedBytes != 0 || o.Part.SHA256 != "" || len(o.Part.Bytes) != 0 {
					t.Fatal("excluded payload charged or exposed")
				}
			}
			if o.Code == "office.part_crc_failed" && o.ReservedBytes != o.DeclaredBytes {
				t.Fatal("CRC reservation refunded")
			}
		}
		if len(remaining) != 0 || reserved != p.Admission.Outcomes.ReservedBytes || reserved > uint64(tc.limits.Package.TotalBytes) {
			t.Fatal("incomplete inventory or bad accounting", remaining)
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		assertTargetSets(t, p, a)
		counts := map[string]int{}
		metadata := 0
		for _, record := range a.Parts {
			var findings []evidence.Finding
			if record.Word != nil {
				for _, s := range record.Word.Scopes {
					findings = append(findings, s.Findings...)
				}
			}
			if record.ODT != nil {
				for _, s := range record.ODT.Scopes {
					findings = append(findings, s.Findings...)
				}
			}
			for _, f := range findings {
				if f.Evidence["code_point"] == "U+200B" {
					counts[record.Part]++
				}
			}
			if record.Metadata != nil {
				metadata += len(record.Metadata.Properties)
			}
		}
		for _, part := range tc.findings {
			if counts[part] != 1 {
				t.Fatal("expected zero-width finding did not survive", part, counts)
			}
		}
		if metadata != tc.metadata {
			t.Fatal("metadata target displaced", metadata, tc.metadata)
		}
		if tc.partialObjects && (a.Objects == nil || a.Objects.State != "partial") {
			t.Fatal("embedding gap did not reach inventory")
		}
		if _, err := BuildPackageRecords(context.Background(), raw, p, 1); err != nil {
			t.Fatal("partial package cannot be retained", err)
		}
		if reverse {
			if !reflect.DeepEqual(phases, p.Admission.Phases) || !reflect.DeepEqual(projection, actual) || !reflect.DeepEqual(findingCounts, counts) {
				t.Fatal("ZIP storage order changed selection, accounting, or findings")
			}
		} else {
			phases = p.Admission.Phases
			projection = actual
			findingCounts = counts
		}
	}
}

func budgetBase(t *testing.T, name string) map[string]string {
	t.Helper()
	raw := coverageFixture(t, name, nil)
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		parts[f.Name] = string(data)
	}
	return parts
}
func budgetSize(parts map[string]string) int {
	n := 0
	for _, body := range parts {
		n += len(body)
	}
	return n
}
func budgetRelationship(parts map[string]string, part, xml string) {
	if parts[part] == "" {
		parts[part] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
	}
	parts[part] = strings.Replace(parts[part], "</Relationships>", xml+"</Relationships>", 1)
}
func budgetEmbedding(parts map[string]string, name, body string) {
	parts["word/embeddings/"+name] = body
	budgetRelationship(parts, "word/_rels/document.xml.rels", `<Relationship Id="`+name+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/`+name+`"/>`)
}
func budgetStory(parts map[string]string, name, kind, root, body string) {
	parts["word/"+name] = `<w:` + root + ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + body + `</w:` + root + `>`
	parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/word/`+name+`" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.`+kind+`+xml"/></Types>`, 1)
	budgetRelationship(parts, "word/_rels/document.xml.rels", `<Relationship Id="`+name+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/`+kind+`" Target="`+name+`"/>`)
}
func budgetMetadata(parts map[string]string) {
	for _, kind := range []string{"core", "app"} {
		contentType, relType, body := "application/vnd.openxmlformats-package.core-properties+xml", "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties", `<cp:coreProperties xmlns:cp="`+officemetadata.CoreNamespace+`" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Example</dc:creator></cp:coreProperties>`
		if kind == "app" {
			contentType = "application/vnd.openxmlformats-officedocument.extended-properties+xml"
			relType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"
			body = `<Properties xmlns="` + officemetadata.AppNamespace + `"><Application>Example</Application></Properties>`
		}
		name := "docProps/" + kind + ".xml"
		parts[name] = body
		parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/`+name+`" ContentType="`+contentType+`"/></Types>`, 1)
		budgetRelationship(parts, "_rels/.rels", `<Relationship Id="`+kind+`" Type="`+relType+`" Target="`+name+`"/>`)
	}
}
func budgetArchive(t *testing.T, parts map[string]string, reverse bool) []byte {
	t.Helper()
	names := []string{}
	for name := range parts {
		if name != "mimetype" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if reverse {
		slices.Reverse(names)
	}
	if _, ok := parts["mimetype"]; ok {
		names = append([]string{"mimetype"}, names...)
	}
	var b bytes.Buffer
	writer := zip.NewWriter(&b)
	for _, name := range names {
		var w io.Writer
		var err error
		if name == "mimetype" {
			data := []byte(parts[name])
			header := zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc32.ChecksumIEEE(data), CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data))}
			w, err = writer.CreateRaw(&header)
		} else {
			w, err = writer.Create(name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, parts[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
