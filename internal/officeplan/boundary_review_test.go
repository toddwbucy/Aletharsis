package officeplan

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"testing"
)

func TestBoundaryDiagnosticsRejectInvalidProducerReason(t *testing.T) {
	parts := budgetBase(t, "office-minimal.docx")
	parts["word/document.xml"] = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:sdt><w:p><w:r><w:t>blocked</w:t></w:r></w:p></w:sdt></w:body></w:document>`
	raw := budgetArchive(t, parts, false)
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	assembly, err := Assemble(context.Background(), raw, p, a, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := p.Limits.Catalog("test")
	if err != nil {
		t.Fatal(err)
	}
	configs := map[string]v2.Config{}
	for _, c := range catalog {
		configs[c.ID] = v2.UnknownConfig()
	}
	if _, err := BuildPartGraph(context.Background(), p, a, assembly, catalog, configs, 5, 0); err != nil {
		t.Fatal(err)
	}
	changed := false
	for key, boundaries := range assembly.Boundaries {
		if len(boundaries) > 0 {
			boundaries[0].Reason = "invalid reason!"
			assembly.Boundaries[key] = boundaries
			changed = true
			break
		}
	}
	if !changed || len(assembly.Evidence.Scopes) != 0 {
		t.Fatal("probe lacks standalone boundaries")
	}
	if _, err := BuildPartGraph(context.Background(), p, a, assembly, catalog, configs, 5, 0); err == nil {
		t.Fatal("invalid boundary diagnostic retained")
	}
	// The exported collector may receive the full ordinal table, but accepts only
	// metadata/text analyses. A relationship operation must not allocate a ref.
	base := partLayer(assembly)
	bad := &Analysis{Parts: []PartAnalysis{{Part: "word/document.xml", Operation: capability.OfficeRelationshipsID, State: "failed"}}}
	if _, err := CollectAnalysisCoverage(context.Background(), base, bad, map[string]int{capability.OfficeRelationshipsID: 7}, 0); err == nil {
		t.Fatal("foreign operation consumed a diagnostic ordinal")
	}
}
