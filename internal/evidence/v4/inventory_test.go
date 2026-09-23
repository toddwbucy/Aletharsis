package v4

import "testing"

func str(s string) *string { return &s }
func inventoryCase() (*Index, Evidence) {
	size := int64(100)
	p := Part{PartRef: "office-part/0", PackageRef: "office-package/0", SHA256: str("part"), ByteLength: &size}
	target := Part{PartRef: "office-part/1", PackageRef: p.PackageRef, SHA256: str("object"), ByteLength: &size}
	doc := XML{XMLRef: "office-xml/0", PartRef: p.PartRef, Elements: []Element{{Index: 0, Namespace: "ns", LocalName: "creator", Span: Span{0, 20}}},
		Tokens:   []Token{{Index: 0, Element: wide(0), Span: Span{0, 20}}},
		Segments: []Segment{{Element: number(0), Scalars: []Scalar{{Index: 0, CodePoint: 65, Source: Span{9, 10}, UTF8: Span{0, 1}, Transformation: "literal"}}}}}
	x := &Index{Parts: map[string]Part{p.PartRef: p, target.PartRef: target}, XML: map[string]XML{doc.XMLRef: doc}}
	loc := StructuralLocation{Kind: "office_structure", PartRef: p.PartRef, PartSHA256: *p.SHA256, XMLRef: &doc.XMLRef, Element: wide(0), Span: &Span{0, 20}}
	rel := Relationship{RelationshipRef: "office-relationship/0", Location: loc, ResolutionState: "resolved", TargetPartRef: &target.PartRef}
	object := Object{ObjectRef: "office-object/0", PartRef: target.PartRef, SHA256: target.SHA256, InclusionEvidence: []string{"relationship"}, RelationshipRefs: []string{rel.RelationshipRef}, Inspection: Outcome{PartRef: &target.PartRef}}
	meta := Metadata{XMLRef: doc.XMLRef, PartRef: p.PartRef, Namespace: "ns", LocalName: "creator", Element: 0, LexicalValue: "A", ValueOrigins: []Origin{{Kind: "stored", XMLRef: doc.XMLRef, TextIndex: number(0), Segment: number(0), Scalar: number(0), Source: Span{9, 10}, UTF8: Span{0, 1}, Transformation: "literal"}}}
	return x, Evidence{Relationships: []Relationship{rel}, Objects: []Object{object}, Metadata: []Metadata{meta}}
}
func TestInventoryCrossReferences(t *testing.T) {
	x, e := inventoryCase()
	if err := x.ValidateInventories(e); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Index, *Evidence){
		func(x *Index, e *Evidence) { e.Objects[0].SHA256 = str("wrong") },
		func(x *Index, e *Evidence) { e.Objects[0].RelationshipRefs = []string{"office-relationship/99"} },
		func(x *Index, e *Evidence) { e.Relationships[0].ResolutionState = "external" },
		func(x *Index, e *Evidence) { e.Relationships[0].TargetPartRef = str("office-part/99") },
		func(x *Index, e *Evidence) { e.Relationships[0].Location.PartSHA256 = "wrong" },
		func(x *Index, e *Evidence) { e.Relationships[0].Location.Element = wide(99) },
		func(x *Index, e *Evidence) { e.Metadata[0].Namespace = "wrong" },
		func(x *Index, e *Evidence) { e.Metadata[0].XMLRef = "office-xml/99" },
		func(x *Index, e *Evidence) { e.Metadata[0].DuplicateOrdinal = 1 },
		func(x *Index, e *Evidence) { e.Metadata[0].LexicalValue = "B" },
		func(x *Index, e *Evidence) {
			p := x.Parts["office-part/1"]
			p.PackageRef = "office-package/1"
			x.Parts[p.PartRef] = p
		},
	} {
		x, e := inventoryCase()
		mutate(x, &e)
		if x.ValidateInventories(e) == nil {
			t.Fatal("accepted damaged inventory linkage")
		}
	}
}
func TestOpaqueObjectAnchorHasNoTextCoordinates(t *testing.T) {
	x, e := inventoryCase()
	o := e.Objects[0]
	loc := StructuralLocation{Kind: "office_structure", PartRef: o.PartRef, PartSHA256: *o.SHA256, ObjectRef: &o.ObjectRef}
	objects := map[string]Object{o.ObjectRef: o}
	if err := x.ValidateStructuralLocation(loc, objects); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*StructuralLocation){
		func(l *StructuralLocation) { l.Element = wide(0) }, func(l *StructuralLocation) { l.Token = wide(0) },
		func(l *StructuralLocation) { l.XMLRef = str("office-xml/0") }, func(l *StructuralLocation) { l.Span = &Span{0, 1} },
	} {
		bad := loc
		mutate(&bad)
		if x.ValidateStructuralLocation(bad, objects) == nil {
			t.Fatal("accepted mixed opaque/XML anchor")
		}
	}
}

func unavailableObjectCase() (*Index, Evidence) {
	x, e := inventoryCase()
	p := x.Parts["office-part/1"]
	p.Name = "word/embeddings/object.bin"
	p.SHA256 = nil
	p.ByteLength = nil
	p.State = "not_run"
	x.Parts[p.PartRef] = p
	e.Objects[0].SHA256 = nil
	e.Relationships[0].Mode = "Internal"
	e.Relationships[0].SourceOwner = "word/document.xml"
	e.Relationships[0].Target = "embeddings/object.bin"
	e.Relationships[0].ResolutionState = "unresolved"
	e.Relationships[0].ResolutionCode = str("opc.target_unavailable")
	e.Relationships[0].TargetPartRef = nil
	return x, e
}
func TestUnverifiedObjectCandidateKeepsOnlyUniqueDeclarationLink(t *testing.T) {
	x, e := unavailableObjectCase()
	if err := x.ValidateInventories(e); err != nil {
		t.Fatal("lost unavailable candidate relationship", err)
	}
	for name, mutate := range map[string]func(*Index, *Evidence){
		"external":        func(x *Index, e *Evidence) { e.Relationships[0].Mode = "External" },
		"unrelated path":  func(x *Index, e *Evidence) { e.Relationships[0].Target = "elsewhere.bin" },
		"missing target":  func(x *Index, e *Evidence) { e.Relationships[0].ResolutionCode = str("opc.target_missing") },
		"unsupported URI": func(x *Index, e *Evidence) { e.Relationships[0].Target = "embeddings/%6fbject.bin" },
		"case ambiguity": func(x *Index, e *Evidence) {
			p := x.Parts["office-part/1"]
			p.PartRef = "office-part/2"
			p.Name = "WORD/EMBEDDINGS/OBJECT.BIN"
			x.Parts[p.PartRef] = p
		},
		"other package": func(x *Index, e *Evidence) {
			p := x.Parts["office-part/1"]
			p.PackageRef = "office-package/1"
			x.Parts[p.PartRef] = p
		},
		"invented verified reference": func(x *Index, e *Evidence) { e.Relationships[0].TargetPartRef = str("office-part/1") },
	} {
		t.Run(name, func(t *testing.T) {
			x, e := unavailableObjectCase()
			mutate(x, &e)
			if x.ValidateInventories(e) == nil {
				t.Fatal("accepted unsupported candidate link")
			}
		})
	}
}
