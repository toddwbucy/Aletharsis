package wire

import (
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"testing"
)

func TestOfficeContractCompilesWithoutExternalResources(t *testing.T) {
	contracts, err := compiled()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0", "2.0", "4.0"} {
		if contracts[version] == nil {
			t.Fatalf("missing compiled %s", version)
		}
	}
	if contracts["3.0"] != nil {
		t.Fatal("3.0 runtime support must not be inferred from 4.0")
	}
	limits := identity.Limits{InputBytes: 1024, OutputBytes: 1024, Nodes: 1024, Depth: 64}
	if _, err := Validate("4.0", []byte(`{"schema_version":"4.0"}`), limits); err != ErrSchema {
		t.Fatalf("incomplete Office report: got %v, want schema rejection", err)
	}
}
