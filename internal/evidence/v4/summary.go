package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"reflect"
	"strings"
)

// aggregateNativeStatus runs after trace/coverage validation. An Office inventory
// is usable evidence even when no analyzer produced a finding or scan result.
func aggregateNativeStatus(t NativeTrace, x *TraceIndex, office Evidence) v2.State {
	usable := len(t.Results) > 0 || len(office.Scopes) > 0 || len(office.Metadata) > 0 || len(office.Relationships) > 0 || len(office.Objects) > 0
	for _, p := range office.Packages {
		usable = usable || len(p.Parts) > 0
	}
	partial, canceled, incomplete, optionalFailure := false, false, false, false
	for _, e := range t.Executions {
		c := x.Capabilities[e.CapabilityRef]
		if c.Participation == v2.Disabled {
			continue
		}
		if e.State == v2.Failed && (c.Role == v2.Acquisition || c.Role == v2.Parser) {
			return v2.Failed
		}
		partial = partial || e.State == v2.Partial
		canceled = canceled || e.State == v2.Canceled
		incomplete = incomplete || e.State != v2.Completed
		optionalFailure = optionalFailure || c.Participation == v2.Optional && e.State != v2.Completed
	}
	if partial {
		return v2.Partial
	}
	if canceled {
		if usable {
			return v2.Partial
		}
		return v2.Canceled
	}
	if incomplete {
		if usable || optionalFailure {
			return v2.Partial
		}
		return v2.Failed
	}
	return v2.Completed
}
func validateSummary(r Report, t *TraceIndex) error {
	if r.Status != aggregateNativeStatus(r.Trace, t, r.Evidence.Office) {
		return ErrLinkage
	}
	counts, err := summaryCounts(r)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(counts, r.Summary) {
		return ErrLinkage
	}
	return nil
}

// Summarize derives status and counts from native execution evidence. Encode
// remains mandatory: this method does not validate all report coordinates.
func (r *Report) Summarize() error {
	if r == nil {
		return ErrLinkage
	}
	trace, err := IndexTrace(r.Trace)
	if err != nil {
		return err
	}
	r.Status = aggregateNativeStatus(r.Trace, trace, r.Evidence.Office)
	r.Summary, err = summaryCounts(*r)
	return err
}

func summaryCounts(r Report) (map[string]int, error) {
	counts := map[string]int{"findings": len(r.Findings), "high": 0, "medium": 0, "low": 0, "info": 0, "exit_code": 0}
	for _, f := range r.Findings {
		key := strings.ToLower(f.Severity)
		if f.Severity != "HIGH" && f.Severity != "MEDIUM" && f.Severity != "LOW" && f.Severity != "INFO" {
			return nil, ErrLinkage
		}
		counts[key]++
		counts["exit_code"] = max(counts["exit_code"], evidence.Rank(f.Severity))
	}
	if r.Status != v2.Completed {
		counts["exit_code"] = 4
	}
	return counts, nil
}
