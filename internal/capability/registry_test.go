package capability

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
)

func TestCatalogDeclarations(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			catalog, err := forPlatform("0.2.0", 8<<20, platform)
			if err != nil {
				t.Fatal(err)
			}
			if len(catalog) != 11 {
				t.Fatal("catalog size changed")
			}
			ids := []string{}
			native, declarations := 0, 0
			for _, c := range catalog {
				ids = append(ids, c.ID)
				raw, err := json.Marshal(c)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := v2.DecodeCapability(raw)
				if err != nil || !reflect.DeepEqual(c, decoded) {
					t.Fatal("invalid descriptor", err)
				}
				if c.Implementation != nil {
					native++
					if c.Participation != v2.Required {
						t.Fatal("optional native operation")
					}
				} else {
					declarations++
					if c.Participation != v2.Disabled || c.Availability.State != "unavailable" {
						t.Fatal("unimplemented detector may run")
					}
					if len(c.SupportedScope.Kinds) != 0 || len(c.SupportedScope.Formats) != 0 {
						t.Fatal("declaration invented support")
					}
					if c.ID == "anthropic.claude_text_watermark" && (*c.Mechanism != v2.Statistical || *c.Availability.ReasonCode != failure.DetectorAccessUnavailable || *c.Purpose != "watermark_detection") {
						t.Fatal(c)
					}
				}
				if c.ID == "aletharsis.acquire" {
					want := "available"
					if platform != "linux" {
						want = "unavailable"
					}
					if c.Availability.State != want {
						t.Fatal("incorrect platform availability")
					}
				}
			}
			if native != 8 || declarations != 3 || !sort.StringsAreSorted(ids) {
				t.Fatal(native, declarations, ids)
			}
		})
	}
}
func TestNoAmbientDiscoveryOrSharedState(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	first, err := Native("0.2.0", 32)
	if err != nil {
		t.Fatal(err)
	}
	first[0].ID = "tampered"
	*first[0].Limits.InputBytes = 999
	first[0].Implementation.ID = "tampered"
	first[0].SupportedScope.Formats[0] = "tampered"
	again, err := Native("0.2.0", 32)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != "aletharsis.acquire" || *again[0].Limits.InputBytes != 32 || again[0].Implementation.ID != "aletharsis.acquire" || again[0].SupportedScope.Formats[0] != "*" {
		t.Fatal("registry shared mutable storage")
	}
}
func TestNonExecutionPrecedence(t *testing.T) {
	catalog, err := forPlatform("0.2.0", 32, "linux")
	if err != nil {
		t.Fatal(err)
	}
	native := catalog[0]
	var declared v2.Capability
	for _, c := range catalog {
		if c.ID == "anthropic.claude_text_watermark" {
			declared = c
		}
	}
	for _, tc := range []struct {
		name                      string
		c                         v2.Capability
		prereq, supported, policy bool
		want                      failure.Code
	}{
		{"disabled", declared, false, false, false, failure.Disabled},
		{"prerequisite", native, false, false, false, failure.PrerequisiteFailed},
		{"unsupported", native, true, false, false, failure.UnsupportedInput},
		{"policy", native, true, true, false, failure.PolicyDenied},
		{"eligible", native, true, true, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, err := NonExecutionReason(tc.c, tc.prereq, tc.supported, tc.policy)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if reason != nil {
					t.Fatal(reason)
				}
			} else if reason == nil || *reason != tc.want {
				t.Fatal(reason, tc.want)
			}
		})
	}
	declared.Participation = v2.Required
	reason, err := NonExecutionReason(declared, false, false, false)
	if err != nil || reason == nil || *reason != failure.DetectorAccessUnavailable {
		t.Fatal("availability must precede prerequisite", reason, err)
	}
	*reason = failure.Disabled
	if *declared.Availability.ReasonCode != failure.DetectorAccessUnavailable {
		t.Fatal("returned reason aliases descriptor")
	}
}
func TestBadCatalogConfiguration(t *testing.T) {
	if _, err := Native("", 32); err == nil {
		t.Fatal("missing implementation version")
	}
	if _, err := Native("0.2.0", 0); err == nil {
		t.Fatal("missing input bound")
	}
	if _, err := NonExecutionReason(v2.Capability{}, true, true, true); err == nil {
		t.Fatal("invalid descriptor accepted")
	}
}

func TestNativeDataManifestIsBoundAndIndependent(t *testing.T) {
	first, err := Native("test", 128)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := NativeDataRevision()
	if err != nil || revision == "" {
		t.Fatal(err)
	}
	for _, c := range first {
		if c.Implementation != nil && len(c.Implementation.Data) != 4 {
			t.Fatal("compiled data identity missing")
		}
	}
	first[0].Implementation.Data[0].SHA256 = "tampered"
	var parser *v2.Implementation
	for _, c := range first {
		if c.ID == ParseTextID {
			parser = c.Implementation
		}
	}
	if parser == nil {
		t.Fatal("parser capability missing")
	}
	if parser.Data[0].SHA256 == "tampered" {
		t.Fatal("descriptors share data slices")
	}
	second, err := Native("test", 128)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Implementation.Data[0].SHA256 == "tampered" {
		t.Fatal("catalog shares data")
	}
	again, err := NativeDataRevision()
	if err != nil || again != revision {
		t.Fatal("data revision is unstable", err)
	}
}
