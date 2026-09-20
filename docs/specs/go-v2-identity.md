# Go v2 implementation: evidence identities

This is the first Go implementation increment under [#17](https://github.com/toddwbucy/Aletharsis/issues/17),
following [EC-001](evidence-contract-v2.md) and the [EC-002 wire contract](report-v2-wire-and-import.md)
accepted in PR #24. It implements `internal/identity`; it does not yet emit v2
reports or alter the CLI, schema 1.0, or frozen parity artifacts.

## Interfaces and authority

- `Canonicalize(raw, limits)` accepts exactly one JSON value and returns its
  RFC 8785 canonical UTF-8 representation. It rejects duplicate decoded keys,
  malformed JSON/UTF-8, lone surrogate escapes and nonfinite/overflowing numbers.
  It preserves strings without Unicode normalization and sorts object keys by
  UTF-16 code units. Array order remains significant.
- `Digest(domain, value, limits)` hashes a canonical `{domain, value}` envelope.
  A value must be independently valid JSON; it cannot inject additional envelope
  members. Limits cover the entire envelope. `ConfigDomain` and `SelectionDomain`
  supply EC-002's versioned identities. Configuration callers exclude `sha256`
  itself from the configuration value, as specified in EC-002.
- `ExactBytes(raw)` hashes the supplied bytes directly. It is the operation for an
  imported report's artifact identity, not canonical JSON. Acquisition, retention
  and byte budgets belong to the caller. Hashing does not validate a report.
- `TextSelectionDigest(source, text, spans, sourceLimit, limits)` verifies a native
  literal-text segment and its entire scalar-to-source-byte boundary array against
  supplied immutable source bytes. It validates ordered, nonoverlapping, nonempty
  half-open spans and hashes each paired span with its exact selected text. It does
  not concatenate away gaps or adjacent selection boundaries.

The selection helper supports the existing UTF-8, UTF-16 LE/BE and UTF-32 LE/BE
native decoders. It preserves any decoded BOM, line endings, invisible characters,
combining sequences and emoji. Its byte coordinates refer to original source bytes,
not the UTF-8 serialization of decoded text. Normalized text cannot be substituted
for the source representation. Structured extraction and approximate mappings need
separate adapters; this helper cannot certify them.

All operations are in memory. They do not open source paths, fetch credentials,
resolve references, execute inspected content or authorize edits. The caller owns
input snapshots and must not mutate them concurrently. Returned identity bytes are
new data; source slices, text segments and supplied spans remain unchanged.

## Numeric and resource boundaries

General JCS uses IEEE-754 binary64 number semantics, including normal decimal
rounding and canonical negative zero. It is **not** a lossless arbitrary-precision
JSON-number format. Schema-specific integer validation must happen before
canonicalization: coordinates/counts are bounded by 2^53−1, while generic JCS may
legitimately serialize larger finite values. Selection regions enforce that bound;
configuration and report validation remain responsibilities of the v2 model layer.

The implementation uses Go's standard JSON binary64 formatter for shortest digits
and ECMAScript exponent thresholds, with negative zero handled explicitly. It adds
strict Unicode/duplicate-key validation, UTF-16 key ordering and JCS string escaping
rather than treating the existing report encoder as a canonicalizer. No production
or CI dependency is added.

Callers provide positive input-byte, output-byte, node and depth limits. Nodes count
values and object keys; the root is depth one. Depth budgets above 256 are rejected
regardless of input. Selection decoding has a separate positive source byte limit.
Bounds are checked before descending into values or growing serialized output.
These controls bound in-process work, not wall-clock deadlines or OS memory isolation.
There is no universal production budget implied by the small test budgets. The future
registry/emitter must select documented limits and map exhausted budgets to its
execution/diagnostic contract.

Errors do not contain document strings or return a partial digest. `ErrLimit`
identifies invalid/exhausted budgets through `errors.Is`; other errors indicate
invalid identity input. Production diagnostic codes are a subsequent boundary task.

## Validation

Go tests consume the existing six portable composite-identity vectors and now
execute all seven numeric canonicalization vectors previously reserved for Go.
They also cover the 24 finite numerical examples in
[RFC 8785 Appendix B](https://www.rfc-editor.org/rfc/rfc8785#appendix-B).
An independently generated, pinned corpus adds 256 deterministic V8 number outputs;
its [provenance and reproduction instructions](../../internal/identity/testdata/README.md)
record Node/V8 versions, seed, generation algorithm and exact artifact digest.
Node is only the original fixture producer; tests and production do not invoke it.

Tests reproduce 24 configuration identities and both text-selection identities
across the eight accepted report fixtures. Additional cases cover UTF-16/32 byte
mapping, astral emoji, combining marks, BOMs, disjoint spans, normalized/mismatched
source rejection, malformed JSON/Unicode, envelope injection, limit exhaustion,
concurrent determinism and preservation of source evidence. A bounded fuzz target
checks rejection without partial output, nonmutation and canonical idempotence.
These checks establish interoperability evidence, not a proof over all JSON/float
inputs or a completed production report importer.

```bash
go test -race ./internal/identity
go test -run '^$' -fuzz FuzzCanonicalize -fuzztime=15s -parallel=2 ./internal/identity
```

## Remaining implementation sequence

1. Identity foundation accepted in PR #25.
2. Review the [Go execution records, catalog and native failure boundaries](go-v2-records-registry.md).
   Artifact/anchor/result assembly and cross-record state/coverage validation remain
   part of the coordinator increment.
3. Implement deterministic reference assignment and the opt-in v2 emitter/view
   reporting, preserving the default schema 1.0 behavior and parity fixtures.
4. Validate emitted reports with the accepted schema and semantic contract;
   complete integration/resource/consumer checks before release approval.

Issue #17, Epic #21 G1 and Issue #7 stay open. Identity helpers do not establish
C2PA or statistical detection, frontend readiness, or completion of the v2 contract.
