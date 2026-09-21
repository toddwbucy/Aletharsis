# EC-003 — Adapter evidence report migration design

| Attribute | Value |
| --- | --- |
| Status | Proposed design; wire schema and Go acceptance remain separate review units |
| Tracking | #36, reuse epic #21, ongoing validation #7 |
| Prerequisite | DA-001 accepted in PR #49; feasibility PRs #47/#48 |
| Target | Report `schema_version: "3.0"`, opt-in only after implementation acceptance |
| Baseline | Schema 1.0 default; schema 2.0 opt-in; neither changes in this design |

This is the next design increment, following the design → wire/fixtures → Go
implementation sequence used for report 2.0. It does not satisfy DA-001's complete
migration gate by itself. In particular, full report fixtures, Go marshal/import
parity and consumer validation must precede adapter implementation. The accompanying
[decision matrix](examples/report-v3-migration-cases.json) is a set of executable
planning expectations, **not report-3.0 wire fixtures**.

## 1. Decision and non-goals

Introduce a major report version because current consumers use closed result,
configuration, artifact and diagnostic contracts. Do not relax schema 2.0, accept
arbitrary provider maps or rename an adapter outcome to `structural_scan`.
Existing mechanisms (`structural`, `cryptographic`, `statistical`), participation
and execution-state enums remain sufficient. No new AI-authorship classification,
reviewer judgment, implicit trust or cleanup authority enters detector evidence.

DA-001 describes what a worker may return. EC-003 describes what the host may
promote into a forensic report, what remains opaque, and how consumers distinguish
validated graph consistency from verified source bytes and truthful provider claims.
Provider response validation never establishes that the provider itself is correct.

No SDK adoption, worker launcher, secret delivery, remote detector, multi-file job,
new source format, signing, semantic rewriting or default-version switch is part
of this migration. Native output/fixtures and their hashes remain unchanged.

## 2. Envelope and version dispatch

Retain every report-2.0 top-level member with its meaning. Add required
`adapter_runs: []` for native-only reports. Each entry joins exactly one adapter
execution by `execution_ref`; native executions must not have one. Every planned
adapter execution has a run entry, even if not run. Host catalog planning, not an
untrusted report or a filename, determines which operations are adapters.

Reference syntax remains the existing canonical ordinal syntax; no new global ID
space. Arrays use deterministic host planning order, independent of worker finish
time. Run entries follow execution order. Result references remain local to the
exact imported report artifact. Request digests may repeat across runs; they do
not replace execution references or constitute replay authorization.

Known-version import validates byte budget, strict JSON, bundled wire schema and
cross-record semantics before exposing usable results. No network schema/blob
resolution. An older importer encountering 3.0 retains exact bounded input bytes,
computes their SHA-256 and reports `unsupported_version` with unknown coverage.
It must not reinterpret missing understood results as a clean audit. A 3.0 importer
retains 1.0/2.0 behavior, preserves original bytes, and never silently upgrades them.
No automatic export to older versions that drops evidence. An explicitly scoped
native-only re-audit is a new report, not a conversion of an adapter report.

## 3. Adapter run record

All listed keys are required; nullable is explicit. Fixed objects are closed.
`artifact_ref` and `execution_ref` below use existing local reference types.

| Field | Type and meaning |
| --- | --- |
| `execution_ref` | Unique reference to this operation's execution |
| `protocol` | Literal `aletharsis.adapter-exchange/1` |
| `request_id` | Nullable DA-001 JCS request digest |
| `request_ref` | Nullable `adapter_request` artifact, exact request-header JSON bytes |
| `response_ref` | Nullable `adapter_response` artifact, exact accepted response-header JSON bytes |
| `raw_result_ref` | Nullable `detector_response` artifact, exact provider bytes |
| `quarantined_refs` | Ordered, unique artifact references to rejected bounded output; never usable results |
| `launch_profile` | Nullable object `{id, version, policy_ref}`; policy_ref names exact retained policy bytes |
| `started` | Boolean host observation: whether the worker was launched |
| `cleanup` | `not_started`, `complete`, or `failed` |

A planned unavailable declaration may have null request/launch identity, but still
has a concrete reason and no results. Once a request is prepared, retain its exact
bytes and digest even if input validation prevents launch. A started run requires
request identity and launch-profile identity. `started: false` requires
`cleanup: not_started`; `started: true` requires complete or failed cleanup. `not_started` is permitted for any
host preflight refusal; DA-001's synthetic host-outcome profile is stricter about
its fixture cleanup label, so the mapper records actual launch state rather than
inferring it from the fixture label alone. Add host launch-state evidence to the
implementation conformance gate; a report may not invent whether a process ran.

Completed/partial runs require request, response and raw-result artifacts,
`cleanup: complete`, and no quarantined response promoted as accepted. Failed or
canceled runs have null response/raw-result refs; optional rejected bytes use
quarantined_refs. A cleanup failure records `cleanup: failed`, produces no usable
adapter result and blocks further launches. Preserve both the original termination
cause and the cleanup failure in diagnostics. No raw path or worker-named filename
is a blob reference. Reading an imported report does not access any blob store.

## 4. Artifacts, transforms and mapping

Retain the existing artifact kinds, including `manifest`, `sample`, `binding_text`
and `detector_response`. Add only `adapter_request`, `adapter_response`,
`configuration`, `policy`, and `trust_material` for exact-byte provenance.
These use `retained_blob` content references. Request/response hashes exclude
transport length prefixes; raw provider hashes cover bytes before interpretation.
A request ID is the DA-001 domain-separated canonical digest, **not** the exact
request-header artifact digest. Validate both independently when bytes are available.

DA-001 input kinds map explicitly: source → source, decoded/normalized → text
(with their respective representation metadata), binding → binding_text, sample →
sample. The request digest/length must match the selected artifact, not merely
another artifact with a similar locator.

Original source remains the sole `source` artifact and must match `file.sha256`
and size. Config/policy/trust artifacts are independent retained inputs, not
children of the inspected source. A manifest is derived from the actual acquired
input, not from decoded provider JSON. Provider response artifacts record their
input parents and the producing operation/version/config digest. A missing manifest
identity never becomes a synthetic hash of an assertion or a certificate name.

Representations retain serialization, encoding, normalization and line-ending
semantics. Empty, absent and unavailable are distinct; nullable digests require
an explicit absence reason under the existing artifact contract. Successful
adapter evidence requires retained non-null identities even if a later consumer
cannot resolve the blobs locally. Import can validate reference consistency without
having the bytes; independently checking hashes/content remains a separate consumer
state, never an implied side effect of import.

Extend `mapping` with a closed inline byte-map variant rather than overloading the
v2 `/evidence/texts/.../byte_offsets` pointer:

```text
quality: exact | derived
from_artifact_ref: artifact/<n>
to_artifact_ref: artifact/<n>
method: aletharsis.byte-map
version: 1
pairs: [{source: {start, end}, target: {start, end}}]
exclusions: [{start, end}]
```

This variant has no `data_ref`. Existing native pointer maps and unavailable maps
remain explicit alternatives. Coordinates are half-open artifact-byte ranges.
Validate bounds, ordered complete target coverage, nonoverlapping exclusions and
no source pair crossing an exclusion. Derived source spans may reorder or repeat;
exactness additionally requires identical selected bytes verified against retained
artifacts. The outer source map is obtained only by validating every transformation
in the chain, never by copying the final stage's offsets. Inline pairs are a
coordinate claim, not proof that the transform algorithm executed correctly.

Normalization, BOM handling and carrier exclusion retain operation/version/config,
parents and explicit exclusions. A zero-change transform may preserve the digest
while representing a distinct stage; artifact references, not distinct hashes,
identify graph nodes. Acyclic parents are ordered before children. Neither a
successful credential result nor a source hash confers navigation or edit authority.

Add a byte-region anchor variant for localized carriers:
`kind: artifact_bytes`, locator `{version: "1", spans: [{start, end}]}`.
Its artifact is the adapter input, and its map to source may be unavailable.
This avoids assigning decoded Unicode scalar coordinates to binary carrier spans.
Native text anchors preserve their original-source encoding semantics; do not
reinterpret them as UTF-8 artifact offsets. Existing credential locators require
a real manifest artifact. Without one, retain the verification result without
manufacturing a credential locator. Statistical sample anchors remain whole-sample
scope; no token-selection score becomes an exact deletion span.

## 5. Configuration and resource identities

Keep native and unknown configuration variants unchanged. Add a closed variant
`schema: aletharsis.adapter-config/1`, with required revision, null `key_ref`,
sha256 and settings:

```text
protocol: aletharsis.adapter-exchange/1
operation: extract | verify | statistical
provider_config_ref: artifact/<n>
policy_ref: artifact/<n>
trust_ref: artifact/<n> | null
validation_time: RFC3339 timestamp (exact request value)
limits: {wall_ms, cpu_ms, memory_bytes, input_bytes, output_bytes, scratch_bytes}
```

All adapter limits are positive and obey DA-001 ceilings. Execution config digest
uses the existing `aletharsis.config/1` domain over the variant excluding sha256.
Referenced raw-byte identities are also bound by the retained DA-001 request;
validate that request's config/policy/trust hashes match their artifact bytes.
Since config refs are report-local, its digest is report-contextual, not a portable
replacement for provider options or the request digest. Consumers must retain the
report identity when citing it. A trust_ref of null explicitly means no supplied
trust material, not an unrecorded system store. No ambient clock/trust/telemetry
settings. Effective upstream options must match the pinned adapter mapping.

Continue recording the native execution limit shape for common bounds:
input_bytes and output_bytes equal adapter caps, execution_ms equals wall_ms;
expanded_bytes/objects/depth disclose parser-specific bounds where enforced.
The adapter config additionally binds CPU/memory/scratch limits. Conflicting limits
invalidate the report. Null common dimensions do not erase a finite adapter cap.
A future keyed statistical algorithm needs a separately reviewed secret-reference
contract; this migration does not add keys or secrets to report settings.

## 6. Closed result variants

Retain `structural_scan/1` for native analyzers unchanged. Add three discriminated
result kinds with `contract_version: "1"`. Common envelope fields remain
result_ref, execution_ref, kind, contract_version, anchor_refs, payload, limitations.
The run supplies raw_result_ref; there is no untyped provider object in payload.
Exactly one aggregate result is required per completed/partial adapter execution.
No results are permitted from not_run, failed or canceled executions.

| Result kind | Role / mechanism | Closed payload fields |
| --- | --- | --- |
| `carrier_extraction` | analyzer / structural | operation, scopes, outcome, input_ref, spans, manifest_ref |
| `credential_verification` | verifier / cryptographic | operation, scope, input_ref, discovery, manifest_ref, signature, binding, trust, revocation, freshness |
| `statistical_analysis` | analyzer / statistical | operation, scope, sample_ref, algorithm, algorithm_version, configuration_sha256, outcome, score |

`operation` equals the capability ID; it is not DA-001's generic selector. Register
`aletharsis.c2pa.carrier` and `aletharsis.c2pa.verify` at revision 2 only when their
actual implementation scope is accepted. Their v2 declarations remain revision 1.
Statistical capability IDs must name an approved detector/algorithm profile and
purpose; no generic SynthID capability implies Anthropic detector access. Until
adoption, production availability remains unavailable/disabled.

Extraction `scopes` equals ordered execution analyzed_scope. `input_ref` identifies
the selected DA-001 input artifact (`source`, `text`, `binding_text`, or `sample`);
`request_ref` identifies the `adapter_request` header artifact. Spans refer to the
checked regions of the selected input artifact. Outcome/spans/manifest rules
are exactly DA-001: absent has no spans/manifest; observed/malformed/ambiguous have
spans; only observed may have a manifest. An absent result from partial extraction
applies only to its scopes. Parsing disagreement or unsupported localization cannot
be converted to absent. An incomplete operation requires explicit exclusions and
at least one usable checked region/result.

Verification and statistical results require completed whole-input coverage in
DA-001/1. Verification enums match DA-001 independently: signature-valid does not
imply binding-match, accepted trust, checked revocation or current freshness.
Absent discovery requires null manifest and all checks unevaluated. Evaluated trust
requires a bound trust artifact. Present discovery with unavailable manifest bytes
must include a limitation. Scope and input_ref identify the bytes actually checked,
not automatically the outer source or the extracted manifest.

Statistical sample_ref resolves to an identified sample artifact in the source
transformation chain. Configuration_sha256 is the exact provider config bytes hash,
not the report configuration digest. A nullable score, when present, has exactly
value, units and semantics_ref; preserve finite numeric semantics and provider
outcome wording. No probability conversion or universal threshold. Provider score
numbers are finite binary64, unlike bounded integer offsets; reject nonfinite values
without imposing an invented probability range. Provider strings are inert data.

Initially adapter results do not create new finding IDs or severity classifications.
Existing native findings retain their schemas. Reporting must show adapter result
and execution panels even when findings is empty. New finding projections require
separate detector-specific review, closed evidence variants and explicit severity
semantics. A completed signature failure is substantive evidence, not a process
failure; with no severity findings the audit may return 0 while reporting that
failure prominently. Exit code means operational/severity policy, not credential
validity. Pipelines requiring valid credentials inspect the typed result dimensions.

## 7. Host outcomes, diagnostic codes and coverage

The [machine-readable plan](examples/report-v3-migration-cases.json) enumerates every
DA-001 terminal reason and its proposed namespaced report mapping. Preserve existing
execution codes where they already express the cause. Add adapter-specific codes
for malformed/oversized output, source mismatch, provider failure and cleanup failure.
The word `unavailable` alone does not identify a root cause: use the host's capability
availability reason (missing runtime, unsupported launch profile, policy denial,
disabled or not provisioned), with `capability.not_implemented` reserved for actual
nonimplementation. Never choose a reason by parsing provider error prose.

Add diagnostic `stage: adapter`, closed details
`{protocol, primary_code, cleanup, limit, limit_value}`. Primary_code is nullable
only when there is no prior cause; it preserves timeout/cancellation/provider
failure when cleanup itself fails. Limit/limit_value are both null or describe
one DA-001 resource dimension and safe positive value. Preserve existing diagnostic
shapes for native stages. Unknown codes remain unknown diagnostic evidence, never
successful execution or proof of safe input.

Not-run/failed/canceled executions have empty analyzed_scope and no accepted results.
Completed execution covers its request. Partial extraction retains analyzed regions
and disjoint known exclusions accounting for all requested bytes. Each omission gets
`adapter.partial_scope`; this describes unassessed scope, not a suspicious artifact.
Quarantined bytes are not usable results and do not make aggregate status completed.

Retain v2 aggregate precedence: acquisition/parser failure → failed; partial operation
→ partial; cancellation → partial when usable results exist, otherwise canceled;
other incompleteness → partial with usable results or optional failure, otherwise
failed; all active operations completed → completed. Disabled operations are ignored.
Required/optional describe operational policy and do not change result truthfulness.
All noncompleted reports return 4; completed reports retain severity-based 0–3.
Filtering findings cannot change capability coverage, results, status or failures.

## 8. Import, presentation and rollout

Separate consumer states: wire/graph validated; referenced bytes available;
referenced hashes verified; exact source correspondence verified; provider signature
result; trust-policy result; reviewer assessment. None substitutes for another.
Import never resolves a content_ref path/URL, starts a worker, fetches a trust list,
executes HTML or authorizes a derivative. Render strings/filenames as untrusted text.

Console/JSON consumers must retain capabilities/results when a finding view filters
out native findings. Display unavailable and partial operations beside completed
ones. A statistical sample gets an analysis-scope panel, not a hidden-byte highlight.
An artifact-byte carrier anchor highlights source only after its map is verified.
A valid credential does not certify assertion truth or make embedded instructions
legitimate. Raw blobs are opt-in, locally authorized evidence with restricted access.

Implement explicit `--schema-version 3.0` only after producer/import acceptance.
Default remains 1.0 until a separate product decision. Existing 1.0/2.0 flags,
outputs, exit codes and reference hashes are regression requirements. If a user
requests an adapter under an older schema, refuse with a clear unsupported-contract
error; never drop its output or silently switch versions. Unknown imported versions
remain inert. Rollback disables adapters without making historical reports unreadable.

## 9. Review units and exit gates

1. **This design:** review the field authority, result distinctions, migration and
   consumer policy; executable planning cases validate DA-001 mapping coverage.
2. **Wire and full fixtures:** implement self-contained closed report-v3 schema,
   native-only and mixed reports, offline graph validator and digest/coordinate
   vectors. Include all outcome rows plus malformed graphs; keep 1.0/2.0 frozen.
3. **Go model/import:** typed variants, bounded strict decode/encode and semantic
   validation. All full fixtures pass Go/Python parity; malformed fixtures fail
   both. Import preserves exact bytes and unknown versions; no worker execution.
4. **Producer/consumer opt-in:** deterministic native 3.0 reports and synthetic
   adapter injection through test seams, console/result panels, aggregate/exit/view
   coverage and consumer coordinate tests. No production upstream adoption yet.
5. **Migration acceptance:** independent review of full reports and Go parity,
   platform/resource tests, compatibility corpus and rollback evidence. Only then
   can an adoption PR add a production adapter under G6.

Each is a separate reviewable PR under #36/#21, with bounded tests and evidence.
Do not close #36 after this design alone. No new dependencies or external-repository
changes are authorized. Runtime isolation, real provider differential vectors,
license notices and native per-platform checks remain per-adoption gates in #45.

## 10. Design validation record

Baseline: PR #49 merge `7cfc061` (full identity in the decision matrix). Local
validation on Python 3.12.13 / Linux amd64: `python -m pytest -q` passed **265 tests**,
including 24 migration-planning checks; `git diff --check` passed. GNU time measured
2.17 seconds wall, 2.12 user CPU, 0.04 system CPU and 92,280 KiB peak RSS. The isolated
checkout remained below 7 MiB. This design increment is within #36's remaining
12-hour / 1-CPU-hour / 1-GiB initial budget; no provider, SDK, model or remote API
was invoked. Existing CI validates the unchanged Go backend separately.

Planning cases retain exact accepted DA fixture hashes, and structural schema
hashes use Python JSON serialization with sorted keys, compact separators and
ASCII escapes over parsed JSON. Those hashes check schema meaning independently
of checkout line endings; they are explicitly not source/report artifact hashes
or a general RFC 8785 implementation. Full future wire fixtures must use the
existing exact-report identity and canonical composite-identity contracts.

These checks verify the proposed mapping is exhaustive and internally consistent.
They do not prove a future schema or Go implementation conforms. Review units 2–5
remain mandatory; this record is not permission to skip them or close #7/#21.
