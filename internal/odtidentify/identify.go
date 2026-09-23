// Package odtidentify identifies ODT from its own package and manifest evidence.
// It performs no decryption, URI resolution, rendering or source-file access.
package odtidentify

import (
	"archive/zip"
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"github.com/toddwbucy/Aletharsis/internal/xmlparts"
)

const Version = "odt-identification/1"
const MIME = "application/vnd.oasis.opendocument.text"
const ManifestNS = "urn:oasis:names:tc:opendocument:xmlns:manifest:1.0"
const OfficeNS = "urn:oasis:names:tc:opendocument:xmlns:office:1.0"
const maxEntries = 4096

type Anchor struct {
	Part, PartSHA256 string
	Element          int
	Span             xmlparts.Span
}
type Entry struct {
	Namespace, Kind                        string
	SizeDeclared                           bool
	Path, MediaType, Version, DeclaredSize string
	Encrypted                              bool
	State, Code, StoredPartSHA256          string
	Anchor                                 Anchor
}
type Membership struct {
	// Manifest membership and payload admission are independent facts.
	AdmissionState, AdmissionCode string
	Part, PartSHA256, State, Code string
	Entry                         int
}
type Issue struct {
	Code, Part string
	Element    int
}
type Result struct {
	// Borrowed admission view: Parts[i].Bytes aliases reader payloads (staged)
	// or Package.Parts[i].Bytes (strict). Callers must not mutate the bytes.
	Outcomes                                      *packageparts.OutcomeView
	Parser, State, Format, ManifestVersion        string
	Package                                       *packageparts.Package
	ManifestSHA256, MimetypeSHA256, ContentSHA256 string
	ManifestXML, ContentXML                       *xmlparts.Document
	Entries                                       []Entry
	Memberships                                   []Membership
	Issues                                        []Issue
	RootEntry, ContentEntry                       int
}

func (r *Result) issue(code, part string, element int) {
	r.State = "partial"
	r.Issues = append(r.Issues, Issue{code, part, element})
}
func parseCode(err error) string {
	switch {
	case errors.Is(err, xmlparts.ErrLimit):
		return "xml.resource_limit"
	case errors.Is(err, xmlparts.ErrUnsupported):
		return "xml.unsupported"
	default:
		return "xml.invalid"
	}
}
func attribute(e xmlparts.Element, ns, name string) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name.Namespace == ns && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}
func knownVersion(v string) bool { return v == "1.2" || v == "1.3" }
func validPath(p string) bool {
	if p == "/" {
		return true
	}
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00") {
		return false
	}
	for _, c := range p {
		if c < 32 || c == 127 {
			return false
		}
	}
	for _, s := range strings.Split(strings.TrimSuffix(p, "/"), "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}

// Inspect uses exact case-sensitive ZIP names for ODF manifest membership. It
// never applies OPC URI equivalence, case folding, or percent decoding.
func Inspect(ctx context.Context, source []byte, expectedSHA256 string) (*Result, error) {
	pkg, err := packageparts.Read(ctx, source, expectedSHA256, packageparts.DefaultLimits())
	if err != nil {
		return nil, err
	}
	view := packageparts.CompletedView(pkg)
	return inspectView(ctx, pkg, &view)
}

// InspectVerified checks already admitted package bytes and preserves unavailable
// prerequisite identities. No archive reading or decompression occurs here.
func InspectVerified(ctx context.Context, reader *packageparts.OutcomeReader) (*Result, error) {
	view, err := reader.InspectionView(ctx)
	if err != nil {
		return nil, err
	}
	return inspectView(ctx, nil, &view)
}

func inspectView(ctx context.Context, pkg *packageparts.Package, view *packageparts.OutcomeView) (*Result, error) {
	r := &Result{Parser: Version, State: "not_applicable", Package: pkg, Outcomes: view, Entries: []Entry{}, Memberships: []Membership{}, Issues: []Issue{}, RootEntry: -1, ContentEntry: -1}
	parts := map[string]int{}
	for i, p := range view.Parts {
		if !p.Directory {
			parts[p.Name] = i
		}
	}
	mi, hasMIME := parts["mimetype"]
	fi, hasManifest := parts["META-INF/manifest.xml"]
	if !hasMIME && !hasManifest {
		return r, nil
	}
	r.State = "completed"
	if !hasMIME {
		r.issue("odt.mimetype_missing", "mimetype", -1)
		return r, nil
	}
	mt := view.Parts[mi]
	if mt.State != "completed" {
		r.issue("odt.mimetype_unavailable", mt.Name, -1)
		if mt.Code != "" {
			r.issue(mt.Code, mt.Name, -1)
		}
		return r, nil
	}
	r.MimetypeSHA256 = mt.SHA256
	if string(mt.Bytes) != MIME {
		r.issue("odt.mimetype_unsupported", "mimetype", -1)
		return r, nil
	}
	// With DP-001's contiguous ZIP records, offset 38 proves the first local
	// header has the eight-byte mimetype name and no local extra field.
	if mt.Method != zip.Store || mt.CompressedSpan.Start != 38 {
		r.issue("odt.mimetype_layout_invalid", "mimetype", -1)
		return r, nil
	}
	if !hasManifest {
		r.issue("odt.manifest_missing", "META-INF/manifest.xml", -1)
		return r, nil
	}
	manifest := view.Parts[fi]
	if manifest.State != "completed" {
		r.issue("odt.manifest_unavailable", manifest.Name, -1)
		if manifest.Code != "" {
			r.issue(manifest.Code, manifest.Name, -1)
		}
		return r, nil
	}
	r.ManifestSHA256 = manifest.SHA256
	doc, err := xmlparts.Parse(ctx, manifest.Bytes, manifest.SHA256, xmlparts.DefaultLimits())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r.issue(parseCode(err), manifest.Name, -1)
		return r, nil
	}
	r.ManifestXML = doc
	if !r.readManifest() {
		return r, nil
	}
	r.membership(parts)
	if r.RootEntry < 0 {
		r.issue("odt.root_entry_missing", manifest.Name, -1)
		return r, nil
	}
	root := r.Entries[r.RootEntry]
	if root.State != "package" || root.MediaType != MIME {
		r.issue("odt.root_identity_mismatch", manifest.Name, root.Anchor.Element)
		return r, nil
	}
	if root.Version != "" && r.ManifestVersion != "" && root.Version != r.ManifestVersion {
		r.issue("odt.root_version_mismatch", manifest.Name, root.Anchor.Element)
		return r, nil
	}
	if r.ContentEntry < 0 {
		r.issue("odt.content_entry_missing", manifest.Name, -1)
		return r, nil
	}
	contentEntry := r.Entries[r.ContentEntry]
	if contentEntry.Encrypted {
		r.issue("odt.content_encrypted", "content.xml", contentEntry.Anchor.Element)
		return r, nil
	}
	if contentEntry.State != "resolved" || contentEntry.MediaType != "text/xml" {
		if contentEntry.Code == "office.part_not_admitted" {
			r.issue(contentEntry.Code, "META-INF/manifest.xml", contentEntry.Anchor.Element)
		}
		r.issue("odt.content_identity_unavailable", "content.xml", contentEntry.Anchor.Element)
		return r, nil
	}
	ci, exists := parts["content.xml"]
	if !exists {
		r.issue("odt.content_missing", "content.xml", -1)
		return r, nil
	}
	content := view.Parts[ci]
	if content.State != "completed" {
		r.issue("odt.content_unavailable", content.Name, -1)
		return r, nil
	}
	r.ContentSHA256 = content.SHA256
	d, err := xmlparts.Parse(ctx, content.Bytes, content.SHA256, xmlparts.DefaultLimits())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r.issue(parseCode(err), content.Name, -1)
		return r, nil
	}
	r.ContentXML = d
	if len(d.Elements) == 0 || d.Elements[0].Name.Namespace != OfficeNS || d.Elements[0].Name.Local != "document-content" {
		r.issue("odt.content_root_mismatch", content.Name, 0)
		return r, nil
	}
	version, _ := attribute(d.Elements[0], OfficeNS, "version")
	if r.ManifestVersion != "" && version != r.ManifestVersion {
		r.issue("odt.content_version_mismatch", content.Name, 0)
		return r, nil
	}
	body, bodyCount := -1, 0
	for i, e := range d.Elements {
		if e.Parent == 0 && e.Name.Namespace == OfficeNS && e.Name.Local == "body" {
			body = i
			bodyCount++
		}
	}
	texts, bodyChildren := 0, 0
	if bodyCount == 1 {
		for _, e := range d.Elements {
			if e.Parent == body {
				bodyChildren++
			}
			if e.Parent == body && e.Name.Namespace == OfficeNS && e.Name.Local == "text" {
				texts++
			}
		}
	}
	if bodyCount != 1 || texts != 1 || bodyChildren != 1 {
		r.issue("odt.text_body_ambiguous", content.Name, 0)
		return r, nil
	}
	r.Format = "odt"
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Result) readManifest() bool {
	d := r.ManifestXML
	part := "META-INF/manifest.xml"
	if len(d.Elements) == 0 || d.Elements[0].Name.Namespace != ManifestNS || d.Elements[0].Name.Local != "manifest" {
		r.issue("odt.manifest_root_mismatch", part, 0)
		return false
	}
	r.ManifestVersion, _ = attribute(d.Elements[0], ManifestNS, "version")
	if !knownVersion(r.ManifestVersion) {
		r.issue("odt.manifest_version_unsupported", part, 0)
	}
	for _, a := range d.Elements[0].Attributes {
		if !a.NamespaceDeclaration && (a.Name.Namespace != ManifestNS || a.Name.Local != "version") {
			r.issue("odt.manifest_structure_unknown", part, 0)
		}
	}
	encrypted := map[int]bool{}
	opaque := map[int]bool{}
	bad := map[int]bool{}
	for i, e := range d.Elements {
		if e.Parent < 0 {
			continue
		}
		parent := d.Elements[e.Parent]
		if opaque[e.Parent] {
			opaque[i] = true
			continue
		}
		if e.Name.Namespace == ManifestNS && e.Name.Local == "encryption-data" && parent.Parent == 0 && parent.Name.Namespace == ManifestNS && parent.Name.Local == "file-entry" {
			encrypted[e.Parent] = true
			opaque[i] = true
			continue
		}
		if e.Parent > 0 {
			bad[e.Parent] = true
		}
	}
	for _, t := range d.Tokens {
		if t.Kind == "text" && !opaque[t.Element] && strings.Trim(t.Value, " \t\r\n") != "" {
			bad[t.Element] = true
		}
	}
	if bad[0] {
		r.issue("odt.manifest_structure_unknown", part, 0)
	}
	paths := map[string][]int{}
	for i, e := range d.Elements {
		if e.Parent != 0 {
			continue
		}
		if len(r.Entries) >= maxEntries {
			r.issue("odt.manifest_entry_limit", part, i)
			return false
		}
		entry := Entry{Namespace: e.Name.Namespace, Kind: e.Name.Local, State: "declared", Encrypted: encrypted[i], Anchor: Anchor{part, r.ManifestSHA256, i, e.Full}}
		unknown := e.Name.Namespace != ManifestNS || e.Name.Local != "file-entry" || bad[i]
		for _, a := range e.Attributes {
			if a.NamespaceDeclaration {
				continue
			}
			if a.Name.Namespace != ManifestNS {
				unknown = true
				continue
			}
			switch a.Name.Local {
			case "full-path":
				entry.Path = a.Value
			case "media-type":
				entry.MediaType = a.Value
			case "version":
				entry.Version = a.Value
			case "size":
				entry.SizeDeclared = true
				entry.DeclaredSize = a.Value
			case "preferred-view-mode":
				if a.Value != "edit" && a.Value != "presentation" {
					unknown = true
				}
			default:
				unknown = true
			}
		}
		_, hasType := attribute(e, ManifestNS, "media-type")
		switch {
		case unknown:
			entry.State, entry.Code = "unsupported", "odt.manifest_structure_unknown"
		case !validPath(entry.Path) || !hasType:
			entry.State, entry.Code = "invalid", "odt.manifest_entry_invalid"
		case entry.Path == "mimetype" || entry.Path == part:
			entry.State, entry.Code = "invalid", "odt.manifest_entry_forbidden"
		}
		if entry.Code != "" {
			r.issue(entry.Code, part, i)
		}
		if entry.manifestEntry() {
			paths[entry.Path] = append(paths[entry.Path], len(r.Entries))
		}
		r.Entries = append(r.Entries, entry)
	}
	for i := range r.Entries {
		e := &r.Entries[i]
		if e.manifestEntry() && e.Path != "" && len(paths[e.Path]) > 1 {
			e.State, e.Code = "ambiguous", "odt.manifest_path_ambiguous"
			r.issue(e.Code, part, e.Anchor.Element)
		}
	}
	return true
}
func (e Entry) manifestEntry() bool {
	return e.Namespace == ManifestNS && e.Kind == "file-entry"
}

func (r *Result) membership(parts map[string]int) {
	declared := map[string]int{}
	for i := range r.Entries {
		e := &r.Entries[i]
		if !e.manifestEntry() {
			continue
		}
		declared[e.Path] = i
		// Rejected/ambiguous declarations remain evidence, never membership authority.
		if e.State != "declared" {
			continue
		}
		switch {
		case e.Path == "/":
			r.RootEntry = i
			e.State = "package"
		case e.Path == "content.xml":
			r.ContentEntry = i
		}
		if e.Path != "/" {
			if index, ok := parts[e.Path]; ok {
				e.State = "resolved"
				e.StoredPartSHA256 = r.Outcomes.Parts[index].SHA256
				target := r.Outcomes.Parts[index]
				if target.State != "completed" {
					e.State, e.Code = target.State, target.Code
				}
			} else if strings.HasSuffix(e.Path, "/") {
				j := sort.Search(len(r.Outcomes.Parts), func(j int) bool { return r.Outcomes.Parts[j].Name >= e.Path })
				if j < len(r.Outcomes.Parts) && strings.HasPrefix(r.Outcomes.Parts[j].Name, e.Path) {
					e.State = "directory"
				} else {
					e.State, e.Code = "missing", "odt.manifest_target_missing"
				}
			} else {
				e.State, e.Code = "missing", "odt.manifest_target_missing"
			}
		}
		if e.Encrypted {
			switch e.State {
			case "resolved":
				e.State, e.Code = "encrypted", "odt.encrypted_entry_unavailable"
			case "package", "directory":
				e.State, e.Code = "unsupported", "odt.encryption_target_invalid"
			default:
				r.issue("odt.encrypted_entry_unavailable", "META-INF/manifest.xml", e.Anchor.Element)
			}
		} else if e.SizeDeclared && e.State == "resolved" {
			size, err := strconv.ParseUint(e.DeclaredSize, 10, 64)
			if index, ok := parts[e.Path]; err != nil || !ok || size != uint64(len(r.Outcomes.Parts[index].Bytes)) {
				e.State, e.Code = "unresolved", "odt.declared_size_mismatch"
			}
		}
		if e.Code != "" && e.Code != "office.part_not_admitted" {
			// Payload admission failures are emitted once at the payload below;
			// declaration errors keep their manifest anchor.
			index, found := parts[e.Path]
			if !found || r.Outcomes.Parts[index].Code != e.Code {
				r.issue(e.Code, "META-INF/manifest.xml", e.Anchor.Element)
			}
		}
	}
	for _, p := range r.Outcomes.Parts {
		if p.Directory {
			continue
		}
		m := Membership{Part: p.Name, PartSHA256: p.SHA256, AdmissionState: p.State, AdmissionCode: p.Code, Entry: -1}
		if i, ok := declared[p.Name]; ok {
			m.Entry = i
			m.State = r.Entries[i].State
			m.Code = r.Entries[i].Code
		} else if p.Name == "mimetype" || strings.HasPrefix(p.Name, "META-INF/") {
			m.State = "manifest_exempt"
		} else {
			m.State, m.Code = "unlisted", "odt.part_unlisted"
			r.issue(m.Code, p.Name, -1)
		}
		if p.State != "completed" && p.Code != "" && p.Code != "office.part_not_admitted" {
			r.issue(p.Code, p.Name, -1)
		}
		r.Memberships = append(r.Memberships, m)
	}
}
