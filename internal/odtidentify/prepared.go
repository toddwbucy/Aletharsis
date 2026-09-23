package odtidentify

import (
	"context"
	"slices"
	"strconv"

	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// PreparedManifest retains the exact mime/manifest prerequisites once, before
// content admission. It does not identify a document without verified content.
type PreparedManifest struct {
	result   *Result
	ready    bool
	source   string
	declared map[string]uint64
}

func PrepareManifest(ctx context.Context, reader *packageparts.OutcomeReader) (*PreparedManifest, error) {
	if ctx == nil || reader == nil {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view := reader.ReadOnlyView()
	if view.SourceSHA256 == "" {
		return nil, packageparts.ErrIdentity
	}
	declared := map[string]uint64{}
	for _, o := range view.Parts {
		if !o.Part.Directory {
			declared[o.Part.Name] = o.DeclaredBytes
		}
	}
	r, ready, err := prepareManifest(ctx, nil, &view)
	if err != nil {
		return nil, err
	}
	return &PreparedManifest{r, ready, view.SourceSHA256, declared}, nil
}
func (p *PreparedManifest) ContentCandidate() string {
	if p == nil || !p.ready {
		return ""
	}
	var root, content *Entry
	for i := range p.result.Entries {
		e := &p.result.Entries[i]
		if !e.manifestEntry() || e.State != "declared" || e.Encrypted {
			continue
		}
		if e.Path == "/" {
			root = e
		}
		if e.Path == "content.xml" {
			content = e
		}
	}
	if root == nil || root.MediaType != MIME || content == nil || content.MediaType != "text/xml" {
		return ""
	}
	if root.Version != "" && p.result.ManifestVersion != "" && root.Version != p.result.ManifestVersion {
		return ""
	}
	declared, exists := p.declared["content.xml"]
	if !exists {
		return ""
	}
	if content.SizeDeclared {
		size, err := strconv.ParseUint(content.DeclaredSize, 10, 64)
		if err != nil || size != declared {
			return ""
		}
	}
	return "content.xml"
}

// InspectPrepared borrows the reader's immutable verified payload bytes; the
// returned package bytes must not be modified by the inspection pipeline.
func InspectPrepared(ctx context.Context, reader *packageparts.OutcomeReader, p *PreparedManifest) (*Result, error) {
	if ctx == nil || reader == nil || p == nil || p.result == nil {
		return nil, packageparts.ErrIdentity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view := reader.ReadOnlyView()
	if view.SourceSHA256 != p.source {
		return nil, packageparts.ErrIdentity
	}
	r := *p.result
	r.Outcomes = &view
	r.Entries = slices.Clone(r.Entries)
	r.Issues = slices.Clone(r.Issues)
	r.Memberships = []Membership{}
	if !p.ready {
		return &r, nil
	}
	for name, digest := range map[string]string{"mimetype": r.MimetypeSHA256, "META-INF/manifest.xml": r.ManifestSHA256} {
		matches := 0
		for _, o := range view.Parts {
			if o.Part.Name == name && !o.Part.Directory {
				matches++
				if o.State != "completed" || digest == "" || o.Part.SHA256 != digest {
					return nil, packageparts.ErrIdentity
				}
			}
		}
		if matches != 1 {
			return nil, packageparts.ErrIdentity
		}
	}
	return finishPackage(ctx, &view, &r)
}
