package officeplan

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func TestPackageRecordsPreserveVerifiedAndUnavailableIdentities(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			records, err := BuildPackageRecords(context.Background(), raw, p, 2)
			if err != nil {
				t.Fatal(err)
			}
			again, err := BuildPackageRecords(context.Background(), raw, p, 2)
			if err != nil || !reflect.DeepEqual(records, again) {
				t.Fatal("nondeterministic records", err)
			}
			pkg := records.Evidence.Packages[0]
			if len(pkg.Parts) != len(p.Admission.Outcomes.Parts) || len(records.Artifacts) != len(pkg.Parts)+1 || len(pkg.Outcomes) != len(pkg.Parts) {
				t.Fatal("incomplete inventory")
			}
			for i, part := range pkg.Parts {
				original := p.Admission.Outcomes.Parts[i]
				if part.Name != original.Part.Name || part.State != original.State || part.CompressedSHA256 != original.Part.CompressedSHA256 || records.PartRefs[part.Name] != part.PartRef {
					t.Fatal("part identity lost")
				}
				outcome := pkg.Outcomes[i]
				if outcome.Operation != capability.ParseOfficeID || outcome.ExecutionRef != "exec/2" || *outcome.PartRef != part.PartRef {
					t.Fatal(outcome)
				}
				if part.State == "completed" {
					if part.SHA256 == nil || *part.SHA256 != evidence.Hash(original.Part.Bytes) || *part.ByteLength != int64(len(original.Part.Bytes)) || len(outcome.Assessed) != 1 {
						t.Fatal("verified identity lost")
					}
				} else {
					if part.SHA256 != nil || part.ByteLength != nil || len(outcome.Assessed) != 0 || len(outcome.Codes) != 1 || len(part.Issues) != 1 {
						t.Fatal("unavailable part gained identity")
					}
					if *records.Artifacts[i+1].UnavailableReason != "extraction_unavailable" {
						t.Fatal("wrong absence semantics")
					}
				}
			}
			for _, a := range records.Artifacts {
				encoded, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := v2.DecodeArtifact(encoded); err != nil {
					t.Fatal("invalid artifact", err)
				}
			}
			index, err := v4.IndexEvidence(records.Evidence, evidence.Hash(raw), int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			state := v2.Completed
			if pkg.State == "partial" {
				state = v2.Partial
			}
			trace := &v4.TraceIndex{Executions: map[string]v2.Execution{"exec/2": {CapabilityRef: capability.ParseOfficeID, State: state}}}
			if err := index.ValidateOutcomes(records.Evidence, trace); err != nil {
				t.Fatal("invalid outcomes", err)
			}
		})
	}
}

func TestPackageRecordsRejectMutatedIdentities(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Prepared){
		func(p *Prepared) { p.Admission.Outcomes.SourceSHA256 = evidence.Hash([]byte("wrong")) },
		func(p *Prepared) {
			p.Admission.Outcomes.Parts[0].Part.CompressedSHA256 = evidence.Hash([]byte("wrong"))
		},
		func(p *Prepared) { p.Admission.Outcomes.Parts[0].Part.Bytes = []byte("wrong") },
		func(p *Prepared) { p.Admission.Outcomes.Parts[0].State = "failed" },
		func(p *Prepared) {
			p.Admission.Outcomes.Parts[0], p.Admission.Outcomes.Parts[1] = p.Admission.Outcomes.Parts[1], p.Admission.Outcomes.Parts[0]
		},
	} {
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		change(p)
		if r, err := BuildPackageRecords(context.Background(), raw, p, 2); err == nil || r != nil {
			t.Fatal("mutated identity accepted")
		}
	}
}

func TestPackageRecordsDistinguishEmptyFromBudgetSkipped(t *testing.T) {
	raw := analysisFixture(t, func(parts map[string]string) {
		parts["empty.bin"] = ""
		parts["oversize.bin"] = string(make([]byte, 1024))
	})
	limits := DefaultLimits()
	limits.Package.PartBytes = 512
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildPackageRecords(context.Background(), raw, p, 2)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, part := range r.Evidence.Packages[0].Parts {
		switch part.Name {
		case "empty.bin":
			found++
			if part.State != "completed" || part.ByteLength == nil || *part.ByteLength != 0 || part.SHA256 == nil || *part.SHA256 != evidence.Hash(nil) {
				t.Fatal("empty part treated as unavailable")
			}
		case "oversize.bin":
			found++
			if part.State != "not_run" || part.ByteLength != nil || part.SHA256 != nil || len(part.Issues) != 1 || part.Issues[0].Code != "office.part_limit" {
				t.Fatal("skipped part treated as empty")
			}
		}
	}
	if found != 2 {
		t.Fatal("missing part inventory")
	}
}
