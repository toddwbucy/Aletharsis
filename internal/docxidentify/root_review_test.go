package docxidentify

import (
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
	"testing"
)

func TestRootRelationshipAliasesNeverSelectByOrder(t *testing.T) {
	a := opcrels.PartResult{Part: "_rels/.rels", State: "completed", XML: &xmlparts.Document{}}
	b := a
	b.Part = "_RELS/.rels"
	if !acceptableRootRelationships(&opcrels.Result{Parts: []opcrels.PartResult{a}}) {
		t.Fatal("single root rejected")
	}
	for _, parts := range [][]opcrels.PartResult{{a, b}, {b, a}} {
		if acceptableRootRelationships(&opcrels.Result{Parts: parts}) {
			t.Fatal("ambiguous roots depend on enumeration")
		}
	}
}
