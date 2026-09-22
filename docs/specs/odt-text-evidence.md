# ODF stored text and control evidence — OT-001

Status: proposed #14 increment, stacked on WT-001 (#70), using XM-001 independently
of the Word extractor. `odttext.Extract` accepts an identity-checked ODF content XML
part. It does not change report schemas or advertise ODT support in the CLI.

## Scope and identity

Admission requires an `office:document-content` root, one direct `office:body`,
and exactly one child element of that body:
`office:text`. Namespace URIs, not prefixes, determine element identity. The caller
must separately establish package identity, part name, manifest membership and
unencrypted content through OI-001. XML shape alone does not identify an ODT file. Versions other than 1.2/1.3
(including an absent version) retain recognized text with partial state and
`odt.content_version_unsupported`; this is observation of the admitted structures,
not conformance certification or full semantic support for those versions.

The result retains the complete XM-001 mapped XML and part SHA-256, parser version
`odt-text/1`, document version, ordered text/control/declaration observations,
unselected character data, issues and limitations. All locations refer to original
part bytes; they are not ZIP-container locations or rendered-text offsets.

Text observations select stored character-data segments with a `text:p` or `text:h`
ancestor inside office:text. Each references its segment, element, nearest paragraph
and ordered context elements (self first, ending at office:text). Original entities,
CDATA and XML line-ending transformations retain the exact XM-001 maps. No whitespace
collapsing, inter-segment concatenation, field refresh or Unicode normalization runs.
Empty elements remain in XML without manufactured text.

All other character-data segments receive a retained unselected reason. Non-whitespace
unclassified content produces a partial result. A narrow set of paragraph, inline,
section, list, note, annotation, ruby, hidden/conditional and revision contexts is
recognized; unfamiliar wrappers preserve text and anchors but make context partial.
For example, table/drawing layout, arbitrary fields and script elements remain
unresolved. Recognition is not an ODF grammar or a statement that content is benign.
Note citations, note bodies, annotations, deleted text and ruby text remain separately
located segments; they are not silently concatenated into a rendered paragraph.

## Hidden and conditional declarations

`text:hidden-text`, `text:hidden-paragraph` and `text:conditional-text` elements are
retained as declaration references, with their attributes preserved in XML. Text
observations link hidden/conditional ancestors and every hidden-paragraph declaration
whose nearest paragraph matches their own, including declarations nested in spans.
Lists preserve ancestor order followed by paragraph declarations in document order.

Conditions, declared `text:is-hidden` values and alternative strings are never
executed, refreshed or interpreted as effective visibility. Styles, application
state, change ranges and rendering can affect what a viewer shows. No declaration
means only that this extractor found no linked declaration; it does not mean visible.
Section display/style attributes remain accessible through context elements but
are not evaluated. Tracked-change ancestry is preserved; paired change-marker ranges
are not resolved. No watermark classification follows automatically from a field.

## Structural whitespace

Namespace-qualified `text:s`, `text:tab` and `text:line-break` elements inside the
text body produce separate control observations with element, nearest paragraph,
kind and exact full-element span. They do not produce scalar mappings or synthesized
characters. A future assembly layer must describe any expansion as a transformation
linked to the element rather than pretend generated spaces existed in source bytes.

Space counts default to one. An explicit text:c admits nonnegative decimal integers,
XML whitespace at the ends, an optional plus and leading zeroes; counts must fit
uint64. Invalid/overflowing values, unexpected attributes, child elements, non-whitespace
content or missing paragraph context produce an unknown count and partial result.
The numeric count is zero when unknown and is never used to allocate or expand text.
Tab references are retained in XML without resolving tab-stop layout. Unknown control
ancestry is also partial. This is conservative admission, not full schema validation.

## Resources, integrity and limits

XP-001 default parse budgets and XM-001's 200,000-scalar document budget apply. At
most 200,000 context/declaration references may be retained across text observations;
XML depth bounds ancestor walks. Element/token limits bound declarations and controls.
Stored repetition counts do not increase output size. These bounds cover declared
work/storage dimensions, not total RSS or a hard execution-time guarantee.

Errors, including source identity, XML, unsupported root, cancellation and resource
failures, return no partially usable result. Partial successful results preserve
observed evidence and typed coverage issues. `completed` only describes this stored
text-selection scope; limitations still include unresolved rendering, styles,
conditions, whitespace semantics, change ranges and cross-segment assembly.

The caller must not mutate input during extraction. The operation does not open
files, fetch links, execute scripts/fields, run subprocesses or modify source bytes.
Neither an exact location nor a declaration authorizes removal.

## Validation and remaining work

Tests assert entity/CDATA/Unicode source mappings; separate controls; default,
large, malformed and overflowing counts; nested hidden-paragraph declarations;
condition preservation; note/revision/annotation contexts; unfamiliar wrappers;
namespace spoofing; versions/body selection; complete segment accounting;
determinism, ownership, source integrity, cancellation and context/scalar limits.
The committed hidden ODT fixture passes package identification and extraction of
64 zero-width characters with each character checked against original part bytes.

```sh
go test ./internal/odttext
go test -race ./internal/odttext
go test ./...
go vet ./...
go test ./internal/odttext -run '^$' -fuzz FuzzExtract -fuzztime 3s -parallel 2
```

Remaining work includes cross-segment analyzer scopes and control expansion maps,
style/metadata/structure observations, supported field/layout interpretation or
explicit coverage gaps, reviewed profile producers, report/import and CLI integration,
plus independent Office-oracle validation under #40. #14 remains open.

Reference: [OASIS ODF 1.3 Part 3](https://docs.oasis-open.org/office/OpenDocument/v1.3/os/part3-schema/OpenDocument-v1.3-os-part3-schema.pdf),
text controls (§6.1) and hidden fields (§7.7). Resource limits, conservative selection,
unknown handling and non-evaluation are Aletharsis implementation policies.

Validation record: Linux amd64 / Go 1.27.1 full suite, vet and odttext race checks
passed, as did 265 offline contract tests. A three-second two-worker fuzz configuration
exercised 172,110 inputs without failure (about four seconds including shutdown).
These native checks do not establish full ODF conformance or renderer equivalence.

[OA-001](odt-analysis-scopes.md) proposes the analysis consumer, with bounded inline
assembly and separately classified origins for explicit whitespace expansion.

A known zero space count is legal under the [ODF 1.3 schema](https://docs.oasis-open.org/office/OpenDocument/v1.3/os/schemas/OpenDocument-v1.3-schema.rng)
(`text:c` uses `nonNegativeInteger`). `CountState` distinguishes known zero from
unknown counts. Zero contributes no characters or origins and does not split an
analysis scope or create an empty scope; its control/XML evidence remains retained.
