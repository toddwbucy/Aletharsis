package v4

import "testing"

func TestMetadataOrdinalIncludesUnprojectedSiblings(t *testing.T) {
	root := int64(0)
	doc := XML{XMLRef: "office-xml/0", PartRef: "office-part/0", Elements: []Element{
		{Index: 0, LocalName: "root"},
		{Index: 1, Parent: &root, Namespace: "urn:metadata", LocalName: "creator"},
		{Index: 2, Parent: &root, Namespace: "urn:metadata", LocalName: "creator"},
	}}
	index := &Index{XML: map[string]XML{doc.XMLRef: doc}}
	// The first creator has unprojected structured content. The second is an
	// extracted empty scalar; its original duplicate ordinal is still one.
	e := Evidence{Metadata: []Metadata{{PartRef: doc.PartRef, XMLRef: doc.XMLRef, Element: 2, Namespace: "urn:metadata", LocalName: "creator", DuplicateOrdinal: 1, ValueOrigins: []Origin{}}}}
	if err := index.ValidateInventories(e); err != nil {
		t.Fatal("valid original ordinal rejected", err)
	}
	e.Metadata[0].DuplicateOrdinal = 0
	if err := index.ValidateInventories(e); err == nil {
		t.Fatal("renumbered retained projection accepted")
	}
	// Different namespace and parent do not contribute to the sibling count.
	doc.Elements[1].Namespace = "urn:other"
	index.XML[doc.XMLRef] = doc
	if err := index.ValidateInventories(e); err != nil {
		t.Fatal("namespace conflated", err)
	}
	doc.Elements[1].Namespace = "urn:metadata"
	other := int64(1)
	doc.Elements[2].Parent = &other
	index.XML[doc.XMLRef] = doc
	if err := index.ValidateInventories(e); err != nil {
		t.Fatal("parent conflated", err)
	}
}
