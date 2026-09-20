# Text audit reference

This reference describes the implemented Go 0.2.0 text analyzers and schema 1.0.
For installation and commands, see the [README](../README.md). Planned capabilities
are tracked in the [roadmap](../ROADMAP.md).

## Evidence and classification

Findings carry stable rule IDs, category, severity, confidence, classification,
description, evidence and location. IDs identify rules, not unique occurrences;
multiple findings can share an ID. Classification is one of `observed_fact`,
`suspicious_pattern`, `likely_mechanism`, or `undetermined`. Built-in text checks
use the first two; they do not claim confirmed watermarks. Things the tool cannot
determine are recorded separately in `limitations`.

| Severity | Interpretation in this release |
| --- | --- |
| INFO | Common observable artifact, such as a BOM or normalization difference |
| LOW | Artifact worth reviewing, such as an isolated zero-width character |
| MEDIUM | Stronger review priority: identifiers, unexpected controls, periodic insertions or long selector/tag runs |
| HIGH | Long, structured two-symbol zero-width pattern consistent with encoding |

Severity is review priority. Confidence describes support for the stated fact or
pattern, **not** the probability of a watermark; heuristic values are not calibrated.
UUID syntax does not establish persistence or uniqueness. Legitimate ZWJ/ZWNJ,
emoji, variation selectors, bidi controls, combining marks, typography and multilingual
text can produce findings. Mixed-script checks flag Latin with Greek/Cyrillic in
one token; they are not a comprehensive Unicode confusables implementation.

Literal emoji-capable characters produce `unicode.emoji` findings at LOW severity
in every supported text file, including source code. Comments, docstrings, string
literals and test data are all inspected; their location does not automatically
justify an exemption. Reviewers must determine whether the content is needed for
the application or document. No automatic approval, removal, or language-semantic
analysis is performed.

Detection uses bundled [Unicode 17.0 Emoji property data](https://www.unicode.org/Public/17.0.0/ucd/emoji/emoji-data.txt)
with no runtime network access or extra dependency. Ordinary ASCII digits, `#`
and `*` are excluded unless part of a keycap sequence. Text/emoji-capable symbols
such as copyright marks are reported as candidates even with text presentation;
their appearance depends on presentation selectors and rendering. Counts and
offsets refer to individual code points, not complete rendered emoji. Flags,
skin-tone modifiers and joined emoji can therefore contribute multiple code
points. Joiners, tags and selectors remain separately inventoried. Code-point
names use pinned Unicode 15.0.0 data matching the Python reference and may be unavailable for newer characters,
but detection still uses the pinned table. The bundled data's source hash and
Unicode license are included in `internal/analyzers/data/` and in the frozen Python source.

This is literal-source inspection: an ASCII escape such as `\\U0001F600` is not
decoded into an emoji, and Emojicode or other language semantics are not analyzed.
Emoji findings indicate a review obligation, not a confirmed threat or watermark.

The Unicode inventory reports code point, name, count, **all** positions and up to
eight escaped context samples per character. Context omission counts are explicit.
Normalization checks use NFC and NFKC. Text hashes use UTF-8 encoding of the exact
extracted string, NFC, NFKC, and a separate formatting-removed view. Removal for that
comparison includes the documented zero-width set (U+200B/C/D, U+2060–2064, U+FEFF),
bidi controls, variation selectors, tags, and soft hyphen. It retains spaces and
combining marks and does not imply those removed characters are unnecessary.

Pattern rules are deliberately inspectable:

- ZWSP/ZWNJ binary candidate: at least 32 occurrences, both symbols, minority share
  at least 10%, and at least 75% adjacent pairs or constant spacing. Reports counts,
  adjacency and exact sequence periods up to 32 characters (at least three repeats).
- Periodic insertion: at least 12 zero-width characters at the same interval greater
  than one character. The initial BOM and U+200D (ZWJ) are excluded from this
  heuristic; joiners remain visible in the Unicode inventory.
- Contiguous variation-selector runs of at least 8, or tag runs of at least 16.
  Tag ASCII projection is evidence of character values, not proof of a payload.
- UUID-shaped values (including repeated occurrences), explicit provenance labels,
  and long syntactically Base64-compatible tokens. These are neither validated
  tracking mechanisms nor recursive payload inspection.

Statistical model-output watermarking requires a known algorithm and often a key.
Aletharsis cannot determine vendor or model authorship from prose alone. Future explicitly configured statistical detectors require the separate mechanism
and result contracts described in the [parent PRD](product/aletharsis-PRD.md).

## JSON contract

The complete versioned schema is [schemas/report.schema.json](../schemas/report.schema.json).
Nested contracts use reusable `$defs` and finding variants keyed by stable rule
IDs. Evidence, locations, context samples, thresholds, and text structure reject
unknown keys and require their documented fields. Normalized metadata accepts
optional string-valued creator/editor/application/date/revision/template/document
identifier fields defined in the schema; M0/M1 text parsing still emits no metadata.
Unknown metadata keys and structured values require an explicit schema extension.
Read/parse failures retain valid empty evidence structures and their own error
contracts. The `unicode.emoji` contract also supports the separately reviewed emoji
analyzer without requiring it to be installed.

Schema validation checks shapes and types, not semantic integrity: consumers must
still verify source hashes, bounds, monotonic offsets, correspondence between
offset arrays and text, and consistency of counts. New detector IDs or evidence
shapes require an explicit schema update; there is no permissive fallback.

The migration also retains a frozen compatibility snapshot in
[reference/python-behavior/report.schema.json](../reference/python-behavior/report.schema.json).

Top-level fields:

| Field | Contents |
| --- | --- |
| `aletharsis_version`, `schema_version` | Tool and report contract versions |
| `file` | Supplied path, filename, extension, detected format/MIME and basis, size, byte SHA-256, parser |
| `status` | `completed` or `failed` |
| `summary` | Finding count, severity counts, exit code |
| `evidence` | Extracted text, encoding, BOM, byte offsets, line endings, hashes, normalization, metadata and structure |
| `findings` | Classified findings with evidence and locations |
| `limitations` | Known blind spots and interpretation constraints |

Offsets are zero-based Unicode code-point indices and original-file byte offsets,
not graphemes or screen columns. Each text segment's `byte_offsets` maps every
character index to its byte start and includes the EOF offset as the last element.
Full source text is retained in JSON. `file.sha256` hashes original bytes, which can
differ from `raw_text_sha256` for non-UTF-8 input. Unavailable identity fields are
null on read failure. Failures are valid JSON reports with `parser.failure` findings.

Go output uses fixed struct field order, sorted map keys, deterministic finding
ordering, escaped Unicode, and no runtime
timestamps or random IDs. It is reproducible for the same supplied path, bytes,
configuration and pinned Unicode database version (recorded in evidence).
Serialization whitespace/key/finding order and human-readable failure prose may
differ from Python; those differences are disclosed in the migration record.

