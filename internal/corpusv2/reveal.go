package corpusv2

import (
	"encoding/json"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/wire"
)

type Artifact struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Source struct {
	RelativePath         string     `json:"relative_path"`
	DetectionState       string     `json:"detection_state"`
	PresentationOutcome  string     `json:"presentation_outcome"`
	Reason               string     `json:"reason"`
	Artifacts            []Artifact `json:"artifacts"`
	SourceSHA256         *string    `json:"source_sha256,omitempty"`
	ReportArtifactSHA256 *string    `json:"report_artifact_sha256,omitempty"`
}
type RevealTree struct {
	Contract string   `json:"contract"`
	Corpus   Artifact `json:"corpus"`
	Sources  []Source `json:"sources"`
}

// DecodeRevealTree binds presentation receipts to exact corpus bytes and their
// validated reports. It validates identities, not the unprovided derivative files.
func DecodeRevealTree(raw, corpusRaw []byte, envelopeLimits, reportLimits identity.Limits) (RevealTree, error) {
	canonical, err := wire.ValidateEnvelope("reveal-tree-v2", raw, envelopeLimits)
	if err != nil {
		return RevealTree{}, err
	}
	var tree RevealTree
	if err := json.Unmarshal(canonical, &tree); err != nil {
		return RevealTree{}, err
	}
	if tree.Corpus.Size != int64(len(corpusRaw)) || tree.Corpus.SHA256 != identity.ExactBytes(corpusRaw) {
		return RevealTree{}, ErrLinkage
	}
	corpus, err := DecodeStream(corpusRaw, envelopeLimits, reportLimits)
	if err != nil {
		return RevealTree{}, err
	}
	if len(tree.Sources) != len(corpus.Entries) {
		return RevealTree{}, ErrLinkage
	}
	names := map[string]bool{"corpus.jsonl": true}
	for i, source := range tree.Sources {
		entry := corpus.Entries[i]
		if source.RelativePath != entry.RelativePath {
			return RevealTree{}, ErrLinkage
		}
		expected := entry.State
		var report struct {
			Status   string `json:"status"`
			Schema   string `json:"schema_version"`
			Evidence struct {
				Office struct {
					Packages []json.RawMessage `json:"packages"`
				} `json:"office"`
			} `json:"evidence"`
			File struct {
				SHA256 *string `json:"sha256"`
				Format string  `json:"format"`
			} `json:"file"`
		}
		if len(entry.Report) > 0 {
			if err := json.Unmarshal(entry.Report, &report); err != nil {
				return RevealTree{}, err
			}
			if entry.State == "failed" && entry.Reason == "execution.resource_limit" && (report.Status == "completed" || report.Status == "partial") {
				expected = report.Status
				if expected == "completed" {
					expected = "no_reported_findings"
					if entry.HighestFindingSeverity != nil {
						expected = "requires_review"
					}
				}
				if source.PresentationOutcome != "failed" || source.Reason != "execution.resource_limit" {
					return RevealTree{}, ErrLinkage
				}
			}
			if source.ReportArtifactSHA256 == nil || *source.ReportArtifactSHA256 != entry.ReportCanonicalSHA256 {
				return RevealTree{}, ErrLinkage
			}
			if source.SourceSHA256 != nil && (report.File.SHA256 == nil || *source.SourceSHA256 != *report.File.SHA256) {
				return RevealTree{}, ErrLinkage
			}
			eligible := expected == "partial" || expected == "requires_review" || expected == "no_reported_findings"
			if eligible && (report.Schema == "4.0" && len(report.Evidence.Office.Packages) > 0) && source.PresentationOutcome != "failed" && source.PresentationOutcome != "unsupported" {
				return RevealTree{}, ErrLinkage
			}
		} else if source.PresentationOutcome != "not_attempted" || source.ReportArtifactSHA256 != nil || len(source.Artifacts) > 0 {
			return RevealTree{}, ErrLinkage
		}
		if source.DetectionState != expected {
			return RevealTree{}, ErrLinkage
		}
		foundReport := false
		for _, a := range source.Artifacts {
			if names[a.Name] {
				return RevealTree{}, ErrLinkage
			}
			names[a.Name] = true
			if a.Name == "reports/"+source.RelativePath+".json" {
				if source.ReportArtifactSHA256 == nil || a.SHA256 != *source.ReportArtifactSHA256 {
					return RevealTree{}, ErrLinkage
				}
				canonical, err := identity.Canonicalize(entry.Report, reportLimits)
				if err != nil || a.Size != int64(len(canonical)) {
					return RevealTree{}, ErrLinkage
				}
				foundReport = true
			}
		}
		if source.ReportArtifactSHA256 != nil && !foundReport {
			return RevealTree{}, ErrLinkage
		}
	}
	return tree, nil
}
