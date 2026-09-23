package officeplan

import (
	"context"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

// Assembly contains linked evidence plus the inputs still needed to build
// execution coverage and findings. It is deliberately not a publishable report:
// the host must include every gap and validate the completed execution graph.
type Assembly struct {
	Evidence      v4.Evidence
	Artifacts     []v4.Artifact
	Findings      []ScopedFindings
	XMLFailures   []XMLFailure
	InventoryGaps []InventoryGap
	Boundaries    map[string][]v4.Boundary
}

// Assemble combines previously acquired and analyzed evidence without rerunning
// the detectors. Execution ordinals come from the host's deterministic plan.
func Assemble(ctx context.Context, source []byte, p *Prepared, a *Analysis, parseExecution, objectExecution int) (*Assembly, error) {
	base, err := BuildPackageRecords(ctx, source, p, parseExecution)
	if err != nil {
		return nil, err
	}
	xml, err := CollectXML(ctx, p, a, base)
	if err != nil {
		return nil, err
	}
	scopes, err := collectScopes(ctx, base, a, xml, false)
	if err != nil {
		return nil, err
	}
	inventories, err := collectInventories(ctx, p, a, base, xml, objectExecution, false)
	if err != nil {
		return nil, err
	}
	result := &Assembly{Evidence: base.Evidence, Artifacts: append(base.Artifacts, scopes.Artifacts...), Findings: scopes.Findings,
		XMLFailures: xml.Failures, InventoryGaps: inventories.Gaps, Boundaries: scopes.Boundaries}
	result.Evidence.XML = xml.Documents
	result.Evidence.Scopes = scopes.Scopes
	result.Evidence.Metadata = inventories.Metadata
	result.Evidence.Relationships = inventories.Relationships
	result.Evidence.Objects = inventories.Objects
	pkg := result.Evidence.Packages[0]
	index, err := v4.IndexEvidence(result.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	if err := index.ValidateArtifactBindings(result.Artifacts); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
