# OI-001 — Office CLI evidence and consumer integration

| Attribute | Value |
| --- | --- |
| Status | Proposed; implementation may proceed on reviewable branches, no merge authorization |
| Tracking | #40 (B1–B5), #14, #21, #7 |
| Inspected baseline | `5b1ca93`; subsequent main changes were documentation only |
| Wire target | Explicit opt-in report `4.0`; 1.0/2.0 remain unchanged; 4.0 includes the 3.0 adapter envelope plus Office evidence |
| Authority | Owner authorized version selection and continued work during review; owner retains merges and gate dispositions |

## 1. Scope and delivery

Connect acquired Office packages to the CLI using existing PP-001, XP-001/XM-001,
OPC, DOCX/ODT identity, WT-001/OT-001, WA-001/OA-001 and profile implementations.
Add DOCX core/application metadata and embedded-object inventory. Preserve all
four #40 comparison targets: package parts, metadata, relationships, embedded
objects. ODT extraction is integrated but is not substituted for DOCX comparison.

The initial command is `aletharsis audit FILE --schema-version 4.0`, including
bounded directory/JSONL use. The default remains 1.0. Requests for Office evidence
in 1.0/2.0 yield existing schema-valid typed unsupported-format failures, not
flattened success. This explicitly replaces the goal's earlier requirement to
encode Office evidence in the frozen versions. Plain-text reports and frozen
fixtures remain unchanged in 1.0/2.0.

The 4.0 wire contract extends 3.0, retaining its required `adapter_runs` member
and adapter record/result definitions. Native-only reports emit `adapter_runs: []`.
Office evidence and adapter runs can coexist in 4.0; they are not parallel,
incompatible branches. Office support does not implement adapter execution.
A nonempty adapter run requires the separately reviewed EC-003 runtime and
validation capability; until that exists the native runtime must reject it as
unsupported, preserving the bounded imported bytes rather than claiming validation.
Wire validity and runtime support are separate declarations.

Version dispatch uses exact supported versions, never numeric comparisons.
A new support-aware importer API exposes `support_reason` beside the unchanged
EC-002 `status` (not in its place); the legacy `Read` API and its closed statuses
remain unchanged: legacy `reportimport.Read` continues to dispatch only 1.0/2.0;
all 4.0 input remains unsupported to that older API. The new `ReadSupported` API
performs exact 4.0 dispatch and exposes the 4.0 decoded view only after full
supported semantic validation. Future 4.0 import/CLI consumers must call this
new API; there are no production `reportimport` callers to migrate at the inspected
baseline. Legacy fixtures/tests continue exercising `Read`. The support-aware
API's compatibility status may cover unsupported features as well as versions;
that broader meaning does not change EC-002's legacy API. Import identifies 3.0 as a known contract with unimplemented Go support
(`known_version_unimplemented`), distinct from an unrecognized version
(`unknown_version`). The support-aware result has
`status: unsupported_version` for both, with the distinguishing `support_reason`.
For a wire-valid 4.0 report requiring nonempty adapter runs before that feature is
implemented, retain the same compatibility status with
`support_reason: known_feature_unimplemented`. Supported imports have a null
support reason. These sibling reasons do not rename existing importer statuses,
report failure codes or the historical EC-002/EC-003 behavior.
All unsupported cases retain bounded original bytes/digest and unknown coverage;
no decoded report is exposed as validated. Strict JSON and input limits precede
support dispatch. For implemented versions, structural wire errors remain errors,
not unsupported-feature results. This does not claim adapter semantic validation
on a wire-valid unsupported report. No unsupported case is an audit success or
conversion. Native-only 4.0 reports declare disabled provenance/statistical capabilities as before.

No remediation, renderer, Office writer, external resource fetching, macro
execution, style inference, new profile content, or production Python dependency.

## 2. Evidence representations and exact coordinates (B3)

Each coordinate names its byte artifact. Three representations are distinct:

1. Acquired ZIP container: exact source SHA-256 and byte size. Acquisition is
   unchanged and runs once. Every parser consumes the same immutable snapshot.
2. Decompressed part: exact name, SHA-256, byte size and extraction implementation
   identity, linked to the container. Preserve the existing compressed payload
   span, compression method and compressed digest. These identify compressed
   bytes; they do not map individual XML characters into ZIP offsets.
3. Analysis scope: exact UTF-8 text/hash, scope identity, role and assembly version.
   Office `scope_character_offsets` and `scope_byte_offsets` refer to this artifact.
   Every Office location requires `scope_ref` and `coordinate_artifact_ref`. Every scalar
   retains the existing WA-001/OA-001 origin, including XP/XM segment identity,
   lexical part span and transformation. Generated ODT controls stay derived.

All intervals are zero-based half-open. No normalization precedes source hashing.
`&#x200B;` occupies eight XML bytes and three UTF-8 scope bytes: both spans must
remain inspectable. Cross-run origins remain a list of lexical regions, never an
editable envelope across intervening markup. Duplicate part contents retain
separate part identities. Scope identity binds source hash, part name/hash,
extractor/assembler versions and local scope ID, not just its text digest.

Do not insert Office scopes into legacy `evidence.texts` with invented file-byte
maps. Keep that array and `identity.VerifyText` meaningful for flat files only.
The Office wire variant carries scoped text and origin records separately. Its
verifier reconstructs mappings from the acquired snapshot using the existing
XML mapper. Synthetic controls prove derivation, not literal byte equality.
Unverifiable mappings yield an explicit gap; no guessed exact anchor is emitted.

Structural anchors identify the part digest plus existing element/token index
and part-byte region where applicable. Opaque embedded objects have object/part
identity, not a text locator. Graph reference and hash validation establishes
internal consistency; only verification against the acquired snapshot establishes
source correspondence. Imported reports never trigger filesystem access.

## 3. Closed Office evidence contract

The wire increment defines explicit closed `$defs`, Go records, positive fixtures
and negative semantic tests for the following logical records. This specification
fixes their required information. The four Office location names `scope_ref`,
`coordinate_artifact_ref`, `scope_character_offsets` and `scope_byte_offsets` below
are normative and may not be renamed in the wire increment. The wire PR fixes
remaining property spellings and canonical reference grammar before production
emission.

| Record | Required information |
| --- | --- |
| Package | Source identity, parser/version, identification result, ordered part outcomes, limits, state, issues |
| Part | Exact name, container reference, method, compressed span/digest, decompressed digest/size when verified, state, failure/gap references |
| XML evidence | Part reference, parser version, retained token/element identities and exact lexical spans needed by downstream records |
| Text scope | Part reference, scope/assembler identity, role, exact text/hash, scalar origins, analytical hashes, boundaries and extraction issues |
| Metadata occurrence | Namespace/local name, lexical value, normalized key if supported, part reference, element and lexical value mapping, duplicate ordinal |
| Relationship | Original ID/type/target/mode, declaring part/anchor, source owner, resolution state/code, resolved target identity only when verified |
| Embedded object | Candidate part/identity, evidence for inclusion, relationship references, declared type, observed signature if bounded inspection supports it, inspection coverage |
| Part outcome | Operation, part reference, state, stable codes, diagnostic references, assessed and excluded scope |

Fixed objects reject unknown keys. Nullable identities distinguish unavailable
bytes from empty bytes. No failed decompression receives a digest computed from
partial output. Inventories include zero-count success distinctly from an unrun
operation. Bytes are not gratuitously duplicated in reports: identities and exact
locators allow verification from the source snapshot; missing retained content
is explicit. Report limits cap scopes, origins, diagnostic lists and string data.
Truncation must be declared and may not produce dangling graph references.

Findings retain the four classifications and detector-specific evidence contracts.
The Office finding location is a closed, separately discriminated variant. Its
`scope_ref` resolves to exactly one Office scope record; `coordinate_artifact_ref`
must equal that scope's assembled-text artifact reference. The occurrence arrays
are named `scope_character_offsets` and `scope_byte_offsets`, never legacy
`character_offsets`/`byte_offsets`. Legacy location keys are forbidden in this
variant. Scope-wide findings retain the same two references without fabricated
occurrence arrays; structural observations instead use the structural-anchor
variant and may not include scope-coordinate arrays.

For report serialization, OI-001 supersedes WA-001/OA-001 finding-location key
naming; scope-relative coordinate semantics are unchanged. WA/OA library results
retain their existing local field names and are not themselves report wire records.
Existing WA/OA analyzers may continue using temporary `evidence.Text` inputs with
UTF-8 boundary maps internally. The 4.0 assembler translates their locations to
the Office variant before serialization. It checks count agreement, scalar bounds,
UTF-8 byte starts, scope hash and scalar-origin linkage. A dedicated 4.0 coordinate
validator resolves the Office scope table and artifact references, not
`document.Texts`. It does not call v2 `findingCoordinates` for Office findings.
Flat-file findings keep the legacy validator and meanings unchanged. Tests reject
mixed coordinate variants, mismatched scope/artifact pairs, duplicate scope IDs,
dangling references and byte offsets that point into the container instead.
Context, profile assessments, detection and human judgment remain separate.
A finding is not needed for every ordinary inventory entry.

## 4. Producers and orchestration (B1/B2)

### Package dispatch

After acquisition, perform only a bounded signature check before package validation.
For the 4.0 Office path, do not call the legacy `parsers.Identify` ZIP-member sniff:
it uses `archive/zip.NewReader` before the hardened package limits. Validate the
container through `packageparts`, then dispatch identification over that one result.
Add verified-package entry points to `opcrels`, `docxidentify` and `odtidentify`:
the current `Inspect(ctx, source, hash)` functions do not accept package evidence
and recursively invoke the strict reader. Preserve those legacy entry points;
the new 4.0 coordinator never calls them. The new entry points accept the bounded,
identity-checked package outcome view, including failed/unassessed part identities,
and must not re-decompress or rerun the all-or-nothing reader. OPC inventory is
computed once and shared with DOCX identification and later producers. Each format
inspector runs at most once per source; it reports incomplete prerequisites rather
than losing good package evidence because an unrelated part failed. Extensions are hints, never authority.
The legacy version paths remain unchanged; this is a new bounded dispatch path,
not an assertion that the legacy ZIP sniff is already hardened. Preserve conflicting or incomplete identity
as diagnostics. A ZIP that cannot be safely identified is not plain text.
Package parsing precedes XML/relationship/text analysis. Process verified parts in
ordinal name order, never ZIP storage order or worker completion order.

Separate global container validation from local content outcomes. Duplicate names,
ambiguous spans, inconsistent headers or source-identity failure can invalidate the
container because safe independent part identities cannot be established. Once
container identities are sound, malformed XML, a CRC/decompression failure, or a
part-local limit fails that part and leaves unrelated verified parts usable.
The existing all-or-nothing reader therefore needs an explicit outcome API;
keep its current strict API for existing callers rather than quietly changing it.
Global span-contiguity/header validation happens in physical offset order before
local decompression. This validation is independent of canonical processing order:
a disagreement or overlap is a fatal container identity error, never permission to
skip a check. After that pass, allocate the aggregate decompression budget first
to present identification-critical parts in this fixed order: `mimetype`, `[Content_Types].xml`,
`_rels/.rels`, `META-INF/manifest.xml`; then the resolved content prerequisites
below; then remaining parts in ordinal name order. Use validated names, with no duplicate aliases entering
the priority list. ODF prerequisite names are exact and case-sensitive. For OPC,
resolve fixed prerequisite names using the inspectors' existing ASCII case
comparison: a unique case-equivalent candidate receives the same slot; ambiguous
case-equivalent candidates produce an identity gap and are not selected arbitrarily.
The candidate's original name/digest remain authoritative. Priority does not prove
a part's type or bypass any local limit.
Before ordinary parts, a second prerequisite phase admits `content.xml` for an
ODF candidate and the unique internal main part resolved from verified OPC root
relationships plus content-type declarations for a DOCX candidate. Resolve using
the existing inspector rules, without guessing `word/document.xml`, following
external targets, or recursively prioritizing arbitrary relationships. If both
candidate paths exist, process ODF content first, then the OPC target; a shared
part is charged only once. Inconsistent/ambiguous declarations retain a gap, not
an arbitrary target. Only after these prerequisites are checked does ordinal
allocation begin. This prevents large Pictures/customXml payloads from starving
otherwise verifiable main-content identity. No extra total budget is created.
Missing critical names consume no reservation. An oversized critical part still
yields a bounded explicit gap; this policy improves identification availability
but does not promise identification when required bytes cannot be verified.
A part whose declared size exceeds the remaining budget is unassessed; that part
and all subsequent parts in this priority order get the aggregate-limit reason
(no opportunistic fit of later small parts). Charge a reserved admitted size even if decompression
later fails; partial output never refunds budget to later parts. Actual output is
also bounded against the reservation. This deterministic allocation applies before
workers are scheduled. ZIP storage order/concurrency cannot change coverage sets.
Aggregate decompression limits remain hard limits. Never continue by disabling
CRC or bounds verification.

### Metadata

Add namespace-aware core and application property extraction using XP/XM lexical
maps, not regex or a second XML decoder. Select parts by verified package
relationships/content identification; do not trust a familiar filename alone.
Retain every duplicate/conflicting occurrence in document order. Project recognized
keys (creator, last modifier, application/version, company, timestamps, revision,
template and identifiers) without interpreting timestamps as verified facts.
Missing optional metadata is an assessed absence; malformed metadata is a partial
operation. Unknown elements remain inventory evidence with explicit extraction
coverage, not asserted metadata values. Custom properties/ODT metadata are explicitly
unassessed unless a supported producer is implemented in this increment.

### Embedded objects

Inventory internal targets of recognized embedded-object/package relationships
and conventional embedding-directory candidates. Record the distinct reasons;
a directory name alone does not prove object type. Include orphan candidates,
multiple references and unresolved/external declarations. Never follow external
relationships. Reuse OPC resolution and its ambiguity codes.
Retain target part digest/size and declared type; bounded magic-byte observations
are facts, not executable-type assurance. Recursive OLE parsing, unpacking nested
archives, attachment extraction to disk and macro analysis are unavailable checks.

### Text and profiles

Feed only admitted WA/OA scopes to their existing analyzers. Preserve exclusion
boundaries, `word.text_context_not_analyzed`, `odt.control_unresolved`, all `opc.*`
issues and extraction state. Do not broaden allowlists to make output look complete.
Evaluate only existing versioned profiles whose contexts can be established.
Record no-applicable-profile when appropriate; profile authoring is out of scope.
Expectedness neither suppresses observations nor prevents other analyzers running.

## 5. Capabilities, execution and failure (B4)

The Office catalog has a new identity; legacy text catalogs and report identities
remain unchanged. Acquisition still supports `*`. Proposed stable operation IDs:

| Operation | Supported scope |
| --- | --- |
| `aletharsis.parse.office_package` | DOCX/ODT source |
| `aletharsis.office.identify` | Verified package |
| `aletharsis.office.relationships` | OPC relationship parts (DOCX) |
| `aletharsis.office.metadata` | Recognized DOCX core/app parts |
| `aletharsis.office.embedded_objects` | DOCX package/relationship inventory |
| `aletharsis.office.text` | Admitted DOCX/ODT text parts |
| `aletharsis.office.profiles` | Applicable preserved Office observations |

Unicode/emoji/pattern operations declare Office analysis scopes separately from
flat source text. Other native text analyzers must not acquire Office coverage
merely because strings exist. Disabled statistical/C2PA declarations stay disabled.
Capabilities declare fixed per-source/part limits, never remaining corpus budgets.

Each operation records executed, skipped or failed scope. Parent operation state
aggregates part outcomes: recoverable bad children yield partial, not completed;
a prerequisite failure yields not-run with its reason. No parent claims all parts
were covered if enumeration itself was bounded or failed. Fatal acquisition or
container identity failure yields failed. Cancellation retains usable evidence
and declares unfinished scopes. Ordering and reference assignment are deterministic.

Schema 4.0 supports completed/partial/failed/canceled/not-run operation states.
Document partial coverage returns exit 4, retaining the existing v2 convention.
The CLI usage text must describe 4 as failure or incomplete audit, not only failure.
A partial report with HIGH findings still exits 4; callers must inspect JSON status,
coverage and severity counts to distinguish incomplete coverage from failed execution.
An exit code alone must not be used as a retry policy or a finding-severity value.
Coverage is shown before findings in the console and retained in every subview.
A bounded report-serialization failure uses a minimal schema-valid failure report
where the output budget permits; an output-device failure cannot promise JSON.
Never return a truncated JSON document as a valid report.

## 6. Consumer inventory (B5)

This table is the implementation checklist. A sibling sharing the same assumption
is in scope even when its filename is not listed here.

| Consumer / assumption | Required behavior |
| --- | --- |
| `audit/audit.go` | Dispatch Office only through new orchestration; preserve text path and acquisition invariants |
| `parsers/identify.go`, `docxidentify`, `odtidentify` | New 4.0 bounded signature/package dispatch bypasses the raw ZIP-member sniff; add verified-package entry points to OPC and both format inspectors; share one parsed outcome/OPC result, no strict-reader re-entry or duplicate decompression |
| `evidence/model.go`, analyzer location helper | Existing flat records unchanged; new typed Office records and explicit coordinate artifacts |
| `audit/v2.go`, `v2_findings.go`, native trace | Leave 2.0 frozen; new assembler handles multiple parts/scopes and per-part outcomes; allow verified partial flat-text snapshots for corpus-v2 presentation |
| `capability/registry.go`, native data identity | New versioned catalog with exact supported inputs/limits; old catalog unchanged |
| `evidence/v2` graph, anchors, aggregate | Preserve old validators; new graph validates Office origin chains and partial child outcomes without fabricating native text pointers |
| `identity.VerifyText`, selection hashing | Flat verification unchanged; Office verifier reuses source/part/XML identities and origins, no competing offset mapper |
| `reporters/report.go`, `reporters/v2.go` | Existing serializers unchanged; 4.0 console/JSON prints typed inventory, source locations and coverage, inertly |
| `wire`, `schemas/embed.go`, `reportimport` | Explicit 4.0 dispatch and closed schema/semantic validation; support-aware importer adds sibling support_reason while preserving legacy Read/status; distinguish unknown version, known-unimplemented version and feature; retain EC-002/EC-003 regressions and bounded bytes |
| CLI version flags and all audit subviews | Accept 4.0 deliberately; finding filters never remove required evidence/coverage or break reference closure |
| `cli/reveal.go`, `reveal` | Flat reveal unchanged; Office presentation unsupported; partial flat text may be presented only after VerifyText, no ZIP passed to flat verifier |
| `cli/directory_reveal.go` | Retain report/digest and verified snapshots for partial entries; present supported flat text, declare Office unsupported; source limits degrade one entry; version reveal-tree enum |
| `corpus`, `cli/directory.go` | Derive corpus-v2 entry state from report status, never exit 4; retain partial evidence distinctly from failed entries for both 2.0 and 4.0; stream records declare report version |
| `workspace` format discovery | Admit requested Office candidates deterministically; do not follow uncontrolled links or trust extension as identity |
| `publication` | File safety/rollback unchanged; do not publish fictitious Office reveal products |
| Profile evaluator | Existing content unchanged; bind assessments to retained observations, preserve gaps |
| Schema/differential/parity/distribution tests | Keep historical fixtures unchanged; add 4.0 conformance, multi-part and compiled-CLI cases; production distribution remains Go-only |

Corpus and reveal wrapper schemas are independently versioned. `corpus-v1` and
`corpus-document-v1` currently admit only 1.0/2.0 reports, and `corpus.Run` rejects
other versions. New `corpus-v2` and `corpus-document-v2` schemas, exact runtime
selection/validation and updated option guards are mandatory for initial directory
4.0 support. Add `--corpus-version 1|2` for directory commands and an explicit
wrapper-version option to `corpus.Run`. When omitted, schema 4.0 selects corpus-v2;
schema 1.0/2.0 selects corpus-v1, preserving existing defaults. Explicit corpus-v2
admits report 2.0 and 4.0; report 1.0 is deliberately not admitted in this increment.
Reject unsupported combinations before discovery; corpus-v1 with 4.0 is invalid.
Thus `--schema-version 2.0 --corpus-version 2` exercises the new semantics without
changing any existing v1 stream. The wire increment must encode this matrix. Review-tree changes
also require a versioned envelope if presentation-only failures cannot be expressed
by its existing contract; never mutate corpus-v1/reveal-tree-v1 meanings.

Corpus-v2 classifies entries from their report status and typed acquisition outcome,
never from exit code 4 alone. A `partial` report yields a `partial` entry and
increments a dedicated partial count, not failed or `execution.failed`. A failed
report yields failed (or unsupported when the typed format failure says so);
canceled remains canceled. Completed reports use finding severity only to choose
requires-review versus no-reported-findings. Both report 2.0 and 4.0 must use this
status-derived rule when emitted in corpus-v2. A mixed run with partial entries
has partial summary and exit 4; failed counts exclude partial entries. Preserve
usable reports and continue good neighbors. Global cancellation and stream failure
retain their distinct existing precedence. Corpus-v1 compatibility is unchanged;
its current exit-derived classification must not be copied into corpus-v2.
Regression cases cover completed/partial/failed reports of both versions, including
partial plus HIGH findings, and prove failed counts remain zero for partial-only
runs. Every report-bearing corpus-v2 entry carries `highest_finding_severity`
(null for no findings, otherwise INFO/LOW/MEDIUM/HIGH), independent of its state
and exit code. Summary `finding_severity_counts` counts documents by that field,
including partial entries; these counts overlap operational counts intentionally.
Partial plus HIGH must increment partial and HIGH, never failed. Console summaries
show both coverage-state and severity counts.

Treat partial as report-bearing across the entire observer/presentation chain:
retain report/canonical digest and any verified acquired snapshot; do not clear it
solely because state is partial. A partial 2.0 report with one independently verified
flat text is eligible for reveal; Office remains explicitly unsupported. Audit v2
snapshot retention must allow this verified partial-text case instead of its current
completed-only guard. No partial snapshot is assumed valid without `VerifyText`.
`corpus.Observer.Visit`, its source-limit handling, directory-reveal's state guard,
the directory console state list and the reveal-tree state enum are all consumers.
Add a versioned reveal-tree envelope for new states rather than changing v1.
An observer `ErrSourceLimit` on any report-bearing state degrades that entry with
`execution.resource_limit`, retains available evidence, and continues the run;
it must not abort the stream. It remains an entry failure, unlike partial detection
coverage. `execution.report_limit` (no serializable report) likewise remains failed
with its existing reason. These no-report/presentation failures are explicit
exceptions to deriving state from an available report. Test both 2.0 and 4.0
partial reports beside good neighbors and observer failures.

A failed Office presentation is separate from successful audit evidence. Include the originating
report identity in a presentation outcome.

## 7. Validation requirements

Before claiming B1–B5 delivered, tests must establish:

- Compiled CLI identifies DOCX/ODT despite misleading extensions; 4.0 output
  validates; old-version requests produce valid unsupported outcomes.
- Parts, metadata, relationships and embedded-object identities reach reports.
  Core/app duplicates, unusual names, orphan embeddings and external references
  are retained. Source bytes and timestamps remain unchanged.
- Cross-run invisible patterns, entities, CDATA, non-BMP scalars and ODT generated
  controls resolve to exact appropriate artifacts. Altered part hashes, lexical
  spans or scalar origins fail verification. Normal text verification stays strict.
- Malformed XML/CRC failures beside good parts preserve good evidence. Bad
  container identity fails closed. A bad document between good documents does
  not stop corpus or reveal processing. Test cancellation and each local/global
  limit at both boundaries, including empty inventories and failed prerequisites.
- Unknown Word contexts, ODT unresolved controls and OPC issues remain visible
  through console, JSON, subviews, import and corpus; none yields completed coverage.
- Repeated runs are byte-identical; ZIP member order and concurrency do not change
  logical ordering (source hashes still reflect actual container bytes).
- New and legacy schema tests, native Go tests/race/vet, frozen parity, compiled
  integration tests and distribution checks pass for the branch under review.

Tests compare evidence identities and mappings, not just finding counts. Tests of
negative cases must also show that valid neighboring content is still analyzed.

## 8. Reviewable increments and #40 boundary

1. This B1–B5 design PR; no implementation or comparator clearance implied.
2. Closed 4.0 wire contract (including the 3.0 adapter envelope), fixtures and
   cross-record checks, plus mandatory corpus-v2/corpus-document-v2 contracts.
   Review any additionally needed versioned presentation envelope.
3. Package outcome API plus verified-package entry points in OPC/DOCX/ODT
   inspectors (one decompression pass), metadata and embedded-object producers
   with unit tests, including unrelated bad-CRC parts and identification-priority
   budgeting beside large images.
4. Office orchestration, capability/graph assembly, import/CLI/consumer integration
   and compiled end-to-end tests. Stack branches as necessary; no main merges.
5. Cleared comparator provisioning and independent #40 corpus/comparison receipts.

B0 proceeds separately before comparator execution/artifacts. Full oletools
installation is not cleared by its root license. A scoped runtime requires exact
module/dependency/license inventory, immutable provisioning and enforced isolation.
Unresolved licensing stops that work; no successful comparison can waive it.

The #40 budget is 12 engineering hours, 2 CPU hours, 2 GiB study disk,
1 GiB/process, 60 seconds/case, 32 MiB input/output, no GPU/API spend. Instrument
actual commands before execution; do not represent unmeasured historical CPU as
measured zero. Native package limits may be stricter. Stop at the first ceiling
with partial receipts and a revise/reject recommendation. Independent expected
identities, six reachable adjudication categories and object-specific exemptions
remain mandatory. Only the owner disposes of #40/#21 gates.
