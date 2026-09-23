package capability

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"testing"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

func TestOfficeCatalogCoversSpecification(t *testing.T) {
	raw, err := os.ReadFile("../../docs/specs/office-cli-evidence.md")
	if err != nil {
		t.Fatal(err)
	}
	// Discover every Office operation in the accepted contract, rather than
	// asserting only a hand-maintained list of operations in the implementation.
	ids := regexp.MustCompile("`(aletharsis\\.(?:office\\.[a-z_]+|parse\\.office_package))`").FindAllSubmatch(raw, -1)
	if len(ids) == 0 {
		t.Fatal("no Office operations found in spec")
	}
	catalog, err := Office("test", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range catalog {
		if seen[c.ID] {
			t.Fatal("duplicate operation", c.ID)
		}
		seen[c.ID] = true
		if err := c.Validate(); err != nil {
			t.Fatal(c.ID, err)
		}
	}
	for _, m := range ids {
		if !seen[string(m[1])] {
			t.Fatal("missing operation", string(m[1]))
		}
	}
}

func TestOfficeCatalogPreservesFlatAndDisabledDeclarations(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		native, err := forPlatform("test", 8<<20, platform)
		if err != nil {
			t.Fatal(err)
		}
		office, err := officeForPlatform("test", 8<<20, platform)
		if err != nil {
			t.Fatal(err)
		}
		for _, old := range native {
			i := slices.IndexFunc(office, func(c v2.Capability) bool { return c.ID == old.ID })
			if i < 0 {
				t.Fatal("missing legacy operation", old.ID)
			}
			got := office[i]
			switch old.ID {
			case UnicodeInventoryID, EmojiID, PatternsID:
				if !reflect.DeepEqual(got.SupportedScope, v2.SupportedScope{Kinds: []string{"text", "office_scope"}, Formats: []string{"text", "docx", "odt"}}) {
					t.Fatal(got)
				}
				got.Revision = old.Revision
				got.SupportedScope = old.SupportedScope
			}
			if !reflect.DeepEqual(got, old) {
				t.Fatal("unexpected legacy change", old.ID, platform)
			}
		}
	}
}
