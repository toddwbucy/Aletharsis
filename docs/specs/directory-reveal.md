# Directory reveal integration

```sh
aletharsis audit ./documents --recursive --reveal-out ../review-new --jsonl
aletharsis audit ./documents --recursive --reveal-out ../review-v2 --schema-version 2.0 --json
```

`--reveal-out` is explicit and works with both existing report schemas. Recursion
remains opt-in. It cannot be combined with `--output`; the tree contains native
reports and the exact corpus JSONL already. The destination must be new and
outside the source tree, with resolvable directory ancestors. Existing destinations,
source aliases and unsafe ancestry are rejected before publication.

## One snapshot, separate authorities

The corpus executor pins the source root, discovers once, and calls a trusted
compiled Observer with the complete plan. The observer preflights all potential
artifact paths using the tree publisher before file acquisition begins.

Each audit acquires its source exactly once beneath that root. Completed entries
carry their native report and original snapshot to the observer; schema-1 and
schema-2 wire output is unchanged. The observer checks the canonical report hash,
checks its source identity against the native snapshot, and calls the same verified
reveal comparison engine used by single-file exports. No source reread occurs.
Failed, skipped and canceled entries receive no success snapshot. Their reports
and reasons remain evidence; no revealed content is invented.

Observer callbacks are compiled application code, never instructions loaded from
an inspected file, imported rule or profile. Supplied evidence is read-only and
must not be retained. The corpus layer copies the report bytes passed to a callback
so its serialized output does not share that buffer. A per-source render or diff
resource limit retains the audit report alone, records a failed
`execution.resource_limit` outcome, and continues with other sources. The native
audit report remains unchanged; the corpus completes as partial with exit code 4.
Other callback errors, including publication and aggregate resource failures, stop
execution without a corpus completion record and trigger derivative rollback.

## Published layout and identity

For `notes/a.md`, successful analysis produces:

```text
revealed/notes/a.md
reports/notes/a.md.json
mappings/notes/a.md.json
diffs/notes/a.md.diff
display-diffs/notes/a.md.diff
```

The root also contains `corpus.jsonl` and `manifest.json`. Every planned observation
has one manifest entry: revealed, failed, unsupported, skipped or canceled.
A failed/unsupported source may have a native report but has no fabricated reveal.
A complete publication can therefore contain a partial or canceled audit; the
corpus summary and source ledger disclose those outcomes.

Native reports are saved as the exact canonical bytes retained in the corpus
entry. Their artifact hashes equal `report_canonical_sha256`. These report files
contain raw UTF-8, potentially including bidi controls, and are not safe terminal
display artifacts. Handle them like raw revealed text and faithful diffs; review
with an escaping JSON viewer or the escaped display diffs. Corpus stdout JSON/JSONL
is ASCII-escaped, as is the schema-1 single-file bundle report; directory reports
intentionally use canonical bytes instead to preserve this hash binding.
Manifests and mapping JSON use ASCII escaping for source-controlled names and
text. Parsing restores the exact Unicode strings; artifact sizes/hashes bind the
escaped bytes, and publication budgets count those bytes. Mappings retain source,
decoded, revealed and display identities and exact occurrence spans. The single-file
`comparison.json` follows the same escaping rule.
Finding reference indices resolve into the saved report’s finding order for either
schema. The manifest binds every artifact size/hash and the exact corpus stream. A saved
manifest is necessary but not sufficient: import must validate paths, all hashes,
source/report linkage, ledger/corpus agreement and report contracts.

Preflight does not rename or normalize source paths. Portable-name and
case/normalization/prefix collisions fail explicitly. For example `a` and `a.json/b`
would collide between `reports/a.json` and the directory required beneath it, so
that export fails before source auditing or artifact writes. Audit-only JSONL
remains available for paths that cannot be exported under this policy. A portable
name rejection reports `execution.unsupported_input` with a fixed policy message;
it does not claim an artifact write failed or a published tree was rolled back.

## Failure, delivery and bounds

The tree commits before stdout is delivered. Publication errors roll back only
still-identical nodes created by the transaction, never recursively deleting a
foreign directory. Cleanup failure has its own diagnostic. If discovery fails
before a plan exists, the unused output directory is removed and the failed/canceled
corpus result is delivered without a misleading empty derivative tree.

A successful commit is retained if later stdout delivery fails; the CLI returns
4 and reports that the published tree remains. Finding exits 1–3 still mean a
successful audit with findings; failed/unsupported/canceled corpus entries yield
4 even when the tree itself was successfully published.

The exact corpus stream is buffered up to its 128 MiB default output bound before
commit. This is additional memory to per-file parsing/comparison and buffer capacity;
it is not an RSS guarantee. Total tree file bytes are capped at 256 MiB including
the corpus and manifest, with 10,000 observations and 100,000 planned nodes. Existing
per-file, acquisition, line, occurrence, mapping and path limits continue to apply.
The operation performs no cleanup/removal of original artifacts. Namespace control
and the existing non-atomic crash/publication limitations remain explicit.

## Validation

Integration tests exercise both schemas, repeated byte-identical manifests, exact
JSONL stdout/corpus agreement, all artifact hashes, nested layout, pattern counts,
Unicode/emoji/combining text, UTF-16/32, empty input, failed/unsupported reports,
skipped links, source bytes/timestamps, collision rollback and post-commit output
failure. Observer tests verify successful snapshot identity and absence of success
snapshots on failed audits, as well as fail-stop callback behavior.

## Compiled nested-corpus demonstration

The compiled CLI was run twice with schema 2.0 against a generated nested corpus:
clean ASCII, one isolated ZWSP, 64 alternating ZWSP/ZWNJ characters, multilingual
text, malformed UTF-8, a PDF signature, and a skipped symbolic link. Both exports
returned 4 with no stderr: the corpus was partial, with two no-findings entries,
two requiring review, one failed, one unsupported and one skipped.

The two manifests were byte-identical. Each export contained 22 per-source
artifacts plus `corpus.jsonl` and `manifest.json`. Verification checked the manifest
schema, every artifact size/hash, canonical report equality with the corpus,
stdout equality with saved JSONL, and reconstruction of decoded text through all
65 revealed occurrences. All six source files retained their bytes and SHA-256;
source file/directory access, modification and change timestamps were unchanged.

To reproduce the command shape with a newly generated corpus and fresh outputs:

```sh
go build -o /tmp/aletharsis-demo ./cmd/aletharsis
/tmp/aletharsis-demo audit /tmp/corpus/source --recursive --schema-version 2.0 --jsonl --reveal-out /tmp/corpus/review-first
/tmp/aletharsis-demo audit /tmp/corpus/source --recursive --schema-version 2.0 --jsonl --reveal-out /tmp/corpus/review-repeat
cmp /tmp/corpus/review-first/manifest.json /tmp/corpus/review-repeat/manifest.json
```

`TestDirectoryRevealCompleteWorkflow` generates the deterministic fixtures itself,
checks both schemas, and additionally covers UTF-16/32, empty files, emoji,
combining characters and occurrence references into saved reports. Manifest hashes
include the workspace identity and are not portable golden hashes across paths.

Global publication limits remain transaction failures: the 256 MiB tree budget
includes artifacts plus final corpus/manifest bytes. Exhaustion rolls back the
uncommitted tree, including earlier sources. This differs from a per-source
render/diff limit, where a report-only entry can still fit and the transaction
can finish. A failed `Tree.Record` has already aborted the tree and cannot be
converted to `corpus.ErrSourceLimit`. Retaining a prefix would require a separate
checkpoint/partial-publication contract with reserved completion-record capacity.

## Proposed Office consumer boundary

This specification retains its existing versioned behavior. Proposed
[OC-001 §6](office-cli-evidence.md#6-consumer-inventory-b5) defines the separate
4.0 audit, corpus-v2 and reveal-tree-v2 contracts and selection rules. Single-file
reveal remains limited to 1.0/2.0; directory Office presentation is explicitly
unsupported while its audit evidence remains reportable. No legacy envelope is
silently extended by that proposal.
