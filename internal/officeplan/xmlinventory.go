package officeplan

import (
	"context"

	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

type XMLFailure struct {
	Part  string
	Error error
}

// XMLInventory records only parsed evidence, never guesses XML from a filename.
// Failures must become coverage gaps before a host can publish the report.
type XMLInventory struct {
	Documents   []v4.XML
	ByPart      map[string]int
	ControlRefs map[string]map[int]int
	Failures    []XMLFailure
}

func CollectXML(ctx context.Context, p *Prepared, a *Analysis, base *PackageRecords) (*XMLInventory, error) {
	if ctx == nil || p == nil || p.Admission == nil || a == nil || base == nil || len(base.Evidence.Packages) != 1 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if base.Evidence.Packages[0].SourceSHA256 != p.Admission.Outcomes.SourceSHA256 {
		return nil, v4.ErrLinkage
	}
	docs := map[string]*xmlparts.Document{}
	mapped := map[string]*xmlparts.MappedDocument{}
	odt := map[string]*odtanalysis.Result{}
	add := func(name string, doc *xmlparts.Document) {
		if doc != nil {
			docs[name] = doc
		}
	}
	if p.DOCX != nil {
		add(p.DOCX.TypesPart, p.DOCX.TypesXML)
		add(p.DOCX.MainPart, p.DOCX.MainXML)
		if p.DOCX.OPC != nil {
			for _, part := range p.DOCX.OPC.Parts {
				add(part.Part, part.XML)
			}
		}
	}
	if p.ODT != nil {
		add("META-INF/manifest.xml", p.ODT.ManifestXML)
		add("content.xml", p.ODT.ContentXML)
	}
	for _, part := range a.Parts {
		var m *xmlparts.MappedDocument
		if part.Word != nil {
			if part.Word.Extraction == nil {
				return nil, v4.ErrLinkage
			}
			m = part.Word.Extraction.XML
		}
		if part.ODT != nil {
			if part.ODT.Extraction == nil {
				return nil, v4.ErrLinkage
			}
			if m != nil {
				return nil, v4.ErrLinkage
			}
			m = part.ODT.Extraction.XML
			odt[part.Part] = part.ODT
		}
		if part.Metadata != nil {
			if m != nil {
				return nil, v4.ErrLinkage
			}
			m = part.Metadata.XML
		}
		if m != nil {
			if mapped[part.Part] != nil || m.Document == nil {
				return nil, v4.ErrLinkage
			}
			mapped[part.Part] = m
			docs[part.Part] = m.Document
		}
	}
	raw := map[string][]byte{}
	for _, part := range p.Admission.Outcomes.Parts {
		raw[part.Part.Name] = part.Part.Bytes
	}
	result := &XMLInventory{Documents: []v4.XML{}, ByPart: map[string]int{}, ControlRefs: map[string]map[int]int{}, Failures: []XMLFailure{}}
	visited := map[string]bool{}
	for ordinal, part := range base.Evidence.Packages[0].Parts {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		doc := docs[part.Name]
		if doc == nil {
			continue
		}
		visited[part.Name] = true
		m := mapped[part.Name]
		if m == nil {
			var err error
			m, err = xmlparts.MapParsed(ctx, raw[part.Name], doc, xmlparts.MaxScalarMappings)
			if err != nil {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				result.Failures = append(result.Failures, XMLFailure{part.Name, err})
				continue
			}
		}
		x, err := buildXMLRecord(m, part, ordinal)
		if err != nil {
			return nil, err
		}
		if analysis := odt[part.Name]; analysis != nil {
			controls, refs, err := ODTControlsForScopes(analysis)
			if err != nil {
				return nil, err
			}
			x.Controls = controls
			result.ControlRefs[part.Name] = refs
		}
		if err := v4.ValidateXML(x, part); err != nil {
			return nil, err
		}
		result.ByPart[part.Name] = len(result.Documents)
		result.Documents = append(result.Documents, x)
	}
	for name := range docs {
		if !visited[name] {
			return nil, v4.ErrLinkage
		}
	}
	return result, nil
}
