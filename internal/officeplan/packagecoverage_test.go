package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestPackageCoverageUsesSourceCoordinatesAndPreservesRecords(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx", "part-limit"} {
		t.Run(name, func(t *testing.T) {
			limits := DefaultLimits()
			var raw []byte
			var err error
			if name == "part-limit" {
				raw = analysisFixture(t, func(parts map[string]string) { parts["oversize.bin"] = string(make([]byte, 1024)) })
				limits.Package.PartBytes = 512
			} else {
				raw, err = os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
				if err != nil {
					t.Fatal(err)
				}
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			records, err := BuildPackageRecords(context.Background(), raw, p, 2)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(records)
			if err != nil {
				t.Fatal(err)
			}
			// Configuration allocation is independent of this coverage projection.
			config := v2.UnknownConfig()
			e, ds, err := PackageCoverage(records, config, 2, 7)
			if err != nil {
				t.Fatal(err)
			}
			e2, ds2, err := PackageCoverage(records, config, 2, 7)
			if err != nil || !reflect.DeepEqual(e, e2) || !reflect.DeepEqual(ds, ds2) {
				t.Fatal("nondeterministic coverage", err)
			}
			after, err := json.Marshal(records)
			if err != nil || string(before) != string(after) {
				t.Fatal("records mutated", err)
			}
			pkg := records.Evidence.Packages[0]
			if string(e.State) != pkg.State {
				t.Fatal("parent state changed", e.State, pkg.State)
			}
			var excluded uint64
			n := 0
			for _, part := range pkg.Parts {
				if part.State == "completed" {
					continue
				}
				if n >= len(ds) || ds[n].Ref != e.DiagnosticRefs[n] || ds[n].Code != e.Exclusions[n].ReasonCode {
					t.Fatal("gap diagnostic lost")
				}
				x := e.Exclusions[n]
				if part.CompressedSpan.Start == part.CompressedSpan.End {
					if !x.UnknownRemainder || x.Scope != nil {
						t.Fatal("empty gap falsely localized")
					}
				} else {
					want := identity.Region{Start: uint64(part.CompressedSpan.Start), End: uint64(part.CompressedSpan.End)}
					if x.Scope == nil || x.Scope.ArtifactRef != pkg.SourceArtifactRef || x.Scope.Unit != "byte" || !reflect.DeepEqual(x.Scope.Regions, []identity.Region{want}) {
						t.Fatal("gap not in compressed source coordinates")
					}
					excluded += want.End - want.Start
				}
				n++
			}
			if n != len(ds) || n != len(e.Exclusions) {
				t.Fatal("unexpected diagnostics")
			}
			if n == 0 {
				if e.State != v2.Completed || !reflect.DeepEqual(e.AnalyzedScope, []v2.Scope{e.RequestedScope}) {
					t.Fatal("complete package not covered")
				}
			} else {
				var assessed uint64
				for _, s := range e.AnalyzedScope {
					for _, r := range s.Regions {
						assessed += r.End - r.Start
					}
				}
				if e.State != v2.Partial || assessed+excluded != uint64(len(raw)) {
					t.Fatal("partial coverage lost bytes")
				}
			}
			if _, _, err := PackageCoverage(records, config, 3, 7); err == nil {
				t.Fatal("incorrect execution backlink accepted")
			}
		})
	}
}

func TestPackageCoverageEmptyUnavailablePayloadsRemainPartial(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	if _, err := writer.CreateHeader(&zip.FileHeader{Name: "empty.bin", Method: zip.Store}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	raw := source.Bytes()
	for _, state := range []string{"failed", "not_run", "canceled"} {
		t.Run(state, func(t *testing.T) {
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			// Model an unavailable empty stored payload. Header parsing still succeeded.
			o := &p.Admission.Outcomes.Parts[0]
			if o.Part.CompressedSpan.Start != o.Part.CompressedSpan.End {
				t.Fatal("fixture is not empty stored data")
			}
			o.State = state
			o.Code = "execution.canceled"
			o.Part.SHA256 = ""
			o.Part.Bytes = nil
			p.Admission.Outcomes.State = "partial"
			records, err := BuildPackageRecords(context.Background(), raw, p, 2)
			if err != nil {
				t.Fatal(err)
			}
			e, ds, err := PackageCoverage(records, v2.UnknownConfig(), 2, 0)
			if err != nil {
				t.Fatal(err)
			}
			if e.State != v2.Partial || len(e.Exclusions) != 1 || !e.Exclusions[0].UnknownRemainder || ds[0].Scope != nil {
				t.Fatal("empty unverified payload claimed complete or fabricated byte span")
			}
			if len(e.AnalyzedScope) != 1 || !reflect.DeepEqual(e.AnalyzedScope[0].Regions, []identity.Region{{Start: 0, End: uint64(len(raw))}}) {
				t.Fatal("inspected framing lost")
			}
		})
	}
}
