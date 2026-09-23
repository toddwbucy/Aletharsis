package officeplan

import (
	"context"
	"errors"
	"sort"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"github.com/toddwbucy/Aletharsis/internal/officeobjects"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/scopelimits"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
)

// PartAnalysis keeps a failed part separate from usable sibling evidence.
// Error is retained for diagnostic mapping by the report coordinator.
type PartAnalysis struct {
	Operation, Part, State, Code string
	Error                        error
	Declaration                  *opcrels.Anchor
	Word                         *wordanalysis.Result
	ODT                          *odtanalysis.Result
	Metadata                     *officemetadata.Result
}

type Analysis struct {
	Parts        []PartAnalysis
	Objects      *officeobjects.Result
	ObjectsError error
}

// AnalyzePrepared operates only on admitted, verified package bytes. It neither
// discovers files nor follows relationships outside the package. Independently
// declaration-selected targets do not depend on a successful overall identity;
// each payload must still pass its own namespace/root and admission checks. Cancellation retains every selected
// target: unstarted operations are not_run, and completed siblings remain usable.
func AnalyzePrepared(ctx context.Context, p *Prepared) (*Analysis, error) {
	if ctx == nil || p == nil || p.Admission == nil || p.DOCX == nil || p.ODT == nil {
		return nil, v4.ErrLinkage
	}
	if err := p.Limits.Validate(); err != nil {
		return nil, err
	}
	result := &Analysis{Parts: []PartAnalysis{}}
	parts := map[string]packageparts.Outcome{}
	for _, o := range p.Admission.Outcomes.Parts {
		parts[o.Part.Name] = o
	}
	texts, metadata := []AnalysisTarget{}, []AnalysisTarget{}
	for _, target := range p.Targets {
		switch target.Operation {
		case capability.OfficeTextID:
			texts = append(texts, target)
		case capability.OfficeMetadataID:
			metadata = append(metadata, target)
		default:
			return nil, v4.ErrLinkage
		}
	}
	if p.hasDOCXDeclarations() {
		if err := ctx.Err(); err != nil {
			result.ObjectsError = err
		} else {
			result.Objects, result.ObjectsError = officeobjects.Inspect(ctx, p.DOCX)
		}
	}
	limits := scopelimits.Limits{TextUTF8Bytes: p.Limits.ScopeTextUTF8Bytes, ScalarOrigins: p.Limits.ScopeScalarOrigins}
	for _, group := range []struct {
		op      string
		targets []AnalysisTarget
	}{{capability.OfficeMetadataID, metadata}, {capability.OfficeTextID, texts}} {
		sort.Slice(group.targets, func(i, j int) bool {
			a, b := group.targets[i], group.targets[j]
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Namespace < b.Namespace
		})
		for i := 0; i < len(group.targets); {
			selected := group.targets[i]
			end := i + 1
			conflict := false
			for end < len(group.targets) && group.targets[end].Name == selected.Name {
				if other := group.targets[end]; other.Kind != selected.Kind || other.Namespace != selected.Namespace || other.Format != selected.Format {
					conflict = true
				}
				end++
			}
			i = end
			record := PartAnalysis{Operation: group.op, Part: selected.Name, State: "not_run"}
			if selected.Declaration.Part != "" {
				declaration := selected.Declaration
				record.Declaration = &declaration
			}
			o, exists := parts[selected.Name]
			switch {
			case ctx.Err() != nil:
				record.Error = ctx.Err()
				record.Code = string(failure.Canceled)
				if errors.Is(record.Error, context.DeadlineExceeded) {
					record.Code = string(failure.Timeout)
				}
			case conflict:
				record.Code = "office.target_role_conflicting"
				record.Error = errors.New("conflicting Office part roles")
			case !exists:
				record.Code = "opc.target_missing"
				record.Error = errors.New("declared Office target is absent")
			case o.State != "completed":
				// Extraction did not run. Retain the exact prerequisite code;
				// the package outcome separately preserves failed/canceled/etc.
				record.Code = o.Code
				record.Error = errors.New("Office target bytes are unavailable")
			case o.Part.SHA256 == "":
				record.Error = packageparts.ErrIdentity
			case group.op == capability.OfficeMetadataID:
				record.Metadata, record.Error = officemetadata.Extract(ctx, o.Part.Bytes, o.Part.SHA256)
				if record.Error == nil && record.Metadata.Kind != selected.Kind {
					record.Metadata = nil
					record.Error = officemetadata.ErrStructure
				}
				record.State = "failed"
				if record.Error == nil {
					record.State = record.Metadata.State
				}
			case selected.Kind == "odt":
				if p.ODT.ContentXML != nil && selected.Name == "content.xml" {
					record.ODT, record.Error = odtanalysis.AnalyzeParsedWithLimits(ctx, o.Part.Bytes, p.ODT.ContentXML, limits)
				} else {
					record.ODT, record.Error = odtanalysis.AnalyzeWithLimits(ctx, o.Part.Bytes, o.Part.SHA256, limits)
				}
				record.State = "failed"
				if record.Error == nil {
					record.State = record.ODT.State
				}
			default:
				if p.DOCX.MainXML != nil && selected.Name == p.DOCX.MainPart {
					record.Word, record.Error = wordanalysis.AnalyzeParsedWithLimits(ctx, o.Part.Bytes, p.DOCX.MainXML, limits)
				} else {
					record.Word, record.Error = wordanalysis.AnalyzeWithLimits(ctx, o.Part.Bytes, o.Part.SHA256, limits)
				}
				if record.Error == nil && (record.Word.Extraction.Story != selected.Kind || selected.Namespace != "" && record.Word.Extraction.Namespace != selected.Namespace) {
					record.Word = nil
					record.Error = errors.New("Office story declaration/root mismatch")
				}
				record.State = "failed"
				if record.Error == nil {
					record.State = record.Word.State
				}
			}
			if errors.Is(record.Error, context.Canceled) && record.State == "failed" {
				record.State = "canceled"
			}
			result.Parts = append(result.Parts, record)
		}
	}
	return result, ctx.Err()
}
