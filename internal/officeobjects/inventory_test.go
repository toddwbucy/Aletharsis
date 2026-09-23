package officeobjects

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"sort"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/docxidentify"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/opcrels"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func fixture(t *testing.T, extras map[string]string, declarations string, skip string) *docxidentify.Result {
	t.Helper()
	source, err := zip.OpenReader("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	parts := map[string]string{}
	for _, f := range source.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		ce := r.Close()
		if err != nil || ce != nil {
			t.Fatal(err, ce)
		}
		parts[f.Name] = string(b)
	}
	if declarations != "" {
		parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="` + opcrels.Namespace + `">` + declarations + `</Relationships>`
	}
	for k, v := range extras {
		parts[k] = v
	}
	names := []string{}
	for n := range parts {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(parts[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	admitted := []string{}
	for _, n := range names {
		if n != skip {
			admitted = append(admitted, n)
		}
	}
	if err := reader.Admit(context.Background(), admitted); err != nil {
		t.Fatal(err)
	}
	opc, err := opcrels.InspectVerified(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := docxidentify.InspectVerified(context.Background(), opc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "docx" {
		t.Fatal("invalid test document", doc.Issues)
	}
	return doc
}
func rel(id, kind, target, mode string) string {
	return `<Relationship Id="` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + kind + `" Target="` + target + `" TargetMode="` + mode + `"/>`
}

func TestCandidateReasonsAndOpaqueIdentity(t *testing.T) {
	declarations := rel("a", "oleObject", "embeddings/a.bin", "Internal") + rel("b", "package", "embeddings/a.bin", "Internal") + rel("c", "package", "../custom/object.bin", "Internal") + rel("x", "package", "https://example.invalid/x", "External") + rel("m", "package", "missing.bin", "Internal")
	doc := fixture(t, map[string]string{"word/embeddings/a.bin": "PK\x03\x04payload", "word/embeddings/orphan.bin": "PK\x03\x04payload", "custom/object.bin": "%PDF-test", "word/not-embedding.bin": "PK\x03\x04payload"}, declarations, "")
	r, err := Inspect(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Objects) != 3 || len(r.Declarations) != 5 || r.State != "partial" {
		t.Fatal("inventory counts/coverage", r)
	}
	if r.Objects[0].Anchor.Part != "custom/object.bin" || r.Objects[0].Reasons[0] != "embedded_relationship" || r.Objects[0].Signature != "pdf_header_magic" {
		t.Fatal("relationship-only candidate")
	}
	linked, orphan := r.Objects[1], r.Objects[2]
	if len(linked.Reasons) != 2 || len(linked.Relationships) != 2 || len(orphan.Relationships) != 0 || len(orphan.Reasons) != 1 {
		t.Fatal("lost independent inclusion evidence")
	}
	if linked.Anchor.PartSHA256 != orphan.Anchor.PartSHA256 || linked.ID == orphan.ID || linked.Anchor.Part == orphan.Anchor.Part {
		t.Fatal("collapsed identical-byte parts")
	}
	if linked.Signature != "zip_local_header_magic" || linked.Assessed[0].End != SignatureBytes {
		t.Fatal("signature coverage")
	}
	if r.Declarations[3].ObjectID != "" || r.Declarations[3].State != "external" || r.Declarations[4].ObjectID != "" || r.Declarations[4].Code != "opc.target_missing" {
		t.Fatal("invented external/missing target")
	}
	if len(r.Limitations) != 5 {
		t.Fatal("missing unavailable checks")
	}
}
func TestUnadmittedObjectRetainsCandidateWithoutDigest(t *testing.T) {
	name := "word/embeddings/a.bin"
	doc := fixture(t, map[string]string{name: "object"}, rel("a", "package", "embeddings/a.bin", "Internal"), name)
	r, err := Inspect(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != "completed" || len(r.Objects) != 1 {
		t.Fatal("lost unadmitted candidate")
	}
	o := r.Objects[0]
	if o.State != "not_run" || o.Code != "office.part_not_admitted" || o.ByteLength != nil || o.Anchor.PartSHA256 != "" || len(o.Assessed) != 0 || o.Signature != "" {
		t.Fatal("invented inspection", o)
	}
	if len(o.Relationships) != 1 || r.Declarations[0].ObjectID != o.ID {
		t.Fatal("lost unavailable target declaration")
	}
}
func TestEmptyInventoryAndUnrecognizedSignatures(t *testing.T) {
	doc := fixture(t, nil, "", "")
	r, err := Inspect(context.Background(), doc)
	if err != nil || r.State != "completed" || len(r.Objects) != 0 || r.Objects == nil {
		t.Fatal("empty assessed inventory", err)
	}
	doc = fixture(t, map[string]string{"word/embeddings/empty": "", "word/embeddings/unknown": "innocent"}, "", "")
	r, err = Inspect(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range r.Objects {
		if o.State != "completed" || o.Signature != "" || o.ByteLength == nil {
			t.Fatal("ordinary bytes asserted as type")
		}
	}
	if len(r.Objects[0].Assessed) != 0 || *r.Objects[0].ByteLength != 0 {
		t.Fatal("empty bytes are not unavailable")
	}
}
func TestSignatureAndRelationshipVocabulary(t *testing.T) {
	for _, prefix := range []string{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/", "http://purl.oclc.org/ooxml/officeDocument/relationships/"} {
		for _, kind := range []string{"oleObject", "package"} {
			if !recognizedType(prefix+kind) || recognizedType(prefix+kind+"X") {
				t.Fatal("relationship vocabulary")
			}
		}
	}
	for _, b := range [][]byte{{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, {'P', 'K', 3, 4}, []byte("%PDF-"), {0x89, 'P', 'N', 'G', 13, 10, 26, 10}} {
		if signature(b) == "" || signature(b[:len(b)-1]) != "" {
			t.Fatal("signature prefix bounds")
		}
	}
}

func TestUnavailableObjectFailureStillDegradesInventory(t *testing.T) {
	for _, code := range []string{"office.part_crc_failed", "office.part_limit", "office.aggregate_limit", "execution.canceled"} {
		name := "word/embeddings/a.bin"
		doc := fixture(t, map[string]string{name: "object"}, rel("a", "package", "embeddings/a.bin", "Internal"), name)
		for i := range doc.OPC.Outcomes.Parts {
			p := &doc.OPC.Outcomes.Parts[i]
			if p.Part.Name == name {
				p.Code = code
			}
		}
		r, err := Inspect(context.Background(), doc)
		if err != nil || r.State != "partial" || len(r.Objects) != 1 || r.Objects[0].Code != code {
			t.Fatal(code, r, err)
		}
	}
}
