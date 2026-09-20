# Go backend migration

Status: Go migration accepted; Python application retired. This document retains the migration rationale and historical validation. See the [retirement record](python-retirement.md) for current test dependencies; frontend design gates now follow [Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17).

## Decision and boundaries

The backend moves to Go before frontend service contracts and component assumptions harden. The product remains the same three-phase pipeline: read-only detection, evidence review/tagging, and separately confirmed derivative creation. This port covers the existing detection CLI only. It adds no server, directory scan, office parser, editor, or cleanup engine.

The frozen frontend PRD's evidence/coordinate and approval invariants remain requirements. Its recorded Python implementation baseline is superseded by this explicit migration decision once accepted; no frozen PRD text is silently rewritten. Finalize the F0/F1 technical specifications against the accepted Go implementation, after stable failure-code work and the required coordinate/large-report spike.

## Frozen reference

- Python source: merged commit `a0401f93e948b334b51f7332a5729df7647eab24`, including the explicit emoji analyzer.
- Oracle runtime: CPython 3.12.13, Unicode 15.0.0; Emoji property data 17.0.
- `reference/python-behavior/manifest.json` retains hashes of the historical Python modules/data and packaging metadata, plus unchanged inputs and reports.
- 37 complete report oracles cover the existing deterministic fixtures plus source emoji, malformed encodings, UTF-16/32 byte offsets, long combining runs, normalization, boundaries and unsupported inputs.
- `unicode.json` records exhaustive scalar-property/name/normalization digests and 155 seeded/edge normalization cases.
- `report.schema.json` is a compatibility snapshot from PR #3 commit `36c88ee18e65154e8694e117f6db06d40c2b36ec`. Keeping it as a reference does not merge or freeze that PR.
- The original Python source, application tests and generation scripts are recoverable from Git history. They were removed after acceptance; the [retirement record](python-retirement.md) identifies the exact source revision and replacement frozen cases.

## Preserved semantics

The Go implementation retains the documented audit/unicode/metadata/structure CLI forms, source identification and byte SHA-256, strict UTF decoders with retained BOMs, original byte/code-point offsets, normalization/comparison hashes, every existing detector and threshold, emoji inventory, severity/confidence/classification, filtering, failures, and exit codes 0–4. Both implementations remain conservative about watermark intent and application necessity.

Reports preserve schema 1.0 field names, including `character_offsets` and `byte_offsets`. Original-byte hashes and extracted-text hashes keep their distinct meanings. JSON output is deterministic within the Go implementation; original source bytes and timestamps are untouched. Output creation refuses existing files and aliases. The 8 MiB single-file cap remains.

The Go data model and parser/analyzer interfaces remain separate from CLI and presentation code. Calls can run independently in concurrent jobs; this is tested with the race detector. This port deliberately does not introduce a scheduling/service contract before the reference behavior is accepted.

## Explicit observable differences

- Go reports identify the implementation release as `aletharsis_version: 0.2.0`; Python reference reports remain 0.1.0. The report schema remains 1.0.
- JSON whitespace, float spelling, struct/key ordering, and order among otherwise equivalent findings may differ. Human-readable help/layout and error-message wording may also differ. Deterministic semantic comparison ignores only implementation version, finding ordering and `parser.failure.evidence.message`, not identifiers, classifications, numerical evidence, offsets, hashes, decode reasons, or exit codes.
- Exception-type field names/values are retained for compatibility even though Go uses errors internally. They are not proposed as a future service protocol. Stable failure codes remain a separate pre-schema-freeze task.
- The Go CLI accepts the documented flags, not Python argparse's undocumented option abbreviations. Platform-native path handling and OS diagnostic wording remain platform-specific.
- The required no-atime reader is implemented and tested on Linux. Windows/Darwin binaries can be compiled but fail acquisition explicitly; portability of a binary is not proof of equivalent filesystem evidence integrity. Additional platform readers require their own tests before being advertised as supported.

No new schema fields are introduced in the parity port. Failure-code evolution, report-scoping capability fields and frontend contracts must be explicit follow-up changes rather than concealed migration differences.

## Unicode compatibility

Go's toolchain Unicode tables must not silently alter findings. Classification uses an embedded table generated from the pinned Python Unicode 15.0.0 reference. Normalization and names use `golang.org/x/text v0.28.0` (Unicode 15.0.0); Emoji detection uses the same bundled 17.0 property table and license as Python. No network is needed during an audit.

Two normalization differences required compatibility handling:

1. The x/text normalizer can insert CGJ for stream safety after long nonstarter runs. Python's normalization does not. The port uses canonical decomposition, stable ordering and composition when needed without dropping original CGJs.
2. Differential testing found supplementary tag characters followed by combining marks could collide with unrelated BMP composition candidates. Candidate compositions are now checked for canonical equivalence before acceptance. A tag letter plus acute must not become an unrelated accented Latin letter.

The adapter also supplies Python's algorithmic CJK/Hangul names and its Unicode whitespace/word classification. Tests compare every Unicode scalar's relevant properties, every inventoried code-point name, and NFC/NFKC digests over all scalar values, plus seeded combinations. These tests establish substantial compatibility, not a claim that a finite corpus proves equivalence for every possible document.

## Validation and reproduction

```bash
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
python3 scripts/check_parity.py bin/aletharsis
python3 scripts/check_seeded.py bin/aletharsis
```

Go-only tests consume the frozen artifacts and therefore need no Python installation. Frozen seeded comparisons retain the 210 deterministic mixed Unicode, marker-whitespace and identifier-boundary cases formerly tested against the live Python oracle. Negative/Unicode read failures retain detailed byte evidence. Acquisition tests assert original bytes and nanosecond access/modified/change timestamps remain identical. CLI tests cover output aliases, malformed options, filtered views and escaped console text. The frozen schema is used in an additional report-validation pass.

Historical port validation on Linux/amd64 passed: all Go tests, race tests, `go vet`, 62 retained Python tests, 37/37 frozen report comparisons (including repeated-output checks), 210/210 live differential cases, and schema validation of all 37 Go reports. Static builds succeeded for Linux/amd64, Linux/arm64, Darwin/arm64 and Windows/amd64; only Linux/amd64 acquisition was exercised. No throughput or peak-memory improvement is claimed by these correctness checks.

Build cross-platform binaries explicitly with `CGO_ENABLED=0 GOOS=... GOARCH=... go build`. The module requires Go 1.24+; development validation used the checksum-verified Go 1.27.1 Linux/amd64 toolchain. Published binaries and build hashes should be attached to a release only after review; no release is published by this migration.

## Review order and completion gate

Review the frozen behavior/oracle commit first, then the implementation and compatibility differences. Accept the port only after parity checks, race tests, static checks, schema validation and native CLI smoke tests pass. Cross-build success alone is insufficient.

After acceptance: land the machine-readable failure-code contract, ratify the Go evidence/Occurrence adapter boundary, run the frontend Unicode/large-report spike, and only then finalize the F0/F1 technical specifications. Changes in inspection policy, batch behavior, office support or cleanup authorization remain independently reviewable work.
