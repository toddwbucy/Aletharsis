# Word stored-text analysis scopes — WA-001

Status: proposed #14 increment, stacked on OT-001 (#71). `wordanalysis.Analyze`
performs WT-001 extraction, bounded cross-run assembly, and native Unicode, emoji
and pattern analysis. It does not change report schemas or CLI format support.

## Assembly policy

Stored characters in consecutive Word text segments can form one analysis scope
across ordinary run formatting. This allows a zero-width sequence distributed
among formatting runs to reach the existing pattern detector's threshold. Direct
visibility flags can differ across these runs; per-character origins retain the
individual WT-001 contexts rather than flattening them to a single visibility claim.

Each scope has one paragraph, text role and revision/foreign-wrapper context.
A changed context ends the scope. XML start/end tokens other than supported run/text
containers or direct run-property subtrees also end a scope. Thus controls, field
markers, drawings, paragraphs, revisions, arbitrary structures and bookmark markers
are conservative boundaries. XML comments and processing instructions also split
analysis; their contents are never executed. Non-whitespace unselected character
data splits scopes. XML indentation is not added to selected text.

CDATA boundaries and character-reference syntax do not split the selected character
sequence. Empty text can retain an empty scope. No synthetic separator or rendered
character is introduced. Scope boundaries record the terminating token and reason
when there was an active selected scope, except the explicit excluded-text
coverage records described below; this is not an inventory of every structural
element. Complete structural evidence remains in the retained WT-001 result.

This is an explicit stored-text analysis projection, not a reconstruction of what a
viewer displays. Cross-paragraph patterns and patterns crossing conservative
boundaries are not assessed by these scopes. Effective visibility, style inheritance,
layout and field evaluation remain unresolved. Unknown extraction coverage remains
available through the retained extraction state/issues; findings do not erase it.

## Source mappings and analyzer results

Every scope records a deterministic local ID (`word-scope/N`), exact assembled text,
UTF-8 SHA-256, paragraph, role, scalar origins, normalization/hash observations and
native findings. IDs are local to this result; durable identity also requires the
assembly/extractor versions, package source identity, part name/hash and scope index.
A text digest alone does not identify a unique location.

Each scalar origin references its WT-001 text observation, XM-001 segment and scalar,
original part-byte span, assembled UTF-8 span and XML transformation. Source spans
can be discontiguous and can differ in width from assembled UTF-8 spans. Formatting
and XML markup are not included in the assembled text. A character encoded as
`&#x200B;` retains its eight-byte lexical region even though it occupies three bytes
in the scope. No larger envelope spanning intervening markup is an edit range.

Findings use the existing native Unicode, Emoji and Patterns analyzers and their
existing IDs, severities, confidence and thresholds. Their `character_offsets`
index the scope's scalar origins; their `byte_offsets` index the **assembled UTF-8
artifact**, never original XML. Scalar origins supply the second explicit mapping
to source evidence. Findings without occurrence offsets (such as normalization
observations) describe the whole scope and do not acquire fabricated locations.

The Unicode analyzer's raw/NFC/NFKC/formatting-removed hashes and normalization
observations are retained. Transformations are analytical only. A leading FEFF
selected from XML is not treated as an input-file BOM. Emoji observations remain
available for contextual review. No native detector is asked to infer model/vendor
watermarks, and these findings carry no removal authorization.

## Bounds and validation

WT-001/XP-001/XM-001 limits apply before assembly. Selected scalars are copied once
into scopes, so their aggregate count cannot exceed the 200,000-scalar XML mapping
budget; assembled UTF-8 is bounded by four bytes per selected scalar. Token limits
bound scope/boundary counts. Builders avoid repeatedly copying growing prefixes.
Analyzer normalization, findings, contexts and scratch memory add overhead; these
are not total RSS promises. Cancellation is checked during assembly, between scalars
and between analyzer phases/scopes. Individual native analyzers run synchronously
within the bounded input; cancellation is cooperative, not preemptive.

Every error returns no result. Expected source-part identity is checked before
extraction, and input must remain immutable during the call. No path, renderer,
network, shell, field executor or writer is involved. Returned evidence owns its
storage and source bytes remain unchanged.

Tests detect a 48-character binary-looking sequence split among three formatting
runs (each individually below threshold), including numeric XML references and a
direct hidden-text flag. Every finding position joins to the exact original lexical
bytes and individual run context. Negative cases keep controls, fields, drawings,
comments, paragraphs, revision instances and text roles from creating false
adjacency. Further cases cover CDATA, normalization across segments, emoji, FEFF,
indentation, cancellation, identity, determinism and source integrity. Fuzz tests
check mapping and finding bounds.

```sh
go test ./internal/wordanalysis
go test -race ./internal/wordanalysis
go test ./...
go vet ./...
go test ./internal/wordanalysis -run '^$' -fuzz FuzzAnalyze -fuzztime 3s -parallel 2
```

Remaining work: ODT assembly with explicit control transformations; multi-story and
package orchestration; broader structural/metadata analyzers; reviewed profile
producers; report/import and CLI integration; PDF and independent Office-oracle
validation. #14 remains open. This increment alone does not enable DOCX auditing
in the public CLI.

Validation record: Linux amd64 / Go 1.27.1 full Go suite, vet and wordanalysis race
checks passed, alongside 265 offline contract tests. A three-second two-worker fuzz
configuration exercised 87,447 inputs without failure (about four seconds including
shutdown). These checks are bounded native validation, not renderer equivalence or
independent Office-oracle acceptance.

## Context admission and exclusions

WT-001's [stored-text context grammar](word-text-evidence.md#stored-text-context-admission)
is authoritative. WA-001 consumes each text's `AnalysisContext.BlockedElement`
rather than reconstructing a separate property exclusion list. Every selected
segment with an unsupported ancestor remains extraction evidence but contributes
no analyzer input. It flushes any active scope and always records a
`text_context_not_analyzed` boundary at its token, even with no active scope or
with whitespace-only text. Extraction declares partial coverage with the located
`word.text_context_not_analyzed` issue and first unsupported ancestor edge.

Ordinary direct run properties remain transparent only for adjacency between
otherwise admitted text segments. Unknown containers, foreign wrappers and
misplaced recognized containers cannot authorize analysis or reset blocked
ancestry. Deleted text and field instructions on admitted paths continue to be
analyzed in their distinct roles. These exclusions declare coverage gaps, never
negative detector results. Full OOXML validity and rendered visibility remain
outside this grammar's claims.

## Office report serialization boundary

[OI-001](office-cli-evidence.md) supersedes this specification's finding-location
key names only when serializing Office findings into report 4.0. Library results
keep `character_offsets` and `byte_offsets` with their existing scope-relative
semantics; the report assembler translates to `scope_character_offsets` and
`scope_byte_offsets`, with required `scope_ref` and `coordinate_artifact_ref`.
These local library findings must not be serialized directly as legacy file-byte
locations. The 4.0 scope-table validator verifies the translated coordinates.
