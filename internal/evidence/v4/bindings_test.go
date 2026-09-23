package v4

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func bindingCase() (*Index, []Artifact) {
	size := int64(100)
	sourceSize := uint64(100)
	partSize := int64(20)
	partBytes := uint64(20)
	scope := Scope{ScopeRef: "office-scope/0", PartRef: "office-part/0", Text: "A", SHA256: identity.ExactBytes([]byte("A"))}
	scopeBytes := uint64(1)
	p := Package{PackageRef: "office-package/0", SourceArtifactRef: "artifact/0", SourceSHA256: "source", SourceByteLength: size}
	part := Part{PartRef: "office-part/0", PackageRef: p.PackageRef, Name: "word/document.xml", ArtifactRef: "artifact/1", SHA256: str("part"), ByteLength: &partSize}
	x := &Index{Packages: map[string]Package{p.PackageRef: p}, Parts: map[string]Part{part.PartRef: part}, Scopes: map[string]Scope{scope.ScopeRef: scope}}
	return x, []Artifact{
		{Ref: "artifact/0", Kind: "source", SHA256: &p.SourceSHA256, ByteLength: &sourceSize, Parents: []string{}},
		{Ref: "artifact/1", Kind: "package_part", SHA256: part.SHA256, ByteLength: &partBytes, Parents: []string{"artifact/0"}},
		{Ref: "artifact/2", Kind: "text", SHA256: &scope.SHA256, ByteLength: &scopeBytes, Parents: []string{"artifact/1"},
			ContentRef: &ContentRef{Kind: "office_scope", ScopeRef: &scope.ScopeRef}, Representation: json.RawMessage(`{"encoding":"utf-8"}`)},
	}
}
func TestArtifactIdentityChain(t *testing.T) {
	x, a := bindingCase()
	if err := x.ValidateArtifactBindings(a); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Index, []Artifact){
		func(x *Index, a []Artifact) { a[1].Parents = []string{"artifact/2"} },
		func(x *Index, a []Artifact) { a[2].Parents = []string{"artifact/0"} },
		func(x *Index, a []Artifact) { a[1].SHA256 = str("other") },
		func(x *Index, a []Artifact) { a[0].Parents = []string{"artifact/1"} },
		func(x *Index, a []Artifact) { a[1].Ref = "artifact/0" },
		func(x *Index, a []Artifact) {
			p := x.Parts["office-part/0"]
			p.PartRef = "office-part/1"
			p.Name = "word/header.xml"
			x.Parts[p.PartRef] = p
		},
	} {
		x, a := bindingCase()
		mutate(x, a)
		if x.ValidateArtifactBindings(a) == nil {
			t.Fatal("accepted broken identity chain")
		}
	}
}
