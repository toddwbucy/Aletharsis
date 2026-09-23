package officeplan

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

func TestAnalyzePreparedOfficeFixtures(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-partial-crc.docx", "office-odt-minimal.odt"} {
		raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		a, err := AnalyzePrepared(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		assertTargetSets(t, p, a)
		if len(a.Parts) != 1 || a.Parts[0].State != "completed" || a.Parts[0].Operation != capability.OfficeTextID {
			t.Fatal(name, a)
		}
		r := a.Parts[0]
		if strings.HasSuffix(name, ".odt") {
			if r.ODT == nil || len(r.ODT.Scopes) != 1 || len(r.ODT.Scopes[0].Findings) == 0 || a.Objects != nil {
				t.Fatal(name)
			}
		} else if r.Word == nil || len(r.Word.Scopes) != 1 || len(r.Word.Scopes[0].Findings) == 0 || a.Objects == nil || a.ObjectsError != nil {
			t.Fatal(name)
		}
	}
}

func analysisFixture(t *testing.T, mutate func(map[string]string)) []byte {
	t.Helper()
	zr, err := zip.OpenReader("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	parts := map[string]string{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		closeErr := r.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		parts[f.Name] = string(b)
	}
	mutate(parts)
	names := []string{}
	for n := range parts {
		names = append(names, n)
	}
	sort.Strings(names)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
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
	return b.Bytes()
}

func TestAnalyzePreparedMetadataIsolationAndBinding(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"valid", `<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Example</dc:creator></cp:coreProperties>`, "completed"},
		{"malformed", "<broken>", "failed"},
		{"wrong kind", `<Properties xmlns="` + officemetadata.AppNamespace + `"/>`, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := analysisFixture(t, func(parts map[string]string) {
				parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/unusual/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`, 1)
				parts["_rels/.rels"] = strings.Replace(parts["_rels/.rels"], "</Relationships>", `<Relationship Id="core" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="unusual/core.xml"/></Relationships>`, 1)
				parts["unusual/core.xml"] = tc.body
				parts["docProps/core.xml"] = tc.body // familiar unbound filename is a decoy
			})
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			a, err := AnalyzePrepared(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Parts) != 2 {
				t.Fatal("decoy admitted or selected part lost", a)
			}
			m, body := a.Parts[0], a.Parts[1]
			if m.Part != "unusual/core.xml" || m.State != tc.want || body.State != "completed" || body.Word == nil {
				t.Fatal(a)
			}
			if tc.want == "completed" && (len(m.Metadata.Properties) != 1 || m.Metadata.Properties[0].Value != "Example") {
				t.Fatal(m)
			}
			if tc.want == "failed" && (m.Error == nil || m.Metadata != nil) {
				t.Fatal("failed metadata retained values")
			}
		})
	}
}

func TestAnalyzePreparedRelatedStoryBindings(t *testing.T) {
	for _, story := range []struct{ kind, root string }{{"header", "hdr"}, {"footer", "ftr"}, {"comments", "comments"}, {"footnotes", "footnotes"}, {"endnotes", "endnotes"}} {
		for _, wrongRoot := range []bool{false, true} {
			t.Run(story.kind+map[bool]string{true: " mismatch", false: ""}[wrongRoot], func(t *testing.T) {
				raw := analysisFixture(t, func(parts map[string]string) {
					parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/word/story.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.`+story.kind+`+xml"/></Types>`, 1)
					parts["word/_rels/document.xml.rels"] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="story" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + story.kind + `" Target="story.xml"/><Relationship Id="object" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/a.bin"/></Relationships>`
					root := story.root
					if wrongRoot {
						root = "document"
					}
					body := `<w:p><w:r><w:t>x</w:t></w:r></w:p>`
					switch root {
					case "comments":
						body = `<w:comment>` + body + `</w:comment>`
					case "footnotes":
						body = `<w:footnote>` + body + `</w:footnote>`
					case "endnotes":
						body = `<w:endnote>` + body + `</w:endnote>`
					case "document":
						body = `<w:body>` + body + `</w:body>`
					}
					parts["word/story.xml"] = `<w:` + root + ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + body + `</w:` + root + `>`
					parts["word/embeddings/a.bin"] = "opaque"
				})
				p, err := Prepare(context.Background(), raw, evidence.Hash(raw), DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				a, err := AnalyzePrepared(context.Background(), p)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.Parts) != 2 || a.Objects == nil || len(a.Objects.Objects) != 1 {
					t.Fatal("missing story/object", a)
				}
				r := a.Parts[1]
				if r.Part != "word/story.xml" {
					t.Fatal(r)
				}
				if wrongRoot {
					if r.State != "failed" || r.Error == nil || r.Word != nil {
						t.Fatal("mismatched story admitted")
					}
				} else if r.State != "completed" || r.Word == nil || len(r.Word.Scopes) != 1 || r.Word.Scopes[0].Text != "x" {
					t.Fatal("valid story excluded", r)
				}
			})
		}
	}
}

func TestAnalyzePreparedUsesConfiguredScopeLimits(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	l := DefaultLimits()
	l.ScopeTextUTF8Bytes = 1
	p, err := Prepare(context.Background(), raw, evidence.Hash(raw), l)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePrepared(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Parts) != 1 || a.Parts[0].State != "partial" || len(a.Parts[0].Word.Omitted) != 1 || len(a.Parts[0].Word.Scopes) != 0 {
		t.Fatal("configured scope ceiling not applied", a)
	}
}
