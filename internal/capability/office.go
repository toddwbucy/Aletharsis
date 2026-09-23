package capability

import (
	"runtime"
	"sort"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
)

const OfficeCatalogVersion = "aletharsis.native-office/1"

const (
	ParseOfficeID         = "aletharsis.parse.office_package"
	OfficeIdentifyID      = "aletharsis.office.identify"
	OfficeRelationshipsID = "aletharsis.office.relationships"
	OfficeMetadataID      = "aletharsis.office.metadata"
	OfficeObjectsID       = "aletharsis.office.embedded_objects"
	OfficeTextID          = "aletharsis.office.text"
	OfficeProfilesID      = "aletharsis.office.profiles"
)

// Office declares the native 4.0 operations. Operation-specific resource limits
// are attached by the coordinator from its fixed configuration, never from a
// remaining corpus budget. Package/scope/report limits additionally belong in
// the 4.0 package limit record; the frozen v2 descriptor cannot express them all.
// Calling this function neither reads a document nor claims execution coverage.
func Office(version string, maxBytes uint64) ([]v2.Capability, error) {
	return officeForPlatform(version, maxBytes, runtime.GOOS)
}

func officeForPlatform(version string, maxBytes uint64, platform string) ([]v2.Capability, error) {
	catalog, err := forPlatform(version, maxBytes, platform)
	if err != nil {
		return nil, err
	}
	for i := range catalog {
		c := &catalog[i]
		switch c.ID {
		case UnicodeInventoryID, EmojiID, PatternsID:
			c.Revision = "3"
			c.SupportedScope.Kinds = []string{"text", "office_scope"}
			c.SupportedScope.Formats = []string{"text", "docx", "odt"}
		}
	}
	for _, entry := range []struct {
		id             string
		role           v2.Role
		kinds, formats []string
	}{
		{ParseOfficeID, v2.Parser, []string{"source"}, []string{"docx", "odt"}},
		{OfficeIdentifyID, v2.Parser, []string{"package"}, []string{"docx", "odt"}},
		{OfficeRelationshipsID, v2.Parser, []string{"package_part"}, []string{"docx", "odt"}},
		{OfficeMetadataID, v2.Parser, []string{"package_part"}, []string{"docx", "odt"}},
		{OfficeObjectsID, v2.Parser, []string{"package"}, []string{"docx", "odt"}},
		{OfficeTextID, v2.Parser, []string{"package_part"}, []string{"docx", "odt"}},
		{OfficeProfilesID, v2.Analyzer, []string{"office_observation"}, []string{"docx", "odt"}},
	} {
		c := v2.Capability{ID: entry.id, Revision: "1", Role: entry.role, Participation: v2.Required,
			Implementation: &v2.Implementation{ID: entry.id, Version: version, Data: []v2.DataIdentity{}},
			Availability:   v2.Availability{State: "available"},
			SupportedScope: v2.SupportedScope{Kinds: entry.kinds, Formats: entry.formats}}
		// Revision 2 also retains declared OOXML evidence inside ODT hybrids.
		// This is not native ODF metadata/object support; the coordinator must
		// declare restricted hybrid coverage rather than a whole-source scan.
		if c.ID == OfficeRelationshipsID || c.ID == OfficeMetadataID || c.ID == OfficeObjectsID {
			c.Revision = "2"
		}
		if c.Role == v2.Analyzer {
			c.Mechanism = ptr(v2.Structural)
		}
		if c.ID == ParseOfficeID {
			c.Limits.InputBytes = ptr(maxBytes)
		}
		catalog = append(catalog, c)
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].ID < catalog[j].ID })
	for _, c := range catalog {
		if err := c.Validate(); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}
