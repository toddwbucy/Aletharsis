package officeplan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

// PackageRecords is the package layer of a native report, not a complete report.
// Subsequent assembly adds XML/scopes/inventories and binds operation references
// to the execution graph before the complete report is validated.
type PackageRecords struct {
	Evidence  v4.Evidence
	Artifacts []v4.Artifact
	PartRefs  map[string]string
}

// BuildPackageRecords uses canonical package order to allocate stable part and
// artifact references. Source artifact/0 is followed by one artifact per part.
// parseExecution is the ordinal allocated by the report's execution coordinator.
func BuildPackageRecords(ctx context.Context, source []byte, p *Prepared, parseExecution int) (*PackageRecords, error) {
	if ctx == nil || p == nil || p.Admission == nil || parseExecution < 0 {
		return nil, v4.ErrLinkage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.Limits.Validate(); err != nil {
		return nil, err
	}
	view := p.Admission.Outcomes
	if identity.ExactBytes(source) != view.SourceSHA256 {
		return nil, packageparts.ErrIdentity
	}
	format := "unknown"
	switch p.Identity {
	case IdentityDOCX:
		format = "docx"
	case IdentityODT:
		format = "odt"
	case IdentityUnidentified, IdentityConflicting:
	default:
		return nil, v4.ErrLinkage
	}
	pkg := v4.Package{PackageRef: "office-package/0", SourceSHA256: view.SourceSHA256, SourceByteLength: int64(len(source)), SourceArtifactRef: "artifact/0",
		ParserVersion: view.Parser, Format: format, State: view.State, Issues: []v4.Issue{}, Parts: []v4.Part{}, Outcomes: []v4.Outcome{}}
	// Wire format "unknown" means no unique format was selected. The conflict
	// code distinguishes dual identity from unidentified content without changing
	// the closed 4.0 format vocabulary or the package parser's completion state.
	if p.Identity == IdentityConflicting {
		pkg.Issues = append(pkg.Issues, v4.Issue{Code: IdentityConflictCode})
	}
	if err := p.Limits.Record(&pkg); err != nil {
		return nil, err
	}
	result := &PackageRecords{Evidence: v4.Evidence{Packages: []v4.Package{}, XML: []v4.XML{}, Scopes: []v4.Scope{}, Metadata: []v4.Metadata{}, Relationships: []v4.Relationship{}, Objects: []v4.Object{}}, Artifacts: []v4.Artifact{}, PartRefs: map[string]string{}}
	sourceSize := uint64(len(source))
	asset, err := packageArtifact("artifact/0", "source", &pkg.SourceSHA256, &sourceSize, []string{})
	if err != nil {
		return nil, err
	}
	result.Artifacts = append(result.Artifacts, asset)
	for i, o := range view.Parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		native := o.Part
		if i > 0 && view.Parts[i-1].Part.Name >= native.Name {
			return nil, v4.ErrLinkage
		}
		span := v4.Span{Start: native.CompressedSpan.Start, End: native.CompressedSpan.End}
		if !span.Within(int64(len(source))) || identity.ExactBytes(source[span.Start:span.End]) != native.CompressedSHA256 {
			return nil, packageparts.ErrIdentity
		}
		part := v4.Part{PartRef: fmt.Sprintf("office-part/%d", i), PackageRef: pkg.PackageRef, Name: native.Name, ArtifactRef: fmt.Sprintf("artifact/%d", i+1), Method: int64(native.Method), CompressedSpan: span, CompressedSHA256: native.CompressedSHA256, State: o.State, Issues: []v4.Issue{}}
		var length *uint64
		if o.State == "completed" {
			if native.SHA256 == "" || identity.ExactBytes(native.Bytes) != native.SHA256 {
				return nil, packageparts.ErrIdentity
			}
			n := int64(len(native.Bytes))
			u := uint64(n)
			digest := native.SHA256
			part.ByteLength = &n
			part.SHA256 = &digest
			length = &u
		} else if native.SHA256 != "" || len(native.Bytes) != 0 {
			return nil, packageparts.ErrIdentity
		}
		outcome := v4.Outcome{Operation: capability.ParseOfficeID, ExecutionRef: fmt.Sprintf("exec/%d", parseExecution), PartRef: &part.PartRef, State: o.State, Codes: []string{}, DiagnosticRefs: []string{}, Assessed: []v4.Span{}, Excluded: []v4.Span{}}
		if part.ByteLength != nil {
			outcome.Assessed = append(outcome.Assessed, v4.Span{Start: 0, End: *part.ByteLength})
		}
		if o.Code != "" {
			part.Issues = append(part.Issues, v4.Issue{Code: o.Code, PartRef: &part.PartRef})
			outcome.Codes = append(outcome.Codes, o.Code)
		}
		asset, err := packageArtifact(part.ArtifactRef, "package_part", part.SHA256, length, []string{pkg.SourceArtifactRef})
		if err != nil {
			return nil, err
		}
		result.Artifacts = append(result.Artifacts, asset)
		result.PartRefs[part.Name] = part.PartRef
		pkg.Parts = append(pkg.Parts, part)
		pkg.Outcomes = append(pkg.Outcomes, outcome)
	}
	result.Evidence.Packages = append(result.Evidence.Packages, pkg)
	index, err := v4.IndexEvidence(result.Evidence, pkg.SourceSHA256, pkg.SourceByteLength)
	if err != nil {
		return nil, err
	}
	if err := index.ValidateArtifactBindings(result.Artifacts); err != nil {
		return nil, err
	}
	return result, nil
}

func packageArtifact(ref, kind string, digest *string, length *uint64, parents []string) (v4.Artifact, error) {
	unavailable := "not_retained"
	if digest == nil {
		unavailable = "extraction_unavailable"
	}
	reason := failure.Code("mapping.not_applicable")
	a := v2.Artifact{Ref: ref, Kind: kind, Representation: v2.Representation{Serialization: "original-bytes/1", Normalization: "none", LineEndings: "preserved"}, SHA256: digest, ByteLength: length, UnavailableReason: &unavailable, Parents: parents, Mapping: v2.Mapping{Quality: "unavailable", ReasonCode: &reason}}
	raw, err := json.Marshal(a)
	if err != nil {
		return v4.Artifact{}, err
	}
	var result v4.Artifact
	err = json.Unmarshal(raw, &result)
	return result, err
}
