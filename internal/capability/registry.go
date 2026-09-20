// Package capability declares the native catalog without plugin discovery or
// execution. Descriptors are fresh per call; callers cannot mutate global state.
package capability

import (
	"encoding/json"
	"errors"
	"runtime"
	"sort"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

const CatalogVersion = "aletharsis.native-text/2"

// Native operation IDs are shared with the audit coordinator.
const (
	AcquireID          = "aletharsis.acquire"
	ParseTextID        = "aletharsis.parse.text"
	UnicodeInventoryID = "aletharsis.unicode.inventory"
	EmojiID            = "aletharsis.unicode.emoji"
	TextID             = "aletharsis.text"
	PatternsID         = "aletharsis.patterns"
	IdentifiersID      = "aletharsis.identifiers"
	MetadataID         = "aletharsis.metadata"
)

// Native returns the eight compiled native operations plus three disabled
// declarations. Platform availability is a fact about the reader implementation,
// not a promise that any particular path can be opened. No filesystem/environment
// discovery or network request is performed.
func Native(version string, maxBytes uint64) ([]v2.Capability, error) {
	return forPlatform(version, maxBytes, runtime.GOOS)
}
func forPlatform(version string, maxBytes uint64, platform string) ([]v2.Capability, error) {
	if version == "" || maxBytes == 0 || maxBytes > identity.MaxSafeInteger {
		return nil, errors.New("invalid native catalog configuration")
	}
	data, err := nativeData()
	if err != nil {
		return nil, err
	}
	result := make([]v2.Capability, 0, 11)
	for _, entry := range []struct {
		id   string
		role v2.Role
	}{
		{AcquireID, v2.Acquisition}, {ParseTextID, v2.Parser},
		{UnicodeInventoryID, v2.Analyzer}, {EmojiID, v2.Analyzer},
		{TextID, v2.Analyzer}, {PatternsID, v2.Analyzer},
		{IdentifiersID, v2.Analyzer}, {MetadataID, v2.Analyzer},
	} {
		kinds := []string{"text"}
		if entry.role != v2.Analyzer {
			kinds = []string{"source"}
		}
		c := v2.Capability{ID: entry.id, Revision: "2", Role: entry.role, Participation: v2.Required,
			Implementation: &v2.Implementation{ID: entry.id, Version: version, Data: append([]v2.DataIdentity{}, data...)},
			Availability:   v2.Availability{State: "available"}, SupportedScope: v2.SupportedScope{Kinds: kinds, Formats: []string{"text"}}, Limits: v2.Limits{InputBytes: ptr(maxBytes)}}
		if entry.role == v2.Analyzer {
			c.Mechanism = ptr(v2.Structural)
			c.Limits = v2.Limits{} // The acquisition bound applies to source bytes, not decoded UTF-8 bytes.
		}
		if entry.role == v2.Acquisition {
			c.SupportedScope.Formats = []string{"*"}
			if platform != "linux" {
				c.Availability = v2.Availability{State: "unavailable", ReasonCode: ptr(failure.NoAtimeUnavailable)}
			}
		}
		result = append(result, c)
	}
	for _, entry := range []struct {
		id        string
		role      v2.Role
		mechanism v2.Mechanism
		reason    failure.Code
	}{
		{"aletharsis.c2pa.carrier", v2.Analyzer, v2.Structural, failure.NotImplemented},
		{"aletharsis.c2pa.verify", v2.Verifier, v2.Cryptographic, failure.NotImplemented},
		{"anthropic.claude_text_watermark", v2.Analyzer, v2.Statistical, failure.DetectorAccessUnavailable},
	} {
		c := v2.Capability{ID: entry.id, Revision: "1", Role: entry.role, Mechanism: ptr(entry.mechanism), Participation: v2.Disabled, Availability: v2.Availability{State: "unavailable", ReasonCode: ptr(entry.reason)}, SupportedScope: v2.SupportedScope{Kinds: []string{}, Formats: []string{}}}
		if entry.mechanism == v2.Statistical {
			c.Purpose = ptr("watermark_detection")
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	for _, c := range result {
		if err := c.Validate(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// NonExecutionReason applies EC-001 precedence without performing the operation.
// Eligibility/policy are explicit coordinator inputs, never document instructions.
func NonExecutionReason(c v2.Capability, prerequisiteOK, supported, policyAllowed bool) (*failure.Code, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.Participation == v2.Disabled {
		return ptr(failure.Disabled), nil
	}
	if c.Availability.State != "available" {
		return ptr(*c.Availability.ReasonCode), nil
	}
	if !prerequisiteOK {
		return ptr(failure.PrerequisiteFailed), nil
	}
	if !supported {
		return ptr(failure.UnsupportedInput), nil
	}
	if !policyAllowed {
		return ptr(failure.PolicyDenied), nil
	}
	return nil, nil
}
func ptr[T any](v T) *T { return &v }

func nativeData() ([]v2.DataIdentity, error) {
	hashes, err := analyzers.EmbeddedDataDigests()
	if err != nil {
		return nil, err
	}
	return []v2.DataIdentity{
		{ID: "aletharsis.emoji", Version: "17.0", SHA256: hashes["emoji-17.0.txt"]},
		{ID: "aletharsis.limitations", Version: "1", SHA256: hashes["limitations.json"]},
		{ID: "aletharsis.messages", Version: "1", SHA256: hashes["messages.json"]},
		{ID: "aletharsis.unicode.categories", Version: u.Version, SHA256: u.CategoryDataSHA256()},
	}, nil
}

// NativeDataRevision binds configuration identity to the ordered compiled data
// manifest. It is not a claim to identify the compiler or every dependency byte.
func NativeDataRevision() (string, error) {
	data, err := nativeData()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	canonical, err := identity.Canonicalize(raw, v2.RecordLimits())
	if err != nil {
		return "", err
	}
	return "native-data/1:" + identity.ExactBytes(canonical), nil
}
