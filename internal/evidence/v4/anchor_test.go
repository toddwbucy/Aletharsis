package v4

import (
	"encoding/json"
	"testing"
)

func TestScopeAnchorCannotSubstituteEqualTextArtifact(t *testing.T) {
	x, arts := bindingCase()
	artifacts := map[string]Artifact{}
	for _, a := range arts {
		artifacts[a.Ref] = a
	}
	locator, _ := json.Marshal(ScopeLocation{Kind: "office_scope", ScopeRef: "office-scope/0"})
	anchor := Anchor{Ref: "anchor/0", Kind: "office_scope", ArtifactRef: "artifact/2", ExecutionRef: str("exec/0"), Locator: locator}
	if err := x.ValidateOfficeAnchor(anchor, artifacts, nil); err != nil {
		t.Fatal(err)
	}
	other := arts[2]
	other.Ref = "artifact/3"
	other.ContentRef = &ContentRef{Kind: "office_scope", ScopeRef: str("office-scope/1")}
	artifacts[other.Ref] = other
	anchor.ArtifactRef = other.Ref
	if x.ValidateOfficeAnchor(anchor, artifacts, nil) == nil {
		t.Fatal("accepted same text with wrong scope identity")
	}
	anchor.ArtifactRef = "artifact/2"
	anchor.ExecutionRef = nil
	if x.ValidateOfficeAnchor(anchor, artifacts, nil) == nil {
		t.Fatal("accepted unbound execution")
	}
}
