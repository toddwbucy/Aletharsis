package parsers

import (
	"archive/zip"
	"bytes"
	"strings"
)

var TextTypes = map[string]string{".txt": "text/plain", ".md": "text/markdown", ".rst": "text/x-rst", ".csv": "text/csv", ".json": "application/json", ".xml": "application/xml", ".html": "text/html", ".htm": "text/html"}

func Identify(data []byte, extension string) (string, string, string) {
	if bytes.HasPrefix(bytes.TrimLeft(data[:min(1024, len(data))], " \t\r\n\v\f"), []byte("%PDF-")) {
		return "pdf", "application/pdf", "PDF signature"
	}
	for _, sig := range []string{"PK\x03\x04", "PK\x05\x06", "PK\x07\x08"} {
		if bytes.HasPrefix(data, []byte(sig)) {
			if z, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
				parts := map[string]bool{}
				for _, f := range z.File {
					parts[f.Name] = true
				}
				if parts["[Content_Types].xml"] && parts["word/document.xml"] {
					return "docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "OOXML archive members"
				}
			}
			return "zip", "application/zip", "ZIP signature"
		}
	}
	for _, sig := range []string{"\x89PNG", "\xff\xd8\xff", "GIF87a", "GIF89a", "\x7fELF", "\x1f\x8b", "\xd0\xcf\x11\xe0"} {
		if bytes.HasPrefix(data, []byte(sig)) {
			return "binary", "application/octet-stream", "binary signature"
		}
	}
	encoding, bom := Encoding(data)
	s, _, err := Decode(data, encoding)
	mime, known := TextTypes[extension]
	if err != nil {
		if known {
			return "text", mime, "text extension; decoding requires validation"
		}
		return "binary", "application/octet-stream", "not valid supported Unicode text"
	}
	controls, count := 0, 0
	for _, r := range s {
		count++
		if r < 32 && r != '\t' && r != '\r' && r != '\n' {
			controls++
		}
	}
	if bom == nil && count > 0 && float64(controls)/float64(count) > .1 {
		return "binary", "application/octet-stream", "high binary-control density"
	}
	head := strings.ToLower(strings.TrimLeft(s, "\ufeff \t\r\n"))
	if strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html") {
		return "text", "text/html", "HTML signature; source text inspection"
	}
	if strings.HasPrefix(head, "<?xml") {
		return "text", "application/xml", "XML declaration; source text inspection"
	}
	if !known {
		mime = "text/plain"
	}
	return "text", mime, "valid Unicode text; extension is a MIME hint"
}
