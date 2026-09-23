package v4

import (
	"os"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func TestRetainedMetadataNeedsElementCoverageIncludingEmptyValues(t *testing.T) {
	for _, mode := range []string{"valid", "empty values", "value bytes only"} {
		t.Run(mode, func(t *testing.T) {
			raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/metadata-inventory.json")
			if err != nil {
				t.Fatal(err)
			}
			limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
			r, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			for i := range r.Evidence.Office.Packages[0].Outcomes {
				o := &r.Evidence.Office.Packages[0].Outcomes[i]
				if o.Operation != "aletharsis.office.metadata" {
					continue
				}
				switch mode {
				case "empty values":
					o.Assessed = []Span{{0, 1}}
					for j := range r.Evidence.Office.Metadata {
						m := &r.Evidence.Office.Metadata[j]
						m.LexicalValue = ""
						m.ValueOrigins = []Origin{}
					}
				case "value bytes only":
					o.Assessed = nil
					for _, m := range r.Evidence.Office.Metadata {
						if o.PartRef != nil && m.PartRef == *o.PartRef {
							for _, origin := range m.ValueOrigins {
								o.Assessed = append(o.Assessed, origin.Source)
							}
						}
					}
				}
			}
			_, err = r.Encode(limits)
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}

func TestRelationshipsCannotUseEmptyLocations(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/metadata-inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
	r, err := DecodeReport(raw, limits)
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.Evidence.Office.Relationships {
		s := r.Evidence.Office.Relationships[i].Location.Span
		s.End = s.Start
	}
	for i := range r.Evidence.Office.Packages[0].Outcomes {
		o := &r.Evidence.Office.Packages[0].Outcomes[i]
		if o.Operation == "aletharsis.office.relationships" {
			o.Assessed = []Span{{0, 1}}
		}
	}
	if _, err = r.Encode(limits); err == nil {
		t.Fatal("empty relationship anchors bypassed coverage")
	}
}

func TestGeneratedControlRequiresNonemptySource(t *testing.T) {
	for _, span := range []Span{{1, 2}, {1, 1}} {
		doc := XML{PartRef: "office-part/0", Tokens: []Token{{Index: 0, Kind: "start", Element: wide(0), Span: span}, {Index: 1, Kind: "end", Element: wide(0), Span: Span{span.End, span.End}}}, Segments: []Segment{}, Controls: []Control{{Index: 0, Element: 0, Kind: "tab", Count: 1, Source: span}}, Elements: []Element{{Index: 0, Namespace: "urn:oasis:names:tc:opendocument:xmlns:text:1.0", LocalName: "tab", Span: span}}}
		part := Part{PartRef: doc.PartRef, SHA256: str("hash"), ByteLength: wide(3)}
		err := ValidateXML(doc, part)
		if (err == nil) != (span.Start < span.End) {
			t.Fatal(span, err)
		}
	}
}

func TestMetadataCannotEscapeProjectionThroughAnotherRoot(t *testing.T) {
	for _, mode := range []string{"reparent all", "drop last", "one-byte relationship"} {
		t.Run(mode, func(t *testing.T) {
			raw, err := os.ReadFile("../../../tests/contracts_v4/fixtures/metadata-inventory.json")
			if err != nil {
				t.Fatal(err)
			}
			limits := identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64}
			r, err := DecodeReport(raw, limits)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "reparent all", "drop last":
				for i := range r.Evidence.Office.XML {
					doc := &r.Evidence.Office.XML[i]
					for j := range doc.Elements {
						e := &doc.Elements[j]
						if e.Parent != nil && *e.Parent == 0 && (mode == "reparent all" || e.LocalName == "lastModifiedBy") {
							e.Parent = nil
						}
					}
				}
				if mode == "drop last" {
					for i, m := range r.Evidence.Office.Metadata {
						if m.LocalName == "lastModifiedBy" {
							r.Evidence.Office.Metadata = append(r.Evidence.Office.Metadata[:i], r.Evidence.Office.Metadata[i+1:]...)
							break
						}
					}
				}
			case "one-byte relationship":
				for i := range r.Evidence.Office.Relationships {
					s := r.Evidence.Office.Relationships[i].Location.Span
					s.End = s.Start + 1
				}
			}
			if _, err := r.Encode(limits); err == nil {
				t.Fatal("accepted", mode)
			}
		})
	}
}
