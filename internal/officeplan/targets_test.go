package officeplan

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"hash/crc32"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func assertTargetSets(t *testing.T, p *Prepared, a *Analysis) {
	t.Helper()
	for _, op := range []string{capability.OfficeMetadataID, capability.OfficeTextID} {
		planned, actual := []string{}, []string{}
		for _, phase := range p.Admission.Phases {
			if op == capability.OfficeMetadataID && phase.Name == "metadata" || op == capability.OfficeTextID && (phase.Name == "main_content" || phase.Name == "related_text") {
				planned = append(planned, phase.Parts...)
			}
		}
		present := map[string]bool{}
		for _, o := range p.Admission.Outcomes.Parts {
			present[o.Part.Name] = true
		}
		for _, part := range a.Parts {
			if part.Operation == op && present[part.Part] {
				actual = append(actual, part.Part)
			}
		}
		sort.Strings(planned)
		planned = slices.Compact(planned)
		sort.Strings(actual)
		actual = slices.Compact(actual)
		if !reflect.DeepEqual(planned, actual) {
			t.Fatalf("%s admission=%v analysis=%v", op, planned, actual)
		}
	}
}

func TestUnifiedTargetsCoverBothWordNamespacesAndRelationshipFamilies(t *testing.T) {
	for _, ns := range []string{docxidentify.TransitionalWord, docxidentify.StrictWord} {
		for _, prefix := range []string{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/", "http://purl.oclc.org/ooxml/officeDocument/relationships/"} {
			for _, story := range []struct{ kind, root string }{{"header", "hdr"}, {"footer", "ftr"}, {"comments", "comments"}, {"footnotes", "footnotes"}, {"endnotes", "endnotes"}} {
				raw := analysisFixture(t, func(parts map[string]string) {
					if ns == docxidentify.StrictWord {
						parts["word/document.xml"] = strings.ReplaceAll(parts["word/document.xml"], docxidentify.TransitionalWord, ns)
						parts["_rels/.rels"] = strings.ReplaceAll(parts["_rels/.rels"], docxidentify.TransitionalRelationship, docxidentify.StrictRelationship)
					}
					parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/word/story.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.`+story.kind+`+xml"/></Types>`, 1)
					parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="` + prefix + story.kind + `" Target="story.xml"/></Relationships>`
					body := `<w:p><w:r><w:t>signal` + "\u200b" + `here</w:t></w:r></w:p>`
					switch story.kind {
					case "comments":
						body = `<w:comment>` + body + `</w:comment>`
					case "footnotes":
						body = `<w:footnote>` + body + `</w:footnote>`
					case "endnotes":
						body = `<w:endnote>` + body + `</w:endnote>`
					}
					parts["word/story.xml"] = `<w:` + story.root + ` xmlns:w="` + ns + `">` + body + `</w:` + story.root + `>`
				})
				p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				a, err := AnalyzePrepared(context.Background(), p)
				if err != nil {
					t.Fatal(err)
				}
				assertTargetSets(t, p, a)
				found := false
				for _, record := range a.Parts {
					if record.Part == "word/story.xml" {
						found = true
						if record.State != "completed" || record.Word == nil || len(record.Word.Scopes) != 1 || len(record.Word.Scopes[0].Findings) == 0 {
							t.Fatal("typed story target not analyzed", ns, prefix, story.kind, record)
						}
					}
				}
				if !found {
					t.Fatal("missing declared story")
				}
			}
		}
	}
}

func TestUnavailableAndAbsentTargetsRetainDistinctReasons(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
		for _, name := range []string{"a_crc", "b_large", "c_good", "d_missing"} {
			rels += `<Relationship Id="` + name + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="` + name + `.xml"/>`
			parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/word/`+name+`.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`, 1)
			if name == "d_missing" {
				continue
			}
			body := "signal\u200bhere"
			if name == "b_large" {
				body = strings.Repeat("x", 8192)
			}
			parts["word/"+name+".xml"] = `<w:hdr xmlns:w="` + docxidentify.TransitionalWord + `"><w:p><w:r><w:t>` + body + `</w:t></w:r></w:p></w:hdr>`
		}
		parts["word/_rels/document.xml.rels"] = rels + `</Relationships>`
		parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="absent-core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="absent-core.xml"/></Relationships>`, 1)
	})
	raw = wrongCRCFixture(t, raw, "word/a_crc.xml")
	for _, budgetCode := range []string{"office.part_limit", "office.aggregate_limit"} {
		limits := DefaultLimits()
		if budgetCode == "office.part_limit" {
			limits.Package.PartBytes = 4096
		} else {
			limits.Package.TotalBytes = 4096
		}
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
		if err != nil {
			t.Fatal(err)
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		assertTargetSets(t, p, a)
		expected := map[string]string{"word/a_crc.xml": "office.part_crc_failed", "word/b_large.xml": budgetCode, "word/d_missing.xml": "opc.target_missing", "absent-core.xml": "opc.target_missing"}
		good := false
		for _, r := range a.Parts {
			if code, ok := expected[r.Part]; ok {
				if r.State != "not_run" || r.Code != code || r.Error == nil {
					t.Fatal("reason lost", r)
				}
				delete(expected, r.Part)
			}
			if r.Part == "word/c_good.xml" && r.Word != nil && len(r.Word.Scopes) > 0 && len(r.Word.Scopes[0].Findings) > 0 {
				good = true
			}
		}
		if len(expected) != 0 || !good {
			t.Fatal("lost target or usable sibling", expected)
		}
		base, err := BuildPackageRecords(context.Background(), raw, p, 1)
		if err != nil {
			t.Fatal(err)
		}
		coverage, err := CollectAnalysisCoverage(context.Background(), base, a, map[string]int{capability.OfficeMetadataID: 3, capability.OfficeTextID: 4}, 0)
		if err != nil {
			t.Fatal(err)
		}
		missing := 0
		for _, o := range coverage.Outcomes {
			if slices.Contains(o.Codes, "opc.target_missing") {
				missing++
				if o.PartRef != nil || len(o.Assessed) != 0 || len(o.Excluded) != 0 {
					t.Fatal("invented absent part identity")
				}
			}
		}
		if missing != 2 {
			t.Fatal("absent target gaps lost")
		}
		for _, d := range coverage.Diagnostics {
			if d.Code == "opc.target_missing" && (d.Scope == nil || d.Scope.Unit != "byte" || len(d.Scope.Regions) != 1) {
				t.Fatal("absent target lacks located declaration")
			}
		}
		if _, err := Assemble(context.Background(), raw, p, a, 1, 5); err != nil {
			t.Fatal("missing targets discarded usable evidence", err)
		}
	}
}

// Correct compressed data with the same incorrect CRC in both ZIP headers
// isolates a local checksum failure from global container inconsistencies.
func wrongCRCFixture(t *testing.T, raw []byte, name string) []byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var result bytes.Buffer
	writer := zip.NewWriter(&result)
	for _, file := range z.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		var compressed bytes.Buffer
		encoder, err := flate.NewWriter(&compressed, flate.DefaultCompression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := encoder.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := encoder.Close(); err != nil {
			t.Fatal(err)
		}
		crc := crc32.ChecksumIEEE(data)
		if file.Name == name {
			crc ^= 1
		}
		header := zip.FileHeader{Name: file.Name, Method: zip.Deflate, CRC32: crc, CompressedSize64: uint64(compressed.Len()), UncompressedSize64: uint64(len(data))}
		w, err := writer.CreateRaw(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(compressed.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}
