# OC-001 — Office CLI evidence and consumer integration

| Attribute | Value |
| --- | --- |
| Status | Proposed; implementation may proceed on reviewable branches, no merge authorization |
| Tracking | #40 (B1–B5), #14, #21; historical testing tracker #7 (closed) |
| Inspected baseline | `5b1ca93`; subsequent main changes were documentation only |
| Wire target | Explicit opt-in report `4.0`; 1.0/2.0 remain unchanged; 4.0 includes the 3.0 adapter envelope plus Office evidence |
| Authority | Owner authorized version selection and continued work during review; owner retains merges and gate dispositions |

## 1. Scope and delivery

Connect acquired Office packages to the CLI using existing DP-001, XP-001/XM-001,
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
The proposed `ReadSupported` API is a superset dispatcher for exact versions
1.0, 2.0 and native-only 4.0. For 1.0/2.0 it delegates to legacy `Read`, preserving
its decoded views, validation, statuses and errors, with null `support_reason`.
Legacy `Read` itself remains unchanged and supports only 1.0/2.0; it continues to
reject 3.0/4.0 with its existing `unsupported_version` status.

The new API adds `unsupported_feature` to its own result status contract, without
changing EC-002's legacy enum. A structurally wire-valid 4.0 report with nonempty
`adapter_runs` returns `unsupported_feature` and
`support_reason: known_feature_unimplemented` until adapter semantics are supported.
The proposed dispatcher returns `unsupported_version` for 3.0 with
`support_reason: known_version_unimplemented`, or for unrecognized versions with
`support_reason: unknown_version`. These are new requirements, not current Go
behavior. Supported 4.0 exposes a decoded view only after full supported semantic
validation. New consumers use `ReadSupported` for all admitted versions; legacy
fixtures/tests continue exercising `Read`. Successful 4.0 imports return `status: validated` and null
`support_reason`. Report failure codes are unchanged.
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
   Every text-scope location requires `scope_ref`; the referenced scope identifies
   its exact text/hash as the coordinate artifact. Structural locations have no scope. Every scalar
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
fixes their required information. The three Office text-location names `scope_ref`,
`scope_character_offsets` and `scope_byte_offsets` below
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
| Relationship | Original ID/type/target/mode, declaring part/anchor, source owner, resolution state/code, resolved package-part reference when a unique internal target exists; verified payload identity is carried by the referenced part record, not this record (proposed normative amendment) |
| Embedded object | Candidate part/identity, evidence for inclusion, relationship references, declared type, observed signature if bounded inspection supports it, inspection coverage |
| Part outcome | Operation, part reference, state, stable codes, diagnostic references, assessed and excluded scope |

The following proposed normative amendments apply to the 4.0 wire increment;
they require acceptance independently of the implementation PRs and do not alter
frozen 1.0/2.0 behavior:

- `resolution_state` describes relationship name resolution, independently of
  payload admission. A unique internal target may have `target_part_ref` even
  when its bytes were not admitted or failed verification. That reference binds
  the verified container entry and compressed identity; the target part's `state`,
  issues and parse outcome carry its admission result. The target part record's decompressed SHA-256 and
  size remain null until verified. An absent target has no part reference and
  retains the relationship's `missing` state/code. Runtime `TargetSHA256`, `TargetState` and
  `TargetCode` convenience fields are derived observations, not additional wire
  properties or a second source of admission authority.
- A stored origin's `text_index` is a producer-local ordinal, not an XML segment
  ordinal or a coordinate. For metadata it indexes projected properties in
  document order, including empty properties; the importer checks it against
  that retained inventory. For Word/ODT text, the wire's XML subset does not
  reconstruct the extractor's complete text inventory. The importer checks
  nonnegativity but does not attest that ordinal. Authoritative retained origin
  checks use `xml_ref` plus, for stored origins, segment/scalar identity, source
  span, UTF-8 span and declared transformation; generated controls additionally
  bind control/element/repetition identity and derivation. The ordinal exception
  does not apply to these generated-control fields. Source-backed replay of the named extractor is
  required to independently attest its local text ordinals. Consumers must not
  use an unverified ordinal for highlighting or transformation authority.
- A completed or partial metadata projection retains its XML map so property completeness
  and exact omission locations can be checked. Every direct property is either
  retained or covered by a property-located omission diagnostic bound to that
  outcome's own execution and excluded region; another outcome's diagnostic or
  exclusion cannot supply omission authority. Only a partial outcome may authorize
  a property omission; the property span must be nonempty and covered by that
  outcome's own exclusion. Distinct direct properties must not share a lexical
  extent. A retained property's full element span, including an empty-valued
  property's span, must lie inside the union of assessed metadata regions for
  that part; value origins alone do not establish property coverage. At most one
  omission diagnostic may authorize omission of a direct property across all
  executions.
  Unsupported metadata roots do not become completed empty projections.
- Completed or partial executions of the same operation may contribute assessed
  regions to a part through their completed or partial outcomes. Only these
  contributing outcomes supply assessed/excluded regions for the combined
  projection; failed, canceled and unrun outcomes remain historical failure/gap
  evidence and do not contribute to that union. Validate retained evidence against
  the contributing union, while preserving every execution's own states, diagnostics and exclusions. Failed,
  canceled and unrun executions contribute no assessed coverage. A diagnostic
  used to explain an omission must still bind to its actual producing execution.
  An omitted property must lie outside the union of assessed regions for that
  operation and part. No contributing assessed region may overlap any contributing
  excluded region for the same operation and part, including across executions. This is an
  unordered snapshot, not a retry log: one partial contribution cannot be
  superseded by a later completed contribution in the same report. Re-audits
  that supersede earlier coverage produce separate reports; no ordering or
  supersedes relationship is inferred from execution ordinals. Failed, canceled
  and unrun parents cannot authorize omissions through otherwise successful child outcomes.
- XML maps require an attesting XML-parsing operation: identification, text,
  metadata or relationships. Its assessed coverage must contain a nonempty region
  of the part; an empty or zero-width region cannot attest an XML map. Maps may
  retain lexical structure for analytically excluded regions without claiming
  those regions were analyzed. This nonempty-region requirement is deliberately a
  minimum producer linkage check, not proof that the whole map was independently
  replayed from source. Analytical coverage and XML parse coverage are different:
  requiring all map element extents inside analytical coverage would incorrectly
  reject lexical evidence for excluded content. Exact map attestation requires
  source-backed parser replay; a future distinct XML parse-coverage contract must
  be approved before its coverage can be inferred from analytical outcomes.
  Embedded-object signature inspection and profiles consume existing evidence; they do not parse XML and cannot independently
  attest an XML map.

XML structural locations and generated-control source spans must be nonempty;
zero-width anchors cannot authorize relationship or text coverage. This does not
prohibit empty text values, empty text scopes, or empty opaque payloads.

Fixed objects reject unknown keys. Nullable identities distinguish unavailable
bytes from empty bytes. No failed decompression receives a digest computed from
partial output. Inventories include zero-count success distinctly from an unrun
operation. Every admitted text scope carries its complete exact text and SHA-256 on the
wire. Importers hash that retained text and validate offsets without filesystem
access; this proves internal consistency, not correspondence to the original ZIP.
Never truncate a scope string or scalar-origin list. If its text or complete
origin list exceeds its declared per-scope cap,
omit that scope and dependent findings/origins, retain a located resource-limit
outcome, and mark coverage incomplete. No hash-only or truncated scope variant
is permitted in this increment. Total report limits (bytes, nodes and depth) are not scope-omission
triggers: exceeding any of them produces `execution.report_limit`. Corpus uses
its existing report-limit path. Single-file 4.0 must emit a minimal schema-valid
failure report with that reason where the output budget permits, distinct from
acquisition `execution.resource_limit`; this requires a new 4.0 failure path.
Do not choose additional scopes to drop to fit a total limit. Add over-limit scope tests verifying
that no dependent coordinates survive, and hash-mismatch import rejection.

For non-text inventories, bytes are not gratuitously duplicated in reports: identities and exact
locators allow verification from the source snapshot; missing retained content
is explicit. Report limits cap scopes, origins, diagnostic lists and string data.
Inventory/list truncation must be declared and may not produce dangling graph
references; text-scope strings and scalar-origin lists both follow the
all-or-omitted rule above.

Findings retain the four classifications and detector-specific evidence contracts.
The Office text finding location is a closed, separately discriminated variant. Its
`scope_ref` resolves to exactly one Office scope record, which supplies the
exact text/hash as the coordinate artifact; no separate artifact reference is
required in either record. The occurrence arrays
are named `scope_character_offsets` and `scope_byte_offsets`, never legacy
`character_offsets`/`byte_offsets`. Legacy location keys are forbidden in this
variant. Scope-wide findings retain the same scope reference without fabricated
occurrence arrays; structural observations instead use the structural-anchor
variant and may not include `scope_ref` or scope-coordinate arrays. Structural
anchors require part identity and applicable element/token index and part-byte
region; opaque object anchors use object/part identity without text coordinates.

For report serialization, OC-001 supersedes WA-001/OA-001 finding-location key
naming; scope-relative coordinate semantics are unchanged. WA/OA library results
retain their existing local field names and are not themselves report wire records.
Existing WA/OA analyzers may continue using temporary `evidence.Text` inputs with
UTF-8 boundary maps internally. The 4.0 assembler translates their locations to
the Office variant before serialization. It checks count agreement, scalar bounds,
UTF-8 byte starts, scope hash and scalar-origin linkage. A dedicated 4.0 coordinate
validator resolves the Office scope table and its text/hash identities, not
`document.Texts`. It does not call v2 `findingCoordinates` for Office findings.
Flat-file findings keep the legacy validator and meanings unchanged. Tests reject
mixed coordinate variants, scope records with invalid text hashes, duplicate scope IDs,
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

An unidentified ZIP without OPC relationship candidates reports that check as
unrun with identity unconfirmed; an identified ODT with no OPC candidates retains
the unsupported-input result. Actual OPC candidates may still be inspected
independently of final file format, with explicit per-part coverage. Conflicting
DOCX/ODT signatures do not suppress independently confirmed text, metadata or
embedded-object inspection; the package remains format `unknown` with the
conflict diagnostic.

A failed main payload likewise must not suppress independently declaration-selected
sibling evidence. Each candidate still requires its own admitted bytes and
namespace/root checks; inspecting it does not establish overall format identity.

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
an arbitrary target. In this same prerequisite phase, after the main targets,
admit core/app targets from verified root declarations in ordinal name order;
they do not depend on main-part relationship parsing.

Next admit the uniquely resolved main part's canonical relationship part and
wait for admission, decompression and parsing to finish. From that parsed result,
admit verified internal WT-001 text-part targets (header, footer, comments,
footnotes, endnotes) in ordinal name order before any embedding target. Existing
WT-001 namespace/type/context admission rules still apply; priority alone cannot
authorize text extraction. Then admit relationship-resolved embedded targets in
ordinal name order, followed by conventional embedding-directory candidates not
already admitted, also in ordinal name order. These groups have explicit barriers
before later allocation; shared parts are charged once. Names are candidates,
not XML-validated relationships until their bytes are admitted and parsed.
Ambiguous or missing declarations retain gaps, not guessed targets.
After this target-dependent set, process all other canonical relationship-part
candidates in ordinal name order; non-canonical relationship-like names receive
ordinary-part priority and retain the existing OPC diagnostic. Then admit ordinary
parts in ordinal name order. Never recursively promote arbitrary relationship
chains. Shared parts are charged once. Thus unrelated customXml relationship
parts cannot consume budget ahead of the selected comparison targets. The target
set prioritizes main-related WT-001 text over embedding inventory by design:
text coverage serves the core detection use case. Both groups have attacker-controlled counts;
many or large text parts can consume the budget before embeddings,
which then receive aggregate-limit gaps and incomplete inventory coverage, not a
fatal failure. WT-001 parts not related from the main part (including glossary
headers or targets reachable only from another part's relationships) retain
ordinary priority. Large admitted embeddings can exhaust the budget before later
relationship candidates. Those
receive explicit aggregate-limit gaps, not a fatal package failure. The set
remains subject to package-count, per-part and aggregate limits: enumeration
and content gaps remain explicit, never a promise of complete target coverage.

Missing critical names consume no reservation. Check per-part limits before the
aggregate budget: an over-per-part-limit member gets that reason, no reservation,
and processing continues, including when it is a decoy prerequisite. For every
priority tier, a member larger than the remaining aggregate allowance gets an
aggregate-limit gap without reservation; continue considering later members.
There is no all-subsequent cutoff. This deterministic bounded-fit policy means
an oversized decoy cannot starve a later small prerequisite. Charge a reserved
admitted size even if decompression later fails; partial output never refunds it.
Actual output is also bounded against the reservation. Within each phase,
allocation is deterministic before its workers are scheduled; dependent phases
wait for the preceding declarations to be verified. ZIP storage order/concurrency cannot change coverage sets.
Aggregate decompression limits remain hard limits. Never continue by disabling
CRC or bounds verification.

### Metadata

Add namespace-aware core and application property extraction using XP/XM lexical
maps, not regex or a second XML decoder. Select parts by verified package
relationships/content identification; do not trust a familiar filename alone.
Retain every duplicate/conflicting occurrence in document order. Project recognized
keys (creator, last modifier, application/version, company, timestamps, revision,
template and identifiers) without interpreting timestamps as verified facts.
Applicability is distinct from an empty inventory: a confirmed DOCX with no
optional metadata can report an assessed absence. Once the coverage-contract
decision below is accepted, coverage shall identify the selection evidence
actually inspected, rather than claim whole-source payload analysis merely
because there are no applicable metadata parts. The selection
pass may inspect package declarations and relationships even when the selected
metadata inventory is empty.

**Open coverage-contract decision:** the frozen execution record requires a
completed execution's analyzed scope to equal its requested scope. An empty
completed analyzed scope cannot be introduced silently. Before accepting the
assessed-absence implementation, specify a bounded scope for inspected selection
evidence or explicitly amend the execution contract. Until that decision is
accepted, the current whole-artifact summary is not clearance of this requirement.
The bounded-selection option also needs an explicit representation: current
metadata part outcomes require core/app projection roots, so declarations and
relationship parts cannot be added as metadata projection outcomes. One option
is an execution-level byte scope over inspected declarations' compressed spans,
separate from projection outcomes; implementing it requires changing the coverage
builder, not relaxing property-completeness checks by implication. Alternatively,
a new selection-evidence contract must explicitly distinguish selection from
projection. Neither option is approved by recording it here.

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
They also declare `max_scope_text_utf8_bytes` and `max_scope_scalar_origins`;
report serialization declares its fixed byte budgets (InputBytes and OutputBytes),
node count and depth limits. Record all limits in the report capability/limit
records: per-scope limits explain scope omission; total limits explain report
serialization failure. An origin
limit omits the whole scope and dependent findings just like a text-size limit;
no retained scope may carry an incomplete origin chain.

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
| `cli/cli.go`, `cli/v2.go` version flags and audit subviews | Dispatch every single-file audit site by exact version with no fallback to 1.0/2.0; reject unlisted versions before acquisition. The `unicode`, `metadata` and `structure` subviews share the single-file dispatch site. In the new 4.0 path, stop folding Encode's `ErrLimit` into `execution.resource_limit` as `cli/v2.go` does today; preserve frozen 2.0 behavior. Accept 4.0 deliberately for audit/subviews, except single-file reveal as specified below; finding filters never remove required evidence/coverage or break reference closure |
| `cli/reveal.go`, `reveal` | Single-file reveal rejects schema 4.0 as usage (exit 4) in this increment before audit dispatch; flat reveal retains its completed-only guard for 1.0/2.0; Office presentation unsupported; no ZIP passed to flat verifier |
| `cli/directory_reveal.go` | Under reveal-tree-v2 retain report/digest and verified snapshots for partial entries; present supported flat text, declare Office unsupported; source limits degrade one entry; version reveal-tree enum |
| `corpus`, `cli/directory.go` | Dispatch corpus workers by exact supported version with no else-to-RunV2 fallback; reject unlisted versions before discovery/work. Derive corpus-v2 entry state from report status, never exit 4; retain partial evidence distinctly from failed entries for both 2.0 and 4.0; stream records declare report version |
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
changing any existing v1 stream. The wire increment must encode this matrix. Directory reveal selects reveal-tree-v1 exactly when corpus-v1 is selected, and
reveal-tree-v2 exactly when corpus-v2 is selected (including explicit 2.0/v2).
There is no independent reveal-tree flag. V2 retains detection entry state
(including partial) separately from presentation outcome (`revealed`, `failed`,
`unsupported`, or `not_attempted`). Use `not_attempted` when detection is failed,
canceled, unsupported or skipped, or no usable report exists; do not turn audit
failure into presentation failure. If an otherwise eligible completed/partial
Office report reaches presentation, its outcome is `unsupported`. Successful
reveal never promotes partial coverage to completed.
The wire increment must define this closed v2 envelope and test the full matrix.
Never mutate corpus-v1/reveal-tree-v1 meanings.

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

For corpus-v2/reveal-tree-v2 only, treat partial as report-bearing across the
entire observer/presentation chain:
retain report/canonical digest and any verified acquired snapshot; do not clear it
solely because state is partial. A partial 2.0 report with one independently verified
flat text is eligible for reveal; Office remains explicitly unsupported. Audit v2
snapshot retention must allow this verified partial-text case through an explicit
v2-consumer option instead of its current completed-only guard. Corpus-v1 runs
retain current behavior and discard partial snapshots, including with report 2.0. No partial snapshot is assumed valid without `VerifyText`.
`corpus.Observer.Visit`, its source-limit handling, directory-reveal's state guard,
the directory console state list and the reveal-tree state enum are all consumers.
Add a versioned reveal-tree envelope for new states rather than changing v1.
An observer `ErrSourceLimit` on any report-bearing state degrades that entry with
`execution.resource_limit`, retains available evidence, and continues the run;
it must not abort the stream. For a partial report whose observer hits this
limit, corpus-document-v2 entry state is failed; reveal-tree-v2 detection state
remains partial and presentation outcome is failed with execution.resource_limit.
This intentional difference records detection separately from publication failure.
Widen the observer source-limit guard to every report-bearing v2 state, including
partial; never overwrite the retained report's own detection status. `execution.report_limit` (no serializable report) likewise remains failed
with its existing reason. These no-report/presentation failures are explicit
exceptions to deriving state from an available report. Test both 2.0 and 4.0
partial reports beside good neighbors and observer failures.

A failed Office presentation is separate from successful audit evidence. Include the originating
report identity in a presentation outcome.

## 7. Validation requirements

Before claiming B1–B5 delivered, tests must establish:

- Single-file `audit FILE --reveal-out DIR --schema-version 4.0` exits 4 as usage
  before acquisition: no source read, report or bundle directory. Usage keeps
  `1.0|2.0` on the FILE reveal form while ordinary audit gains 4.0. Directory
  schema-4.0 reveal follows its separate envelope/presentation rules.
- Unlisted versions at single-file and corpus dispatch fail as usage/options,
  never execute a legacy audit by default.
- Single-file 4.0 and corpus report serialization failures carry the same
  `execution.report_limit` reason, distinct from acquisition resource limits;
  single-file emits a minimal valid failure report where the output budget permits.
- Compiled CLI identifies DOCX/ODT despite misleading extensions; 4.0 output
  validates; old-version requests produce valid unsupported outcomes.
- Parts, metadata, relationships and embedded-object identities reach reports.
  Core/app duplicates, unusual names, orphan embeddings and external references
  are retained. Source bytes and timestamps remain unchanged.
- Cross-run invisible patterns, entities, CDATA, non-BMP scalars and ODT generated
  controls resolve to exact appropriate artifacts. Altered part hashes, lexical
  spans or authoritative scalar-origin fields fail verification. The explicit
  exception is a text scope origin's producer-local `text_index`: the importer
  checks nonnegativity but does not attest its ordinal; independent attestation
  requires source-backed extractor replay. Metadata ordinals are checked against
  the retained property inventory, including empty values. Normal text verification stays strict.
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
   Deliver the mandatory reveal-tree-v2 envelope and selection matrix.
3. Package outcome API plus verified-package entry points in OPC/DOCX/ODT
   inspectors (one decompression pass), metadata and embedded-object producers
   with unit tests, including unrelated bad-CRC parts and identification-priority
   budgeting beside large images.
4. Office orchestration, capability/graph assembly, import/CLI/consumer integration
   and compiled end-to-end tests. Stack branches as necessary; each merge requires separate owner authorization.
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

Exercise each total report limit (byte budgets, nodes and depth) independently,
with per-scope limits satisfied. Require the same report-limit outcome across
permuted ZIP member order; no opportunistic scope dropping changes the result
or any emitted bounded diagnostics.

Required budget regressions include large ordinary Pictures/customXml payloads
beside metadata, relationship and embedding targets, and an oversized decoy
`[Content_Types].xml` beside a valid ODT manifest/content pair. Assert target
coverage under the stated remaining budget, stable selection across ZIP/worker
order, explicit per-part versus aggregate reasons, and no budget refunds. Large
customXml/_rels members must not displace main relationships/core/app/embedding
targets. The mirror case (large embeddings ahead of customXml relationships)
must retain deterministic aggregate-limit gaps and continue without fatal failure.
Large embeddings beside main-related headers/footnotes/comments must leave those
text parts prioritized: their findings survive when they fit their own limits,
while excluded embeddings produce explicit gaps and partial coverage. Test an
over-limit origin list as whole-scope omission with no dependent findings.
Also exercise many/large main-related text parts ahead of a small embedding:
text findings survive, the embedding has an aggregate-limit gap and inventory
coverage is incomplete, without fatal failure and independent of ZIP order.
Priority tests must reach decompression: use deflate-compressible members below
the source acquisition cap or lower configurable aggregate limits, not stored
oversized archives rejected before priority selection. At default limits, use
at least three members individually below 32 MiB whose combined expanded sizes
exceed 64 MiB; an individually oversized part only exercises the per-part limit.
Assert the expected footnote finding and embedding aggregate-limit gap so neither
acquisition failure nor per-part rejection can satisfy the test.

Merge policy: this proposal and implementation increments remain reviewable;
"no merge authorization" records the current owner instruction, not an inherent
ban on accepting this specification. A later explicit owner instruction may
merge the documentation alone without clearing implementation or comparator gates.
