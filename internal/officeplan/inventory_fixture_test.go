package officeplan

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"os"
	"path/filepath"
	"slices"
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
	report, err := buildFixtureReport(context.Background(), raw, evidence.File{Path: "metadata-inventory.docx", Filename: "metadata-inventory.docx", Extension: ".docx", SHA256: &digest, Size: &size}, p, a, "fixture")
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
	limits := DefaultLimits().Report
	canonical, err := identity.Canonicalize(report, limits)
	if err != nil || !bytes.Equal(report, canonical) {
		t.Fatal("producer bytes are not canonical", err)
	}
	current, err := v4.DecodeReport(report, limits)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit compatibility projection to the frozen revision-1 capture. Only
	// the reviewed boundary addition and three revised descriptors/configs differ.
	for i := range current.Evidence.Office.Scopes {
		current.Evidence.Office.Scopes[i].Boundaries = []v4.Boundary{}
	}
	changed := func(id string) bool {
		return id == capability.OfficeMetadataID || id == capability.OfficeRelationshipsID || id == capability.OfficeObjectsID
	}
	for i := range current.Trace.Capabilities {
		c := &current.Trace.Capabilities[i]
		if changed(c.ID) {
			if c.Revision != "2" || !slices.Equal(c.SupportedScope.Formats, []string{"docx", "odt"}) {
				t.Fatal("unexpected capability revision", c.ID)
			}
			c.Revision = "1"
			c.SupportedScope.Formats = []string{"docx"}
		}
	}
	for i := range current.Trace.Executions {
		e := &current.Trace.Executions[i]
		if changed(e.CapabilityRef) {
			if e.Config.Settings == nil {
				t.Fatal("missing settings")
			}
			e.Config, err = v2.NewNativeConfig("1", e.Config.Settings.DataRevision, e.Config.Settings.Limits)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	raw, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := identity.Canonicalize(raw, limits)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(projected, want) {
		t.Fatal("native report differs beyond explicit reviewed projection")
	}
}
