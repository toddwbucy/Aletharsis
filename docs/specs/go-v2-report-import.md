# Go v2 report envelope and exact-byte import

| Attribute | Value |
| --- | --- |
| Status | Proposed implementation; stacked on the evidence-graph PR |
| Contract | [EC-002](report-v2-wire-and-import.md) |
| Dependencies | [Evidence graph](go-v2-evidence-graph.md), [validator record](../dependencies/README.md) |
| Tracking | [#17](https://github.com/toddwbucy/Aletharsis/issues/17), [#21 G1](https://github.com/toddwbucy/Aletharsis/issues/21), [#7](https://github.com/toddwbucy/Aletharsis/issues/7) |

## Report boundary

`v2.Report.Encode(limits)` checks producer strings and structure, validates against
the bundled schema, checks graph/finding/view semantics and returns canonical JSON.
`v2.DecodeReport(bytes, limits)` applies strict lexical and resource checks before
schema validation and typed decoding. Direct `json.Marshal`, `json.Unmarshal` and
`ValidateSemantics` alone are not substitutes for these service boundaries.

The schema remains the source of truth for all 23 finding payload/location variants.
`internal/wire` wraps a pinned Go validator, compiles only bundled contracts and
rejects external resource loads. No report URL selects a schema. Upstream detailed
validation errors are replaced with a generic schema error so their text cannot
disclose document payloads. There is no arbitrary upstream detector-result field.

Beyond the graph checks, reports validate unique finding references, execution and
mechanism linkage, positive structural results for observations, matching typed
parser-failure diagnostics, original coordinates, occurrence anchor coverage,
view membership, summary counts and operational exit precedence. A schema-valid
report is still rejected when these relationships contradict each other.

`SelectView` preserves all evidence, capabilities, executions, results, anchors,
diagnostics and surviving finding references. It allocates a new finding slice and
summary. Empty filtered findings do not turn a positive result into a negative one.
An already filtered report cannot reconstruct a broader finding view. This method
shares the unchanged evidence graph as an immutable service view; it is not a deep
copy intended for concurrent mutation.

## Import identity and version handling

`internal/reportimport.Read` accepts bounded bytes, computes their exact SHA-256
before interpreting the report, and retains a defensive copy on successful import.
`Imported.Bytes()` returns another copy. The decoded v2 report is a separate consumer
view; modifying it cannot alter the retained bytes or their identity. Caller buffers
must not be mutated concurrently while import is running.

- `2.0`: complete wire and semantic validation; status `validated`, coverage `declared`.
- `1.0`: unchanged bundled schema; status `legacy`, coverage `unknown`. Parser failures
  gain separate adapter diagnostics with `legacy.failure_code_unavailable`, the
  original finding pointer and null failure code. Error prose is never classified.
- Unknown exact version: status `unsupported_version`, coverage `unknown`, inert
  retained bytes only. No v2 executions or conclusions are synthesized.
- Malformed known-version reports: error, no partially trusted result. The application
  may separately retain its original input for quarantine.

Imports never open source paths, retained blobs, schema URLs or document resources.
`declared` coverage establishes internal consistency, not authenticity or source-byte
verification. Report imports never authorize detector execution or remediation.

## Resource limits and validation

Callers supply explicit `identity.Limits` for bytes, JSON nodes and depth. Graph
records also retain the existing 1 MiB / 32,768-node / depth-32 decoder limits.
Producer checks bound structure and aggregate string content before serialization;
these limits are not a claim of a fixed heap/RSS ceiling. No generic workbench import
limit or service concurrency budget is frozen by this increment.

Schema validation sees original numeric tokens before canonical rounding. Tokens
are limited to 1,024 bytes and exponent magnitude 4,096 before the upstream exact
rational validator runs. This prevents tiny inputs with enormous exponents from
causing disproportionate numeric allocation.

Tests cover all eight complete v2 fixtures, all 37 frozen legacy reports, every
native finding wire variant, invalid booleans/fractional coordinates, unknown
properties, corrupted references and summaries, filtered positive results, exact
byte retention, input aliasing, unsupported versions, import budgets, concurrent
validation and bounded import fuzzing. Existing schemas and frozen reports are
unchanged.

## Still required for Issue #17

This is the full report and consumer-import boundary, not a native report emitter.
Deterministic reference assignment, native execution/report assembly, opt-in v2 CLI
and human coverage output, output-stage failure codes, emitted-report compatibility,
resource integration and release review remain. The CLI continues to emit schema
1.0 by default; no detector, source parser or network capability is added here.
