package officeplan

import (
	"errors"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

// Limits is a per-source configuration, independent of corpus consumption.
// Zero is invalid, not unlimited. Scope limits apply to a complete scope; callers
// must omit an oversized scope and its findings, never truncate its origin map.
type Limits struct {
	Package            packageparts.Limits
	ScopeTextUTF8Bytes int
	ScopeScalarOrigins int
	Report             identity.Limits
}

func DefaultLimits() Limits {
	return Limits{Package: packageparts.DefaultLimits(), ScopeTextUTF8Bytes: 4 << 20,
		ScopeScalarOrigins: xmlparts.MaxScalarMappings,
		Report:             identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4 << 20, Depth: 64}}
}

func (l Limits) Validate() error {
	hard := packageparts.DefaultLimits()
	for _, p := range [][2]int{{l.Package.SourceBytes, hard.SourceBytes}, {l.Package.Entries, hard.Entries},
		{l.Package.PartBytes, hard.PartBytes}, {l.Package.TotalBytes, hard.TotalBytes},
		{l.Package.NameBytes, hard.NameBytes}, {l.ScopeTextUTF8Bytes, xmlparts.DefaultLimits().Bytes},
		{l.ScopeScalarOrigins, xmlparts.MaxScalarMappings}} {
		if p[0] <= 0 || p[0] > p[1] {
			return errors.New("invalid Office resource configuration")
		}
	}
	for _, n := range []int{l.Report.InputBytes, l.Report.OutputBytes, l.Report.Nodes, l.Report.Depth} {
		if n <= 0 || uint64(n) > identity.MaxSafeInteger {
			return errors.New("invalid Office report resource configuration")
		}
	}
	_, err := identity.Canonicalize([]byte(`{}`), l.Report)
	return err
}

// Catalog discloses the fixed package limits enforced by the outcome reader.
// Other operation limits remain null until their own coordinator enforces them.
func (l Limits) Catalog(version string) ([]v2.Capability, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	catalog, err := capability.Office(version, uint64(l.Package.SourceBytes))
	if err != nil {
		return nil, err
	}
	for i := range catalog {
		if catalog[i].ID == capability.ParseOfficeID {
			expanded, objects := uint64(l.Package.TotalBytes), uint64(l.Package.Entries)
			catalog[i].Limits.ExpandedBytes = &expanded
			catalog[i].Limits.Objects = &objects
		}
	}
	return catalog, nil
}

// Record copies the exact configured limits to the 4.0 package record. These are
// ceilings, not measurements or a reduced allowance after earlier admissions.
func (l Limits) Record(p *v4.Package) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if p == nil {
		return errors.New("nil Office package limit destination")
	}
	p.Limits.SourceBytes = int64(l.Package.SourceBytes)
	p.Limits.PartCount = int64(l.Package.Entries)
	p.Limits.PartBytes = int64(l.Package.PartBytes)
	p.Limits.AggregateBytes = int64(l.Package.TotalBytes)
	p.Limits.MaxScopeTextUTF8Bytes = int64(l.ScopeTextUTF8Bytes)
	p.Limits.MaxScopeScalarOrigins = int64(l.ScopeScalarOrigins)
	p.Limits.ReportInputBytes = int64(l.Report.InputBytes)
	p.Limits.ReportOutputBytes = int64(l.Report.OutputBytes)
	p.Limits.ReportNodes = int64(l.Report.Nodes)
	p.Limits.ReportDepth = int64(l.Report.Depth)
	return nil
}
