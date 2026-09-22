// Package profiles assesses preserved observations without changing findings,
// parsing documents, executing profile content, or granting edit authority.
package profiles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/schemas"
)

// Sentinels let future hosts map failures without parsing human error prose.
var (
	ErrDefinition  = errors.New("invalid profile definition")
	ErrEvidence    = errors.New("invalid profile evaluation evidence")
	ErrLimit       = errors.New("profile resource limit")
	ErrUnavailable = errors.New("profile unavailable")
	ErrConflict    = errors.New("immutable profile version conflict")
)

type Scope struct {
	Format        string `json:"format"`
	Variant       string `json:"variant"`
	Parser        string `json:"parser"`
	ParserVersion string `json:"parser_version"`
}

// Fact identifies the implementation that established this context. Document
// assertions are not authoritative context; callers must verify producer links.
type Fact struct {
	Value               string `json:"value"`
	Producer            string `json:"producer"`
	Version             string `json:"version"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
}

type Rule struct {
	ID         string          `json:"id"`
	Revision   string          `json:"revision"`
	Kind       string          `json:"kind"`
	Facts      map[string]Fact `json:"facts"`
	Assessment string          `json:"assessment"`
	Queue      string          `json:"queue"`
	Rationale  string          `json:"rationale"`
}

type definition struct {
	Contract string `json:"contract"`
	ID       string `json:"id"`
	Version  string `json:"version"`
	Scope    Scope  `json:"scope"`
	Rules    []Rule `json:"rules"`
}

type Identity struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	SemanticSHA256 string `json:"semantic_sha256"`
}

type registered struct {
	definition definition
	identity   Identity
	raw        []byte
}

// Registry owns immutable definitions. Reusing an ID/version with different
// bytes is rejected, even when only JSON formatting changed. Historical profile
// publication and retained artifact verification remain the host's responsibility.
type Registry struct {
	mu       sync.RWMutex
	profiles map[string]registered
	rules    map[string]string
}

type noResources struct{}

func (noResources) Load(string) (any, error) {
	return nil, errors.New("external profile schema resources disabled")
}

var compiled = sync.OnceValues(func() (*jsonschema.Schema, error) {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.Profile()))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(noResources{})
	const url = "https://aletharsis.invalid/bundled/profile-v1.json"
	if err = c.AddResource(url, value); err != nil {
		return nil, err
	}
	return c.Compile(url)
})

// Register validates a closed, bounded definition and retains a private copy.
// Duplicate keys, executable operators and unknown members are rejected. Profile
// loading alone does not select a profile or establish support for its format.
func (r *Registry) Register(raw []byte) (Identity, error) {
	budget := identity.Limits{InputBytes: 256 << 10, OutputBytes: 256 << 10, Nodes: 32768, Depth: 16}
	canonical, err := identity.Canonicalize(raw, budget)
	if err != nil {
		if errors.Is(err, identity.ErrLimit) {
			return Identity{}, errors.Join(ErrLimit, err)
		}
		return Identity{}, errors.Join(ErrDefinition, err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(canonical))
	if err != nil {
		return Identity{}, ErrDefinition
	}
	schema, err := compiled()
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	if schema.Validate(value) != nil {
		return Identity{}, ErrDefinition
	}
	var def definition
	if err = json.Unmarshal(canonical, &def); err != nil {
		return Identity{}, ErrDefinition
	}
	seen := map[string]bool{}
	ruleDigests := map[string]string{}
	for _, rule := range def.Rules {
		if seen[rule.ID] {
			return Identity{}, fmt.Errorf("%w: duplicate rule ID", ErrDefinition)
		}
		seen[rule.ID] = true
		encoded, err := json.Marshal(rule)
		if err != nil {
			return Identity{}, ErrDefinition
		}
		ruleDigests[def.ID+"/"+rule.ID+"@"+rule.Revision] = identity.ExactBytes(encoded)
		if rule.Kind == "unicode_character" {
			code := rule.Facts["code_point"].Value
			cp, err := strconv.ParseUint(code[2:], 16, 32)
			if err != nil || !utf8.ValidRune(rune(cp)) || fmt.Sprintf("U+%04X", cp) != code {
				return Identity{}, fmt.Errorf("%w: noncanonical code point", ErrDefinition)
			}
		}
	}
	semantic, err := identity.Digest("aletharsis.profile/1", canonical, identity.Limits{InputBytes: 512 << 10, OutputBytes: 512 << 10, Nodes: 65536, Depth: 20})
	if err != nil {
		if errors.Is(err, identity.ErrLimit) {
			return Identity{}, errors.Join(ErrLimit, err)
		}
		return Identity{}, errors.Join(ErrDefinition, err)
	}
	id := Identity{def.ID, def.Version, identity.ExactBytes(raw), semantic}
	key := def.ID + "@" + def.Version
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.profiles == nil {
		r.profiles = map[string]registered{}
	}
	if old, ok := r.profiles[key]; ok && old.identity != id {
		return Identity{}, ErrConflict
	}
	if _, exists := r.profiles[key]; !exists && len(r.profiles) >= 128 {
		return Identity{}, ErrLimit
	}
	for key, digest := range ruleDigests {
		if old, exists := r.rules[key]; exists && old != digest {
			return Identity{}, ErrConflict
		}
	}
	if r.rules == nil {
		r.rules = map[string]string{}
	}
	for key, digest := range ruleDigests {
		r.rules[key] = digest
	}
	r.profiles[key] = registered{definition: def, identity: id, raw: bytes.Clone(raw)}
	return id, nil
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Artifact returns an independent copy of the exact registered profile bytes for
// forensic retention. It does not write them or choose a storage location.
func (r *Registry) Artifact(id, version string) ([]byte, error) {
	if !namePattern.MatchString(id) || len(version) > 64 || !versionPattern.MatchString(version) {
		return nil, ErrUnavailable
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, ok := r.profiles[id+"@"+version]
	if !ok {
		return nil, ErrUnavailable
	}
	return bytes.Clone(profile.raw), nil
}
