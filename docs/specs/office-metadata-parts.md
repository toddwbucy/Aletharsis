# OM-001 — Core/application metadata part evidence

Status: implementation proposal stacked on OI-001. This is a part producer, not
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
unsupported by this producer; they must retain explicit coverage outcomes.

Selected core values are DC creator/identifier, DCTERMS created/modified and core
lastModifiedBy/revision. Selected app values are Application, AppVersion, Company
and Template. Only direct, leaf properties under the admitted root yield values.
A nested recognized element cannot reset ancestry or authorize extraction.
Unknown direct properties and structured values remain in the retained XML and
emit located issues with partial coverage; good neighboring values survive.
Only literal XML whitespace at the root is ignored. Other root text produces a
coverage issue, including non-breaking and zero-width characters.

Each property keeps expanded name, element index, normalized key, decoded value,
zero-based occurrence ordinal among the same expanded name and ordered references
to XM text segments. CDATA, character references and XML newline normalization
retain their original lexical scalar maps. Concatenated property values do not
claim a contiguous source byte interval. Empty leaf values remain observations
with an empty, non-null segment list. Comments, attributes and processing
instructions remain XML evidence, never executed or incorporated into values.

Duplicate values are not collapsed or last-writer-wins. Date/revision strings are
preserved verbatim after XML decoding even when invalid as dates/numbers. These
are observed metadata, not verified author identities or provenance assertions.
The producer performs no finding classification or profile suppression.

Validation covers duplicates, entities, CDATA/non-BMP mapping, empty values,
namespace spoofing, nested known elements, unknown siblings, root text, malformed
XML/external declarations, identity mismatch, cancellation, scalar limits,
deterministic outputs and immutable source bytes. These tests do not establish
package-wide or CLI isolation; those belong to the orchestration increment.
