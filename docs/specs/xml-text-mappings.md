# XML decoded-text mappings — XM-001

Status: proposed #14 extraction prerequisite, stacked on OI-001 (#68).
`xmlparts.ParseWithTextMaps` performs one XP-001 XML parse and maps every resulting
character-data token back to original part bytes. Existing `Parse`, identification
behavior and report schemas remain unchanged. This is neither rendered document
text nor a DOCX/ODT body extractor.

## Evidence and coordinates

The result retains the original parsed document and its part SHA-256, mapper version
`xml-text-map/1`, and ordered text segments. Every segment references its XML token
and element, full token span, lexical content span, decoded text/hash and scalar maps.
A CDATA flag distinguishes literal CDATA content from ordinary XML character data;
its delimiters belong to the token span and are excluded from the content span.

Each mapping links one decoded Unicode scalar to:

- Its zero-based index within the segment.
- Its Unicode code point.
- Its complete half-open original part-byte region.
- Its half-open UTF-8 region within the decoded segment text.
- The observed transformation: literal, character_reference, entity_reference or
  line_ending.

These are original **part** bytes, not ZIP-container bytes or UTF-16 coordinates.
Source and decoded UTF-8 regions may have different widths. `&#x200B;`, for example,
maps eight lexical ASCII bytes to one scalar occupying three decoded UTF-8 bytes.
A literal CRLF pair maps two source bytes to one decoded LF; a referenced CR remains
CR and does not merge with a following literal LF. CDATA references remain literal
characters. Supplementary emoji are one scalar, while combining characters retain
their own scalar indices. No Unicode normalization occurs.

Scalar source regions partition each lexical content span, and decoded UTF-8 regions
partition each segment's text. Reconstructing decoded text from the mapped scalars
must exactly equal XP-001's independently parsed value; disagreement fails with
`ErrMapping`. This check prevents a second decoding implementation from silently
creating a different text interpretation. It is not proof that source bytes and
extracted bytes are interchangeable or authorization to remove either region.

The mapper handles XML character data only. Attributes, comments, processing
instructions and declarations retain their existing structural anchors and do not
silently become body text. Outside-root whitespace is included with element `-1`.
Empty CDATA can yield an empty segment with zero scalar mappings. BOM-aware part
coordinates retain their original three-byte shift.

## Bounds and authority

The caller supplies XP-001 limits and a positive document-wide scalar-map budget,
with a hard maximum of 200,000. The budget covers all segments together. It does
not change base XML parsing limits or force identification-only callers to allocate
scalar maps. XML token count also bounds segment count; parser storage, mapping
structs, decoded strings and builder scratch space are additional memory, so the
budget is not an RSS promise. Cancellation is checked between mapped scalars and
segments, in addition to XP-001's parse boundaries.

Every error returns no partially mapped document. Expected part identity is checked
by the XML parser; no file path is opened or reread. Input must remain immutable
during the call. Output owns its map slices. No external entity, renderer, detector,
subprocess or source modification is involved.

A native Unicode finding's character index can join to a segment's scalar map. Its
existing byte offset still belongs to the analyzed decoded-text artifact; it must
not be relabeled as a source-part offset. The future report integration must retain
both artifact identities and the explicit transformation map. Existing report
versions are not widened by this internal helper. Exact source navigation also
requires the correct package source digest and part name, not a part hash alone.

## Validation and next work

Manual vectors assert literal, entity, numeric reference, CRLF, referenced CR,
CDATA, BOM, supplementary emoji and combining-character coordinates. Tests cover
empty CDATA, document-wide budgets, invalid XML/identity, cancellation, decoder/map
mismatch, ownership, source immutability, deterministic output and partition/reconstruction
invariants. A native Unicode analyzer integration test joins its ZWSP finding at
scalar index 1 to the original `&#x200B;` region `[4,12)`, independently of the
analyzer's decoded-text byte offsets.

```sh
go test ./internal/xmlparts
go test -race ./internal/xmlparts
go test ./...
go vet ./...
go test ./internal/xmlparts -run '^$' -fuzz FuzzXMLTextMaps -fuzztime 3s -parallel 2
```

Next: format-specific selection of DOCX/ODT text, structural whitespace and hidden
contexts; metadata/structure observations; reviewed profiles and report/CLI mapping
integration. Concatenation across runs/paragraphs is not implemented here and must
preserve separate source regions and any synthetic separators. #14 remains open.

Validation record: Linux amd64 / Go 1.27.1 full Go suite, vet and XML package race
checks passed. The 265 offline contract tests passed. A three-second, two-worker
fuzz configuration executed 101,424 inputs without failure (about four seconds
including shutdown). The analyzer-join regression also passed. These bounded checks
do not certify complete Office extraction, format conformance or remediation safety.

The next proposed per-part consumer is [WT-001](word-text-evidence.md), which selects
WordprocessingML text and direct context while retaining these mappings unchanged.
The proposed ODF consumer [OT-001](odt-text-evidence.md) likewise retains stored
character maps, with structural whitespace represented separately as XML controls.

### Practical size limitation

The 200,000-scalar cap is independent of the 4 MiB XML byte cap. An ordinary
approximately 310 KB story containing 300 paragraphs of 1,000 ASCII characters
exceeds it despite satisfying XML byte, token and element limits. Word/ODT
mapped extraction and dependent analysis return `ErrLimit` with no result in
this case. This is an unassessed part, never a completed scan with no findings.
Identification-only parsing remains available under its own limits.

Partial mapping is deferred: it requires an explicit coverage contract for
mapped versus omitted regions, scope boundaries at omissions, and downstream
report/CLI handling before it can safely replace all-or-nothing failure. The
package/story orchestration work must preserve this failure as incomplete
coverage; this internal foundation does not yet expose structured auditing in
the public CLI. Raising a byte limit alone does not lift the scalar cap.
