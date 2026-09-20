# Report 2.0 wire contract and legacy import

| Attribute | Value |
| --- | --- |
| Document | EC-002, revision 0.1 |
| Status | Proposed schema and conformance gate; production still emits schema 1.0 |
| Depends on | [EC-001](evidence-contract-v2.md), accepted design direction in [PR #23](https://github.com/toddwbucy/Aletharsis/pull/23) |
| Tracking | [#17](https://github.com/toddwbucy/Aletharsis/issues/17), [#21 G1](https://github.com/toddwbucy/Aletharsis/issues/21), [#7](https://github.com/toddwbucy/Aletharsis/issues/7) |
| Machine-readable contract | [report-v2.schema.json](../../schemas/report-v2.schema.json) |
| Conformance suite | [tests/contracts](../../tests/contracts) |

This increment defines wire shapes, portable fixtures and consumer obligations.
It does not add a Go emitter, CLI flag, network detector, parser, frontend or
production importer. Schema 1.0 and all frozen migration artifacts remain unchanged.
Python here is test infrastructure, not a restored application dependency.

## 1. Schema boundary

The v2 schema is self-contained Draft 2020-12. Its `.invalid` identifier is an
identity, never a schema download endpoint. Importers select a bundled schema by
exact `schema_version`; document-controlled schema URLs must not be resolved.
All fixed objects are closed. Finding variants retain the native rule IDs and
payload names, adding only the EC-001 fields and `failure_code` for parser failures.
Native counts/offsets have the web-safe integer ceiling, 2^53−1; exit codes remain
0–4. `profile_assessments` remains empty. The only result payload is
`structural_scan/1`; cryptographic and statistical result schemas are deliberately
absent until their adapter contracts are reviewed.

A schema-valid document is not necessarily valid evidence. A consumer must also
check references, scope, identities, coordinates, aggregate state and budgets.
Validation establishes internal consistency, not that a producer was honest or
that the source still matches. Source verification remains a separate operation.

Unknown diagnostic codes retain their namespaced string and are displayed as
unrecognized failures; they never imply success. Execution state and aggregate
status remain authoritative. Unknown result kinds fail the known schema and are
available only for inert raw inspection, not silently interpreted as results.

## 2. Concrete wire choices

EC-001's conceptual fields have these initial closed shapes:

- Local ordinals start at zero. Diagnostics use `diagnostic/<ordinal>` alongside
  `exec/`, `artifact/`, `anchor/`, `result/` and `finding/` references.
- `supported_scope` has `kinds` and `formats` arrays. Capability limits and effective
  execution limits name `input_bytes`, `expanded_bytes`, `objects`, `depth`,
  `output_bytes` and `execution_ms`. A null dimension is **not enforced by this
  operation**, never a claim of an unlimited safe budget. Adapter acceptance must
  justify each applicable finite limit; outer acquisition limits still apply.
- `implementation` records `id`, `version`, nullable `upstream` and a `data` array.
  An upstream identity carries repository, pinned revision and nullable distribution
  digest. Each data identity carries ID, version and digest. Declarations have null
  implementation. Available capabilities require a concrete implementation.
- `config` initially supports only `aletharsis.native-text-config/1`: revision,
  settings (`limits`, `preprocessing: none`, `data_revision`), digest and null
  `key_ref`. The other variant is explicitly unknown configuration identity.
  Secret references, tokenizers and trust-policy settings require a reviewed adapter
  configuration variant; an arbitrary settings dictionary is not an escape hatch.
- Scope uses `whole_artifact` or ordered half-open `byte`/`scalar` regions. Token
  scopes are not yet supported. Whole-artifact scope can request acquisition of
  an unknown extent; an analyzed scope requires a known extent. Empty documents
  use whole-artifact scope; region spans must be nonempty.
- An exclusion has nullable `scope`, `unknown_remainder` and `reason_code`.
  Unknown remainder requires null scope; known exclusions require a scope.
- Artifact representations identify `serialization`, nullable `encoding`,
  `normalization` and `line_endings`. A retained text pointer serializes exactly
  the pointed-to Unicode string as UTF-8, not JSON-escaped string bytes.
- `content_ref` is either a text pointer into `/evidence/texts/<index>/text` or a
  `retained_blob` digest. Blob resolution uses a separately authorized local store;
  imports do not read paths or fetch URLs. Other embedded representations need a
  reviewed content-reference variant. Artifact absence reasons follow EC-001 §6.
- Transforms name operation/version, nullable configuration digest, ordered inputs
  equal to the artifact's parents, and exclusions. `mapping` is unavailable with
  a reason, or names source/target artifacts, method/version and nullable mapping
  data pointer. The initial mapping pointer is a text `byte_offsets` array.
  A method name alone does not prove an exact mapping: consumers must recognize
  its version and validate its data before granting exact-coordinate navigation.
- Text anchor locator `/1` names the segment pointer, paired scalar/source-byte
  spans and a selection digest. The artifact is the UTF-8 text representation;
  its paired byte spans refer to the **original source encoding**, through the
  recorded text-to-source mapping. They are not UTF-8 text-artifact byte offsets
  when the source is UTF-16/32. Structural locators name format, nullable package
  part, object ID and nullable object digest. They are navigation references, not
  byte deletion ranges. Credential, statistical-sample and legacy-unknown locators
  are closed navigation contracts, not implemented detector result schemas.
- Diagnostics select closed acquisition/parsing/execution/import/output/audit
  details by stage. Unicode decode details retain the byte range and reason.
  Code meaning comes from a typed failure boundary, never message matching.
- `view.finding_categories: []` means no category filter for `audit`. Other view
  names declare a sorted category selection: unicode selects possible_steganography
  and unicode; metadata selects identifier, metadata and provenance; structure
  selects document_structure, embedded_content, hidden_content and visual_watermark.
  Parser failures are always retained independently of those lists.

Source, sample and binding identities are different domains. A structured object's
stored bytes and decompressed bytes require separate representations and digests.
An unknown extraction cannot manufacture a digest or an exact mapping.

## 3. Identity serialization

Configuration digest input is UTF-8 [RFC 8785 JCS](https://www.rfc-editor.org/rfc/rfc8785)
for `{ "domain": "aletharsis.config/1", "value": CONFIG_WITHOUT_SHA256 }`.
Unknown configuration has no digest. Settings, revision, schema and key reference
participate; keys themselves must never be serialized or hashed into this record.

Text selection digest input uses domain `aletharsis.text-selection/1`; `value` is
an ordered array of `{ "span": { "scalar": {"start": S, "end": E},
"byte": {"start": B, "end": C} }, "text": EXACT_SELECTED_TEXT }` records.
Each span's identity remains distinct; gaps are not concatenated away. Report-byte
identity is instead SHA-256 of the exact imported bytes, with no canonicalization.

[Identity vectors](../../tests/contracts/identity-vectors.json) contain literal
expected UTF-8 hex and SHA-256 values for Unicode ordering, normalization-preserving
strings, controls, safe integers and ordered regions. They are portable to Go and
TypeScript. The test oracle implements only the integer/string identity subset
needed by these configurations and selections, and rejects nonintegral floats.
Integral JSON spellings such as `1.0` canonicalize identically to `1`. The file also
reserves explicit number-format vectors for the future general Go JCS implementation.
The current tests check those numeric fixtures' value preservation, **not** a full
floating-point canonicalizer. Full RFC conformance and deterministic reference
assignment remain Go implementation gates before publishing schema 2.0.

## 4. Import and legacy adapter contract

Import proceeds as follows:

1. Acquire report bytes within an explicit import byte/depth/object budget and
   retain those exact bytes according to the workbench retention policy.
2. Compute and retain `report_artifact_sha256` from those bytes before interpreting
   evidence. Never use a reserialized report as the imported identity.
3. Decode strict UTF-8 JSON; reject duplicate keys, invalid Unicode, non-finite
   numbers, unsafe coordinate integers and exceeded budgets. Invalid reports are
   quarantined for inert inspection, never partially trusted.
4. Dispatch by exact version. Schema 1.0 uses the unchanged bundled schema; 2.0
   uses the new bundled schema plus semantic checks. Unknown versions remain
   `unsupported_version` with unknown coverage. Do not guess by field layout.
5. Preserve the original report. Store consumer annotations separately. Imported
   paths, messages, snippets, rules and URLs are untrusted data, never instructions.

The **test-only** adapter returns these required consumer fields:

| Field | Contract |
| --- | --- |
| `report_artifact_sha256` | Lowercase SHA-256 of exact import bytes |
| `status` | `legacy`, `validated`, or `unsupported_version` |
| `coverage` | `unknown` for legacy/unsupported; `declared` for validated v2 |
| `report` | Original decoded object, with no synthesized or rewritten fields |
| `adapter_diagnostics` | Separate legacy diagnostic records, never source findings |

Malformed known-version imports fail validation rather than obtaining one of these
successfully parsed statuses. A legacy parser failure yields a sidecar diagnostic
with code `legacy.failure_code_unavailable`, original `/findings/<index>` pointer
and `failure_code: null`. Its message does not permit inference of a stable code.
Legacy empty findings do not mean all mechanisms were examined. Do not synthesize
v2 executions, negative results or capability availability for historical reports.

Legacy locations may be exposed as candidate coordinates only after verification
against the exact reported representation. Ambiguous locations remain inert with
the original JSON pointer. No legacy import acquires edit authority. A production
frontend adapter must implement the independent Occurrence contract and source
verification, including report hash, source hash and exact span identity.

The oracle uses an 8 MiB report limit and depth 64 for these small tests. These are
**test harness limits**, not a frontend product limit: report expansion and the
large-report spike must determine production import budgets. No source path,
external schema, retained blob or network resource is opened during import.

## 5. Required semantic validation

Consumers and the future Go emitter must enforce:

- Unique typed references, no dangling links, an accounted-for execution for each
  declared capability, and acyclic parent-before-child artifact lineage.
- Source/file identity agreement; retained text length/digest agreement; ordered
  transform inputs and honest absent content. A reported blob digest is not proof
  that the blob is present or its bytes have been verified.
- Bounded ordered nonoverlapping regions; analyzed regions inside the request;
  nonoverlapping known exclusions; partial coverage accounted for by regions or
  explicit unknown remainder. A partial execution requires retained usable results.
- Completed operations cover the entire declared request. Unrun/failed/canceled
  operations produce no substantive results. Disabled or unavailable operations
  cannot run. Structural scan results cannot be emitted by a statistical detector.
- Text boundary count, encoding widths, paired anchor coordinates and selection
  digests. Unknown mapping methods stay unverified. NFC-derived coordinates cannot
  substitute for original-source byte coordinates.
- Finding/anchor/execution linkage, matching parser failure codes and diagnostics,
  accurate view counts, and operational failure precedence over severity exits.
- No finding filter changes evidence, result outcome or execution coverage.

The conformance oracle covers the initial native-text fixtures and these mutation
classes. It is not a general structured-format validator or production security
boundary. Format-specific locator validation, general transform verification,
source reacquisition, remote response authenticity, deterministic sorting under
concurrency and full resource isolation require their respective implementation
and integration tests. Unknown maps must not gain authority from schema validity.

## 6. Fixtures and gates

Eight complete synthetic v2 reports cover clean text, a Unicode observation,
required detector unavailability, decode failure, acquisition failure, cancellation,
partial timeout and a filtered observation. Their `conformance-subset/1` catalog
contains acquisition, parsing, Unicode inventory and disabled provenance declarations.
It is deliberately a **test subset**, not the complete future production registry.
All three mechanism classes coexist as declarations; only structural results run.
No fixture is a vendor-positive watermark or a verified credential.

Native evidence in these fixtures comes from the existing frozen public text
fixtures. Timeout, cancellation and denied acquisition are synthetic orchestration
scenarios. A partial negative applies only to its completed region. A filtered
empty finding list preserves the original positive structural result.

Tests additionally cover every existing finding payload, all frozen v1 imports,
lossless native payload projection, artifact absence variants, anchor shapes,
malformed references/coordinates, unknown result types, malicious import strings,
unsafe offsets, aggregate failure semantics and exact report-byte identities.
The existing suite still validates schema 1.0 and the unchanged frozen manifest.

Run locally:

```bash
python -m pytest -q
```

Review this schema and import contract before implementing the Go registry/emitter.
The next PR must exercise the same fixture expectations through Go types, implement
stable failure codes at their typed boundaries, run native parity/integration tests,
and test deterministic ordering/JCS. Neither #17 nor Epic #21 G1 is complete until
that implementation and consumer compatibility evidence are accepted. A production
output-version default change requires separate release approval.
