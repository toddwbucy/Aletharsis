package v4

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func TestScopeArtifactsBindExactText(t *testing.T) {
	scope := Scope{ScopeRef: "office-scope/0", Text: "😀A", SHA256: identity.ExactBytes([]byte("😀A"))}
	size := uint64(len(scope.Text))
	artifact := Artifact{Ref: "artifact/2", Kind: "text", SHA256: &scope.SHA256, ByteLength: &size,
		ContentRef: &ContentRef{Kind: "office_scope", ScopeRef: &scope.ScopeRef}, Representation: json.RawMessage(`{"encoding":"utf-8"}`)}
	x := Index{Scopes: map[string]Scope{scope.ScopeRef: scope}}
	if err := x.ValidateScopeArtifacts([]Artifact{artifact}); err != nil {
		t.Fatal(err)
	}
	if !ScopeArtifactEquivalent(artifact, scope) {
		t.Fatal("scope artifact does not link")
	}
	for _, mutate := range []func(*Artifact){
		func(a *Artifact) { a.SHA256 = str("wrong") }, func(a *Artifact) { n := uint64(2); a.ByteLength = &n },
		func(a *Artifact) { a.ContentRef.ScopeRef = str("office-scope/1") },
		func(a *Artifact) { a.ContentRef.Pointer = str("/evidence/texts/0/text") },
		func(a *Artifact) { a.UnavailableReason = str("not_retained") },
		func(a *Artifact) { a.Representation = json.RawMessage(`{"encoding":"utf-16-le"}`) },
		func(a *Artifact) { a.Kind = "source" },
	} {
		bad := artifact
		c := *artifact.ContentRef
		bad.ContentRef = &c
		mutate(&bad)
		if x.ValidateScopeArtifacts([]Artifact{bad}) == nil {
			t.Fatal("accepted mismatched scope artifact")
		}
	}
	if x.ValidateScopeArtifacts(nil) == nil {
		t.Fatal("accepted missing scope artifact")
	}
	if x.ValidateScopeArtifacts([]Artifact{artifact, artifact}) == nil {
		t.Fatal("accepted duplicate scope artifact")
	}
}
