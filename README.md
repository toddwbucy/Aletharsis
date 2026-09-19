# Aletharsis

A read-only command-line forensic auditor for Unicode artifacts, text structure,
identifier candidates, and provenance labels. Version 0.1.0 implements **M0/M1**.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

## Installation

Python 3.12+ is required. This release requires Linux with `O_NOATIME` for strict
timestamp-preserving file reads. Run as the source file's owner; insufficient
permissions cause an explicit audit failure instead of a normal read fallback.

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -e '.[dev]'
aletharsis --version
```

Alternatively, with uv: `uv venv --python 3.12`, then `uv pip install -e '.[dev]'`.
There are **no runtime dependencies** beyond Python. Setuptools builds the package;
pytest runs tests and jsonschema validates reports during development.

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

Output uses sorted keys, deterministic ordering, escaped Unicode, and no runtime
timestamps or random IDs. It is reproducible for the same supplied path, bytes,
configuration and Python Unicode database version (recorded in evidence).

## Architecture and integrity

`identify` → parser registry → `DocumentEvidence` → analyzer protocols → `Report`
→ console/JSON reporters. Parsers extract facts; analyzers classify them. New
format parsers and optional analyzers can be registered without changing reporters.
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
python tests/generate_fixtures.py
python -m pytest
aletharsis audit tests/fixtures/binary_zero_width.txt --verbose
aletharsis audit tests/fixtures/binary_zero_width.txt --json
```

Fixtures are deterministic and checked in. Tests assert findings, legitimate-language
cases, offsets, hashes, JSON schema, exit codes, failure behavior, report overwrite
protection and byte/timestamp integrity. Representative complete reports are in
`examples/`. M0/M1 stop here; DOCX/PDF fixtures and analyzers belong to subsequent
milestones after this implementation is reviewed.
