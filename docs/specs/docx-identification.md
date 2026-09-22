# Evidence-based DOCX identification — DI-001

Status: proposed #14 increment stacked on OR-001 (#66). `internal/docxidentify`
identifies a Word main document from package declarations and located XML. It does
not render text, interpret hidden runs/styles, assess profiles, enable DOCX in the
CLI or certify complete OPC/OOXML conformance.

## Source and selection chain

`Inspect` receives one acquired snapshot, its expected SHA-256 and a context. The
OR-001 package/relationship inventory is retained intact; no source reread occurs.
Identification uses this chain rather than an extension or conventional part path:

```text
source SHA-256
  → exact package parts and relationship evidence
  → [Content_Types].xml identity + located declarations
  → one resolved package-root officeDocument relationship
  → selected part's effective content type
  → selected XML identity, document namespace and body element
```

The result retains content-type XML, declaration anchors, a per-part assignment
ledger, diagnostic issues, the chosen relationship/assignment indices and the main
XML document. Source, part, declaration and main-document identities stay distinct.
The original part bytes remain accessible through the package inventory.

A positive result requires exactly one resolved root `officeDocument` relationship,
a complete root-relationship inventory, the correct relationship-part MIME type,
the DOCX main-document MIME type, and a namespace-qualified `document` root with
exactly one direct `body` child. Strict versus Transitional relationship/Word namespace
pairs must agree. The selected part can be anywhere allowed by the relationship
policy; `word/document.xml` is not assumed. Selection never fetches an external main
part, executes embedded content or resolves namespace URIs over the network.

## Content-type declarations and assignments

The special content-types item must have its exact conventional name. ASCII-case
aliases are detected; ambiguous or differently cased special-item names produce an
explicit limitation rather than choosing arbitrarily. Its root must be `Types` in
the OPC content-types namespace. Default/Override records retain their original
keys, normalized MIME type, state/code and exact XP-001 element anchors.

Defaults match final filename extensions with ASCII case equivalence. Overrides
match package-root names with the same equivalence and take precedence. The admitted
override path policy shares OR-001's resolver but requires an absolute, already
canonical part path: no dot-segment rewriting, percent encoding, query or fragment
is guessed. Extensions use ASCII letters/digits/underscore/hyphen. MIME syntax is
checked with the standard library; media-type parameters are explicitly unsupported.
Raw lexical values remain in the retained content-types XML.

Duplicate equivalent defaults/overrides mark every duplicate ambiguous, even when
values happen to agree. Override collision checks use the same single-leading-slash
removal and ASCII case equivalence as assignment, including rejected declarations;
missing-slash aliases cannot change selection merely by being reordered. Unknown
element kinds never enter the default or override assignment maps. Unknown namespaces/elements/attributes, nested declaration
content, non-whitespace character content and unsupported key/type forms prevent
content-type selection authority. The complete parsed XML is retained for forensic
inspection, so an unsupported declaration is not deleted from evidence. Defects
in unrelated declarations mark coverage partial but do not stop main-part
inspection. Rejected or ambiguous declarations never supply assignments, and an
invalid override cannot fall back to a valid default for the same part. Root
structure failures and declaration-budget exhaustion still stop selection. A
malformed media type and malformed key on one declaration each retain an issue;
the declaration Code keeps the first validation failure.

When declarations are usable, every non-directory package part except the special
content-types item receives an assignment or an explicit unknown result. Missing
types, case-ambiguous names and overrides pointing at absent parts remain diagnostics.
An unrelated untyped part does not erase established main-document identity, but
it does prevent a completed package assessment. No profile may interpret an unknown
assignment as expected merely because DOCX identification succeeded.

## Outcome meanings

`Format: docx` records the supported identification chain and `Variant` is `strict`
or `transitional`. It does not prove schema conformance, rendering behavior, trust,
authorship or absence of hidden content. `State` is:

- `not_applicable`: neither a content-types item nor relationship candidates provide
  an OPC identification entry point (for example the retained ODT fixtures).
- `completed`: the supported identification checks passed without recorded gaps.
- `partial`: declarations, selection or other package observations contain explicit
  failures/unknowns. Format may remain `docx` if its main identity is nevertheless
  established; format is empty if selection itself remains unproven.

Main-relationship and assignment indices start at `-1` and become references only
when actually selected. Main part/hash and XML are recorded at their respective
verification stages; their presence without `Format: docx` is not identification
success. Unsupported macro-enabled/template/other Office main MIME types are recorded
as limitations rather than mislabeled DOCX. There is no negative watermark result.

Source/package errors return no Result. Later XML or semantic failures retain original
package evidence and stable issue codes. Cancellation preserves the context error at
checked execution boundaries. All inspected strings remain untrusted review data.

## Bounds and validation

DP-001 and OR-001 limits continue to apply. Content-types and selected-main XML each
use XP-001 limits (4 MiB source, 100,000 tokens, 50,000 elements, depth 128, 128 attributes
per element and 8 MiB retained name/value bytes). At most 4,096 content-type declarations
are emitted; exceeding that count retains the XML and marks the operation partial.
Per-part assignments are bounded by the package's 4,096-entry ceiling. These additional
XML passes are bounded but add memory/work to the existing inventory; they do not
promise a hard RSS or wall-clock bound. No third-party dependency was added.

Tests cover nonstandard main paths, default/override precedence, ASCII-case equivalence,
Strict/Transitional variants, source/part/declaration links, ambiguous declarations,
unsupported URI/MIME forms, missing/aliased/spoofed parts, external or multiple main
relationships, wrong relationship content type, malformed main XML, missing/duplicate
bodies, unrelated partial coverage, retained DOCX/ODT fixtures, immutable sources,
determinism, cancellation, declaration limits and fuzzed main XML.

```sh
go test ./internal/docxidentify ./internal/opcrels
go test -race ./internal/docxidentify ./internal/opcrels
go test ./...
go vet ./...
go test ./internal/docxidentify -run '^$' -fuzz FuzzMainXML -fuzztime 2s -parallel 2
```

Next: the separate ODT manifest/identification path, followed by DOCX/ODT text,
metadata and hidden-content observations with extraction mappings. Reviewed profiles,
report/import contracts and CLI/forensic views remain outstanding under #14. Office
oracle comparison remains gated under #40. This increment does not close either issue.

References: [Open XML main-part relationships](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oi29500/32be961f-d71a-4812-913f-b675c79aa88a),
[OPC parts and content types](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/opc/parts-overview),
and [DOCX Strict identification](https://www.loc.gov/preservation/digital/formats/fdd/fdd000400.shtml).
The bounded admission policy and evidence model are Aletharsis choices; no Office SDK
or document activation behavior described by those references is invoked.

## Increment validation record

Linux amd64 / Go 1.27.1: full Go suite, vet and race checks for identification and
relationships passed. The 265 offline contract tests passed. A two-second, two-worker
main-XML fuzz configuration completed 3,482 executions without failure (about 2.5
seconds including shutdown); each execution includes package construction and the
full package/relationship/identification path. An additional mixed-variant regression
also passed. This is bounded native validation, not Office conformance certification
or an independent forensic-tool comparison. No source file or external resource was
modified or accessed by the identification API.

## Independent ODT path

[OI-001](odt-identification.md) identifies ODT through its mimetype and manifest,
without reusing OPC case equivalence or URI resolution. Both paths preserve part
identity and expose unsupported or partial evidence before later profile integration.

Declarations with absent/empty keys retain their required-attribute defect rather
than being grouped as duplicate empty keys. Nonempty lookup-equivalent keys still
invalidate every colliding declaration, including malformed slashless overrides;
this avoids order-dependent assignment or fallback authority.

Declaration records retain the expanded element namespace and local kind. Foreign
attributes remain in located XML and mark coverage partial, but cannot overwrite
unqualified key/content-type facts. Only OPC-namespace Default/Override elements
participate in type lookup and collision detection; other elements remain evidence
without claiming that keyspace. An absent-target diagnostic requires an accepted
Override, not a missing or unsupported key.

## Proposed Office coordinator boundary

[OI-001](office-cli-evidence.md) adds an outcome-based package path and
verified-package inspector entry points for the 4.0 coordinator. Existing
`Read`/`Inspect` contracts here remain strict and unchanged. New entry points
consume one shared, bounded package outcome, preserving unassessed/failed parts
and incomplete prerequisites without re-running the strict reader or
decompressing again. OI-001's staged identification-priority budget applies to
that new path; it does not silently change this legacy API's failure semantics.
