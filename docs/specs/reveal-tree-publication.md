# Source-relative reveal tree publication

This internal #15 increment plans and publishes a complete derivative tree. The [directory reveal CLI](directory-reveal.md) supplies source snapshots,
verified comparisons and the outside-source output root; these remain caller
responsibilities at the package boundary. The publisher never acquires sources or interprets their content.

## Preflight and layout

`publication.NewTree` receives a new private, empty output root and a bounded list
of source-relative observations. Each entry declares whether it is a regular-file
candidate. Every candidate reserves all five possible artifact paths:

| Artifact | Path for `notes/example.md` |
| --- | --- |
| Revealed text | `revealed/notes/example.md` |
| Native report | `reports/notes/example.md.json` |
| Occurrence/comparison mapping | `mappings/notes/example.md.json` |
| Faithful diff | `diffs/notes/example.md.diff` |
| Display diff | `display-diffs/notes/example.md.diff` |

Non-candidate directory/link/failure observations have ledger entries but reserve
no artifact paths. This allows a failed directory observation and successfully
observed descendants to coexist without inventing a file artifact for the directory.

Preflight rejects duplicate paths, file/directory prefix conflicts and NFC/full
case-fold aliases, including aliases in parent directories. Comparison keys never
rewrite source names. Portable-name guards reject traversal, backslashes, ASCII
controls, trailing spaces/dots, reserved device stems and forbidden punctuation;
overlong artifact components also fail before writes. These are explicit export
restrictions, not findings about the source or silent path normalization. Actual
filesystem errors remain possible; exclusive creation is the final boundary.

## Ledger and commit

Each planned source must receive exactly one `Record`:

- `revealed` requires the source SHA-256 and all five artifact kinds.
- `failed`, `unsupported` or `canceled` may retain a report for a candidate when
  one exists; a missing report never acquires a fabricated identity.
- `skipped` retains its stable reason and no artifact.

Reasons are bounded machine codes, not arbitrary finding prose. Hashes are
computed over the exact supplied artifact bytes, including report formatting.
The source hash is a caller-verified identity; the writer does not authenticate it.
Artifact/report contents are opaque here: upstream code must verify native reports,
source coordinates, comparisons and the corpus completion record before supplying
those bytes. This layer does not turn arbitrary bytes into trusted evidence.

`Commit` requires a terminal record for every planned source. It writes exact
`corpus.jsonl` bytes and then `manifest.json`. The closed
[manifest schema](../../schemas/reveal-tree-v1.schema.json) records source states,
source/report identities, artifact paths/sizes/hashes and the corpus stream hash.
The returned receipt separately hashes the exact manifest bytes. Manifest order
is deterministic and contains no timestamps/random identifiers.

Consumers must additionally validate relative paths, uniqueness, source/report
cross-links, corpus/ledger agreement and every artifact hash; schema shape alone
is insufficient. Treat manifests as untrusted data on import. This publisher is
not an importer or a cleanup authorization boundary.

## Failure and resource behavior

Directories use 0700 and files use exclusive 0600 creation. A validation, budget,
write or commit failure aborts the transaction and rolls back only nodes it created,
in reverse order. Rollback verifies inode identity and preserves substituted paths;
cleanup errors remain visible. It never recursively deletes the output directory.
A successful committed tree cannot later be aborted through this API.

The caller must own the root, exclude the source tree, and prevent concurrent
namespace or input-buffer mutation. Inode checks are defense in depth, not a
sandbox against hostile same-user/privileged filesystem manipulation. A crash may
leave incomplete artifacts; this is not atomic directory publication or a
power-loss durability guarantee. Consumers require a validated final manifest.

Bounds are caller-lowered with hard ceilings of 10,000 source observations,
100,000 planned filesystem nodes, 4 MiB cumulative source-relative path bytes,
4,096 bytes per artifact path, 64 source parent levels and 256 MiB total written
bytes including corpus and manifest. Failed writes cannot resume the transaction.
All possible candidate paths count toward node preflight even if later unsupported.
The publisher retains bounded metadata, not all previously written artifact bodies.

Tests cover exact nested layout, byte hashes, repeatable manifests, empty input,
failed/skipped ledger records, alias/prefix collisions, invalid names, resource
bounds, missing ledger entries, late manifest collisions and identity-preserving
rollback. Corpus callbacks, CLI wiring and the nested demonstration are documented in
the directory reveal integration. Review/merge acceptance remains separate.
