package wire

import "testing"

func TestEnvelopeSchemasCompileOffline(t *testing.T) {
	contracts, err := envelopeContracts()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"corpus-v2", "corpus-document-v2", "reveal-tree-v2"} {
		if contracts[name] == nil {
			t.Fatal(name)
		}
	}
}
