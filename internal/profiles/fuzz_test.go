package profiles

import (
	"bytes"
	"os"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func FuzzProfileDefinition(f *testing.F) {
	raw, err := os.ReadFile("testdata/text.json")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte(`{"id":"a","id":"b"}`))
	f.Add([]byte{0xff, 0, 0xfe})
	f.Fuzz(func(t *testing.T, raw []byte) {
		r := &Registry{}
		id, err := r.Register(raw)
		if err != nil {
			return
		}
		retained, err := r.Artifact(id.ID, id.Version)
		if err != nil || !bytes.Equal(raw, retained) || id.ArtifactSHA256 != identity.ExactBytes(raw) {
			t.Fatal("successful load lost original identity", err)
		}
		again, err := r.Register(retained)
		if err != nil || again != id {
			t.Fatal("successful profile cannot be replayed", err)
		}
	})
}
