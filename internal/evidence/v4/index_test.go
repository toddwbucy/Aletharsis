package v4

import "testing"

func TestPackageIdentityAndPartialPartRules(t *testing.T) {
	hash := "part"
	size := int64(3)
	part := Part{PartRef: "office-part/0", PackageRef: "office-package/0", Name: "word/document.xml", CompressedSpan: Span{10, 20}, SHA256: &hash, ByteLength: &size, State: "completed"}
	pkg := Package{PackageRef: "office-package/0", SourceSHA256: "source", SourceByteLength: 100, State: "partial", Parts: []Part{part,
		{PartRef: "office-part/1", PackageRef: "office-package/0", Name: "bad.bin", CompressedSpan: Span{20, 30}, State: "failed"}}}
	e := Evidence{Packages: []Package{pkg}}
	if _, err := IndexEvidence(e, "source", 100); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Evidence){
		func(v *Evidence) { v.Packages[0].SourceSHA256 = "wrong" },
		func(v *Evidence) { v.Packages[0].Parts[0].PackageRef = "office-package/1" },
		func(v *Evidence) { v.Packages[0].Parts[0].CompressedSpan.End = 101 },
		func(v *Evidence) { v.Packages[0].Parts[0].PartRef = "office-part/00" },
		func(v *Evidence) { v.Packages[0].Parts[1].Name = part.Name },
		func(v *Evidence) { v.Packages[0].Parts[1].PartRef = part.PartRef },
		func(v *Evidence) { v.Packages[0].Parts[1].SHA256 = &hash },
		func(v *Evidence) { v.Packages[0].Parts[1].SHA256 = &hash; v.Packages[0].Parts[1].ByteLength = &size },
		func(v *Evidence) { v.Packages = append(v.Packages, v.Packages[0]) },
	} {
		bad := e
		bad.Packages = append([]Package{}, e.Packages...)
		bad.Packages[0].Parts = append([]Part{}, pkg.Parts...)
		mutate(&bad)
		if _, err := IndexEvidence(bad, "source", 100); err == nil {
			t.Fatal("accepted damaged package identity")
		}
	}
}
func TestCanonicalOfficeReferences(t *testing.T) {
	for _, kind := range []string{"package", "part", "scope", "xml", "object", "relationship"} {
		if !canonicalRef("office-"+kind+"/0", kind) {
			t.Fatal(kind)
		}
		for _, suffix := range []string{"00", "01", "-1", "+1", "1\n", "1.0", "", "9007199254740992"} {
			if canonicalRef("office-"+kind+"/"+suffix, kind) {
				t.Fatalf("accepted %s %q", kind, suffix)
			}
		}
	}
}
