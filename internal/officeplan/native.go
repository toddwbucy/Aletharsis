package officeplan

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/capability"
	"path"
	"slices"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/odtidentify"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// NativeResolver owns one staged OPC session and reusable type/manifest parses.
// Construct once per acquired source and use with Admit on that same reader.
type NativeResolver struct {
	reader   *packageparts.OutcomeReader
	opc      *opcrels.Session
	types    *docxidentify.PreparedTypes
	manifest *odtidentify.PreparedManifest
	targets  []AnalysisTarget
}

func NewNativeResolver(reader *packageparts.OutcomeReader) (*NativeResolver, error) {
	session, err := opcrels.NewSession(reader)
	if err != nil {
		return nil, err
	}
	return &NativeResolver{reader: reader, opc: session}, nil
}
func unique(view packageparts.OutcomeView, name string) string {
	match := ""
	for _, o := range view.Parts {
		if !o.Part.Directory && fold(o.Part.Name) == fold(name) {
			if match != "" {
				return ""
			}
			match = o.Part.Name
		}
	}
	return match
}
func (r *NativeResolver) Roots(ctx context.Context, view packageparts.OutcomeView) (Targets, error) {
	if view.SourceSHA256 != r.reader.SourceSHA256() {
		return Targets{}, packageparts.ErrIdentity
	}
	if name := unique(view, "_rels/.rels"); name != "" {
		if err := r.opc.Parse(ctx, []string{name}); err != nil {
			return Targets{}, err
		}
	}
	opc, err := r.opc.Result(ctx)
	if err != nil {
		return Targets{}, err
	}
	r.types, err = docxidentify.PrepareTypes(ctx, opc)
	if err != nil {
		return Targets{}, err
	}
	r.manifest, err = odtidentify.PrepareManifest(ctx, r.reader)
	if err != nil {
		return Targets{}, err
	}
	result := Targets{ODFContent: r.manifest.ContentCandidate(), Metadata: []string{}}
	if result.ODFContent != "" {
		r.targets = append(r.targets, AnalysisTarget{PartTarget: docxidentify.PartTarget{Name: result.ODFContent, Kind: "odt"}, Format: IdentityODT, Operation: capability.OfficeTextID})
	}
	if main, ok := r.types.MainTarget(); ok {
		result.MainPart = main.Name
		r.targets = append(r.targets, AnalysisTarget{PartTarget: main, Format: IdentityDOCX, Operation: capability.OfficeTextID})
	}
	for _, target := range r.types.MetadataTargets() {
		result.Metadata = append(result.Metadata, target.Name)
		r.targets = append(r.targets, AnalysisTarget{PartTarget: target, Format: IdentityDOCX, Operation: capability.OfficeMetadataID})
	}
	return result, nil
}
func (r *NativeResolver) MainRelationships(ctx context.Context, view packageparts.OutcomeView, main string) (Related, error) {
	result := Related{Text: []string{}, Embedded: []string{}}
	if main == "" {
		return result, nil
	}
	if r.types == nil || view.SourceSHA256 != r.reader.SourceSHA256() || main != r.types.MainCandidate() {
		return result, packageparts.ErrIdentity
	}
	dir := path.Dir(main)
	if dir == "." {
		dir = ""
	} else {
		dir += "/"
	}
	relName := unique(view, dir+"_rels/"+path.Base(main)+".rels")
	if relName == "" {
		return result, nil
	}
	if err := r.opc.Parse(ctx, []string{relName}); err != nil {
		return result, err
	}
	opc, err := r.opc.Result(ctx)
	if err != nil {
		return result, err
	}
	texts, embedded := r.types.RelatedTargets(opc, main, relName)
	for _, target := range texts {
		result.Text = append(result.Text, target.Name)
		r.targets = append(r.targets, AnalysisTarget{PartTarget: target, Format: IdentityDOCX, Operation: capability.OfficeTextID})
	}
	for _, target := range embedded {
		result.Embedded = append(result.Embedded, target.Name)
	}

	return result, nil
}

// Identify finishes the same declaration session after all admission phases.
// Each format inspector runs once; callers must invoke this only once per source.
func (r *NativeResolver) Identify(ctx context.Context) (*docxidentify.Result, *odtidentify.Result, error) {
	if r.types == nil || r.manifest == nil {
		return nil, nil, packageparts.ErrIdentity
	}
	names := []string{}
	for _, name := range r.reader.Names() {
		if strings.HasSuffix(fold(name), ".rels") {
			names = append(names, name)
		}
	}
	if err := r.opc.Parse(ctx, names); err != nil {
		return nil, nil, err
	}
	opc, err := r.opc.Result(ctx)
	if err != nil {
		return nil, nil, err
	}
	docx, err := docxidentify.InspectPrepared(ctx, opc, r.types)
	if err != nil {
		return nil, nil, err
	}
	odt, err := odtidentify.InspectPrepared(ctx, r.reader, r.manifest)
	if err != nil {
		return docx, nil, err
	}
	return docx, odt, nil
}

// AnalysisTargets returns the same typed declaration candidates used at each
// admission barrier. Extraction does not re-derive a second target set.
func (r *NativeResolver) AnalysisTargets() []AnalysisTarget { return slices.Clone(r.targets) }
