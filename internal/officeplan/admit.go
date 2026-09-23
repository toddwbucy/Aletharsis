// Package officeplan orders Office package admission across dependency barriers.
package officeplan

import (
	"context"
	"path"
	"sort"
	"strings"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// Targets contains original package names resolved from verified declarations.
// Empty fields mean unresolved/absent prerequisites, never guessed default names.
type Targets struct {
	ODFContent, MainPart string
	Metadata             []string
}
type Related struct{ Text, Embedded []string }

// Resolver inspects admitted declarations only. It must enforce format-specific
// namespaces, context, types and ambiguity rules. Priority is not type admission.
type Resolver interface {
	Roots(context.Context, packageparts.OutcomeView) (Targets, error)
	MainRelationships(context.Context, packageparts.OutcomeView, string) (Related, error)
}
type Phase struct {
	Name  string
	Parts []string
}

// Result exposes planning observations for deterministic admission tests and
// debugging. Phases are not report coverage: consumers must use Outcomes
// and the namespace-checked producer diagnostics when constructing a report.
type Result struct {
	Phases   []Phase
	Outcomes packageparts.OutcomeView
}

func fold(s string) string { return opcrels.FoldName(s) }
func canonicalRelationship(name string) bool {
	_, ok := opcrels.RelationshipOwner(fold(name))
	return ok
}

// Admit runs fixed sequential phases. The resolver sees completed admissions at
// each barrier; packageparts owns one-time reservation and bounded-fit behavior.
// This is sequential by design so worker scheduling cannot alter coverage.
func Admit(ctx context.Context, reader *packageparts.OutcomeReader, resolver Resolver) (result *Result, err error) {
	if ctx == nil || reader == nil || resolver == nil {
		return nil, v4.ErrLinkage
	}
	view := reader.ReadOnlyView()
	if view.SourceSHA256 == "" {
		return nil, v4.ErrLinkage
	}
	r := &Result{Phases: []Phase{}}
	defer func() {
		r.Outcomes = reader.ReadOnlyView()
		if err != nil {
			result = r
		}
	}()
	names := map[string]bool{}
	folded := map[string][]string{}
	for _, o := range view.Parts {
		if !o.Part.Directory {
			names[o.Part.Name] = true
			folded[fold(o.Part.Name)] = append(folded[fold(o.Part.Name)], o.Part.Name)
		}
	}
	selected := map[string]bool{}
	match := func(name string, ascii bool) string {
		if name == "" {
			return ""
		}
		if !ascii {
			if names[name] {
				return name
			}
			return ""
		}
		found := folded[fold(name)]
		if len(found) == 1 {
			return found[0]
		}
		return ""
	}
	phase := func(name string, candidates []string, ordered bool) error {
		candidates = append([]string(nil), candidates...)
		if !ordered {
			sort.Strings(candidates)
		}
		parts := []string{}
		for _, n := range candidates {
			if n == "" || selected[n] {
				continue
			}
			if !names[n] {
				continue
			}
			selected[n] = true
			parts = append(parts, n)
		}
		if err := reader.Admit(ctx, parts); err != nil {
			return err
		}
		r.Phases = append(r.Phases, Phase{name, parts})
		return nil
	}
	if err := phase("identification", []string{match("mimetype", false), match("[Content_Types].xml", true), match("_rels/.rels", true), match("META-INF/manifest.xml", false)}, true); err != nil {
		return nil, err
	}
	targets, err := resolver.Roots(ctx, reader.ReadOnlyView())
	if err != nil {
		return nil, err
	}
	if err := phase("main_content", []string{targets.ODFContent, targets.MainPart}, true); err != nil {
		return nil, err
	}
	if err := phase("metadata", targets.Metadata, false); err != nil {
		return nil, err
	}
	mainRels := ""
	if targets.MainPart != "" {
		directory := path.Dir(targets.MainPart)
		if directory == "." {
			directory = ""
		} else {
			directory += "/"
		}
		mainRels = match(directory+"_rels/"+path.Base(targets.MainPart)+".rels", true)
	}
	if err := phase("main_relationships", []string{mainRels}, true); err != nil {
		return nil, err
	}
	related, err := resolver.MainRelationships(ctx, reader.ReadOnlyView(), targets.MainPart)
	if err != nil {
		return nil, err
	}
	if err := phase("related_text", related.Text, false); err != nil {
		return nil, err
	}
	if err := phase("related_embeddings", related.Embedded, false); err != nil {
		return nil, err
	}
	embeddings, relationships, ordinary := []string{}, []string{}, []string{}
	for _, o := range view.Parts {
		n := o.Part.Name
		if o.Part.Directory {
			continue
		}
		switch {
		case strings.HasPrefix(fold(n), "word/embeddings/"):
			embeddings = append(embeddings, n)
		case canonicalRelationship(n):
			relationships = append(relationships, n)
		default:
			ordinary = append(ordinary, n)
		}
	}
	for _, p := range []Phase{{"conventional_embeddings", embeddings}, {"remaining_relationships", relationships}, {"ordinary", ordinary}} {
		if err := phase(p.Name, p.Parts, false); err != nil {
			return nil, err
		}
	}
	// OpenOutcomes already verifies empty directory payloads.
	return r, nil
}
