package parsers_test

import (
	"archive/zip"
	"bytes"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
	"strings"
	"testing"
)

func TestIdentificationEvidenceOverExtension(t *testing.T) {
	for _, tc := range []struct{ name, data, ext, format, mime, basis string }{
		{"disguised_pdf", " \r\n%PDF-1.7", ".txt", "pdf", "application/pdf", "PDF signature"},
		{"plain_named_pdf", "ordinary prose", ".pdf", "text", "text/plain", "valid Unicode text; extension is a MIME hint"},
		{"source_code", "print('hello')", ".py", "text", "text/plain", "valid Unicode text; extension is a MIME hint"},
		{"bad_text", "\xff", ".txt", "text", "text/plain", "text extension; decoding requires validation"},
		{"bad_unknown", "\xff", ".dat", "binary", "application/octet-stream", "not valid supported Unicode text"},
		{"html", "\ufeff \n<!DOCTYPE HTML>", ".txt", "text", "text/html", "HTML signature; source text inspection"},
		{"xml", " <?xml version=\"1.0\"?>", ".txt", "text", "application/xml", "XML declaration; source text inspection"},
		{"density_below", "\x00" + strings.Repeat("a", 10), ".txt", "text", "text/plain", "valid Unicode text; extension is a MIME hint"},
		{"density_at", "\x00" + strings.Repeat("a", 9), ".txt", "text", "text/plain", "valid Unicode text; extension is a MIME hint"},
		{"density_above", "\x00" + strings.Repeat("a", 8), ".txt", "binary", "application/octet-stream", "high binary-control density"},
		{"bom_controls", "\ufeff\x00", ".txt", "text", "text/plain", "valid Unicode text; extension is a MIME hint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			format, mime, basis := parsers.Identify([]byte(tc.data), tc.ext)
			if format != tc.format || mime != tc.mime || basis != tc.basis {
				t.Fatalf("got %q %q %q", format, mime, basis)
			}
		})
	}
	for _, sig := range []string{"\x89PNG", "\xff\xd8\xff", "GIF87a", "GIF89a", "\x7fELF", "\x1f\x8b", "\xd0\xcf\x11\xe0"} {
		format, _, basis := parsers.Identify([]byte(sig), ".txt")
		if format != "binary" || basis != "binary signature" {
			t.Fatalf("missed signature %x", sig)
		}
	}
	for _, sig := range []string{"PK\x03\x04", "PK\x05\x06", "PK\x07\x08"} {
		format, _, basis := parsers.Identify([]byte(sig), ".docx")
		if format != "zip" || basis != "ZIP signature" {
			t.Fatalf("invalid ZIP labeled DOCX: %x", sig)
		}
	}
	for _, ext := range []string{".txt", ".md", ".rst", ".csv", ".json", ".xml", ".html", ".htm"} {
		expected := map[string]string{".txt": "text/plain", ".md": "text/markdown", ".rst": "text/x-rst", ".csv": "text/csv", ".json": "application/json", ".xml": "application/xml", ".html": "text/html", ".htm": "text/html"}
		format, mime, _ := parsers.Identify([]byte("ordinary text"), ext)
		if format != "text" || mime != expected[ext] {
			t.Fatalf("MIME hint %s: %s %s", ext, format, mime)
		}
	}
}

func TestOOXMLIdentificationRequiresBothParts(t *testing.T) {
	for _, parts := range [][]string{{}, {"word/document.xml"}, {"[Content_Types].xml"}, {"[Content_Types].xml", "word/document.xml"}} {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		for _, part := range parts {
			w, err := archive.Create(part)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte("<root/>")); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		format, mime, _ := parsers.Identify(buf.Bytes(), ".txt")
		want := "zip"
		if len(parts) == 2 {
			want = "docx"
		}
		if format != want {
			t.Fatalf("parts %v: %s", parts, format)
		}
		if want == "docx" && mime != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			t.Fatal(mime)
		}
	}
}
