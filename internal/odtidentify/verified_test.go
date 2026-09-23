package odtidentify

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"testing"
)

func TestManifestMemberAdmissionBeforeByteComparisons(t *testing.T) {
	for _, mode := range []string{"completed", "withheld", "crc", "absent"} {
		t.Run(mode, func(t *testing.T) {
			parts := base()
			parts["payload.bin"] = "123456789"
			parts["META-INF/manifest.xml"] = manifest(entry("/", MIME, ` m:version="1.3"`) + entry("content.xml", "text/xml", "") + entry("payload.bin", "application/octet-stream", ` m:size="9"`))
			if mode == "absent" {
				delete(parts, "payload.bin")
			}
			raw := archive(t, parts, "mimetype", zip.Store, nil)
			if mode == "crc" {
				z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range z.File {
					if f.Name == "payload.bin" {
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
			names := []string{}
			for _, o := range reader.ReadOnlyView().Parts {
				if mode != "withheld" || o.Name != "payload.bin" {
					names = append(names, o.Name)
				}
			}
			if err := reader.Admit(context.Background(), names); err != nil {
				t.Fatal(err)
			}
			result, err := InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			if result.Package != nil {
				t.Fatal("synthesized legacy package")
			}
			wantState := map[string]string{"completed": "resolved", "withheld": "not_run", "crc": "failed", "absent": "missing"}[mode]
			wantCode := map[string]string{"completed": "", "withheld": "office.part_not_admitted", "crc": "office.part_crc_failed", "absent": "odt.manifest_target_missing"}[mode]
			found := false
			for _, e := range result.Entries {
				if e.Path == "payload.bin" {
					found = true
					if e.State != wantState || e.Code != wantCode {
						t.Fatalf("%+v", e)
					}
					if mode != "completed" && e.StoredPartSHA256 != "" {
						t.Fatal("unverified digest")
					}
				}
			}
			if !found {
				t.Fatal("missing declaration")
			}
			for _, issue := range result.Issues {
				if issue.Code == "odt.declared_size_mismatch" {
					t.Fatal("compared unavailable bytes")
				}
			}
			count := 0
			for _, i := range result.Issues {
				if i.Code == wantCode {
					if mode == "crc" && (i.Part != "payload.bin" || i.Element != -1) {
						t.Fatal("payload failure lost its part anchor", i)
					}
					count++
				}
			}
			if mode == "crc" && count != 1 {
				t.Fatal("duplicated admission diagnostic", result.Issues)
			}
			if mode == "withheld" && (result.State != "completed" || count != 0) {
				t.Fatal("deferred optional member degraded identification", result)
			}
			if mode != "completed" && mode != "withheld" && result.State != "partial" {
				t.Fatal("concealed admission gap")
			}
		})
	}
}

func TestExemptAndUnlistedMembershipKeepAdmissionFailure(t *testing.T) {
	for _, name := range []string{"META-INF/documentsignatures.xml", "Thumbnails/thumbnail.png"} {
		for _, withheld := range []bool{false, true} {
			parts := base()
			parts[name] = "payload"
			raw := archive(t, parts, "mimetype", zip.Store, nil)
			if !withheld {
				z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range z.File {
					if f.Name == name {
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
			names := []string{}
			for _, p := range reader.ReadOnlyView().Parts {
				if !withheld || p.Name != name {
					names = append(names, p.Name)
				}
			}
			if err := reader.Admit(context.Background(), names); err != nil {
				t.Fatal(err)
			}
			r, err := InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			code, state := "office.part_crc_failed", "failed"
			if withheld {
				code, state = "office.part_not_admitted", "not_run"
			}
			found, issue := false, false
			for _, m := range r.Memberships {
				if m.Part == name {
					found = true
					if m.AdmissionCode != code || m.AdmissionState != state || m.PartSHA256 != "" {
						t.Fatal(m)
					}
				}
			}
			for _, i := range r.Issues {
				if i.Part == name && i.Code == code {
					issue = true
				}
			}
			wantState := "partial"
			if withheld && name == "META-INF/documentsignatures.xml" {
				wantState = "completed"
			}
			if !found || issue == withheld || r.State != wantState || r.Format != "odt" {
				t.Fatal("membership concealed unavailable payload", r)
			}
		}
	}
}

func TestRequiredAdmissionReason(t *testing.T) {
	for _, name := range []string{"mimetype", "META-INF/manifest.xml", "content.xml"} {
		t.Run(name, func(t *testing.T) {
			raw := archive(t, base(), "mimetype", zip.Store, nil)
			reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, p := range reader.ReadOnlyView().Parts {
				if p.Name != name {
					names = append(names, p.Name)
				}
			}
			if err := reader.Admit(context.Background(), names); err != nil {
				t.Fatal(err)
			}
			result, err := InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "partial" || !has(result, "office.part_not_admitted") {
				t.Fatal(result)
			}
		})
	}
}
