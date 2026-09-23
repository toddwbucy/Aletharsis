package officeplan

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"os"
	"path/filepath"
	"testing"
)

func inventoryFixture(t *testing.T) []byte {
	t.Helper()
	// The archive is captured evidence. Recompressing it would make this test
	// depend on the Go toolchain's DEFLATE output rather than report semantics.
	raw, err := os.ReadFile(filepath.Join("../../tests/contracts_v4/fixtures", "metadata-inventory.docx"))
	if err != nil {
		t.Fatal(err)
	}
	digest, size := evidence.Hash(raw), len(raw)
	p, err := Prepare(context.Background(), raw, digest, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildReport(context.Background(), raw, evidence.File{Path: "metadata-inventory.docx", Filename: "metadata-inventory.docx", Extension: ".docx", SHA256: &digest, Size: &size}, p, a, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Report.Evidence.Office.Metadata) != 5 || len(report.Report.Evidence.Office.Relationships) < 3 || len(report.Report.Evidence.Office.Objects) != 1 {
		t.Fatal("fixture lost inventory")
	}
	return report.JSON
}
func TestFullInventoryFixtureReproducesNativeReport(t *testing.T) {
	report := inventoryFixture(t)
	want, err := os.ReadFile(filepath.Join("../../tests/contracts_v4/fixtures", "metadata-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(report) != string(want) {
		t.Fatal("native report changed for the captured source archive")
	}
}
