package v4

import (
	"archive/zip"
	"context"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/officemetadata"
	"io"
	"os"
	"testing"
)

func TestInventoryFixtureAgainstMetadataProducer(t *testing.T) {
	const directory = "../../../tests/contracts_v4/fixtures/"
	raw, err := os.ReadFile(directory + "metadata-inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeReport(raw, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Evidence.Office.Metadata) != 5 || len(r.Evidence.Office.Relationships) < 3 || len(r.Evidence.Office.Objects) != 1 {
		t.Fatal("fixture inventories absent")
	}
	z, err := zip.OpenReader(directory + "metadata-inventory.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name != "docProps/core.xml" && f.Name != "docProps/app.xml" {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(io.LimitReader(reader, 1<<20))
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		native, err := officemetadata.Extract(context.Background(), b, evidence.Hash(b))
		if err != nil {
			t.Fatal(err)
		}
		partRef := ""
		for _, part := range r.Evidence.Office.Packages[0].Parts {
			if part.Name == f.Name {
				partRef = part.PartRef
			}
		}
		found := 0
		for _, m := range r.Evidence.Office.Metadata {
			if m.PartRef != partRef {
				continue
			}
			if found >= len(native.Properties) {
				t.Fatal("extra property")
			}
			p := native.Properties[found]
			if m.LexicalValue != p.Value || m.Element != int64(p.Element) || m.LocalName != p.Name.Local {
				t.Fatal("metadata differs from producer")
			}
			for _, o := range m.ValueOrigins {
				if o.TextIndex == nil || *o.TextIndex != found {
					t.Fatal("wrong producer ordinal")
				}
			}
			found++
		}
		if found != len(native.Properties) {
			t.Fatal("missing property")
		}
	}
}
