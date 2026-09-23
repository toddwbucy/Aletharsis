package docxidentify

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"testing"
)

func TestAssignmentsPreserveUnavailablePayloadReasons(t *testing.T) {
	for _, name := range []string{"word/styles.xml", "word/document.xml"} {
		for _, withheld := range []bool{false, true} {
			parts := base("word/document.xml")
			parts["word/styles.xml"] = "<styles/>"
			// Stored entries make the corruption specifically a CRC error.
			var buffer bytes.Buffer
			w := zip.NewWriter(&buffer)
			for _, n := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/styles.xml"} {
				f, err := w.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Store})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.Write([]byte(parts[n])); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			raw := buffer.Bytes()
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
			opc, err := opcrels.InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			r, err := InspectVerified(context.Background(), opc)
			if err != nil {
				t.Fatal(err)
			}
			code, state := "office.part_crc_failed", "failed"
			if withheld {
				code, state = "office.part_not_admitted", "not_run"
			}
			found, issue := false, false
			for _, a := range r.Assignments {
				if a.Part == name {
					found = true
					if a.State != "assigned" || a.AdmissionCode != code || a.AdmissionState != state || a.PartSHA256 != "" {
						t.Fatal(a)
					}
				}
			}
			for _, i := range r.Issues {
				if i.Part == name && i.Code == code {
					issue = true
				}
			}
			wantIssue, wantState := true, "partial"
			if withheld && name == "word/styles.xml" {
				wantIssue, wantState = false, "completed"
			}
			if !found || issue != wantIssue || r.State != wantState {
				t.Fatal("type assignment concealed unavailable payload", r)
			}
		}
	}
}

func TestRequiredAdmissionReason(t *testing.T) {
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels"} {
		t.Run(name, func(t *testing.T) {
			raw := archive(t, base("word/document.xml"))
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
			opc, err := opcrels.InspectVerified(context.Background(), reader)
			if err != nil {
				t.Fatal(err)
			}
			result, err := InspectVerified(context.Background(), opc)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "partial" || !hasIssue(result, "office.part_not_admitted") {
				t.Fatal(result)
			}
		})
	}
}
