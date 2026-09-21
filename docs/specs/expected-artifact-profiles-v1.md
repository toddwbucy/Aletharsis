# PA-001: expected-artifact profile definitions and standalone evaluation

Status: proposed for review under [#14](https://github.com/toddwbucy/Aletharsis/issues/14).
Contract: `aletharsis.profile/1`. Implementation: `internal/profiles`.
This is the first executable profile increment, not completion of #14.

Profiles assess **expectedness in verified context**, not trust, authorship, intent
or edit permission. The evaluator returns separate annotations and queue hints;
it never deletes observations, changes findings/severity/confidence, controls which
analyzers run, changes coverage, or authorizes a derivative. Cryptographic and
statistical results are outside this profile contract.

No CLI option or report schema changes in this increment. Existing report schemas
keep their empty `profile_assessments` contract. This package must not be wired to
a report emitter until a separately reviewed migration defines the closed assessment
variants, references, failures and import/consumer behavior. A profile file is an
inert configuration artifact, not a report or plugin.

## 1. Definition and identity

The bundled [closed schema](../../schemas/profile-v1.schema.json) requires:

| Field | Meaning |
| --- | --- |
| `contract` | Exactly `aletharsis.profile/1` |
| `id`, `version` | Profile family and numeric `major.minor.patch` revision; no floating `latest` or prerelease syntax in v1 |
| `scope` | Exact `format`, extraction `variant`, parser identifier and parser implementation version |
| `rules` | Nonempty bounded list of uniquely named, revisioned rules |

Formats are text, source, HTML, XML, DOCX, ODT and PDF. Declaring a scope does not
create parser support. DOCX and ODT use independent profile families. Born-digital
and OCR-derived PDF require distinct extraction variants; no fallback between them.
Unknown or ambiguous extraction origin must not select a default PDF profile.
The host selects one explicit ID/version for an assessment call. Future bundle
selection must fail visible on ambiguity, rather than silently stack conflicting
profiles or load one from document-supplied metadata.

The registry computes both:

```text
artifact_sha256 = SHA256(exact supplied profile bytes)
semantic_sha256 = SHA256(JCS({domain: "aletharsis.profile/1", value: parsed profile}))
```

It retains a private copy of exact bytes, retrievable through `Artifact` for host
retention. A second registration of the same ID/version with different bytes fails,
even if only formatting differs. Changing rule content under the same profile
family/rule ID/rule revision also fails across registered profile versions. A rule
reference is scoped by the full profile identity; rule IDs are not global identities.
New rule meaning requires a new rule revision and a new profile version.

These in-memory checks complement, rather than replace, an immutable published
bundle manifest. The host must retain accepted historical profile bytes and pins
across sessions. Never edit a released file in place or claim that a new process's
empty registry proves a revision has never previously existed.

## 2. Declarative rules

Each rule contains ID/revision, observation kind, one to sixteen exact-equality
context predicates, assessment, queue choice and human rationale. There are no
regexes, scripts, templates, callbacks, remote resources, arbitrary expressions,
filesystem globs, URLs to load, or negative/missing-field matches.

Kinds are `unicode_character`, `package_part`, `relationship`, `formatting`,
`text_object`, `metadata_field` and `embedded_object`. `unknown_structure` is an
accepted observation kind but cannot be targeted by an expected rule. Credential
verification and statistical result kinds are rejected, not routed into suppression.

Every predicate names a fact key and compares this complete tuple:

```text
value + producer identifier + producer version + configuration_sha256
```

`configuration_sha256` is the EC-002 domain-separated configuration digest
(`aletharsis.config/1`), not a hash of arbitrary formatting of a configuration file.
The fixture contexts use the empty effective configuration `{}`. The host retains
and verifies the effective nonsecret configuration under its execution contract.

String equality is literal; no trimming, Unicode normalization, case folding or
coercion occurs. All predicates are conjunctive. Missing facts or different
producer/version/configuration mean no match. Context facts must be established by
a reviewed parser/context analyzer, not by text asserting “this is expected.”
Frequencies, language context, inherited formatting, visibility and alignment must
be determined by those bounded producers; v1 rules do not implement a second parser.

Unicode rules require both `code_point` and `context` predicates. A code-point-only
whitelist is rejected. Code points use canonical uppercase `U+` notation, at least
four hex digits, valid Unicode scalars only. Merely naming a context does not prove
it: the host validates its producer, configuration and evidence links before evaluation.

Assessment values are `expected`, `noteworthy`, `suspicious`, `unknown`. Queue choice
is `retain` or `omit_default`; only an expected rule may request omission. Rationale
remains untrusted display text. Even an expected structural match says nothing
about the safety of the structure's contents or the truth of a signed assertion.

## 3. Host evidence boundary

`Assess` accepts typed observations, not imported report JSON. The host must first
verify source/report identities, the execution graph and location mappings under
the accepted evidence contract. It must preserve the full evidence and all relevant
strong findings before constructing this view. Input fields are not authentication.

Each observation has a unique **instance** ID (not just a stable detector ID), exact
artifact SHA-256, scope, kind, producer-qualified facts, coverage state and exact-map
flag. The ID must resolve to retained finding/artifact/anchor evidence and the
executions that produced its facts. For structured documents that includes exact
package-part/stream/object identity where applicable, not an invented file-byte span.
Artifact identity and these references are integration prerequisites, not things the
standalone evaluator can infer from a hexadecimal digest.

Coverage is complete, partial, failed, unsupported, unavailable or unknown. This
host-derived value covers parsing, context production and **all relevant structural
analyses that could override this observation**, not merely successful decoding.
Assessment joins after those analyses reach their recorded terminal state. If a
relevant pattern detector failed, was truncated or did not run, the host must not
mark the affected observation complete. Independently unavailable cryptographic or
statistical detectors remain visible in their own coverage dimension and do not
become structural matches or negatives.

A partial
package may contain an independently complete part, but omitting an expected part
must never hide the package's incomplete-coverage banner or unsupported constructs.
Unknown context, approximate mappings and unresolved source references cannot become
expected by calling this API with optimistic flags. Until the host can establish
those facts, it must retain the evidence as unknown or skip profile evaluation.

A `Signal` is an independently preserved suspicious-pattern/likely-mechanism finding:
its unique instance ID, artifact digest, affected observation IDs and whether mapping
is complete. Complete mappings require nonempty, valid targets. Unknown/duplicate
IDs, cross-artifact targets and unknown artifacts are errors. Incomplete mappings
force retention of every supplied observation in that artifact. The host must pass
the full relevant signal set and observations, not an already filtered queue.
Statistical/cryptographic execution and coverage stay outside this input entirely.

## 4. Deterministic conflict and queue behavior

1. Exact scope and every predicate must match; otherwise the observation remains
   unknown and retained. No broad profile fallback or first-rule-wins policy.
2. Retain **all** matching rule IDs, revisions, rationales and proposed queue choices.
   Multiple matches resolve conservatively: suspicious > unknown > noteworthy >
   expected. Any matching `retain` choice prevents default omission.
3. Non-complete coverage or an inexact mapping forces unknown and retained, while
   preserving the matched rule explanations.
4. Related strong signals override any simple expected match: suspicious and
   retained. Their IDs are preserved. An incomplete signal map retains the entire artifact. Known targets remain
   suspicious; other observations become unknown with `unmapped_signals` references,
   rather than falsely claiming exact pattern membership. Unrelated artifacts keep
   their own assessments.
5. Return one annotation per supplied observation, sorted by instance ID. Rule
   matches and override IDs are sorted. The original findings and their order,
   severity/confidence, source bytes and coverage remain unchanged.

Annotations contain observation/artifact identity, full profile identity, resolved
expectedness, `omit_default`, a reason, all matches, exact-target overrides and unmapped signal holds. Reasons are
`no_matching_rule`, `profile_scope_mismatch`, `rules_matched`,
`coverage_or_mapping_incomplete`, `unmapped_pattern_requires_review` or
`pattern_override`. Coverage itself remains
independently visible even when a stronger signal is the primary annotation reason.

A future default queue may honor `omit_default`; the forensic view must expose all
observations and these explanations. Queue counts do not replace full finding counts,
execution coverage, failure diagnostics or existing exit-code semantics. This core
adds no “clean” verdict and supports no severity downgrade. Report/CLI integration
must make these distinctions visible before queue filtering ships.

## 5. Limits and failures

Definition loading uses the bundled schema only. Maximums: 256 KiB input and
canonical JSON, depth 16, 32,768 parsing nodes, 256 rules, 16 predicates per rule and
128 registered profile revisions. Unknown members, duplicate JSON/rule keys,
invalid UTF-8/surrogates and malformed revisions fail without registration.

Evaluation requires positive caller budgets for observations, signals, comparisons
and retained links. Independent hard ceilings are 10,000 observations, 10,000 signals,
1,000,000 rule/predicate comparisons and 100,000 rule-match/override links. Each
observation has at most 16 bounded, valid UTF-8 facts. Invalid input or budget failure
returns **no partial annotations**; the host must retain its unfiltered evidence and
report profile evaluation unavailable/failed, never call an empty result clean.

Go sentinels distinguish definition, evidence, resource, unavailable and immutable
revision conflict failures. Hosts must not parse error prose into protocol codes;
stable report diagnostics need explicit mapping in the later integration contract.
The registry supports concurrent registration/assessment and owns its definitions;
callers must not mutate observation/fact/signal inputs during a call.

## 6. Conformance and delivery gates

Tests include independent text, DOCX, ODT, born-digital PDF and OCR PDF scope examples,
producer/configuration mismatches, missing context, immutable profile/rule revisions,
all expectedness conflicts, incomplete coverage/mapping, exact artifact isolation,
resource/invalid-reference rejection, retained raw bytes, concurrency and a real
native zero-width binary finding overriding a deliberately permissive test context.

**The five JSON fixtures are synthetic normalized contexts, not actual parsed Office
or PDF documents and not approved production profiles.** This does not satisfy #14's
real structured-format fixture requirement. No source-file I/O occurs in the package.

Remaining review increments, without shrinking #14:

- Parser observation/location contracts and bounded executables: DOCX first, ODT
  separately, then born-digital/OCR PDF with explicit unsupported semantics.
- Reviewed context producers and immutable profile bundles, grounded in legitimate
  and adversarial **actual document** fixtures. Investigate #54's Hangul filler
  inventory gap independently of expectedness; profiles cannot hide unobserved data.
- Host adapter that verifies artifact/anchor/producer/configuration references,
  derives complete pattern relationships, and proves all analyzers still receive
  preserved evidence before optional queue filtering.
- Report migration with exact profile artifacts and assessment references, import
  validation, capability/failure states, original full counts and consumer parity.
  Existing report versions are not retroactively widened by PA-001.
- CLI/default queue and forensic explanation integration, replay of historical
  profiles and before/after source hash/timestamp tests across supported formats.

#14 stays open until those requirements and its full acceptance suite pass. #40/#41
oracle execution still depends on reviewed parser contracts and executables. This
core neither adopts external detectors nor closes their gates by implication.

## Validation of this increment

Go 1.27.1: `go test ./...`, `go vet ./...` and
`go test -race ./internal/profiles` pass. The offline Python 3.12.13 contract suite
passes 265 tests. A two-worker, two-second configured `FuzzProfileDefinition` run
completed 51,261 inputs without failure (about three seconds including shutdown).
That bounded run is evidence of this implementation check, not exhaustive assurance.
The profile fixtures retain exact checkout bytes through `.gitattributes`.

## Structured package prerequisite

[DP-001](document-package-parts.md) adds bounded in-memory ZIP part preservation
and exact source/compressed/decompressed identities, with retained DOCX/ODT packages.
It is a separate parser prerequisite, not an approved context producer or profile
bundle. XML locations, format validation, real profile evaluation and report/CLI
integration remain required before closing #14.
