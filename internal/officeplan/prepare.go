package officeplan

import (
	"context"
	"errors"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// FormatIdentity is the single format decision shared by downstream consumers.
// A conflicting identity does not invalidate the acquired bytes or package.
type FormatIdentity string

const (
	IdentityUnidentified FormatIdentity = "unidentified"
	IdentityDOCX         FormatIdentity = "docx"
	IdentityODT          FormatIdentity = "odt"
	IdentityConflicting  FormatIdentity = "conflicting"
)
const IdentityConflictCode = "office.identity_conflicting"

// A verified declaration chain can select Office candidates even when the main
// payload fails admission or parsing. This does not establish overall format.
func (p *Prepared) hasDOCXDeclarations() bool {
	return p != nil && p.DOCX != nil && p.DOCX.OPC != nil && p.DOCX.MainPart != ""
}

func identifyFormat(docx *docxidentify.Result, odt *odtidentify.Result) FormatIdentity {
	word := docx != nil && docx.Format == "docx"
	odf := odt != nil && odt.Format == "odt"
	switch {
	case word && odf:
		return IdentityConflicting
	case word:
		return IdentityDOCX
	case odf:
		return IdentityODT
	default:
		return IdentityUnidentified
	}
}

// Prepared preserves admission evidence even when a later operation fails.
// Source bytes must already be acquired and hashed by the host. This stage has
// no filesystem access and performs no second package read for identification.
// AnalysisTarget retains the declaration binding used for priority selection.
type AnalysisTarget struct {
	docxidentify.PartTarget
	Format    FormatIdentity
	Operation string
}

type Prepared struct {
	Targets   []AnalysisTarget
	Identity  FormatIdentity
	Limits    Limits
	Admission *Result
	DOCX      *docxidentify.Result
	ODT       *odtidentify.Result
}

// Prepare returns nil on pre-admission failure. A non-nil result on error
// retains the admission evidence gathered before cancellation or later failure.
// Callers must inspect both return values.
func Prepare(ctx context.Context, source []byte, sha256 string, limits Limits) (*Prepared, error) {
	if ctx == nil {
		return nil, errors.New("nil Office preparation context")
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	reader, err := packageparts.OpenOutcomes(ctx, source, sha256, limits.Package)
	if err != nil {
		return nil, err
	}
	resolver, err := NewNativeResolver(reader)
	if err != nil {
		return nil, err
	}
	result := &Prepared{Limits: limits, Identity: IdentityUnidentified}
	result.Admission, err = Admit(ctx, reader, resolver)
	result.Targets = resolver.AnalysisTargets()
	if err != nil {
		return result, err
	}
	result.DOCX, result.ODT, err = resolver.Identify(ctx)
	result.Identity = identifyFormat(result.DOCX, result.ODT)
	return result, err
}
