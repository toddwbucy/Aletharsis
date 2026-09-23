package officeplan

import (
	"context"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
)

func TestWordScopeRecordCrossRunCoordinates(t *testing.T) {
	source := []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>e&#x301;😀</w:t></w:r><w:r><w:rPr><w:vanish/></w:rPr><w:t>&#x200B;</w:t></w:r></w:p></w:body></w:document>`)
	hash := evidence.Hash(source)
	size := int64(len(source))
	container := evidence.Hash([]byte("container"))
	p := v4.Part{PartRef: "office-part/0", Name: "word/document.xml", SHA256: &hash, ByteLength: &size}
	a, err := wordanalysis.Analyze(context.Background(), source, hash)
	if err != nil {
		t.Fatal(err)
	}
	x, err := BuildXMLRecord(a.Extraction.XML, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Scopes) != 1 {
		t.Fatal("invalid fixture")
	}
	s, err := WordScopeRecord(a.Scopes[0], p, container, x, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "é😀\u200b" || len(s.Origins) != 4 || s.Origins[3].Source != wireSpan(a.Scopes[0].Origins[3].Source) {
		t.Fatal("lost origin chain")
	}
	other := p
	other.Name = "word/header.xml"
	renamed, err := WordScopeRecord(a.Scopes[0], other, container, x, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.IdentitySHA256 == renamed.IdentitySHA256 || s.SHA256 != renamed.SHA256 {
		t.Fatal("scope identity confused with content hash")
	}
	a.Scopes[0].Origins[0].Source.Start++
	if _, err := WordScopeRecord(a.Scopes[0], p, container, x, 0); err == nil {
		t.Fatal("corrupted coordinate accepted")
	}
}

func TestODTScopeRecordControlIdentity(t *testing.T) {
	source := []byte(`<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.3"><office:body><office:text><text:p><text:s text:c="bad"/>A<text:s text:c="2"/><text:tab/><text:line-break/>😀</text:p></office:text></office:body></office:document-content>`)
	hash := evidence.Hash(source)
	size := int64(len(source))
	container := evidence.Hash([]byte("container"))
	p := v4.Part{PartRef: "office-part/0", Name: "content.xml", SHA256: &hash, ByteLength: &size}
	a, err := odtanalysis.Analyze(context.Background(), source, hash)
	if err != nil {
		t.Fatal(err)
	}
	x, err := BuildXMLRecord(a.Extraction.XML, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	controls, refs, err := ODTControlsForScopes(a)
	if err != nil {
		t.Fatal(err)
	}
	x.Controls = controls
	if len(controls) != 3 || len(refs) != 3 {
		t.Fatal("unknown control gained a count")
	}
	if _, ok := refs[0]; ok {
		t.Fatal("unresolved control acquired an index")
	}
	joined := ""
	for i, native := range a.Scopes {
		s, err := ODTScopeRecord(native, p, container, x, refs, i)
		if err != nil {
			t.Fatal(err)
		}
		joined += s.Text
		for _, o := range s.Origins {
			if o.Kind == "control_expansion" && (o.Control == nil || o.Element == nil || o.Repetition == nil || o.TextIndex != nil) {
				t.Fatal("generated origin became literal")
			}
		}
	}
	if joined != "A  \t\n😀" {
		t.Fatal(joined)
	}
	for i, native := range a.Scopes {
		if strings.Contains(native.Text, "\t") {
			if _, err := ODTScopeRecord(native, p, container, x, map[int]int{}, i); err == nil {
				t.Fatal("missing control reference accepted")
			}
		}
	}
}
