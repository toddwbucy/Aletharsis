package odtidentify

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func archive(t testing.TB, parts map[string]string, first string, method uint16, extra []byte) []byte {
	t.Helper()
	names := []string{}
	if _, ok := parts[first]; ok {
		names = append(names, first)
	}
	others := []string{}
	for n := range parts {
		if n != first {
			others = append(others, n)
		}
	}
	sort.Strings(others)
	names = append(names, others...)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range names {
		h := &zip.FileHeader{Name: n, Method: zip.Store}
		if n == "mimetype" {
			h.Method = method
			h.Extra = extra
		}
		f, err := w.CreateHeader(h)
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
func manifest(entries string) string {
	return `<m:manifest xmlns:m="` + ManifestNS + `" m:version="1.3">` + entries + `</m:manifest>`
}
func entry(path, typ, extra string) string {
	return `<m:file-entry m:full-path="` + path + `" m:media-type="` + typ + `"` + extra + `/>`
}
func base() map[string]string {
	return map[string]string{"mimetype": MIME, "META-INF/manifest.xml": manifest(entry("/", MIME, ` m:version="1.3"`) + entry("content.xml", "text/xml", "")), "content.xml": `<o:document-content xmlns:o="` + OfficeNS + `" o:version="1.3"><o:body><o:text><p xmlns="urn:test">A&#x200B;😀</p></o:text></o:body></o:document-content>`}
}
func inspect(t testing.TB, parts map[string]string) *Result {
	t.Helper()
	b := archive(t, parts, "mimetype", zip.Store, nil)
	r, err := Inspect(context.Background(), b, evidence.Hash(b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func has(r *Result, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
func add(parts map[string]string, e string) {
	parts["META-INF/manifest.xml"] = strings.Replace(parts["META-INF/manifest.xml"], "</m:manifest>", e+"</m:manifest>", 1)
}
func TestManifestIdentityAndLocations(t *testing.T) {
	for _, version := range []string{"1.2", "1.3"} {
		p := base()
		for name, data := range p {
			p[name] = strings.ReplaceAll(data, `version="1.3"`, `version="`+version+`"`)
		}
		r := inspect(t, p)
		if r.State != "completed" || r.Format != "odt" || r.ManifestVersion != version || r.ContentXML.PartSHA256 != r.ContentSHA256 || r.ContentSHA256 != evidence.Hash([]byte(p["content.xml"])) || r.MimetypeSHA256 != evidence.Hash([]byte(MIME)) {
			t.Fatal("identification", r.Issues)
		}
		for _, e := range r.Entries {
			if e.Anchor.PartSHA256 != r.ManifestSHA256 || !strings.HasPrefix(p[e.Anchor.Part][e.Anchor.Span.Start:e.Anchor.Span.End], "<m:file-entry") {
				t.Fatal("entry anchor")
			}
		}
		if r.Entries[r.RootEntry].State != "package" || r.Entries[r.ContentEntry].State != "resolved" {
			t.Fatal("selected entries")
		}
	}
}
func TestExactNamesNoURINormalization(t *testing.T) {
	p := base()
	for _, name := range []string{"media/A.bin", "media/a.bin", "media/a%20b.bin", "media/日本語.bin"} {
		p[name] = name
		add(p, entry(name, "application/octet-stream", ""))
	}
	add(p, entry("media/", "", ""))
	r := inspect(t, p)
	if r.Format != "odt" || r.State != "completed" {
		t.Fatal("ODF names coerced", r.Issues)
	}
	for _, e := range r.Entries {
		if e.Path == "media/" {
			if e.State != "directory" || e.StoredPartSHA256 != "" {
				t.Fatal("invented implicit-directory bytes")
			}
		} else if strings.HasPrefix(e.Path, "media/") {
			if e.StoredPartSHA256 != evidence.Hash([]byte(p[e.Path])) {
				t.Fatal("wrong exact target")
			}
		}
	}
}
func TestMimetypeLayout(t *testing.T) {
	for _, tc := range []struct {
		first  string
		method uint16
		extra  []byte
	}{{"META-INF/manifest.xml", zip.Store, nil}, {"mimetype", zip.Deflate, nil}, {"mimetype", zip.Store, []byte{0x34, 0x12, 0, 0}}} {
		b := archive(t, base(), tc.first, tc.method, tc.extra)
		r, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil || r.Format != "" || !has(r, "odt.mimetype_layout_invalid") {
			t.Fatal("layout accepted", err, r)
		}
	}
	p := base()
	p["mimetype"] += "\n"
	r := inspect(t, p)
	if !has(r, "odt.mimetype_unsupported") || r.Format != "" {
		t.Fatal("trimmed mimetype")
	}
}
func TestEncryptedContentNeverParsed(t *testing.T) {
	p := base()
	p["META-INF/manifest.xml"] = manifest(entry("/", MIME, "") + `<m:file-entry m:full-path="content.xml" m:media-type="text/xml" m:size="999999999999"><m:encryption-data><m:algorithm m:algorithm-name="untrusted"/></m:encryption-data></m:file-entry>`)
	// Even plausible XML bytes must not override the manifest's encrypted status.
	r := inspect(t, p)
	if r.Format != "" || r.ContentXML != nil || !has(r, "odt.content_encrypted") || r.Entries[r.ContentEntry].StoredPartSHA256 != evidence.Hash([]byte(p["content.xml"])) {
		t.Fatal("encrypted bytes treated as plaintext", r)
	}
	delete(p, "content.xml")
	r = inspect(t, p)
	if !has(r, "odt.manifest_target_missing") || !has(r, "odt.encrypted_entry_unavailable") {
		t.Fatal("encryption hid missing payload", r.Issues)
	}
}
func TestCoverageGapsSurviveIdentification(t *testing.T) {
	p := base()
	p["unlisted.bin"] = "not accounted for"
	p["secret.bin"] = "ciphertext"
	add(p, `<m:file-entry m:full-path="secret.bin" m:media-type="application/octet-stream"><m:encryption-data/></m:file-entry>`)
	p["META-INF/documentsignatures.xml"] = "unverified signature bytes"
	r := inspect(t, p)
	if r.Format != "odt" || r.State != "partial" || !has(r, "odt.part_unlisted") || !has(r, "odt.encrypted_entry_unavailable") {
		t.Fatal("lost partial coverage", r.Issues)
	}
	for _, m := range r.Memberships {
		if m.Part == "META-INF/documentsignatures.xml" && m.State != "manifest_exempt" {
			t.Fatal("signature trusted or hidden")
		}
	}
}
func TestInvalidManifestAndContent(t *testing.T) {
	cases := []struct {
		name, code string
		change     func(map[string]string)
	}{
		{"mime_missing", "odt.mimetype_missing", func(p map[string]string) { delete(p, "mimetype") }},
		{"manifest_missing", "odt.manifest_missing", func(p map[string]string) { delete(p, "META-INF/manifest.xml") }},
		{"manifest_ns", "odt.manifest_root_mismatch", func(p map[string]string) {
			p["META-INF/manifest.xml"] = strings.ReplaceAll(p["META-INF/manifest.xml"], ManifestNS, "urn:spoof")
		}},
		{"manifest_version", "odt.manifest_version_unsupported", func(p map[string]string) {
			p["META-INF/manifest.xml"] = strings.ReplaceAll(p["META-INF/manifest.xml"], `version="1.3"`, `version="1.4"`)
		}},
		{"duplicate", "odt.manifest_path_ambiguous", func(p map[string]string) { add(p, entry("content.xml", "text/xml", "")) }},
		{"root_mime", "odt.root_identity_mismatch", func(p map[string]string) {
			p["META-INF/manifest.xml"] = strings.ReplaceAll(p["META-INF/manifest.xml"], MIME, "application/xml")
		}},
		{"forbidden", "odt.manifest_entry_forbidden", func(p map[string]string) { add(p, entry("mimetype", MIME, "")) }},
		{"traversal", "odt.manifest_entry_invalid", func(p map[string]string) { add(p, entry("../x", "text/xml", "")) }},
		{"unknown", "odt.manifest_structure_unknown", func(p map[string]string) { add(p, `<m:unknown/>`) }},
		{"dtd", "xml.unsupported", func(p map[string]string) {
			p["META-INF/manifest.xml"] = "<!DOCTYPE manifest>" + p["META-INF/manifest.xml"]
		}},
		{"content_missing", "odt.content_identity_unavailable", func(p map[string]string) { delete(p, "content.xml") }},
		{"content_case", "odt.content_identity_unavailable", func(p map[string]string) { p["Content.xml"] = p["content.xml"]; delete(p, "content.xml") }},
		{"content_ns", "odt.content_root_mismatch", func(p map[string]string) {
			p["content.xml"] = strings.ReplaceAll(p["content.xml"], OfficeNS, "urn:spoof")
		}},
		{"content_version", "odt.content_version_mismatch", func(p map[string]string) {
			p["content.xml"] = strings.ReplaceAll(p["content.xml"], `version="1.3"`, `version="1.2"`)
		}},
		{"mixed_body", "odt.text_body_ambiguous", func(p map[string]string) {
			p["content.xml"] = strings.ReplaceAll(p["content.xml"], `</o:body>`, `<o:spreadsheet/></o:body>`)
		}},
		{"size", "odt.declared_size_mismatch", func(p map[string]string) {
			p["META-INF/manifest.xml"] = strings.ReplaceAll(p["META-INF/manifest.xml"], `m:full-path="content.xml"`, `m:full-path="content.xml" m:size=""`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base()
			tc.change(p)
			r := inspect(t, p)
			want := ""
			if tc.name == "forbidden" || tc.name == "traversal" || tc.name == "unknown" {
				want = "odt"
			}
			if r.Format != want || r.State != "partial" || !has(r, tc.code) {
				t.Fatal("invalid evidence accepted", r.Format, r.Issues)
			}
		})
	}
}
func TestRetainedFixturesAndDeterminism(t *testing.T) {
	for _, name := range []string{"minimal.odt", "hidden.odt", "minimal.docx", "hidden.docx"} {
		b, err := os.ReadFile(filepath.Join("..", "packageparts", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(b)
		r, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil {
			t.Fatal(err)
		}
		again, err := Inspect(context.Background(), b, evidence.Hash(b))
		if err != nil || !reflect.DeepEqual(r, again) || !bytes.Equal(before, b) {
			t.Fatal("nondeterministic/mutated source")
		}
		if strings.HasSuffix(name, ".odt") {
			if r.Format != "odt" || r.State != "completed" {
				t.Fatal(name, r.Issues)
			}
		} else if r.Format != "" || r.State != "not_applicable" {
			t.Fatal("DOCX became ODT")
		}
	}
}
func TestBoundsIdentityAndCancellation(t *testing.T) {
	p := base()
	var entries strings.Builder
	for i := 0; i < maxEntries+1; i++ {
		fmt.Fprintf(&entries, `<m:file-entry m:full-path="part%d" m:media-type=""/>`, i)
	}
	p["META-INF/manifest.xml"] = manifest(entries.String())
	r := inspect(t, p)
	if len(r.Entries) != maxEntries || !has(r, "odt.manifest_entry_limit") {
		t.Fatal("entry limit")
	}
	b := archive(t, base(), "mimetype", zip.Store, nil)
	if r, err := Inspect(context.Background(), b, strings.Repeat("0", 64)); r != nil || err == nil {
		t.Fatal("identity")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Inspect(ctx, b, evidence.Hash(b)); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
}
func FuzzODTManifest(f *testing.F) {
	f.Add(base()["META-INF/manifest.xml"])
	f.Add(`<broken>`)
	f.Fuzz(func(t *testing.T, xml string) {
		if len(xml) > 1<<16 {
			t.Skip()
		}
		p := base()
		p["META-INF/manifest.xml"] = xml
		r := inspect(t, p)
		if r.Format == "odt" {
			if r.RootEntry < 0 || r.ContentEntry < 0 || r.Entries[r.ContentEntry].Encrypted || r.ContentSHA256 != evidence.Hash([]byte(p["content.xml"])) || r.ContentXML == nil {
				t.Fatal("invented ODT identity")
			}
		}
	})
}

func TestLegacyVersionAndUnrelatedManifestGaps(t *testing.T) {
	for _, mode := range []string{"legacy", "root_attribute", "entry_attribute"} {
		p := base()
		switch mode {
		case "legacy":
			p["META-INF/manifest.xml"] = strings.ReplaceAll(p["META-INF/manifest.xml"], ` m:version="1.3"`, ``)
			p["content.xml"] = strings.ReplaceAll(p["content.xml"], `o:version="1.3"`, `o:version="1.1"`)
		case "root_attribute":
			p["META-INF/manifest.xml"] = strings.Replace(p["META-INF/manifest.xml"], `<m:manifest `, `<m:manifest foreign="x" `, 1)
		case "entry_attribute":
			add(p, entry("other.bin", "application/octet-stream", ` foreign="x"`))
		}
		r := inspect(t, p)
		if r.Format != "odt" || r.State != "partial" || r.ContentXML == nil {
			t.Fatal(mode, r.Issues)
		}
		if mode == "legacy" && !has(r, "odt.manifest_version_unsupported") {
			t.Fatal("version caveat lost")
		}
	}
	p := base()
	p["META-INF/manifest.xml"] = strings.Replace(p["META-INF/manifest.xml"], `m:full-path="content.xml"`, `foreign="x" m:full-path="content.xml"`, 1)
	r := inspect(t, p)
	if r.Format != "" || r.ContentXML != nil {
		t.Fatal("unsupported content declaration promoted")
	}
}
func TestMissingTargetIsNotSizeMismatch(t *testing.T) {
	p := base()
	add(p, entry("Pictures/x.png", "image/png", ` m:size="10"`))
	r := inspect(t, p)
	if r.Format != "odt" || !has(r, "odt.manifest_target_missing") || has(r, "odt.declared_size_mismatch") {
		t.Fatal(r.Issues)
	}
	for _, e := range r.Entries {
		if e.Path == "Pictures/x.png" && (e.State != "missing" || e.Code != "odt.manifest_target_missing" || e.DeclaredSize != "10") {
			t.Fatal("missing state lost", e)
		}
	}
}
