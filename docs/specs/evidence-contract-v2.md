# Detection evidence and capability contract

| Attribute | Value |
| --- | --- |
| Document | EC-001, design revision 0.1 |
| Status | Proposed for technical review; not an implemented API or published schema |
| Proposed report version | 2.0; current production remains 1.0 |
| Decision | [ADR-0001](../adr/0001-evidence-capability-contract.md) |
| Tracking | [#17](https://github.com/toddwbucy/Aletharsis/issues/17), [#21 G1](https://github.com/toddwbucy/Aletharsis/issues/21), [#7 validation](https://github.com/toddwbucy/Aletharsis/issues/7) |
| Product authority | [Parent PRD](../product/aletharsis-PRD.md); [frontend contract requirements](../product/frontend-PRD.md#9-data-and-integration-contracts) |

The field names and rules below are proposed contract decisions. Review precedes
machine-readable schema work, then Go implementation. Nothing in this document
changes the live schema, frozen reports, current CLI syntax or implemented scope.
Normative “must” statements describe the proposed v2 implementation.

## 1. Scope and baseline

Current code: [model](../../internal/evidence/model.go),
[audit orchestration](../../internal/audit/audit.go),
[analyzers](../../internal/analyzers/base.go),
[CLI](../../internal/cli/cli.go), [schema 1.0](../../schemas/report.schema.json).

This increment defines single-file detection reports and consumer interpretation.
Directory jobs, matching/rules, decisions, writers, viewer components, C2PA SDK
selection and actual vendor APIs remain separate specifications. V2 initially
wraps the existing text engine and declares unimplemented capabilities honestly.
It does not implement office extraction, cryptographic verification or statistical
analysis. Source-size limits and platform acquisition policy remain unchanged.

## 2. Independent concepts

| Concept | Answers | Must not imply |
| --- | --- | --- |
| Mechanism | Which information-bearing family is being examined? | Maliciousness or proof of watermarking |
| Capability | What defined operation can this detector/parser perform? | Availability or execution on this input |
| Availability | Can that operation run in this configured environment? | Permission to run or transmit data |
| Participation | Was it required, optionally requested, or disabled for this audit? | Execution success |
| Execution | What happened when the planned operation was considered/run? | Positive/negative substantive result |
| Coverage | Which exact representations/regions were examined or omitted? | Exhaustive detection of an entire mechanism class |
| Result | What did the particular operation conclude under its contract? | Generic probability of authorship |
| Finding | What observation or evidence-based interpretation warrants reporting? | Edit authority |
| Profile assessment | Is an artifact expected in a declared context? | Trust or suppression of other analyzers |
| Human decision | What does a reviewer judge or authorize for an exact scope? | A change to detector evidence |

Mechanism enum: `structural`, `cryptographic`, `statistical`. Operational records
use explicit null, not a fourth pseudo-mechanism. Statistical capability purpose
must distinguish `watermark_detection`, `channel_analysis` and
`intrinsic_fingerprint_research`; ordinary language variation is not a detector.
All existing native text findings retain their current interpretation. A future
credential can have structural carrier evidence and cryptographic results, linked
through shared artifacts rather than merged into one verdict.

## 3. Report envelope and references

Retain top-level `aletharsis_version`, `schema_version`, `file`, `status`, `summary`,
`evidence`, `findings` and `limitations`. Set `schema_version` to `2.0` only for the
new contract. Add required `catalog_version`, `capabilities`, `executions`,
`artifacts`, `anchors`, `results`, `diagnostics`, `profile_assessments` and `view`.
Empty arrays are explicit; unknown objects/variants are not accepted by a known
version. Required-versus-null cases described below must be expressed in the next
schema PR, with `additionalProperties: false` on fixed objects and discriminated
`oneOf` variants. New wire variants require explicit version/compatibility review.

References are unique report-local strings, not persistent identities by themselves:
capability IDs are stable registered names; executions use `exec/<ordinal>`, artifacts
`artifact/<ordinal>`, anchors `anchor/<ordinal>`, results `result/<ordinal>` and
findings `finding/<ordinal>`. Assign ordinals after deterministic ordering, never
in worker completion order. A persistent reference is the exact report-artifact
SHA-256 plus the local reference. The imported-report hash belongs to the consumer's
audit-reference record, never a self-hash field inside the report.

A finding gains required `finding_ref`, `execution_ref`, nullable `mechanism` and
`anchor_refs`. Existing payload names, especially `character_offsets` and
`byte_offsets`, remain. `Finding.id` is still a rule ID, never a unique record ID.
Acquisition/parser failures use null mechanism and link to their failed stage.
Native findings retain their existing confidence values; those heuristic values
are not calibrated probabilities. New result types do not require invented finding
confidence. Keep the four existing finding classification values unchanged.

`view` records `name` (`audit`, `unicode`, `metadata`, `structure`) and its finding
category filter. Filtered views retain complete evidence, capabilities, executions,
results, anchors and diagnostics; filtering never hides operational failure.
Findings absent from a view cannot leave dangling links: executions and result
records link to evidence anchors, not optional finding records. Filtering preserves
surviving finding references, which may have gaps. Summary counts describe visible
findings; coverage still describes the actual run.

## 4. Capabilities and planning

A capability record has required `id`, `revision`, `role`, `mechanism`, `purpose`,
`implementation`, `availability`, `participation`, `supported_scope` and `limits`.

- `role`: `acquisition`, `parser`, `analyzer` or `verifier`; first two use null mechanism.
- `implementation`: null for a declaration-only capability; otherwise adapter/native
  ID and version, upstream revision/digest where applicable, and pinned data identities.
- `availability`: object with `state` (`available`, `unavailable`, `unknown`) and
  nullable `reason_code`. Unknown is for imported/unevaluated capability state,
  never silently treated as available. Native v2 planning evaluates registered entries.
- `participation`: `required`, `optional` or `disabled`. Built-in acquisition,
  parsing and current text analyzers are required. Explicit detector requests are
  required unless the caller explicitly selected best-effort optional behavior.
- `supported_scope`: declared input representation kinds/formats and operation
  limits; capability eligibility is not evidence that the input was examined.
- `limits`: finite input/expanded bytes, object/depth, output and execution limits
  applicable to that capability. Exact values require measured adapter acceptance;
  unsupported dimensions are explicit, not silently unlimited.

A fixed catalog avoids scanning arbitrary PATH/plugins or contacting services.
Only built-ins and explicitly configured adapters are evaluated. Capability discovery
is separate from file auditing; no new CLI command is selected by this design.
Register native operations with fixed IDs: `aletharsis.acquire`,
`aletharsis.parse.text`, `aletharsis.unicode.inventory`, `aletharsis.unicode.emoji`,
`aletharsis.text`, `aletharsis.patterns`, `aletharsis.identifiers` and
`aletharsis.metadata`. These wrap the existing stage/analyzer boundaries; they do
not change rule IDs or thresholds. Parser eligibility is assessed after acquired
format identification. Disabled declarations are still listed if acquisition fails,
without claiming their prerequisites were examined. Add these declaration-only entries:

| ID | Role/mechanism | Initial availability and participation |
| --- | --- | --- |
| `aletharsis.c2pa.carrier` | analyzer / structural | unavailable, `capability.not_implemented`; disabled |
| `aletharsis.c2pa.verify` | verifier / cryptographic | unavailable, `capability.not_implemented`; disabled |
| `anthropic.claude_text_watermark` | analyzer / statistical, watermark_detection | unavailable, `capability.detector_access_unavailable`; disabled |

The Anthropic declaration makes no assertion about current vendor eligibility,
endpoints, credentials or any document. It has no detector result and performs no
network request. An actual adapter requires a separately verified contract and
scope authorization. Research fingerprint capabilities are not automatically
registered merely because the product has a stretch goal.

## 5. Execution and coverage state machine

Each planned stage has an execution record with `execution_ref`, `capability_ref`,
`config`, `requested_scope`, `analyzed_scope`, `exclusions`, `state`, `reason_code`
and `diagnostic_refs`. One execution covers one capability and one declared scope;
multiple samples require distinct executions. No report-visible retries are hidden:
a retry policy and attempt identity belong in a future adapter contract.

`config` includes a nonsecret schema/revision, effective nonsecret settings and a
configuration digest. Identify secret material only by an opaque revisioned local
reference; never serialize keys or their hashes. Resource limits, preprocessing,
tokenizer/model identity and trust policy affect configuration identity. Unknown
configuration identity must be disclosed and cannot support a reproducibility claim.

| State | Meaning | Result rule |
| --- | --- | --- |
| `not_run` | Disabled, unavailable, unsupported input, missing prerequisite or policy-denied | No substantive results; analyzed scope empty, machine-readable reason required |
| `completed` | Entire requested scope examined within the declared operation | Results may report evidence, absence within that operation, or inconclusive outcome |
| `partial` | Some requested scope examined; remainder omitted or interrupted | Retain valid scoped results, exclusions and diagnostic; no whole-scope negative |
| `failed` | Operation could not produce a usable result | No substantive results; diagnostic required; any raw response is diagnostic evidence only |
| `canceled` | Canceled before producing usable scoped results | No substantive results; explicit cancellation reason |

Interrupted operations with usable scoped results use `partial` and a cancellation
reason; operations interrupted before any usable result use `canceled`. Running
states belong to job progress, not a finalized audit report. Resolve non-execution
reasons in this order: disabled participation, unavailable capability, failed
prerequisite, unsupported input, policy denial. Preserve the independent capability
availability even when `execution.disabled` is the selected execution reason. Downstream executions
blocked by acquisition or parsing use `not_run` with `execution.prerequisite_failed`.

Scope references artifacts with declared coordinate units and ordered half-open
regions, or `whole_artifact`. Disjoint regions remain disjoint. A region's scalar
and byte measures must agree with its mapping. Analyzed regions cannot exceed
requested scope. Exclusions identify missing regions/reasons where known; if a
parser cannot enumerate hidden objects, record unknown remainder explicitly rather
than claiming a complete partition. Count tokens only with tokenizer identity or
explicit provider-defined semantics; do not derive tokens from character counts.

Full requested-scope completion is not universal mechanism coverage. Human output
must say, for example, “Unicode inventory completed for extracted text; C2PA
verification and Claude statistical analysis not run.” A capability catalog version
bounds the declaration; unknown detectors outside it remain outside scope.

## 6. Artifact identity and coordinate mapping

`file.sha256` continues to identify exact acquired source bytes. Acquisition failure
leaves size/hash null; unverified partial reads are not a source snapshot. Parsing
failure after acquisition retains verified source identity without partial decoded
text. A source hash recorded in an imported report is a claim until independently
verified against the source.

An artifact record has `artifact_ref`, `kind`, `representation`, nullable `sha256`,
nullable `byte_length`, `content_ref`, `unavailable_reason`, `parents`, `transform`
and `mapping`:

| Kind | Digest domain | Required relationship |
| --- | --- | --- |
| `source` | Exact original bytes, without normalization | Links the report's acquired file identity |
| `text` | Exact UTF-8 encoding of extracted Unicode scalars, BOM retained if extracted | Text-segment reference plus parser/decoder identity and original-byte map where available |
| `package_part` / `stream` / `object` | Declared stored, decompressed or canonical object bytes | Source/parent identity, structural locator and representation-specific extraction identity |
| `manifest` | Exact extracted manifest-store bytes | Carrier anchor and extraction operation; signature validity is separate |
| `sample` | Exact bytes actually supplied to a detector | Ordered parents/spans, preprocessing and serialization; detector counts refer to this artifact |
| `binding_text` | Exact bytes used for a standard-specific binding check | Transform/specification version and mapping to its source representation |
| `detector_response` | Exact retained response bytes, if retained | Execution identity, media type and retention/redaction declaration |

`representation` specifies the byte serialization, encoding, normalization and
line-ending policy. Object serialization has an explicit version; do not pretend
that an abstract parsed object has inherent bytes. `content_ref` is either an
in-report JSON pointer, a content-addressed retained blob reference, or null with
an explicit unavailable/retention reason. Never dereference a document URL or
arbitrary local path on import. Null digest is permitted only for artifacts whose
bytes cannot be obtained, with a reason; such an artifact cannot stand in for an
exact submitted sample. Acquired sources and actual analyzed samples require hashes.

`unavailable_reason` is a required nullable field with the closed values
`not_acquired`, `extraction_unavailable`, `not_retained` and `redacted`. It must be
non-null whenever `sha256` or `content_ref` is null, and null when both are present.
A null `sha256` requires `not_acquired` or `extraction_unavailable`; these states
also require null `content_ref`. `not_retained` and `redacted` describe unavailable
retained content after hashing and require a non-null digest and null `content_ref`.
Redacted bytes, if retained, are a separately identified derivative, never content
matching the original artifact's digest. The next schema PR must encode these
conditional rules and the closed enum explicitly; free-form explanations belong
in diagnostics, not new reason values.

`transform` records operation/version, nonsecret configuration identity, ordered
input artifact references and exclusions. No transformation mutates the source.
`mapping` is `exact`, `derived`, `approximate` or `unavailable`, names coordinate
spaces and references its mapping data/reason. Derived means a recorded transform,
not an automatically invertible or editable mapping. Many-to-one normalization
cannot be represented as a fabricated one-to-one offset array.

All offsets are zero-based, half-open and explicitly tied to a representation.
Current scalar-to-original-byte arrays retain every scalar start plus the final
end boundary. Integers must be nonnegative and at most 2^53−1 for exact web/JSON
interoperability; out-of-budget representations fail rather than round. UTF-16
viewer offsets are presentation coordinates, never source authority.

For unstructured C2PA text, the binding/exclusion representation can be NFC-normalized
UTF-8 rather than original-file bytes. Record that representation independently;
validate its exclusions and retain source mappings honestly. This is a coordinate
requirement, not a selected verifier implementation. See the
[C2PA text binding specification](https://spec.c2pa.org/specifications/specifications/2.4/specs/C2PA_Specification.html).

## 7. Typed anchors and result boundaries

Each anchor has `anchor_ref`, `kind`, `artifact_ref`, `execution_ref`, `mapping`
and `locator`. `artifact_ref` and `execution_ref` identify its artifact and execution; they
may be null only for a `legacy_unknown` anchor whose original report lacks that
identity. `mapping` states mapping quality, and `locator` is a versioned closed
variant containing the kind-specific location/scope described below. The next schema PR must provide distinct
closed variants; it must not use an unrestricted location dictionary.

| Anchor kind | Required evidence | Interpretation |
| --- | --- | --- |
| `text` | Segment/artifact reference, ordered scalar/byte spans and exact selected-text hash | Exact occurrence only after paired-coordinate validation; no gap-filling between occurrences |
| `structural_object` | Source/part/object reference, parser-defined versioned locator and object identity when available | Navigate a structure; not a compressed-ZIP byte deletion instruction |
| `credential` | Manifest reference, optional assertion locator and verification execution context | Inspect the credential and its separate validation results |
| `statistical_sample` | Exact sample artifact, execution and declared source mapping | Navigate analyzed scope; never infer individual watermark-bearing bytes |
| `legacy_unknown` | Original report pointer plus adapter diagnostic | Inert inspection of unsupported or ambiguous historical locations |

For a disjoint text anchor, selected-text hashing covers a versioned ordered list
of each span and its exact text; it must not concatenate away span boundaries.
The frontend Occurrence contract is a separate consumer record binding the report
artifact hash, anchor/segment, exact spans and originating finding/selection/query.
Backend local references do not replace its deterministic identity contract.
Source verification, coordinate validity and future edit eligibility remain separate.

A result record has `result_ref`, `execution_ref`, `kind`, `contract_version`,
`anchor_refs`, `payload` and `limitations`. Payloads are closed per registered
kind/version. Initially define only `structural_scan` (`observations_present` or
`no_observations`, with the declared operation and exact analyzed scope); existing
findings carry its actual observations. A zero-findings filtered view cannot change
the underlying result. Operational parser/acquisition completion needs no signal result.

Reserve, but do not emit or invent schemas for, credential verification and vendor
statistical payloads until their adapter specifications are reviewed. A credential
contract must separate discovery, manifest parsing, signature, asset binding,
trust-policy evaluation and unavailable checks. A statistical contract must preserve
actual detector categories/score meanings; inconclusive is not failure or negative.
Opaque retained response bytes are evidence only, not an unchecked `payload` escape
hatch. Unsupported result variants are displayed as unsupported/inert by importers.

`profile_assessments` is an empty array in the initial v2 implementation. Its future
closed variants reference preserved findings/artifacts plus profile/rule revision,
context and rationale. Do not add `expected_artifact` or `statistical_result` to
finding classifications. Human judgments, saved rules and approvals remain separate
versioned sidecars and cannot rewrite any execution/result.

## 8. Stable failures and aggregate behavior

Diagnostics contain `diagnostic_ref`, nullable `execution_ref`, `stage`, `code`,
`message`, `scope` and a closed stage-specific `details` variant. Code meanings
are stable; unknown codes display generic failure, never success. Human messages
remain untrusted display text and are never parsed to derive codes.

Acquisition/parser failures retain `parser.failure` for continuity. V2 requires
`evidence.failure_code` in that finding, matching its associated diagnostic code.
Keep existing error type, message and decode byte ranges as supplementary evidence.
Do not recast verifier/sidecar or report-output errors as parser failures.

| Initial code | Structured condition / caution |
| --- | --- |
| `file.not_found` | Acquisition yields ENOENT / `os.ErrNotExist` |
| `file.permission_denied` | EACCES/EPERM / `os.ErrPermission`; do not infer missing no-atime support from permission denial |
| `file.symlink_not_supported` | Source-policy symlink rejection positively established; ELOOP alone may reflect an ancestor loop, so use generic I/O failure when ambiguous |
| `file.not_regular` | Verified opened object is nonregular |
| `file.too_large` | Before-read size or bounded-read count exceeds configured limit |
| `file.changed_during_read` | Snapshot identity/stat checks fail |
| `integrity.no_atime_unavailable` | Platform reader or explicit capability check establishes the required mechanism is unavailable |
| `file.io_failed` | Other open/stat/read failures without stronger evidence |
| `format.unsupported` | Identification completed without an implemented parser |
| `text.decode_failed` | Typed strict-decoder error; preserve precise byte range/reason |
| `audit.failed` | Unclassified orchestration failure, not guessed from prose |
| `capability.not_implemented` / `capability.detector_access_unavailable` | Registered capability cannot run; not automatically an audit error when disabled |
| `execution.disabled` / `execution.unsupported_input` / `execution.prerequisite_failed` / `execution.policy_denied` | Reason an execution did not run |
| `execution.resource_limit` / `execution.timeout` / `execution.canceled` / `execution.failed` | Bounded execution cannot complete; preserve valid partial scope if any |
| `output.write_failed` / `output.close_failed` / `output.create_failed` | Command output stage; no claim that a full report was delivered |

Acquisition must select typed codes at the failure boundary, preserve wrapped
errors for `errors.Is/As`, and never perform unsafe fallback reads to obtain a more
specific code. Do not add racy path-following probes solely to relabel an error.
A diagnostic cannot claim the cause is more specific than available evidence.

Report status becomes `completed`, `partial`, `failed` or `canceled`:

- Acquisition or parsing failure: failed, exit 4; no invented decoded evidence.
- Required operation failed/unavailable/unsupported/policy-denied: failed if it
  produced no usable analysis anywhere, otherwise partial; exit 4 either way.
- Required operation with execution state `partial`: report status `partial`,
  exit 4; preserve valid scoped results and explicit exclusions (EC-07).
- Optional requested operation failed or is partial/unavailable: partial, exit 4.
  “Optional” permits retaining independent results, not a silent successful run.
- Disabled optional declarations: not_run, no failure finding, do not alter status
  or severity-based exit; report their missing coverage prominently.
- Canceled run: canceled before usable analysis, otherwise partial; exit 4.
- All requested operations completed: completed; exit 0–3 from findings, even if
  an actual completed detector's legitimate result is inconclusive.

For completed reports, preserve current severity exit rules and finding counts.
Initial disabled declarations add no findings or exit penalty. Failure/cancellation
precedence applies to all views. Invalid arguments/output delivery failures return
4; a failed output write cannot promise a valid JSON report on the same channel.
Structured stderr diagnostics carry output-stage codes without sensitive payloads.
Batch summaries and new exit-code ranges are outside this increment.

## 9. Go implementation boundary and deterministic serialization

Use small explicit types in `internal/evidence` and orchestration in `internal/audit`.
Keep existing analyzer packages; introduce adapters rather than a directory rewrite.
The production registry is code/configuration owned by Aletharsis, not document data.
The implementation PR will translate these conceptual signatures into compilable,
closed types after the schema is accepted:

```text
Capability.Describe() -> registered descriptor
Capability.Assess(environment, input metadata) -> availability/eligibility
Operation.Run(context, immutable input view, effective config) -> scoped outcome
Outcome -> state + coverage + observations/results + diagnostics
Coordinator -> validate outcome + assign deterministic references + finalize report
```

The immutable view cannot expose a source pathname for upstream reopening; Go
slices require private ownership or defensive copies because `[]byte` itself is
mutable. Legacy analyzers that populate text hashes must finish that native stage
before evidence is shared. Cancellation via context is cooperative, not proof of
hard memory/CPU isolation; adapters that cannot honor bounds require a process
boundary or remain unavailable. An adapter crash/invalid response must not corrupt
retained native evidence. No runtime adapter loading or shell commands from input.

Order capabilities by registered ID/revision; executions by stage dependency order,
then capability ID and canonical scope. Order artifacts by parent-before-child
topological traversal. At each step choose the eligible artifact with the smallest
key (`kind`, canonical `representation`, `sha256`, `byte_length`, canonical
`content_ref`, `unavailable_reason`, canonical `parents`, canonical `transform`,
canonical `mapping`). All these fields are defined for every artifact in Section 6;
nullable values sort before non-null values, and byte lengths compare numerically.
Preserve declared parent-list order when canonicalizing it; replace parent references
with their already assigned traversal ordinals in ordering keys. No artifact's own
`artifact_ref` participates in its key. The resulting traversal position determines
its `artifact_ref` ordinal. A graph with no eligible node while nodes remain is
invalid, not a reason to use insertion order. Sort anchors
by (`execution_ref`, `artifact_ref`, `kind`, canonical `locator`, canonical `mapping`),
with null references ordered first. Sort results by (`execution_ref`, `kind`,
`contract_version`, canonical `anchor_refs`, canonical `payload`, canonical
`limitations`). Scope is carried by the anchor's locator and its execution's
requested/analyzed scope, not an undefined producer field. Canonical values use
JCS bytes compared lexicographically. Reference-valued keys use the referenced
record's deterministic ordering key until final IDs are assigned, never provisional
worker IDs; a record's own reference is excluded from its ordering key. Findings
retain current severity/rule/location ordering with a complete payload tie-breaker.
Assign references after sorting; rewrite all references together and reject dangling
or cyclic artifact lineage. Identical indistinguishable records get deterministic
occurrence ordinals.
Concurrent runs must not use global mutable state or completion time as an ID seed.

For new configuration and composite identity digests, use SHA-256 over UTF-8 JCS
serialization of a version-tagged envelope, for example
`{"domain":"aletharsis.config/1","value":{...}}`. Use
[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) conformance vectors rather than
assuming Go's existing JSON encoder is JCS. This is distinct from the ASCII-escaped
report serialization and exact imported-report hash. Reject duplicate keys, invalid
Unicode and unrepresentable numbers; never normalize strings as part of identity
serialization. The next PR supplies executable cross-language vectors before any
consumer depends on these digests.

Local reports exclude wall-clock durations, random IDs and ambient timestamps;
operational timing belongs to logs. Configuration/catalog versions and unavailable
capability state are part of deterministic inputs. Future trust/remote execution
contracts explicitly retain evaluation time, trust snapshot and response identity;
reproducibility is scoped to that captured context, not a timeless live-service claim.

## 10. Schema migration and legacy import

1. Preserve `schemas/report.schema.json` as schema 1.0 during rollout. Add a distinct
   v2 schema and examples only in the next reviewed PR; use no broad fallback for
   unknown finding/result variants. Preserve frozen reference files byte-for-byte.
2. Implement version-dispatched consumer validation. A schema-1.0 import retains
   original bytes/hash, findings/status/locations and an external legacy adapter
   record. Per-mechanism execution coverage is **unknown**, even for a completed
   zero-finding report. Do not pretend to know the exact registry/configuration or
   add failure codes to original evidence by parsing messages.
3. For legacy parser failures, preserve failed status and diagnostics; expose null
   stable code plus `legacy.failure_code_unavailable` in adapter metadata. Unknown
   report versions remain inert unsupported imports; do not guess their layout.
4. Proposed rollout syntax: `--report-version 2.0`, initially opt-in; `1.0` remains
   the default until a separately reviewed release switch. These flags do not exist
   yet. CLI help, all views, output routing and schema dispatch change together.
5. Native v1 output continues only for its existing text scope. A v1 request with
   v2-only requested operations must fail before running them; never discard
   credential/statistical results or partial execution to manufacture a v1 report.
   The finite baseline capability declarations alone do not forbid legacy output;
   that output retains its documented inability to express coverage.
6. Cross-version tests project new native findings by removing only v2 additions
   and the new failure code, then apply the existing documented parity exclusions.
   Compare source/evidence/locations/severity/confidence/exit semantics with all
   37 original and 210 seeded cases. Keep direct independent Go and executable
   tests; do not regenerate expected artifacts from the new implementation.

The frontend import adapter binds the actual report artifact hash and validates
references, scope, bounds and coordinate correspondence before creating usable
anchors. A valid schema is necessary but insufficient. Legacy unknown locations
remain inspectable without fabricated exact highlights or edit targets.

## 11. Design examples and acceptance matrix

[Design vectors](examples/evidence-contract-scenarios.json) contain input bytes and
expected selected behaviors, **not complete v2 reports** and not current-schema
fixtures. The C2PA case is a synthetic contract scenario, not a signed credential
or interoperability claim. The next schema PR must produce complete positive and
negative wire examples; actual C2PA fixture validation belongs to Epic #21 G2.

| ID | Scenario | Required result |
| --- | --- | --- |
| EC-01 | `a` + U+200B + `b` | Existing LOW finding and exact scalar [1,2)/byte [1,4) anchor; structural scan completed; disabled C2PA/statistical declarations have no result |
| EC-02 | Clean ASCII, Anthropic detector unavailable and disabled | Zero native findings, completed structural scope, exit 0; explicit not_run statistical coverage, no negative statistical result |
| EC-03 | Discovered synthetic C2PA carrier; verification disabled/unavailable | Preserve carrier observation, exact extracted manifest identity and independent not_run verification; no signature/trust conclusion |
| EC-04 | Same input, unavailable detector explicitly required | Retain native evidence; partial status and exit 4 with capability reason, no detector result |
| EC-05 | Legacy schema-1.0 completed report with no findings | Preserve original report bytes/hash/status; coverage unknown, not inferred completed/negative |
| EC-06 | Truncated UTF-16 or permission error | Stable structured failure code, native failure details and source identity only when acquired; downstream stages not_run |
| EC-07 | Requested detector produces valid scope then times out | Partial scope/results plus exclusions, timeout diagnostic, exit 4; no whole-sample negative |
| EC-08 | NFC combines two scalars before a carrier | Binding offsets differ from source; separate identities, declared mapping, no guessed inverse edit span |
| EC-09 | Filtered view omits Unicode findings | Actual executions/results unchanged, counts view-specific, references valid, no negative inferred from filtered count |
| EC-10 | Concurrent scheduling and repeated run | Same deterministic references and bytes for identical effective inputs/configuration |
| EC-11 | Hostile/imported references and output | Reject cycles, duplicate refs, unknown variants, invalid bounds/UTF-16 confusion, oversized results and unknown enums; escape display content |
| EC-12 | Upstream response or file URL requests access | No ambient network, source reopening, preference write or secret logging; explicit policy outcome |
| EC-13 | New profile marks isolated characters expected | Full sequence analysis still runs; original finding/result retained |
| EC-14 | Output creation/write/close failure | Exit 4 and correct output-stage code; source untouched, no claim of delivered report |

Native regression gates: Go minimum/development versions, race/uncached tests,
compiled CLI/schema validation, integrity/fault tests, frozen artifact verification,
reference-tool tests and native supported-platform checks. Measure report growth
against the published resource corpus before default-switch approval. No universal
latency/RSS budget is invented here; numeric limits require measured review.

## 12. Review decisions and follow-on PRs

This design requests explicit review of: schema 2.0 and opt-in rollout; separation
of availability/participation/execution; code registry and exit-4 precedence for
requested incomplete operations; report-local identities plus JCS composite hashes;
and preserving current finding payloads with independent provenance/result records.
Any rejected choice must be resolved in this design before schema implementation.

Follow-on review units:

1. **Schema and conformance fixtures:** closed v2 object/variant definitions, complete
   examples for native/unavailable/failed states, invalid semantic cases, JCS vectors,
   explicit legacy adapter contract and import validation tests. No live C2PA payload
   or vendor schema invented. Define all required/null fields and scope variants.
2. **Go model and emitter:** typed errors, registry/planning, native analyzer adapters,
   deterministic references, version selection, console coverage and compatibility
   tests. Production v1 remains default during review.
3. **Consumer/release acceptance:** frontend Occurrence adapter alignment, resource
   spike and supported-platform evidence; deliberate default-switch decision with
   migration documentation. Adapter adoption stays behind Epic #21 gates.

This PR is design only and does not complete #17, Epic #21 G1, the frontend design
gate or #7. Link accepted decisions and implementation evidence to those trackers;
keep outstanding checklists open. Future API transport, batch accounting, concrete
C2PA/statistical result variants, profile rules and writers remain separately gated.
