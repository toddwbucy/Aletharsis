# OM-001 — Core/application metadata part evidence

Status: implementation proposal stacked on OC-001. This is a part producer, not
CLI support, package identification or completion of #40/B2. Embedded inventory,
package binding and report assembly remain separate increments.

`officemetadata.Extract` accepts immutable part bytes and their expected SHA-256.
It uses XP-001/XM-001 under their existing byte/token/scalar limits. No file or
network access, second XML parser, timestamp interpretation or normalization.
Errors return no result; a coordinator must retain the failed-part outcome and
continue independent parts. An error here never grants permission to discard a
whole batch.

Recognized roots are the exact transitional `cp:coreProperties` and extended
`Properties` expanded names. A caller must independently establish the package
relationship and content type. A namespace/root or filename alone is insufficient.
Strict extended-properties namespaces, custom properties and ODF metadata are
unsupported by this producer. Recognized strict-app, custom-property and ODF roots
return `UnsupportedRootError` with `metadata.strict_app_unsupported`,
`metadata.custom_properties_unsupported` or `metadata.odf_unsupported`. These
errors still wrap `ErrStructure`, but coordinators can distinguish them with
`errors.As`; unrelated roots return plain `ErrStructure`. No failed extraction
returns a partial result.

Selected core values are DC creator/identifier, DCTERMS created/modified and core
lastModifiedBy/revision. Selected app values are Application, AppVersion, Company
and Template. Only direct, leaf properties under the admitted root yield values.
A nested recognized element cannot reset ancestry or authorize extraction.
Unknown direct properties and structured values remain in the retained XML and
emit located issues with partial coverage; good neighboring values survive.
Only literal XML whitespace at the root is ignored. Other root text produces a
coverage issue, including non-breaking and zero-width characters. Ignorability
is checked against literal source bytes and excludes CDATA, so `&#x20;`, `&#xD;`
and CDATA-wrapped whitespace are not silently ignored. Literal CR/CRLF remains
XML whitespace despite XML newline normalization.

Every issue includes an element, a part-byte span and segment/attribute indices
(`-1` when not applicable). Root-text issues select their exact XML segment and
token span; distinct root segments remain distinct observations. Property issues
use the element's full span; attribute issues identify the attribute index and
its enclosing start-tag span (not an invented exact attribute byte interval).
Issues are stably ordered by source-span start, with insertion order for ties.

Known but unselected vocabulary (including title/subject/description, keywords,
category/contentStatus and application statistics/vector properties) yields
`metadata.standard_property_unassessed`. Unknown expanded names yield
`metadata.property_unassessed`. Both retain partial coverage; recognizing a
standard name does not imply its contents are trusted, expected or interpreted.
The finite vocabulary is in `vocabulary`; it is not a format profile.

Each property keeps expanded name, element index, normalized key, decoded value,
zero-based occurrence ordinal among the same expanded name and ordered references
to XM text segments. CDATA, character references and XML newline normalization
retain their original lexical scalar maps. Concatenated property values do not
claim a contiguous source byte interval. Empty leaf values remain observations
with an empty, non-null segment list. Comments, attributes and processing
instructions remain XML evidence, never executed or incorporated into values.
Selected properties also retain non-declaration attribute indices and emit
`metadata.attribute_semantics_unassessed` for each, except namespace-resolved
DCTERMS W3CDTF types on created/modified, which emit
`metadata.standard_attribute_unassessed`. A Value is decoded character
data only: empty character data on an `xsi:nil` property is not a claim that its
logical value is an empty string, and `xsi:type` does not validate a date.

Duplicate values are not collapsed or last-writer-wins. Date/revision strings are
preserved verbatim after XML decoding even when invalid as dates/numbers. These
are observed metadata, not verified author identities or provenance assertions.
Successful results include machine-readable limitations: values are not validated,
identity is not verified, attribute semantics are unresolved, only selected
properties are projected, and package binding is not verified. Completed state means no declared unassessed markup of either class was found
in the part. It never means verified provenance or full Office semantics.
`Coverage.OtherGaps == 0` means no gaps beyond the recognized standard projection
gaps were observed; it does not certify full extraction or schema validity.
StandardProjectionGaps is normally nonzero on real producer metadata. The producer performs no finding classification or profile suppression.

Validation covers duplicates, entities, CDATA/non-BMP mapping, empty values,
namespace spoofing, nested known elements, unknown siblings, root text, malformed
XML/external declarations, identity mismatch, cancellation, scalar limits,
deterministic outputs and immutable source bytes. These tests do not establish
package-wide or CLI isolation; those belong to the orchestration increment.

Coverage distinguishes `StandardProjectionGaps` (recognized unselected properties
and declared date types) from `OtherGaps`. Both retain `partial` state. These
counts distinguish routine projection limits from other unassessed markup; they
are not trust or schema-validity verdicts. Standard subtrees are not validated
(`metadata.standard_subtrees_not_validated`).

Non-declaration root attributes are retained in `RootAttributes` and produce
attribute issues. Comments and processing instructions anywhere in the part produce
`metadata.markup_unassessed`, including those inside selected values. Each has
an exact token index and byte span; prolog/epilog markup has Element -1.
XML declarations are interpreted by the parser and do not produce markup issues. Issue `Token` is -1 when unavailable; text
issues retain their segment's token. Markup never enters concatenated values.

Validation resumed with user authorization after the original resource stop,
using the configurable hard 1 GiB ceiling. The original stop remains historical
budget evidence; resumed validation does not retroactively invalidate it.

Markup inside an already-unassessed subtree still produces a separate other-gap
issue: a standard property name does not make an enclosed PI/comment standard.
Counts describe observations, not disjoint unassessed byte regions; overlapping
parent/child issues are intentional and must not be summed as byte coverage.

Resumed validation evidence is retained in
[office-metadata-resumed.json](../testing/receipts/office-metadata-resumed.json)
and its referenced logs. The historical preflight receipt remains unchanged.
