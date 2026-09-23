package opcrels

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func TestVerifiedInspectionPreservesUnrelatedCRCFailure(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-partial-crc.docx")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, o := range reader.View().Parts {
		names = append(names, o.Part.Name)
	}
	if err := reader.Admit(context.Background(), names); err != nil {
		t.Fatal(err)
	}
	before := reader.View()
	clear(raw)
	got, err := InspectVerified(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "completed" || len(got.Relationships) != 1 || got.Relationships[0].State != "resolved" || got.Relationships[0].ResolvedPart != "word/document.xml" {
		t.Fatal("unrelated CRC failure discarded relationship evidence", got)
	}
	if got.Outcomes == nil || got.Outcomes.State != "partial" || !reflect.DeepEqual(before, reader.View()) {
		t.Fatal("inspection changed package admission")
	}
	failed := 0
	for _, o := range got.Outcomes.Parts {
		if o.Code == "office.part_crc_failed" {
			failed++
		}
	}
	if failed != 1 {
		t.Fatal("lost failed part identity")
	}
	if got.Package != nil {
		t.Fatal("staged inspection synthesized legacy Package")
	}

}

func TestVerifiedInspectionDoesNotInventUnavailableTargetDigest(t *testing.T) {
	raw := archive(t, map[string]string{"_rels/.rels": rels(relationship("r1", "target.xml", "")), "target.xml": "<a/>", "word/_rels/document.xml.rels": rels(relationship("r2", "elsewhere", ""))})
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	before := reader.View()
	got, err := InspectVerified(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "partial" || len(got.Relationships) != 1 {
		t.Fatal("missing incomplete coverage", got)
	}
	rel := got.Relationships[0]
	if rel.ResolvedPart != "target.xml" || rel.TargetSHA256 != "" || rel.State != "resolved" || rel.Code != "" || rel.TargetState != "not_run" || rel.TargetCode != "office.part_not_admitted" {
		t.Fatal("claimed unavailable target content", rel)
	}
	if len(got.Parts) != 2 || got.Parts[1].State != "not_run" || got.Parts[1].Code != "office.part_not_admitted" || got.Parts[1].XML != nil {
		t.Fatal("unadmitted relationship parsed")
	}
	if !reflect.DeepEqual(before, reader.View()) {
		t.Fatal("inspector admitted/decompressed additional parts")
	}
}

func TestVerifiedAndLegacyCompleteInspectionAgree(t *testing.T) {
	raw := archive(t, map[string]string{"_rels/.rels": rels(relationship("r1", "target.xml", "")), "target.xml": "<a/>"})
	legacy, err := Inspect(context.Background(), raw, evidence.Hash(raw))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"_rels/.rels", "target.xml"}); err != nil {
		t.Fatal(err)
	}
	got, err := InspectVerified(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	got.Outcomes = nil
	legacy.Outcomes = nil
	legacy.Package = nil
	if !reflect.DeepEqual(legacy, got) {
		t.Fatal("verified entry point changed completed inspection")
	}
}

func TestTargetNameResolutionSeparatesAdmissionState(t *testing.T) {
	for _, mode := range []string{"completed", "withheld", "crc", "absent"} {
		t.Run(mode, func(t *testing.T) {
			parts := map[string]string{"_rels/.rels": rels(relationship("r1", "target.xml", "")), "target.xml": "<a/>"}
			if mode == "absent" {
				delete(parts, "target.xml")
			}
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for _, name := range []string{"_rels/.rels", "target.xml"} {
				value, exists := parts[name]
				if !exists {
					continue
				}
				file, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write([]byte(value)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			raw := buffer.Bytes()
			if mode == "crc" {
				z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range z.File {
					if f.Name == "target.xml" {
						off, err := f.DataOffset()
						if err != nil {
							t.Fatal(err)
						}
						raw[off] ^= 1
					}
				}
			}
			reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			names := []string{"_rels/.rels"}
			if mode == "completed" || mode == "crc" {
				names = append(names, "target.xml")
			}
			if err := reader.Admit(context.Background(), names); err != nil {
				t.Fatal(err)
			}
			result, err := InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Relationships) != 1 {
				t.Fatal("lost declaration")
			}
			r := result.Relationships[0]
			state := map[string]string{"completed": "completed", "withheld": "not_run", "crc": "failed", "absent": "absent"}[mode]
			code := map[string]string{"completed": "", "withheld": "office.part_not_admitted", "crc": "office.part_crc_failed", "absent": "opc.target_missing"}[mode]
			if r.TargetState != state || r.TargetCode != code {
				t.Fatalf("%+v", r)
			}
			if mode == "absent" {
				if r.State != "missing" || r.ResolvedPart != "" {
					t.Fatalf("%+v", r)
				}
			} else if r.State != "resolved" || r.ResolvedPart != "target.xml" {
				t.Fatalf("%+v", r)
			}
			if (r.TargetSHA256 != "") != (mode == "completed") {
				t.Fatal("digest does not reflect verified bytes")
			}
		})
	}
}
