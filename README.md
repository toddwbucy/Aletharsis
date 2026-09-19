# Aletharsis

A read-only command-line forensic auditor for Unicode artifacts, text structure,
identifier candidates, and provenance labels. Version 0.2.0 ports **M0/M1** to Go,
with the Python 0.1.0 implementation retained as a frozen behavior reference.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

## Product direction

The [draft parent product PRD](docs/product/aletharsis-PRD.md) describes Aletharsis
as a forensic document signal auditor and separates current capabilities from
planned structural, cryptographic and statistical analysis. The
[Evidence Review Workbench PRD](docs/product/frontend-PRD.md) is a draft rewrite;
its previously approved v1.0 baseline is archived alongside it. The parent
architecture is approved; changed child requirements still require product review
before implementation specifications are finalized. These
documents describe future scope, not additional functionality in this release.

## Installation

Build with Go 1.24+; the resulting binary requires no Python runtime. Auditing
currently requires Linux with `O_NOATIME` for strict
timestamp-preserving file reads. Run as the source file's owner; insufficient
permissions cause an explicit audit failure instead of a normal read fallback.

```bash
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
./bin/aletharsis --version
./bin/aletharsis audit tests/fixtures/binary_zero_width.txt
```

Place the binary on your PATH to use the commands below. The sole Go module
dependency is pinned `golang.org/x/text`, compiled into the binary for Unicode
normalization and names; Unicode classification and Emoji data are also bundled.
Audits require no network connection or external program.

Darwin and Windows binaries can be built, but source acquisition fails closed on
those platforms until equivalent timestamp-preserving readers are implemented and
tested. Cross-compilation does not imply cross-platform forensic guarantees. See the
[platform validation matrix](docs/testing/platforms.md) for native tests, build/install
checks, and reader prerequisites.

The original `src/aletharsis`, `pyproject.toml`, and Python tests are retained for
reference verification, not required by the Go CLI. See the
[migration record](docs/migration/go-backend-migration.md) and
[frozen reference manifest](reference/python-behavior/manifest.json).

## Usage

```bash
aletharsis audit suspicious.txt
aletharsis audit suspicious.txt --verbose
aletharsis audit suspicious.txt --json
aletharsis audit suspicious.txt --output report.json
aletharsis unicode suspicious.txt
aletharsis metadata suspicious.txt --json
aletharsis structure suspicious.txt
aletharsis --help
```

`--output` writes complete JSON to a **new** file and refuses existing files,
including symlinks and hard links. JSON goes to stdout with `--json`; verbose
diagnostics are JSON log records on stderr. Console text escapes control characters,
bidi controls and non-ASCII characters to avoid executing or visually hiding evidence.
Use a UTF-aware JSON viewer for the original text. Reports may contain sensitive
source text; choose an appropriate output location and permissions.

Audit exit codes indicate findings, so a nonzero code is not necessarily a failure:

| Code | Meaning |
| --- | --- |
| 0 | Completed; no findings |
| 1 | Highest severity INFO or LOW |
| 2 | Highest severity MEDIUM |
| 3 | Highest severity HIGH |
| 4 | Read, parse, audit, output, or command usage failure |

The `unicode`, `metadata`, and `structure` commands filter finding categories;
their counts and exit codes describe that view. They retain full extracted evidence
and always retain parser failures. The metadata view includes text-level identifier
and provenance findings; it does not yet extract structured document metadata.

## Supported inputs

TXT, Markdown, reStructuredText, CSV, JSON, XML, and HTML are inspected as **literal
source text**, including comments and markup. Other extensions and extensionless
files are accepted if they contain supported Unicode text. JSON string escapes,
HTML/XML entities, CSS visibility, and rendered content are not interpreted.
MIME is a content signature where available and otherwise an extension hint after
Unicode validation; it is not a full format-validity check.

UTF-8 is the default. UTF-8/16/32 BOMs are detected; BOM-marked UTF-16/32 are
supported in both byte orders. BOMs remain in extracted text so offsets are exact.
Malformed encodings fail without replacing or dropping bytes. Legacy encodings and
BOM-less UTF-16/32 are unsupported. PDF and OOXML DOCX are recognized by content
and explicitly reported as unsupported, even when given a `.txt` extension. Known
binary signatures and a high density of binary controls are rejected.

This milestone accepts one regular file at a time, up to 8 MiB. Directories,
recursive scanning, extension filters and JSONL are planned for M4. DOCX (M2) and
PDF (M3) parsing, document metadata, hidden runs, drawings, layers and attachments
are not implemented. No OCR, external utilities, network calls, or AI-text
classifiers run during audits.

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
Aletharsis cannot determine vendor or model authorship from prose alone. The
`analyzers/statistical/` boundary is reserved for future explicitly configured detectors.

## JSON contract

The complete versioned schema is [schemas/report.schema.json](schemas/report.schema.json).
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
[reference/python-behavior/report.schema.json](reference/python-behavior/report.schema.json).

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

## Architecture and integrity

`internal/audit` → `internal/parsers` → `internal/evidence` → analyzer interfaces
→ console/JSON reporters. `cmd/aletharsis` delegates to `internal/cli`.
Parsers extract facts; analyzers classify them. Format-specific readers use Go
build constraints; unsupported platforms fail closed. New analyzers can be added
without changing reporters.
The raw model preserves text, metadata and structural evidence as separate fields.

All transformations occur in memory. Source files are opened with `O_RDONLY`,
`O_NOATIME`, `O_NOFOLLOW`, and `O_NONBLOCK`, and checked to be regular files.
The tool never writes, normalizes, sanitizes, removes artifacts, extracts archives,
or changes source timestamps. It fails closed when no-atime access is unavailable.
Symlink inputs are rejected. Size/mtime/ctime are checked before and after reading
to detect ordinary concurrent modification. This is a snapshot, not an exclusive
filesystem lock or protection against an adversarial concurrent writer or a
filesystem that ignores no-atime semantics. For stronger guarantees use a read-only
forensic image/mount. No attempt is made to restore timestamps by writing them.

## Development and examples

```bash
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
python3 scripts/check_parity.py bin/aletharsis
```

Go tests require no Python runtime: they consume 37 committed Python report
oracles, exhaustive Unicode property/normalization digests, and 155 normalization
vectors. They also exercise concurrent audits, failure handling, safe reporting,
CLI output protection, and unchanged source bytes/timestamps. The optional parity
script needs only Python's standard library and also checks reference-file hashes.

For live differential checks, use Python 3.12 with the frozen modules:
`PYTHONPATH=src python scripts/check_differential.py bin/aletharsis`.
This covers 210 seeded Unicode/identifier/provenance cases. Original Python tests
remain runnable with `PYTHONPATH=src python -m pytest` after installing pytest.

Do not regenerate reference artifacts as part of normal testing. Changes to them
are reviewed behavior changes. Frontend F0/F1 specifications remain paused until
the Go parity and migration review is accepted. Directory auditing, office-format
parsing, the service API, and stable failure codes remain separately scoped work.

The [backend CI guide](docs/testing/backend-ci.md) documents the Go version matrix,
reference checks, retained validation artifacts, and proposed merge gates tracked
in [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7).
