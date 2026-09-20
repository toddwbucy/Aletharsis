# Aletharsis

**A local-first forensic document signal auditor.** Aletharsis inspects artifacts
for information that ordinary viewing can miss: invisible Unicode, structured
character patterns, identifiers, and provenance-bearing content. It preserves
observable evidence and distinguishes it from interpretation.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

The current release is a **read-only Go CLI for text and Unicode analysis**.
The broader product will add document-format inspection, a local evidence review
workbench, and explicitly approved transformations into new files. See the
[roadmap](ROADMAP.md) for planned work and release gates.

## What works today

Go **0.2.0** defaults to deterministic **report schema 1.0** output.
Opt in to the new coverage contract with `--schema-version 2.0`.
It audits one regular file at a time, up to **8 MiB**, on **Linux** with supported
no-atime acquisition.

| Capability | Current behavior |
| --- | --- |
| File identity | Content signatures and MIME hints, source size, SHA-256, encoding and parser identity |
| Unicode inspection | Invisible characters, bidi controls, selectors, tags, unusual whitespace, controls, combining characters, emoji candidates and limited mixed-script checks |
| Pattern analysis | Zero-width binary candidates, periodic insertions, long selector/tag runs and identifier/provenance candidates |
| Text evidence | Exact extracted text, code-point and byte positions, context samples, BOMs, line endings and normalization/comparison hashes |
| Reporting | Escaped console output, complete JSON evidence, filtered finding views and severity-based exit codes |
| Integrity | Read-only acquisition, source-change checks, rejection of symlink inputs and refusal to overwrite report destinations |

TXT, Markdown, RST, CSV, JSON, XML, HTML and source-code files are inspected as
**literal text**. Unknown extensions and extensionless files are accepted when
content passes Unicode-text identification. Comments, docstrings and string
literals are included; the CLI does not interpret programming-language semantics,
JSON escapes, HTML entities, CSS visibility or rendered markup.

UTF-8 and BOM-marked UTF-16/32 in both byte orders are supported. BOMs are retained
in extracted evidence. Malformed encodings fail without replacement; legacy
encodings and BOM-less UTF-16/32 are unsupported. MIME hints do not establish
full document validity.

**Not implemented:** directory/recursive scans, JSONL, reveal/diff exports,
DOCX/ODT/PDF parsing, structured document metadata, expected-artifact profiles,
C2PA validation, statistical watermark detectors, model fingerprinting, the web
workbench, saved rules or cleanup. PDF and DOCX signatures are recognized and
reported as unsupported, including when disguised with a text extension.
No OCR, remote detector or AI-authorship classifier runs during an audit.

## Build and try it

From a checkout, build with Go 1.24 or newer:

```bash
git clone https://github.com/toddwbucy/Aletharsis.git
cd Aletharsis
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
./bin/aletharsis --version
./bin/aletharsis audit tests/fixtures/clean_ascii.txt
./bin/aletharsis audit tests/fixtures/binary_zero_width.txt
```

The last fixture intentionally produces a HIGH finding and exit code **3**.
Nonzero audit codes can describe findings rather than execution failure.
Place the resulting binary on your `PATH` to use `aletharsis` directly.

The binary needs no Python runtime. Unicode data and the pinned `golang.org/x/text`
dependency are compiled in; audits require no network or external programs.
Building may require downloading Go dependencies.

Run as the input file's owner on a Linux filesystem supporting `O_NOATIME`.
If the required access is unavailable, Aletharsis fails instead of falling back to
an ordinary read. macOS and Windows builds currently refuse acquisition; successful
cross-compilation is not functional auditing support. See the
[platform matrix](docs/testing/platforms.md).

## Commands

```bash
aletharsis audit suspicious.txt
aletharsis audit suspicious.txt --verbose
aletharsis audit suspicious.txt --json
aletharsis audit suspicious.txt --schema-version 2.0 --json
aletharsis audit suspicious.txt --output report.json
aletharsis unicode suspicious.txt
aletharsis metadata suspicious.txt --json
aletharsis structure suspicious.txt
aletharsis audit -- -leading-dash.txt
aletharsis --help
```

`--json` writes JSON to stdout. `--output` creates a new JSON report with mode
`0600` and refuses an existing destination, including symlinks and hard links.
Reports contain extracted source text; handle them as evidence with the same
sensitivity as the input. `--verbose` emits structured diagnostics on stderr.
Console output escapes control, bidi and non-ASCII characters.

`unicode`, `metadata` and `structure` filter findings. Their summaries and exit
codes apply to that view, but complete extracted evidence and parser failures
remain in the report. The metadata view currently exposes text-level identifiers
and provenance labels, not office/PDF metadata.

| Exit code | Meaning |
| --- | --- |
| 0 | Audit completed with no findings in the selected view |
| 1 | Highest severity INFO or LOW |
| 2 | Highest severity MEDIUM |
| 3 | Highest severity HIGH |
| 4 | Acquisition, parsing, audit, output or command-usage failure |

## Reading the evidence

Severity is review priority; confidence describes support for a stated observation
or pattern. Neither establishes intent or a probability of AI authorship. Ordinary
multilingual text, emoji and formatting can legitimately produce findings.
Emoji-capable code points are inventoried in source files too; their presence is
not an automatic exemption or a confirmed threat.

JSON retains file identity, status, summary, extracted evidence, findings and
limitations. Locations use zero-based Unicode code-point and original-byte
offsets, not grapheme positions or screen columns. Original-byte hashes and
extracted-text hashes are distinct. Finding IDs name detector rules and can recur
within a report; they are not unique occurrence IDs.

The [text audit reference](docs/text-audit-reference.md) documents detector
thresholds, Unicode versions, emoji behavior, normalization hashes and coordinate
semantics. The [JSON schema](schemas/report.schema.json) defines the strict wire
contract. Schema validation alone does not verify source identity or coordinate
integrity. The opt-in [schema 2.0 contract](schemas/report-v2.schema.json) adds
capabilities, execution states, typed failures, artifact identities and verified
anchors. Its console view displays coverage before findings. C2PA and statistical
analysis remain disabled/unavailable declarations, not negative detector results.
See the [v2 CLI contract](docs/specs/go-v2-cli.md) for migration and limits.

No structural findings is **not** evidence that a statistical watermark is absent.
Statistical analysis is currently unimplemented. Future configured detectors must
preserve their actual result semantics and limitations.

## Evidence integrity

Aletharsis never rewrites, normalizes, sanitizes or removes content from source
files. Comparisons happen in memory. Linux acquisition uses read-only, no-atime,
no-follow and nonblocking flags, requires a regular file, and checks size, mtime
and ctime before and after reading. It never restores timestamps by writing them.

This detects ordinary concurrent changes; it is not an exclusive lock or a guarantee
against an adversarial writer or filesystem behavior outside the supported
acquisition contract. A forensic image or read-only mount can provide stronger
isolation. Current audits make no network requests and do not execute inspected
content.

## Product direction

The approved [parent PRD](docs/product/aletharsis-PRD.md) separates three phases:

1. **Detection** establishes what is present without changing the source.
2. **Evidence review and tagging** records human judgment and reusable rules.
3. **Explicit apply** uses reviewed plans, dry runs and confirmation to produce
   validated derivatives while preserving originals.

Structural evidence, cryptographic provenance and statistical detector results
have different meanings and location capabilities. Expected formatting is not
trusted content. A saved rule never grants deletion authority; future bulk
approval applies to an exact frozen match set.

Intrinsic model fingerprinting is an **experimental stretch goal**, distinct from
keyed watermark detection. The eventual plan uses WeaverTools as the first
controlled reference-corpus producer, with portable hash-bound baselines and blind
evaluation. It is not an implemented detector or a prerequisite for local auditing.
See [PRD §5.5](docs/product/aletharsis-PRD.md#55-experimental-stretch-goal-intrinsic-model-fingerprint-analysis).

The [frontend PRD](docs/product/frontend-PRD.md) is a draft rewrite; its changed
requirements retain a separate review gate. The [roadmap](ROADMAP.md) maps backend
tracks, frontend dependencies and the next specification work.

## Development

On a supported Linux host:

```bash
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
python3 scripts/check_parity.py bin/aletharsis
python3 scripts/check_seeded.py bin/aletharsis
```

Go tests need no Python runtime. The optional parity command uses Python's standard
library to compare frozen reports and verify reference hashes. CI also runs
compiled-CLI integration tests, frozen seeded comparisons, bounded
fuzzing and platform checks; some run in separate workflows. Resource measurements
are documented separately. These checks validate the shipped backend, not future
GUI or remediation workflows.

The legacy Python application, packaging and live-oracle generators have been
retired. Python remains only in development/test harnesses; it is not installed as
an `aletharsis` command. The original 37 reports and Unicode oracle remain unchanged,
and all 210 formerly live differential cases are now frozen references. See the
[retirement record](docs/migration/python-retirement.md); do not regenerate reference
artifacts to silence failures.

- [Backend CI and integration tests](docs/testing/backend-ci.md)
- [Fuzzing](docs/testing/fuzzing.md), [performance](docs/testing/performance.md), and [platform validation](docs/testing/platforms.md)
- [Go migration record](docs/migration/go-backend-migration.md)
- [Frozen reference manifest](reference/python-behavior/manifest.json)

Implementation and contract changes proceed through reviewable PRs with relevant
validation. The next design task is [Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17).

## License

[Apache License 2.0](LICENSE). Bundled Unicode data carries its
[Unicode license](internal/analyzers/data/UNICODE-LICENSE.txt).
