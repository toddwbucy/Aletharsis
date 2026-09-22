# ODF analysis scopes and explicit control expansion — OA-001

Status: proposed #14 increment, stacked on WA-001 (#72). `odtanalysis.Analyze`
uses OT-001 extraction, assembles bounded analysis scopes and invokes native Unicode,
emoji and pattern analyzers. It does not change report schemas or CLI format support.

## Projection and boundaries

Consecutive stored text can join across `text:span` and `text:a` boundaries. XML
character references and CDATA remain part of that sequence. This makes recurring
Unicode patterns detectable across inline formatting while retaining original XML
locations and context. Links are not followed. No XML whitespace collapsing, style
resolution, field execution or effective-visibility inference occurs.

Paragraph changes and other XML structures end scopes. Hidden/conditional fields,
notes, annotations, ruby components, revisions, unknown structures, comments and
processing instructions therefore remain separate. Non-whitespace unselected data
also ends a scope. All underlying OT-001 observations, unselected segments, issues
and limitations remain in the result. Scope boundaries record the terminating token
and reason when a selected scope was active, except `control_expansion_limit`:
every omitted expansion retains its control-token anchor, even without an active
scope (see resource behavior below). Repeated skips identify distinct controls,
not repeated terminations of a nonexistent scope. These records are bounded by
the parsed token inventory and are not a complete XML inventory.
This policy intentionally leaves cross-boundary and cross-paragraph patterns unassessed.

Admitted OT-001 text controls are an explicit exception: known `text:s` counts expand
to ASCII spaces, tabs to U+0009 and line breaks to U+000A **in the analysis artifact**.
Unknown controls produce no invented characters and split scopes. Tab-stop layout
is not computed. Literal stored whitespace remains literal; this is not an ODF
renderer's normalized visible text. A control-only paragraph may produce a scope.

## Two origin classes

Every generated scope scalar has one ordered origin and an assembled UTF-8 span.

- `stored`: references OT-001 text, XM-001 segment/scalar, exact original part-byte
  region and XML transformation. Control and repetition indices are `-1`.
- `control_expansion`: references the OT-001 control, zero-based repetition ordinal,
  full control-element part span and transformation kind (`spaces`, `tab` or
  `line_break`). Text, segment and scalar indices are `-1`.

For expansion origins, the source span is an **element anchor**, not a physical
character region. Several analysis characters may intentionally refer to the same
`<text:s text:c="3"/>` element. These records do not authorize character deletion
or imply a byte-for-byte edit mapping. Original XML elements and attributes remain
available through the extraction evidence.

Each scope records a local ID (`odt-scope/N`), paragraph, exact projected text and
SHA-256, scalar origins, normalization/hash observations and native findings. Durable
identity also requires package source identity, part name/hash and all relevant
parser/assembly versions; a scope text hash alone is not a unique source location.

Native findings retain existing IDs, severities, confidence and thresholds.
`character_offsets` index scope origins; `byte_offsets` belong to the assembled
UTF-8 artifact, not the original XML. A finding's source is its local scope ID.
Whole-scope normalization findings do not gain artificial occurrence locations.
Raw/NFC/NFKC/formatting-removed hashes are retained without changing any source bytes.
A leading FEFF in XML character data is not treated as a transport BOM.

## Limits and failure semantics

OT-001/XP-001/XM-001 bounds apply before assembly. OA-001 additionally admits at most
200,000 projected scalars **across all scopes combined**, including both stored and
expanded characters. Capacity for all selected stored text is reserved before any
control expansion. Stored text alone exceeding this bound still returns
`xmlparts.ErrLimit`. Counts exceeding the remaining expansion capacity are not
expanded: the analysis State becomes `partial`, a `control_expansion_limit` boundary
records the control token even when no scope is active, and analysis continues.
The extraction and its known count remain unchanged; a resource limit does not
make the observed count unknown. Scopes never join across the omitted control.
Unknown or overflowing OT-001 counts also split scopes; analysis State inherits
any partial extraction state. Counts are checked before allocation or conversion.

Builders avoid repeated prefix copies. Token budgets bound scope/boundary counts.
UTF-8 text is at most four bytes per admitted scalar, but origins, extraction data,
normalization and analyzer scratch storage add memory; this is not an RSS promise.
Cancellation is checked during assembly/expansion and between analyzers/scopes.
Native analyzers are synchronous within the bounded input, so cancellation is
cooperative rather than preemptive. All errors return no partial analysis result.

The caller supplies an immutable part snapshot and expected digest. The host must
establish package/manifest membership and unencrypted content separately. No file,
network, renderer, script/field interpreter or writer is invoked. Exact mappings,
pattern findings and profile judgments never confer removal authorization.

## Validation and remaining work

Tests verify a 48-character sequence split across inline spans, joining native
finding offsets to literal/entity XML bytes. Explicit whitespace vectors verify
multiple spaces sharing a control anchor and distinct stored versus expanded
origins. Negative cases retain paragraph, hidden/conditional, non-text and unknown
control boundaries. Budget tests cover the exact ceiling, aggregate scopes, stored
plus generated counts and maximum uint64 declarations. Other tests cover CDATA,
emoji, normalization, FEFF, determinism, cancellation, identity and immutable input.
The committed hidden ODT passes package identification through pattern analysis,
with all 64 hidden characters retaining their declaration context.

```sh
go test ./internal/odtanalysis
go test -race ./internal/odtanalysis
go test ./...
go vet ./...
go test ./internal/odtanalysis -run '^$' -fuzz FuzzAnalyze -fuzztime 3s -parallel 2
```

Remaining #14 work includes package/story orchestration, structural/metadata analysis,
reviewed profile producers and report/import/CLI integration, PDF support and full
review/merge acceptance. Independent Office-oracle validation remains a #40 gate.
This increment does not establish rendered-text equivalence or public ODT CLI support.

Validation record: Linux amd64 / Go 1.27.1 full suite, vet and odtanalysis race
checks passed, alongside 265 offline contract tests. A three-second two-worker fuzz
configuration exercised 192,424 inputs without failure. These bounded native checks
do not establish ODF conformance or independent Office-oracle acceptance.

## Office report serialization boundary

[OC-001](office-cli-evidence.md) supersedes this specification's finding-location
key names only when serializing Office findings into report 4.0. Library results
keep `character_offsets` and `byte_offsets` with their existing scope-relative
semantics; the report assembler translates to `scope_character_offsets` and
`scope_byte_offsets`, with required `scope_ref`; its scope record identifies the coordinate artifact.
These local library findings must not be serialized directly as legacy file-byte
locations. The 4.0 scope-table validator verifies the translated coordinates.
