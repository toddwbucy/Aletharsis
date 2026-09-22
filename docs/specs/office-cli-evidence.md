# OI-001 — Office CLI evidence and consumer integration

| Attribute | Value |
| --- | --- |
| Status | Proposed; implementation may proceed on reviewable branches, no merge authorization |
| Tracking | #40 (B1–B5), #14, #21, #7 |
| Inspected baseline | `5b1ca93`; subsequent main changes were documentation only |
| Wire target | Explicit opt-in report `4.0`; 1.0/2.0 remain unchanged; 3.0 remains the separate adapter contract |
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

The 4.0 design starts from the existing graph concepts and closed wire types,
not a generic provider map. Its wire/schema and Go implementation are separately
reviewable increments. A 4.0 reader must not infer 3.0 support: adapter execution
and 3.0 import remain unavailable until independently implemented. Native-only
4.0 reports declare disabled provenance/statistical capabilities as before.
No adapter fields/results are silently reinterpreted as native Office results.

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
   Scalar offsets and finding byte offsets refer to this artifact. Every scalar
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
fixes their required information; the wire PR fixes exact property spellings and
canonical reference grammar before any production producer emits them.

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
The Office finding variant binds the existing analyzer payload to an Office scope
or structural anchor. Do not change the meaning of legacy `byte_offsets` or
`character_offsets`. Context, profile assessments, detection and human judgment
remain separate. A finding is not needed for every ordinary inventory entry.

## 4. Producers and orchestration (B1/B2)

### Package dispatch

After acquisition, identify from bytes using the existing identification libraries;
extensions are hints, never authority. Preserve conflicting or incomplete identity
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
Aggregate decompression limits remain hard limits: remaining parts become unassessed
with a limit reason. Never continue by disabling CRC or bounds verification.

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
Document partial coverage returns exit 4; severity never hides operational failure.
Coverage is shown before findings in the console and retained in every subview.
A bounded report-serialization failure uses a minimal schema-valid failure report
where the output budget permits; an output-device failure cannot promise JSON.
Never return a truncated JSON document as a valid report.

## 6. Consumer inventory (B5)

This table is the implementation checklist. A sibling sharing the same assumption
is in scope even when its filename is not listed here.

| Consumer / assumption | Required behavior |
| --- | --- |
| `audit/audit.go`, parser identification | Dispatch Office only through new orchestration; preserve text path and acquisition invariants |
| `evidence/model.go`, analyzer location helper | Existing flat records unchanged; new typed Office records and explicit coordinate artifacts |
| `audit/v2.go`, `v2_findings.go`, native trace | Leave 2.0 frozen; new assembler handles multiple parts/scopes and per-part execution outcomes |
| `capability/registry.go`, native data identity | New versioned catalog with exact supported inputs/limits; old catalog unchanged |
| `evidence/v2` graph, anchors, aggregate | Preserve old validators; new graph validates Office origin chains and partial child outcomes without fabricating native text pointers |
| `identity.VerifyText`, selection hashing | Flat verification unchanged; Office verifier reuses source/part/XML identities and origins, no competing offset mapper |
| `reporters/report.go`, `reporters/v2.go` | Existing serializers unchanged; 4.0 console/JSON prints typed inventory, source locations and coverage, inertly |
| `wire`, `schemas/embed.go`, `reportimport` | Explicit 4.0 dispatch and closed schema/semantic validation; unknown versions retained as unsupported, no silent conversion |
| CLI version flags and all audit subviews | Accept 4.0 deliberately; finding filters never remove required evidence/coverage or break reference closure |
| `cli/reveal.go`, `reveal` | Flat reveal unchanged; Office presentation unsupported in this increment, typed explanation, no ZIP passed to flat verifier |
| `cli/directory_reveal.go` | Retain Office audit entry with explicit unsupported presentation outcome; continue with supported neighbors |
| `corpus`, `cli/directory.go` | One malformed document affects one entry; 4.0 reports and exit aggregation stay coherent; stream records declare report version |
| `workspace` format discovery | Admit requested Office candidates deterministically; do not follow uncontrolled links or trust extension as identity |
| `publication` | File safety/rollback unchanged; do not publish fictitious Office reveal products |
| Profile evaluator | Existing content unchanged; bind assessments to retained observations, preserve gaps |
| Schema/differential/parity/distribution tests | Keep historical fixtures unchanged; add 4.0 conformance, multi-part and compiled-CLI cases; production distribution remains Go-only |

Corpus and reveal wrapper schemas are independently versioned. If their closed
contracts cannot express 4.0 or a presentation-only failure, add a new wrapper
version; do not mutate the meaning of corpus-v1/reveal-tree-v1. A failed Office
presentation is separate from successful audit evidence. Include the originating
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
2. Closed 4.0 wire contract, fixtures and cross-record checks; explicitly review
   any independently versioned corpus/presentation envelopes.
3. Package outcome API, metadata and embedded-object producers with unit tests.
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
