package v4

import (
	"errors"
	"strconv"
	"strings"
)

var ErrLinkage = errors.New("Office evidence linkage is invalid")

type Index struct {
	Packages map[string]Package
	Parts    map[string]Part
	XML      map[string]XML
	Scopes   map[string]Scope
}

func canonicalRef(s, kind string) bool {
	prefix := "office-" + kind + "/"
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	tail := strings.TrimPrefix(s, prefix)
	n, err := strconv.ParseUint(tail, 10, 53)
	return err == nil && strconv.FormatUint(n, 10) == tail
}

// IndexEvidence validates package identities, XML maps and scopes before
// returning reference tables. This is one stage of report validation; callers
// must additionally check metadata, relationships, objects and execution links.
func IndexEvidence(e Evidence, sourceHash string, sourceSize int64) (*Index, error) {
	x := &Index{Packages: map[string]Package{}, Parts: map[string]Part{}, XML: map[string]XML{}, Scopes: map[string]Scope{}}
	for _, p := range e.Packages {
		if !canonicalRef(p.PackageRef, "package") || p.SourceSHA256 != sourceHash || p.SourceByteLength != sourceSize {
			return nil, ErrLinkage
		}
		if _, exists := x.Packages[p.PackageRef]; exists {
			return nil, ErrLinkage
		}
		x.Packages[p.PackageRef] = p
		names := map[string]bool{}
		for _, part := range p.Parts {
			if !canonicalRef(part.PartRef, "part") || part.PackageRef != p.PackageRef || names[part.Name] ||
				!part.CompressedSpan.Within(sourceSize) || (part.SHA256 == nil) != (part.ByteLength == nil) {
				return nil, ErrLinkage
			}
			if _, exists := x.Parts[part.PartRef]; exists {
				return nil, ErrLinkage
			}
			if part.State == "completed" && part.SHA256 == nil {
				return nil, ErrLinkage
			}
			if (part.State == "failed" || part.State == "not_run" || part.State == "canceled") && part.SHA256 != nil {
				return nil, ErrLinkage
			}
			names[part.Name] = true
			x.Parts[part.PartRef] = part
		}
	}
	for _, doc := range e.XML {
		part, ok := x.Parts[doc.PartRef]
		if !ok || !canonicalRef(doc.XMLRef, "xml") {
			return nil, ErrLinkage
		}
		if _, exists := x.XML[doc.XMLRef]; exists {
			return nil, ErrLinkage
		}
		if err := ValidateXML(doc, part); err != nil {
			return nil, err
		}
		x.XML[doc.XMLRef] = doc
	}
	identities := map[string]bool{}
	for _, scope := range e.Scopes {
		part, ok := x.Parts[scope.PartRef]
		if !ok || !canonicalRef(scope.ScopeRef, "scope") || identities[scope.IdentitySHA256] {
			return nil, ErrLinkage
		}
		if _, exists := x.Scopes[scope.ScopeRef]; exists {
			return nil, ErrLinkage
		}
		if err := ValidateScope(scope, part, sourceHash, x.XML); err != nil {
			return nil, err
		}
		identities[scope.IdentitySHA256] = true
		x.Scopes[scope.ScopeRef] = scope
	}
	return x, nil
}
