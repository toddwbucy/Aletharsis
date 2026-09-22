# Rooted corpus executor and JSONL

This internal #15/P1 increment joins discovery to native auditing. It is used by the directory audit and directory reveal CLI commands.
Presentation/publication lives in trusted compiled callbacks, outside the native
detector and report wire models.

## One acquisition authority

`workspace.Open` pins a real directory. `DiscoverRoot` enumerates under that same
handle; `audit.InspectRoot` and `RunV2Root` acquire candidate paths beneath it.
Linux acquisition opens **every parent component** with `openat`, `O_DIRECTORY`,
`O_NOFOLLOW` and `O_NOATIME`, then opens the final file with the existing strict
snapshot flags and checks. It never reconstructs an ambient absolute source path.
Traversal and absolute paths are rejected. Root-relative paths are capped at
4,096 bytes and 64 parent levels.

A replaced parent symlink cannot redirect acquisition outside the root. Renaming
or replacing the original root pathname does not change the pinned authority.
The report's file path is relative to the workspace named in the corpus header;
its source hash identifies the bytes actually acquired. Discovery and auditing
are sequential observations, not an atomic whole-filesystem snapshot. A regular
file legitimately replaced between discovery and acquisition is audited as its
current acquired content; downstream decisions must bind that reported hash.

The v2 service retains its typed native failure internally, outside report JSON,
so the corpus layer classifies unsupported formats without reading error prose.
Existing report schemas and single-file output are unchanged.

## Stream contract

`corpus.Run` accepts a context, path, options and writer. It emits exactly one
header, sorted entry records, and one final summary when delivery completes.
[corpus-v1.schema.json](../../schemas/corpus-v1.schema.json) closes envelope
objects and reuses the existing report schemas. Schema validation is per record;
consumers must additionally check stream order, counts, report-schema agreement
and report identities. No network schema loader is needed.

The header declares `aletharsis.corpus/1`, workspace, selected report schema and
effective resource limits. Each entry has `relative_path`, `state`, `reason` and,
when an audit report exists, `report` and `report_canonical_sha256`.

The report hash is **SHA-256 of JCS canonical report JSON**, not of the containing
line or a pretty single-file artifact. Consumers canonicalize the extracted
report before comparing it. No raw-report-byte identity is implied by embedding
an object in JSONL. The report still retains its original source-byte SHA-256.

Entry states:

| State | Meaning |
| --- | --- |
| `no_reported_findings` | Native audit completed with exit 0; not proof of absence |
| `requires_review` | Native audit produced INFO/LOW/MEDIUM/HIGH findings |
| `unsupported` | Typed native format-unsupported result; report retained |
| `failed` | Acquisition, parsing, execution, discovery or resource failure |
| `skipped` | Explicit discovery policy skip; no invented audit report |
| `canceled` | A discovered candidate was not completed due to cancellation |

The summary contains counts for all six states, entry count, discovery completeness,
state, reason and aggregate exit code. Failure/unsupported/cancellation yields 4;
otherwise the maximum per-file severity exit code is retained. Discovery failure
produces a failed/canceled summary with zero entries, never a successful empty
scan. Failed or unsupported entries yield a partial summary. Policy skips remain
visible even when discovery itself completed.

Cancellation after discovery accounts for remaining candidates as canceled and
still attempts to deliver the summary. Writer failures or output-budget exhaustion
return an error without a completion marker; the stream prefix is incomplete.
Callers must return failure and consumers must reject it as a completed scan.
A syntactically complete final line does not override an unsuccessful producer
exit or transport failure.
Filesystem work and cancellation checks are cooperative, not hard timeouts.

## Bounds

Execution currently uses one in-flight audit (declared concurrency 1), preserving
deterministic order without retaining a corpus of reports. Per-file source input
is at most 8 MiB; discovery retains its own entry/depth/path bounds. Reports use
16 MiB input/output, 4 Mi-node and depth-64 canonicalization limits in both schemas.
This is a separate budget from source size: an audit accepted individually may exceed the corpus report budget because
its serialized evidence is larger, producing `execution.report_limit` for that
entry. JSONL defaults to 128 MiB with a
256 MiB hard output ceiling.

A remaining acquisition allowance smaller than the configured per-file ceiling
produces `execution.resource_limit` in the envelope and schema-2 diagnostic;
the retained native failure message identifies the corpus allowance in both schemas.
`file.too_large` is reserved for evidence exceeding the configured
per-file ceiling. Both checks use the acquired descriptor; pre-read rejections
charge zero bytes, allowing later smaller candidates to use the remaining pool.

Schema-2 capability limits describe the configured per-file ceiling, not the
shrinking corpus allowance. The allowance constrains only snapshot acquisition.
With the same relative path and configured options, a successfully audited file
retains its report identity regardless of earlier candidates consuming the pool;
failed acquisition still records the actual failure and may depend on scan scope.

Aggregate acquisition defaults to 256 MiB, hard-capped at 1 GiB. A rooted-reader
counter charges actual bytes returned by Read, including partial errors, changed
snapshots, and oversize sentinel bytes, even when downstream reporting fails.
Failures before Read consume zero bytes; discarded data is never exposed as
source evidence. Remaining candidates receive explicit
resource-limit outcomes without fabricated reports. One sentinel byte is reserved
before each attempt, so a final one-byte remainder cannot initiate another read.

## Validation and remaining integration

Tests cover both report schemas, canonical hash recomputation, closed envelope
validation without networking, deterministic mixed supported/unsupported/failed
corpora, pinned-root rename, parent-link substitution between discovery and read,
source timestamp preservation, cancellation accounting, short writes, truncated
output, aggregate acquisition exhaustion and discovery failure.

The directory audit and reveal specifications describe CLI integration and
compiled nested demonstrations. This package writes to its supplied stream; an
optional trusted Observer receives a copied discovery plan and completed native
snapshots, and handles derivative publication. Failed/skipped/canceled outcomes
receive no successful snapshot. A Visit callback may return exactly `ErrSourceLimit`
after retaining a report-only outcome for a successfully audited source; the
executor records a failed `execution.resource_limit` entry and continues. Other
observer errors stop the stream without a summary. Wrapped/joined errors are
intentionally fatal: a joined publication failure must not be recovered merely
because `errors.Is` also finds the resource sentinel.
Callbacks are not a serialized plugin mechanism and cannot be supplied by document,
rule or profile data. They must not mutate or retain evidence; no callback authorizes
source modification.

## Proposed Office consumer boundary

This specification retains its existing versioned behavior.
[OC-001 §6](office-cli-evidence.md#6-consumer-inventory-b5) defines the separate
Office consumer contract; no legacy envelope is silently extended.
Corpus-v2 derives entry state from report status, retains partial evidence and
dispatches workers by exact report version without a legacy fallback.
