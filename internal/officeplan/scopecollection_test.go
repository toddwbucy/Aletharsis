package officeplan

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func TestCollectScopesBindsWholeOriginChain(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		for _, omit := range []bool{false, true} {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
			if err != nil {
				t.Fatal(err)
			}
			limits := DefaultLimits()
			if omit {
				limits.ScopeTextUTF8Bytes = 1
			}
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			base, err := BuildPackageRecords(context.Background(), raw, p, 1)
			if err != nil {
				t.Fatal(err)
			}
			xml, err := CollectXML(context.Background(), p, a, base)
			if err != nil {
				t.Fatal(err)
			}
			scopes, err := CollectScopes(context.Background(), base, a, xml)
			if err != nil {
				t.Fatal(err)
			}
			again, err := CollectScopes(context.Background(), base, a, xml)
			if err != nil || !reflect.DeepEqual(scopes, again) {
				t.Fatal("nondeterministic collection", err)
			}
			combined := base.Evidence
			combined.XML = xml.Documents
			combined.Scopes = scopes.Scopes
			index, err := v4.IndexEvidence(combined, evidence.Hash(raw), int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts := append(append([]v4.Artifact{}, base.Artifacts...), scopes.Artifacts...)
			if err := index.ValidateArtifactBindings(artifacts); err != nil {
				t.Fatal("coordinate identities do not connect", err)
			}
			omissions := 0
			for _, part := range a.Parts {
				if part.Word != nil {
					omissions += len(part.Word.Omitted)
				}
				if part.ODT != nil {
					omissions += len(part.ODT.Omitted)
				}
			}
			if omit {
				if len(scopes.Scopes) != 0 || len(scopes.Artifacts) != 0 || len(scopes.Findings) != 0 || omissions != 1 {
					t.Fatal("omitted scope retained dependent records")
				}
				continue
			}
			if omissions != 0 || len(scopes.Scopes) != 1 || len(scopes.Artifacts) != 1 || len(scopes.Findings) != 1 || len(scopes.Findings[0].Findings) == 0 {
				t.Fatal("missing retained evidence")
			}
			artifact := scopes.Artifacts[0]
			if !v4.ScopeArtifactEquivalent(artifact, scopes.Scopes[0]) || scopes.Findings[0].ArtifactRef != artifact.Ref || scopes.Findings[0].ScopeRef != scopes.Scopes[0].ScopeRef {
				t.Fatal("finding scope detached")
			}
			mapping, err := v2.DecodeMapping(artifact.Mapping)
			if err != nil {
				t.Fatal(err)
			}
			if mapping.Quality != "derived" || mapping.FromArtifactRef != artifact.Ref || mapping.ToArtifactRef != artifact.Parents[0] {
				t.Fatal("false mapping claim")
			}
			// Common artifact fields remain valid under the existing native contract;
			// the separately validated Office content reference is the only extension.
			surrogate := artifact
			surrogate.ContentRef = &v4.ContentRef{Kind: "retained_blob", SHA256: artifact.SHA256}
			encoded, err := json.Marshal(surrogate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeArtifact(encoded); err != nil {
				t.Fatal(err)
			}
		}
	}
}
