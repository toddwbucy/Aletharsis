package officeplan

import (
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

// MetadataRecords preserves original property order and duplicate ordinals.
// Value offsets address the complete decoded property value; source offsets
// continue to address original XML bytes, including references and CDATA.
func MetadataRecords(a *officemetadata.Result, part v4.Part, x v4.XML) ([]v4.Metadata, error) {
	if err := v4.ValidateXML(x, part); err != nil {
		return nil, err
	}
	return metadataRecords(a, part, x)
}

func metadataRecords(a *officemetadata.Result, part v4.Part, x v4.XML) ([]v4.Metadata, error) {
	if a == nil || a.XML == nil || a.XML.Document == nil || part.SHA256 == nil || a.XML.Document.PartSHA256 != *part.SHA256 {
		return nil, v4.ErrLinkage
	}
	records := []v4.Metadata{}
	for i, p := range a.Properties {
		m := v4.Metadata{Namespace: p.Name.Namespace, LocalName: p.Name.Local, LexicalValue: p.Value, PartRef: part.PartRef, XMLRef: x.XMLRef, Element: int64(p.Element), DuplicateOrdinal: int64(p.Occurrence), ValueOrigins: []v4.Origin{}}
		if p.Key != "" {
			key := p.Key
			m.NormalizedKey = &key
		}
		offset := int64(0)
		for _, si := range p.Segments {
			if si < 0 || si >= len(x.Segments) {
				return nil, v4.ErrCoordinates
			}
			segment := x.Segments[si]
			for _, scalar := range segment.Scalars {
				text, segmentIndex, scalarIndex := i, si, scalar.Index
				m.ValueOrigins = append(m.ValueOrigins, v4.Origin{Kind: "stored", XMLRef: x.XMLRef, TextIndex: &text, Segment: &segmentIndex, Scalar: &scalarIndex, Source: scalar.Source, UTF8: v4.Span{Start: offset + scalar.UTF8.Start, End: offset + scalar.UTF8.End}, Transformation: scalar.Transformation})
			}
			offset += int64(len(segment.Text))
		}
		records = append(records, m)
	}
	index := &v4.Index{XML: map[string]v4.XML{x.XMLRef: x}}
	if err := index.ValidateInventories(v4.Evidence{Metadata: records}); err != nil {
		return nil, err
	}
	return records, nil
}
