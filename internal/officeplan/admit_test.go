package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

type resolver struct {
	roots   func(packageparts.OutcomeView) Targets
	related func(packageparts.OutcomeView, string) Related
}

func (r resolver) Roots(ctx context.Context, v packageparts.OutcomeView) (Targets, error) {
	return r.roots(v), ctx.Err()
}
func (r resolver) MainRelationships(_ context.Context, v packageparts.OutcomeView, s string) (Related, error) {
	return r.related(v, s), nil
}
func reader(t *testing.T, names []string, total int) *packageparts.OutcomeReader {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	limits := packageparts.DefaultLimits()
	limits.TotalBytes = total
	r, err := packageparts.OpenOutcomes(context.Background(), b.Bytes(), evidence.Hash(b.Bytes()), limits)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func states(v packageparts.OutcomeView) map[string]string {
	r := map[string]string{}
	for _, o := range v.Parts {
		r[o.Part.Name] = o.State
	}
	return r
}
func TestSequentialPriorityBarriersAndStorageIndependence(t *testing.T) {
	names := []string{"a.xml", "customXml/_rels/a.xml.rels", "word/embeddings/orphan", "word/embeddings/related", "word/header.xml", "word/_rels/main.xml.rels", "docProps/core.xml", "word/main.xml", "content.xml", "META-INF/manifest.xml", "_rels/.rels", "[Content_Types].xml", "mimetype"}
	var expected []Phase
	for _, reverse := range []bool{false, true} {
		order := append([]string(nil), names...)
		if reverse {
			for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
				order[i], order[j] = order[j], order[i]
			}
		}
		r := reader(t, order, 10)
		resolve := resolver{
			roots: func(v packageparts.OutcomeView) Targets {
				s := states(v)
				for _, n := range []string{"mimetype", "[Content_Types].xml", "_rels/.rels", "META-INF/manifest.xml"} {
					if s[n] != "completed" {
						t.Fatal("root barrier", n)
					}
				}
				if s["word/main.xml"] != "not_run" {
					t.Fatal("main admitted before declarations")
				}
				return Targets{ODFContent: "content.xml", MainPart: "word/main.xml", Metadata: []string{"docProps/core.xml"}}
			},
			related: func(v packageparts.OutcomeView, main string) Related {
				s := states(v)
				if main != "word/main.xml" || s["word/_rels/main.xml.rels"] != "completed" || s["docProps/core.xml"] != "completed" || s["word/header.xml"] != "not_run" {
					t.Fatal("dependent barrier")
				}
				return Related{Text: []string{"word/header.xml"}, Embedded: []string{"word/embeddings/related"}}
			},
		}
		got, err := Admit(context.Background(), r, resolve)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcomes.ReservedBytes != 10 {
			t.Fatal("wrong budget")
		}
		s := states(got.Outcomes)
		for _, n := range []string{"word/header.xml", "word/embeddings/related"} {
			if s[n] != "completed" {
				t.Fatal("ordinary part starved priority target", n)
			}
		}
		for _, n := range []string{"a.xml", "customXml/_rels/a.xml.rels", "word/embeddings/orphan"} {
			if s[n] != "not_run" {
				t.Fatal("later tier consumed priority budget", n)
			}
		}
		if expected == nil {
			expected = got.Phases
		} else if !reflect.DeepEqual(expected, got.Phases) {
			t.Fatal("storage order changed admission phases")
		}
	}
}
func TestCriticalCaseMatchingAndAmbiguity(t *testing.T) {
	r := reader(t, []string{"MIMETYPE", "CONTENT.XML", "[content_types].xml", "_RELS/.RELS", "META-INF/MANIFEST.XML"}, 10)
	resolve := resolver{roots: func(v packageparts.OutcomeView) Targets { return Targets{} }, related: func(v packageparts.OutcomeView, s string) Related { return Related{} }}
	got, err := Admit(context.Background(), r, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Phases[0].Parts, []string{"[content_types].xml", "_RELS/.RELS"}) {
		t.Fatal("ODF names were case folded or OPC names missed", got.Phases[0])
	}
	r = reader(t, []string{"[Content_Types].xml", "[CONTENT_TYPES].XML", "_rels/.rels"}, 10)
	got, err = Admit(context.Background(), r, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Phases[0].Parts, []string{"_rels/.rels"}) {
		t.Fatal("arbitrary ambiguous prerequisite selected")
	}
}
func TestSharedTargetsChargedOnceAndNoGuessedMain(t *testing.T) {
	r := reader(t, []string{"shared.xml", "word/document.xml"}, 2)
	resolve := resolver{roots: func(v packageparts.OutcomeView) Targets {
		return Targets{ODFContent: "shared.xml", Metadata: []string{"shared.xml"}}
	}, related: func(v packageparts.OutcomeView, main string) Related {
		if main != "" {
			t.Fatal("guessed main")
		}
		return Related{Text: []string{"shared.xml"}, Embedded: []string{"shared.xml"}}
	}}
	got, err := Admit(context.Background(), r, resolve)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range got.Phases {
		for _, n := range p.Parts {
			if n == "shared.xml" {
				count++
			}
		}
	}
	if count != 1 || got.Outcomes.ReservedBytes != 2 {
		t.Fatal("shared target charged repeatedly")
	}
}

func TestResolverCancellationRetainsAdmittedEvidence(t *testing.T) {
	reader := reader(t, []string{"mimetype", "content.xml"}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	resolver := resolver{roots: func(v packageparts.OutcomeView) Targets { cancel(); return Targets{} }, related: func(v packageparts.OutcomeView, s string) Related {
		t.Fatal("crossed canceled barrier")
		return Related{}
	}}
	got, err := Admit(ctx, reader, resolver)
	if err != context.Canceled || got == nil || len(got.Phases) != 1 || states(got.Outcomes)["mimetype"] != "completed" || states(got.Outcomes)["content.xml"] != "not_run" {
		t.Fatal("lost evidence on canceled resolver", err, got)
	}
}
