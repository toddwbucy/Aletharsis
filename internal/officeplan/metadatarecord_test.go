package officeplan

import (
	"context"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
)

func TestMetadataRecordsPreserveValuesAndOriginalOrdinals(t *testing.T) {
	source := []byte(`<cp:coreProperties xmlns:cp="` + officemetadata.CoreNamespace + `" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator><unknown/></dc:creator><dc:creator>A&#x200B;<![CDATA[😀]]>&amp;</dc:creator><dc:creator/><dc:identifier>identifier</dc:identifier></cp:coreProperties>`)
	hash := evidence.Hash(source)
	size := int64(len(source))
	part := v4.Part{PartRef: "office-part/0", SHA256: &hash, ByteLength: &size}
	a, err := officemetadata.Extract(context.Background(), source, hash)
	if err != nil {
		t.Fatal(err)
	}
	x, err := BuildXMLRecord(a.XML, part, 0)
	if err != nil {
		t.Fatal(err)
	}
	records, err := MetadataRecords(a, part, x)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[0].DuplicateOrdinal != 1 || records[1].DuplicateOrdinal != 2 || records[2].DuplicateOrdinal != 0 {
		t.Fatal("original duplicate positions lost", records)
	}
	first := records[0]
	if first.LexicalValue != "A\u200b😀&" || *first.NormalizedKey != "creator" || len(first.ValueOrigins) != 4 {
		t.Fatal(first)
	}
	if records[1].LexicalValue != "" || records[1].ValueOrigins == nil || len(records[1].ValueOrigins) != 0 {
		t.Fatal("empty property confused with absent property")
	}
	for i, o := range first.ValueOrigins {
		if *o.TextIndex != 0 || *o.Scalar < 0 || *o.Segment < 0 {
			t.Fatal("bad value origin")
		}
		if i > 0 && o.UTF8.Start != first.ValueOrigins[i-1].UTF8.End {
			t.Fatal("segment-relative offset leaked")
		}
	}
	again, err := MetadataRecords(a, part, x)
	if err != nil || !reflect.DeepEqual(records, again) {
		t.Fatal("nondeterministic metadata")
	}
	a.Properties[0].Value = "changed"
	if _, err := MetadataRecords(a, part, x); err == nil {
		t.Fatal("value/map mismatch accepted")
	}
}
