package odtidentify

import (
	"archive/zip"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func prepare(t *testing.T, parts map[string]string) (*packageparts.OutcomeReader, *PreparedManifest) {
	t.Helper()
	raw := archive(t, parts, "mimetype", zip.Store, nil)
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"mimetype", "META-INF/manifest.xml"}); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareManifest(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	return reader, prepared
}
func TestManifestPreparationUsesDeclaredSizeBeforeContentAdmission(t *testing.T) {
	parts := base()
	parts["META-INF/manifest.xml"] = manifest(entry("/", MIME, ` m:version="1.3"`) + entry("content.xml", "text/xml", fmt.Sprintf(` m:size="%d"`, len(parts["content.xml"]))))
	reader, prepared := prepare(t, parts)
	if prepared.ContentCandidate() != "content.xml" {
		t.Fatal("valid stored-size declaration was compared to unadmitted bytes")
	}
	for _, o := range reader.View().Parts {
		if o.Part.Name == "content.xml" && o.State != "not_run" {
			t.Fatal("preparation admitted content")
		}
	}
	if err := reader.Admit(context.Background(), []string{"content.xml"}); err != nil {
		t.Fatal(err)
	}
	got, err := InspectPrepared(context.Background(), reader, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "odt" || got.ManifestXML != prepared.result.ManifestXML {
		t.Fatal("lost identity or reparsed manifest")
	}
	got.Outcomes = nil
	legacy := inspect(t, parts)
	legacy.Outcomes = nil
	legacy.Package = nil
	if !reflect.DeepEqual(got, legacy) {
		t.Fatal("prepared ODT identity differs from legacy")
	}
}
func TestManifestCandidateRejectsIncompleteOrConflictingDeclarations(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"wrong MIME": func(p map[string]string) { p["mimetype"] = "application/xml" },
		"wrong content type": func(p map[string]string) {
			p["META-INF/manifest.xml"] = strings.Replace(p["META-INF/manifest.xml"], "text/xml", "application/xml", 1)
		},
		"case mismatch": func(p map[string]string) { p["CONTENT.XML"] = p["content.xml"]; delete(p, "content.xml") },
		"duplicate content": func(p map[string]string) {
			p["META-INF/manifest.xml"] = manifest(entry("/", MIME, "") + entry("content.xml", "text/xml", "") + entry("content.xml", "text/xml", ""))
		},
		"declared size mismatch": func(p map[string]string) {
			p["META-INF/manifest.xml"] = manifest(entry("/", MIME, "") + entry("content.xml", "text/xml", ` m:size="99999"`))
		},
		"encrypted": func(p map[string]string) {
			p["META-INF/manifest.xml"] = manifest(entry("/", MIME, "") + `<m:file-entry m:full-path="content.xml" m:media-type="text/xml"><m:encryption-data/></m:file-entry>`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			parts := base()
			mutate(parts)
			_, prepared := prepare(t, parts)
			if prepared.ContentCandidate() != "" {
				t.Fatal("invented ODF prerequisite")
			}
		})
	}
}
