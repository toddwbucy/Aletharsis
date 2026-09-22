package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func TestScopeIdentityBindsEveryInput(t *testing.T) {
	inputs := []string{"source", "word/document.xml", "part", "extractor/1", "assembler/1", "scope/0"}
	digest := func(v []string) string { return ScopeIdentity(v[0], v[1], v[2], v[3], v[4], v[5]) }
	expected := digest(inputs)
	for i := range inputs {
		changed := append([]string{}, inputs...)
		changed[i] += "x"
		if digest(changed) == expected {
			t.Fatalf("identity ignores input %d", i)
		}
	}
	if ScopeIdentity("a", "bc", "", "", "", "") == ScopeIdentity("ab", "c", "", "", "", "") {
		t.Fatal("ambiguous tuple encoding")
	}
}

func TestScopeRejectsCrossPartAndDamagedOrigins(t *testing.T) {
	hash := identity.ExactBytes([]byte("<r>A</r>"))
	size := int64(8)
	part := Part{PartRef: "office-part/0", Name: "word/document.xml", SHA256: &hash, ByteLength: &size}
	s := Scope{ScopeRef: "office-scope/0", PartRef: part.PartRef, LocalID: "scope/0",
		ExtractorVersion: "word/1", AssemblerVersion: "analysis/1", Text: "A", SHA256: identity.ExactBytes([]byte("A")),
		Origins: []Origin{{Kind: "stored", XMLRef: "office-xml/0", TextIndex: number(0), Segment: number(0), Scalar: number(0),
			Source: Span{3, 4}, UTF8: Span{0, 1}, Transformation: "literal"}}}
	s.Hashes.RawTextSHA256 = s.SHA256
	s.IdentitySHA256 = ScopeIdentity("source", part.Name, hash, s.ExtractorVersion, s.AssemblerVersion, s.LocalID)
	x := XML{XMLRef: "office-xml/0", PartRef: part.PartRef, Segments: []Segment{{Scalars: []Scalar{{Index: 0, CodePoint: 65, Source: Span{3, 4}, UTF8: Span{0, 1}, Transformation: "literal"}}}}}
	docs := map[string]XML{x.XMLRef: x}
	if err := ValidateScope(s, part, "source", docs); err != nil {
		t.Fatal(err)
	}
	x.PartRef = "office-part/1"
	docs[x.XMLRef] = x
	if ValidateScope(s, part, "source", docs) == nil {
		t.Fatal("accepted cross-part origin")
	}
	x.PartRef = part.PartRef
	docs[x.XMLRef] = x
	for _, mutate := range []func(*Scope){
		func(v *Scope) { v.IdentitySHA256 = "wrong" }, func(v *Scope) { v.SHA256 = "wrong" },
		func(v *Scope) { v.Text = "B" }, func(v *Scope) { v.Origins = nil },
		func(v *Scope) { v.Boundaries = []Boundary{{XMLRef: x.XMLRef, Token: 99}} },
	} {
		bad := s
		mutate(&bad)
		if ValidateScope(bad, part, "source", docs) == nil {
			t.Fatal("accepted damaged scope")
		}
	}
}
