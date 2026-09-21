# Directory audit CLI

The directory command exposes the bounded corpus executor. Source-relative
[reveal publication](directory-reveal.md) is a separate explicit option.

```sh
aletharsis audit ./documents
aletharsis audit ./documents --recursive
aletharsis audit ./documents --recursive --jsonl --schema-version 2.0
aletharsis audit ./documents --recursive --json --output corpus-report.json
```

Only `audit` scans directories. Specialized file views (`unicode`, `metadata`,
`structure`) retain their schema-valid file failure report when given a directory. Immediate regular files are candidates by
default; `--recursive` includes descendants. Non-recursive directories, symlinks
and special files produce explicit skipped records. Unknown extensions remain
eligible for content-based identification. Directory selection uses non-following
stat; a symlink root is not treated as a traversable directory.

## Presentations

- Default console output lists each source-relative path and outcome, then all six
  outcome counts, discovery completeness, aggregate state and exit code. Paths
  and reasons are escaped; source text is never executed or rendered as markup.
- `--jsonl` streams the corpus header, sorted entry records and final summary.
  JSON and JSONL escape non-ASCII scalars, including paths and embedded reports;
  canonical report hashes still identify canonicalized report content.
- `--json` streams one `{header, entries, summary}` object with identical record
  semantics. [The document schema](../../schemas/corpus-document-v1.schema.json)
  references the existing closed corpus record schema. These envelopes do not
  change report schema 1.0 or 2.0.
- Directory `--output NEW_FILE` selects the JSON object by default, or JSONL when
  `--jsonl` is supplied. `--json` and `--jsonl` are mutually exclusive.
- `--verbose` emits a structured completion diagnostic to stderr.

Embedded reports preserve complete native evidence and canonical report hashes.
The console is a triage view; use JSON/JSONL or a single-file audit for detailed
findings. Report paths are relative to the header's workspace. Consumers must
validate stream order, counts, schema agreement and canonical report hashes, not
only individual JSON record syntax. A missing summary or unsuccessful transport
must never be interpreted as a complete scan.

Exit 0 means no reported findings among completed audits, not proof of absence.
Otherwise maximum severity yields 1–3; any failed, unsupported, canceled or
incomplete enumeration yields 4. Incomplete enumeration includes per-entry
unavailability; it does not authorize truncation at resource limits. Entry, depth
and path-budget exhaustion fail discovery with no candidate set and
`discovery_complete: false`, before any source audit. Choosing a bounded subset
requires a separate deterministic selection contract; raw filesystem enumeration
order is not a stable selection rule. Explicit policy skips remain visible. A valid
report that includes failed files is retained; an actual report write/close failure
removes only the new partial output and returns 4.

## Output and source integrity

Report output must be outside the source tree. Canonical path ancestry and inode
ancestry reject overlap; symlink output ancestors and pre-existing destinations
are rejected. The source root is pinned before destination checks. The output
parent is pinned and its identity rechecked before exclusive 0600 creation;
source discovery/acquisition reuses the already-open source root. No existing
report or source alias is overwritten. On unsupported hosts a metadata-only source
pin allows a failed corpus envelope to be saved without reading source contents.
If the source directory cannot be pinned at all (for example it disappears),
`--output` fails before creating a destination: source/output separation cannot be
verified. This publication-preflight failure has stderr diagnostics, not a saved
corpus report. Without `--output`, acquisition failure can still be streamed.

The caller must control the output namespace. These checks do not claim isolation
from privileged mount changes or hostile processes manipulating the namespace as
the same user. Directory and file reads preserve Linux no-atime guarantees; other
platforms still report unavailable rather than falling back to ordinary reads.

Default bounds: 8 MiB per file, 256 MiB aggregate acquisition (actual bytes read,
including discarded partial/changed/oversize snapshots; pre-read failures cost zero), 10,000 encountered entries, 64 levels, 4 MiB cumulative
relative paths, 4,096 bytes per relative path, and 128 MiB output. Report
canonicalization retains the 16 MiB contract budget. One in-flight audit gives
stable ordering. Both serialized JSONL and actually delivered presentation bytes
are bounded, including the JSON wrapper or console escaping. Cancellation is
cooperative, not a hard filesystem-I/O deadline.

## Validation

Tests compare JSON and JSONL entry records under both report schemas, verify
repeatability, exact mixed-corpus counts, source bytes and native timestamps,
non-recursive skips, terminal-control escaping, output-tree and symlink-alias
rejection, report permissions, existing-report preservation, empty/missing roots,
invalid options, short output and presentation byte budgets.

See the [directory reveal integration](directory-reveal.md) for source-relative
artifacts, collision handling and the nested reveal demonstration.

## Compiled nested-corpus demonstration

The built binary audited a temporary source tree containing committed clean,
isolated-zero-width, binary-zero-width and normal UTF-8 fixtures, plus malformed
UTF-8, a PDF signature and a symlink cycle. With schema 2.0 and recursion, JSON
file output and JSONL stdout produced identical header, entry and summary values.
The complete JSON document validated against the closed schema with all references
loaded locally. Both commands returned 4 and no stderr diagnostics, because the
partial report was valid and included failed/unsupported inputs.

```json
{
  "type": "summary",
  "state": "partial",
  "reason": "",
  "discovery_complete": true,
  "entries": 7,
  "counts": {
    "canceled": 0,
    "failed": 1,
    "no_reported_findings": 2,
    "requires_review": 2,
    "skipped": 1,
    "unsupported": 1
  },
  "exit_code": 4
}
```

All six source hashes matched their report values and before/after bytes; all
source file and directory access/modification/change timestamps were unchanged
across both runs. The four ordinary fixtures have the hashes recorded in the
[single-file demonstration](reveal-cli.md#compiled-binary-demonstration).
Malformed UTF-8 hash:
`921648899fbe42a4a1d2d253c41c088d9a9ab897989ab3b1b6e2e4a9a6299362`.
The `%PDF-1.7` plus LF fixture hash:
`0716f9264c9fe19f5d7455276107f3ddcc1d3497f63d60689a73558ae8a1bf5e`.

This demonstrates directory **auditing**, not directory revealed derivatives.
That workflow is demonstrated separately in the directory reveal specification.
