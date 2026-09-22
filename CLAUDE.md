# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository state

Aletharsis is a read-only forensic auditor for text, Unicode and office-document artifacts, shipped
as a Go CLI (`aletharsis` 0.2.0). The original Python application was retired
(`docs/migration/python-retirement.md`); Python survives only as test harnesses, study harnesses and
frozen reference evidence. There is no installable Python package and no Python entry point.

Some long-lived branches predate the Go migration. **If the working tree contains
`src/aletharsis/*.py` and `pyproject.toml`, the checkout is on a pre-migration milestone branch**,
not `main`, and nothing below applies to it.

## Build and test

Linux only for real auditing (acquisition requires `O_NOATIME`). Go >= 1.24 (CI pins a 1.24.0
minimum job and a 1.27.1 development job); Python 3.12 for the harnesses only.

```bash
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
go test ./...
go test -race ./...
go vet ./...
git ls-files -z '*.go' | xargs -0 gofmt -l    # must print nothing; CI enforces this

# One package, or one test
go test ./internal/wordanalysis -run TestFormattingSubtreeTextDoesNotEnterAnalysis
go test ./internal/packageparts ./internal/xmlparts ./internal/wordtext ./internal/odttext
```

Python harnesses (`pip install -r scripts/requirements-ci.txt`; export `PYTHONHASHSEED=0` and
`GOTOOLCHAIN=local`):

```bash
python -m pytest                                                     # tests/ — schemas, contracts, v3 conformance, adoption records, reuse registry, Unicode study
python -m pytest -q integration --aletharsis-binary=bin/aletharsis   # compiled-CLI black box, Linux only
python -m pytest tests/contracts_v3/test_v3_contract.py -k wire      # single file / single test
python scripts/check_parity.py bin/aletharsis                        # 37 frozen reference reports
python scripts/check_seeded.py bin/aletharsis                        # 210 frozen seeded cases
python scripts/check_distribution.py --output artifacts/distribution.json
```

`integration/` has no `PATH` fallback and no in-process `cli.Run` call: it fails setup unless
`--aletharsis-binary` names a built executable. Fuzzing and measurement:

```bash
go test ./internal/parsers -run='^$' -fuzz=FuzzDecodeCoordinates -fuzztime=10s
go test ./internal/benchmarks -run='^$' -bench=BenchmarkStages -benchmem -benchtime=1x
python -m unittest discover -s benchmarks -p 'test_*.py'
```

`git diff --check` passes only from a checkout of the branch under test — several evidence trees rely
on `-text` and `whitespace=` attributes to keep intentional CR/NUL bytes, so running it from a stale
worktree reports false trailing-whitespace errors. The full CI sequence is in
`docs/testing/backend-ci.md`.

## Architecture

`cmd/aletharsis/main.go` is eight lines; all command syntax lives in `internal/cli`. The layering
rule throughout is **parsers and extractors produce facts, analyzers classify them, reporters never
interpret content** — a new format or analyzer registers without touching the reporters.

**Single-file text audit.** acquire (`internal/audit/read_linux.go`) → identify
(`internal/parsers/identify.go`) → parse into `evidence.Document` (`internal/parsers/text.go`) →
each `analyzers.Analyzer` → `evidence.Report` → console/JSON (`internal/reporters`).

**Directory audit** (`aletharsis audit DIR [--recursive] [--jsonl]`). `internal/workspace` discovers
bounded candidates without interpreting them; `internal/corpus` streams per-source outcomes. A
single source's failure must degrade that entry only — the aggregate run continues. Budgets are
per-file and per-corpus and are deliberately kept separate: the remaining corpus allowance must
never reach a report's `capabilities[].limits`, or report identity becomes scan-order dependent.

**Reveal bundles** (`--reveal-out DIR`). `internal/reveal` turns verified evidence into presentation
data (occurrence maps, diffs) and `internal/publication` writes artifact bundles with manifests and
full rollback. Neither acquires files nor authorizes edits.

**Office documents.** `internal/packageparts` reads bounded ZIP containers (span tiling, local/central
header agreement, ZIP64/encryption rejection); `internal/xmlparts` preserves XML structure with
part-byte text maps; `internal/opcrels` inventories OPC relationships without following them.
Identity is `internal/docxidentify` / `internal/odtidentify`; extraction is `internal/wordtext` /
`internal/odttext`; analysis projection is `internal/wordanalysis` / `internal/odtanalysis`.
`internal/profiles` assesses preserved observations without changing findings.

**Report contracts**, selected by `--schema-version`:

- **1.0 (default)** — `internal/evidence/model.go`, `internal/audit/audit.go`,
  `schemas/report.schema.json`.
- **2.0 (opt-in)** — `internal/evidence/v2/`, `internal/audit/v2.go`,
  `internal/capability/registry.go`. Capability catalog, per-operation execution records, typed
  failure codes (`internal/failure`), artifact/anchor identity (`internal/identity`). Its console
  view prints coverage *before* findings so an empty finding list cannot hide an unrun or
  unavailable operation. `internal/wire` validates against schemas compiled in by
  `schemas/embed.go` — only 1.0 and 2.0 are embedded, and no URL inside a report selects a schema.
- **3.0** is `schemas/report-v3.schema.json` plus an offline Python conformance oracle in
  `tests/contracts_v3/`. No Go code implements it.

Other wire contracts: `corpus-v1`, `corpus-document-v1`, `reveal-tree-v1`, `profile-v1`.

`internal/unicoderef` pins Unicode 15.0.0 category and name semantics so results stay identical to
the frozen reference across Go versions; emoji detection uses bundled Unicode 17.0 data under
`internal/analyzers/data/`. Analyzer prose lives in `data/messages.json` and `data/limitations.json`,
not in Go string literals — `analyzers.finding(id, ...)` panics on a missing template.

`internal/capability` declares the compiled native operations plus deliberately **disabled**
declarations (C2PA carrier, C2PA verify, a statistical watermark detector). Disabled or unavailable
is a declaration of coverage, never a negative detector result.

## Invariants to preserve

- **Never write to a source file.** Acquisition opens `O_RDONLY|O_NOATIME|O_NOFOLLOW|O_NONBLOCK`,
  requires a regular file, and compares size/mtime/ctime before and after the read. It fails closed
  rather than falling back to an ordinary read, and never restores timestamps by writing them.
- **Never regenerate frozen artifacts to make a test pass.** CI runs
  `git diff --exit-code -- reference tests/fixtures tests/schema-cases.json`. `reference/` holds
  captured evidence of the retired implementation, not regenerable fixtures.
- **Admission is an allowlist, never a blocklist.** Text reaches analysis only through explicit,
  namespace-checked grammar: `wordtext.role()` names the four character-data elements, and
  `wordtext.analysisContexts` admits a path only via declared parent/context transitions, blocked by
  default, with a blocked ancestor short-circuiting before any child rule. Enumerating containers to
  *exclude* is the bug this replaced — an unlisted container then gets silently trusted. Unrecognized
  context must declare a coverage gap (`word.text_context_not_analyzed`) and emit a located boundary.
- **Missing coverage stays visible.** Excluded or unanalyzed content raises an issue and flips
  extraction `State` to `partial`. Silently dropping content while reporting `completed` is as much a
  defect as a false finding.
- **Exit codes describe findings, not execution failure**: 0 none, 1 INFO/LOW, 2 MEDIUM, 3 HIGH,
  4 acquisition/parse/output/usage failure. A failed audit still emits a schema-valid report
  carrying a `parser.failure` finding.
- **Locations are zero-based code-point indices plus original-file byte offsets** — never graphemes
  or screen columns. `Text.ByteOffsets` maps every character index to its byte start and ends with
  the EOF offset.
- **Output stays inert and reproducible.** JSON is ASCII-escaped, sorted-key and deterministic, with
  no timestamps or random IDs; console rendering escapes control, bidi and non-ASCII characters
  (`unicoderef.Escaped`). The one deliberate exception is a canonical report artifact, which keeps
  raw UTF-8 so `report_artifact_sha256 == report_canonical_sha256`; every other published artifact —
  manifests, mappings, comparisons, corpus streams — is escaped. `README.md` and the specs state this
  split; keep them in agreement when adding a writer.
- **Report identity is path- and content-determined.** The same bytes at the same relative path under
  the same configuration must produce byte-identical reports and digests, whatever else the run
  contains.
- **`--output` creates with `O_EXCL` and mode 0600**, refuses an existing destination including hard
  links and symlinks, rejects ancestor destinations, and removes a partially written report.
- **No network, no subprocess, no execution of inspected content** during an audit. Unicode and
  emoji data are compiled into the binary.
- **Result language is a product invariant**, not decoration (`docs/product/aletharsis-PRD.md`):
  findings are observable evidence classified as `observed_fact`, `suspicious_pattern`,
  `likely_mechanism` or `undetermined`. Nothing may call a file clean, watermark-free or
  AI-generated, and absence of findings is never absence of a watermark. Keep this wording in code,
  docs and commit messages.

## Where the design lives

- `ROADMAP.md` — tracks P0–P7, reuse gates G0–G6, frontend phases F0–F5, and what is deliberately
  not implemented.
- `docs/text-audit-reference.md` — detector thresholds, emoji/normalization/coordinate semantics.
- `docs/specs/` — `evidence-contract-v2.md` (EC-001) and the `go-v2-*` specs are normative for the
  2.0 model; `report-v3-*` covers the in-flight wire contract; `word-analysis-scopes.md`,
  `word-text-evidence.md`, `odt-*`, `docx-identification.md`, `opc-relationship-evidence.md`,
  `corpus-executor.md`, `directory-cli.md`, `directory-reveal.md` and `reveal-*` cover the rest.
- `docs/adr/` — ADR-0001 evidence/capability contract, ADR-0002 detector reuse policy.
- `docs/reuse/` — the gated detector-reuse program: candidate registry, adoption records and
  evaluation evidence. Adoption records bind exact bytes; do not rewrite historical identities.
- `docs/testing/` — CI, fuzzing, performance, platform matrix.

Work lands as bounded PRs from `feat/`, `fix/`, `test/`, `docs/` or `research/` branches carrying the
validation evidence for the relevant gate; specs and ADRs are accepted before implementation.
