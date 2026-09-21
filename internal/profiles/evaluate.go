package profiles

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// Observation is a host-verified view of one preserved evidence item. Its ID must
// map back to retained findings/artifacts/anchors. Coverage includes parsing, context and all relevant structural override analysis;
// coverage and mapping quality come from the host, never an inspected document.
type Observation struct {
	ID             string
	ArtifactSHA256 string
	Scope          Scope
	Kind           string
	Facts          map[string]Fact
	Coverage       string // complete, partial, failed, unsupported, unavailable, unknown
	Exact          bool
}

// Signal is a preserved suspicious-pattern/likely-mechanism finding. Complete
// mappings identify every affected observation. Incomplete mappings force review
// of all supplied observations in the same artifact, including unmatched targets.
type Signal struct {
	ID              string
	ArtifactSHA256  string
	Targets         []string
	MappingComplete bool
}

type Limits struct{ Observations, Signals, Comparisons, Links int }

type Match struct {
	RuleID     string `json:"rule_id"`
	Revision   string `json:"revision"`
	Assessment string `json:"assessment"`
	Queue      string `json:"queue"`
	Rationale  string `json:"rationale"`
}

type Assessment struct {
	ObservationID   string   `json:"observation_id"`
	ArtifactSHA256  string   `json:"artifact_sha256"`
	Profile         Identity `json:"profile"`
	Expectedness    string   `json:"expectedness"`
	OmitDefault     bool     `json:"omit_default"`
	Reason          string   `json:"reason"`
	Matches         []Match  `json:"matches"`
	Overrides       []string `json:"overrides"`
	UnmappedSignals []string `json:"unmapped_signals"`
}

func validKind(kind string) bool {
	switch kind {
	case "unicode_character", "package_part", "relationship", "formatting", "text_object", "metadata_field", "embedded_object", "unknown_structure":
		return true
	}
	return false
}

func validScope(s Scope) bool {
	formats := map[string]bool{"text": true, "source": true, "html": true, "xml": true, "docx": true, "odt": true, "pdf": true}
	return formats[s.Format] && namePattern.MatchString(s.Variant) && namePattern.MatchString(s.Parser) && len(s.ParserVersion) <= 64 && versionPattern.MatchString(s.ParserVersion)
}

// Assess returns a separate annotation for every supplied observation in stable
// ID order. It does not filter findings, alter severity, execute detectors, change
// coverage or decide document status. Caller-owned inputs must not mutate during
// evaluation. The host must supply the full relevant signal set, not a filtered
// view, and preserve coverage/diagnostic banners independently of queue selection.
func (r *Registry) Assess(id, version string, observations []Observation, signals []Signal, limits Limits) ([]Assessment, error) {
	if limits.Observations <= 0 || limits.Signals <= 0 || limits.Comparisons <= 0 || limits.Links <= 0 || len(observations) > limits.Observations || len(observations) > 10000 || len(signals) > limits.Signals || len(signals) > 10000 {
		return nil, ErrLimit
	}
	limits.Comparisons = min(limits.Comparisons, 1_000_000)
	limits.Links = min(limits.Links, 100_000)
	if !namePattern.MatchString(id) || len(version) > 64 || !versionPattern.MatchString(version) {
		return nil, ErrUnavailable
	}
	r.mu.RLock()
	profile, ok := r.profiles[id+"@"+version]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnavailable
	}
	byID := map[string]Observation{}
	byArtifact := map[string][]string{}
	for _, o := range observations {
		if !namePattern.MatchString(o.ID) || !hashPattern.MatchString(o.ArtifactSHA256) || !validScope(o.Scope) || !validKind(o.Kind) || len(o.Facts) > 16 {
			return nil, fmt.Errorf("%w: invalid profile observation", ErrEvidence)
		}
		switch o.Coverage {
		case "complete", "partial", "failed", "unsupported", "unavailable", "unknown":
		default:
			return nil, fmt.Errorf("%w: invalid profile coverage", ErrEvidence)
		}
		if _, exists := byID[o.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate profile observation", ErrEvidence)
		}
		for key, f := range o.Facts {
			if !namePattern.MatchString(key) || len(f.Value) == 0 || len(f.Value) > 2048 || !utf8.ValidString(f.Value) || !namePattern.MatchString(f.Producer) || !hashPattern.MatchString(f.ConfigurationSHA256) || len(f.Version) > 64 || !versionPattern.MatchString(f.Version) {
				return nil, fmt.Errorf("%w: invalid profile context", ErrEvidence)
			}
		}
		byID[o.ID] = o
		byArtifact[o.ArtifactSHA256] = append(byArtifact[o.ArtifactSHA256], o.ID)
	}
	overrides := map[string][]string{}
	unmapped := map[string][]string{}
	seenSignals := map[string]bool{}
	links := 0
	for _, signal := range signals {
		if !namePattern.MatchString(signal.ID) || !hashPattern.MatchString(signal.ArtifactSHA256) || seenSignals[signal.ID] || (signal.MappingComplete && len(signal.Targets) == 0) || len(signal.Targets) > limits.Links {
			return nil, fmt.Errorf("%w: invalid profile signal", ErrEvidence)
		}
		if _, known := byArtifact[signal.ArtifactSHA256]; !known {
			return nil, fmt.Errorf("%w: profile signal artifact unavailable", ErrEvidence)
		}
		seenSignals[signal.ID] = true
		targets := map[string]bool{}
		for _, target := range signal.Targets {
			o, exists := byID[target]
			if !exists || o.ArtifactSHA256 != signal.ArtifactSHA256 || targets[target] {
				return nil, fmt.Errorf("%w: invalid profile signal target", ErrEvidence)
			}
			targets[target] = true
		}
		if !signal.MappingComplete {
			for _, target := range byArtifact[signal.ArtifactSHA256] {
				if !targets[target] {
					links++
					if links > limits.Links {
						return nil, ErrLimit
					}
					unmapped[target] = append(unmapped[target], signal.ID)
				}
			}
		}
		for target := range targets {
			links++
			if links > limits.Links {
				return nil, ErrLimit
			}
			overrides[target] = append(overrides[target], signal.ID)
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Assessment, 0, len(ids))
	work := 0
	rank := map[string]int{"expected": 0, "noteworthy": 1, "unknown": 2, "suspicious": 3}
	for _, key := range ids {
		o := byID[key]
		a := Assessment{ObservationID: o.ID, ArtifactSHA256: o.ArtifactSHA256, Profile: profile.identity, Expectedness: "unknown", Reason: "no_matching_rule", Matches: []Match{}, Overrides: append([]string{}, overrides[key]...), UnmappedSignals: append([]string{}, unmapped[key]...)}
		sort.Strings(a.Overrides)
		sort.Strings(a.UnmappedSignals)
		if o.Scope != profile.definition.Scope {
			a.Reason = "profile_scope_mismatch"
		} else {
			selectedRank := -1
			retain := false
			for _, rule := range profile.definition.Rules {
				work++
				if work > limits.Comparisons {
					return nil, ErrLimit
				}
				if o.Kind != rule.Kind {
					continue
				}
				matches := true
				for fact, expected := range rule.Facts {
					work++
					if work > limits.Comparisons {
						return nil, ErrLimit
					}
					if actual, present := o.Facts[fact]; !present || actual != expected {
						matches = false
					}
				}
				if !matches {
					continue
				}
				links++
				if links > limits.Links {
					return nil, ErrLimit
				}
				a.Matches = append(a.Matches, Match{rule.ID, rule.Revision, rule.Assessment, rule.Queue, rule.Rationale})
				if rank[rule.Assessment] > selectedRank {
					selectedRank = rank[rule.Assessment]
					a.Expectedness = rule.Assessment
				}
				retain = retain || rule.Queue == "retain"
			}
			sort.Slice(a.Matches, func(i, j int) bool { return a.Matches[i].RuleID < a.Matches[j].RuleID })
			if len(a.Matches) > 0 {
				a.Reason = "rules_matched"
				a.OmitDefault = a.Expectedness == "expected" && !retain
			}
		}
		if o.Coverage != "complete" || !o.Exact {
			a.Expectedness = "unknown"
			a.OmitDefault = false
			a.Reason = "coverage_or_mapping_incomplete"
		}
		if len(a.UnmappedSignals) > 0 {
			a.Expectedness = "unknown"
			a.OmitDefault = false
			a.Reason = "unmapped_pattern_requires_review"
		}
		if len(a.Overrides) > 0 {
			a.Expectedness = "suspicious"
			a.OmitDefault = false
			a.Reason = "pattern_override"
		}
		result = append(result, a)
	}
	return result, nil
}
