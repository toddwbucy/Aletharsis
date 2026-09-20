package v2

import (
	"errors"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

// AggregateStatus applies operational failure precedence independently of the
// selected finding view, after validating usable result linkage and coverage.
func (g Graph) AggregateStatus(file evidence.File, document evidence.Document) (State, error) {
	if err := g.Validate(file, document); err != nil {
		return "", err
	}
	caps := map[string]Capability{}
	for _, c := range g.Capabilities {
		if err := c.Validate(); err != nil {
			return "", err
		}
		if _, ok := caps[c.ID]; ok {
			return "", errors.New("duplicate capability")
		}
		caps[c.ID] = c
	}
	partial, canceled, incomplete, optionalFailure := false, false, false, false
	usable := len(g.Results) > 0
	operationalFailure := false
	for _, e := range g.Executions {
		if err := e.Validate(); err != nil {
			return "", err
		}
		c, ok := caps[e.CapabilityRef]
		if !ok {
			return "", errors.New("dangling capability")
		}
		if c.Participation == Disabled {
			continue
		}
		if e.State == Failed && (c.Role == Acquisition || c.Role == Parser) {
			operationalFailure = true
		}
		partial = partial || e.State == Partial
		canceled = canceled || e.State == Canceled
		incomplete = incomplete || e.State != Completed
		optionalFailure = optionalFailure || c.Participation == Optional && e.State != Completed
	}
	switch {
	case operationalFailure:
		return Failed, nil
	case partial:
		return Partial, nil
	case canceled:
		if usable {
			return Partial, nil
		}
		return Canceled, nil
	case incomplete:
		if usable || optionalFailure {
			return Partial, nil
		}
		return Failed, nil
	default:
		return Completed, nil
	}
}
