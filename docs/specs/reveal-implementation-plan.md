# Reveal delivery plan (#15)

Status: implementation in review; no new CLI capability yet.

Reveal is a presentation derivative of acquired evidence, not remediation and
not a new detector. It must not change source bytes or normalize decoded text.
The native report schemas and CLI defaults remain unchanged.

## Delivery gates

1. **Verified renderer (this increment).** Consume native text evidence and its
   findings, verify the complete scalar/byte map against the source snapshot,
   render detected Unicode characters, and retain exact occurrence mappings.
   Test UTF-8/16/32, overlapping findings, literal marker collisions, malicious
   controls, deterministic output, resource limits, and immutable inputs.
2. **Safe artifact publication and diff.** Add exclusive derivative writes,
   rollback on failure, alias/symlink protections, output manifests, and a bounded
   unified-diff implementation. Specify a terminal-safe baseline representation:
   a display diff must not imply it is a byte patch applicable to an encoded
   source. Preserve original line endings in the revealed artifact; terminal
   views must escape carriage returns and other active controls separately.
3. **Single-file CLI integration.** Reuse one acquired snapshot for audit and
   reveal, preserving no-atime acquisition guarantees. Add `--reveal-out`, exact
   source/report/derivative identities, machine-readable occurrence output, and
   end-to-end demonstrations with before/after source hashes.
4. **Bounded directory integration.** Add deterministic traversal, recursion and
   supported-format policy, relative-path output layout, explicit failed,
   unsupported and skipped records, aggregate summaries and JSONL. Reject output
   inside the input tree and unsafe links/collisions. Exercise nested corpora and
   partial failures end to end.

Each increment receives its own review. Issue #15 remains open until all its
acceptance criteria, including CLI, directory, diff and demonstrations, pass.
Structured document extraction requires separate location adapters; this native
literal-text renderer does not pretend that package/object offsets are file spans.

## Renderer contract

`internal/reveal.Render` accepts a bounded original snapshot, its expected
SHA-256, one native text segment, and the unfiltered native findings. Existing
`identity.VerifyText` verifies decoding and the complete original byte map.
Imported reports must pass their own import contract; this function deliberately
accepts native typed coordinate slices, not arbitrary JSON maps.

Unicode inventory findings select characters to reveal. Pattern and other
located findings attach their references at those same positions without
creating duplicate markers or replacing ordinary identifier text. A finding
reference includes its index in the supplied collection, stable detector ID,
category, severity and classification. Unlocated findings do not acquire invented
spans. Finding coordinates must be strictly increasing and agree with the map.

Markers use `⟦U+200B ZERO WIDTH SPACE⟧`. Literal opening delimiters become `⟦⟦`,
with their own escape mappings, so marker-looking source is distinguishable.
Unselected control/format characters are escaped for display, except CR, LF and
tab, which retain line structure. These escapes explicitly carry no newly
invented finding. Output remains inert text, not safe executable HTML/Markdown
or raw terminal output: viewers must use text nodes and console renderers must
escape active line controls. Source filenames and finding prose are never
inserted into the revealed body.

Each emitted occurrence records a half-open scalar span, original byte span,
UTF-8 rendered-byte span, code point, pinned Unicode name, reason and associated
findings. The result records source, decoded-text and rendered-text SHA-256
separately. Literal text between mapped spans remains unchanged. Replacing mapped
spans with their verified source scalars reconstructs the decoded text exactly.
This is a presentation map, not edit authorization or a new report wire schema.

The caller supplies positive source, output and occurrence limits. The occurrence
budget also bounds total finding links. Limit or validation failures return no
partial result. The pure renderer does no file I/O and retains no mutable input
objects. Resource-bounded publication and report linkage are subsequent gates.

## Bounded comparison contract

`internal/reveal.Compare` builds a fresh verified reveal, then returns two
explicitly labeled comparisons. `faithful` compares the exact decoded UTF-8
representation with the revealed UTF-8 representation. Its original side retains
invisible characters, CRLF, and missing final newlines. It is not safe raw terminal
output and is not a patch for original UTF-16/32 bytes.

`presentation` compares ASCII-escaped versions of those representations. LF is
preserved; backslash, CR, tab, controls and all non-ASCII scalars are escaped.
Ordinary multilingual text is escaped for display too; that is not a finding.
Every escape maps its input scalar and UTF-8 byte span to its output byte span.
The before map joins to native text evidence; the after map joins through the
reveal occurrence map and unchanged intervals. Neither map invents original
file-byte offsets. Both full display representations are retained for inspection.

Source encoding, original source digest, decoded/revealed digests, display input
and output digests, and diff digests distinguish every stage. Fixed inert diff
headers contain no source filenames. These are internal package contracts, not
additions to an existing report schema. Neither diff authorizes cleanup or should
be accepted as a cleanup plan or executable source file.

Caller budgets bound combined decoded/revealed bytes, combined escaped bytes,
combined diff output, line indexing and escape mappings. Hard ceilings are 32 MiB
per representation pair, 64 MiB total diff output, 200,000 combined lines,
100,000 combined escapes and 1,000 context lines. Renderer limits apply separately.
Any limit/validation failure returns no partial comparison. Equal line counts use
positional comparison with merged context windows; differing counts use a full
replacement hunk. This deliberately avoids quadratic edit-distance work and does
not promise minimal diffs.

Tests cover exact unified syntax, optional independent `patch` application to
only temporary copies, CRLF/CR and missing-newline cases, context windows,
UTF-8/16/32 identity, reversible display mappings, controls/emoji/combining text,
determinism, source/evidence immutability and failure budgets. Production code
introduces no subprocess or filesystem writes. Safe publication, manifest/report
linkage, single-snapshot CLI acquisition and bounded directory integration remain
separate delivery gates; this increment does not complete #15.

## Artifact publication primitive

`internal/publication.Publish` accepts an already-open `os.Root` for a private,
caller-controlled output directory, caller-verified source/report SHA-256 values,
and named artifact bytes. It creates only flat, constrained lowercase names;
source-derived relative paths are not accepted by this primitive. Directory
layout and source/output alias checks remain responsibilities of the future
acquisition/CLI layer, which must establish the private directory outside the
source tree. Callers must prevent concurrent namespace mutation in that directory
and concurrent mutation of supplied artifact bytes.

All files use exclusive creation and mode 0600 (subject to platform permission
semantics). Existing files, directories, hard links and symlinks cause failure;
they are not replaced. Names and budgets are validated before the first write.
Output is capped at 64 artifacts and 64 MiB including the manifest, or lower
caller limits. The caller's artifact order remains unchanged; publication and
manifest entries use sorted names.

`manifest.json` is reserved and written last. Its internal bundle contract is
`aletharsis.artifact-bundle/1`, recording `source_sha256`,
`report_artifact_sha256`, and each artifact's `name`, `size`, and `sha256`.
The receipt separately records the hash of the exact manifest bytes (JSON plus
LF); timestamps and random identifiers are absent. These supplied source/report
identities are not authenticated by the writer. This does not change a report
wire schema or authorize cleanup.

On create/write/close failure the writer rolls back only its own new files in
reverse order, returning cleanup errors along with the initial failure. It never
recursively deletes an output directory. Under its private-directory precondition
this leaves existing collision targets untouched. It is not a hostile same-user
filesystem sandbox, an atomic bundle transaction, or a crash/power-loss durability
guarantee. A crash can leave incomplete output: consumers must parse the manifest
and verify every listed artifact's size/hash before considering a bundle complete.

Tests exercise deterministic exact manifests, artifact identity and permissions,
source preservation through hard-link/symlink collisions, path rejection,
preflight budgets including manifest bytes, and write/short-write/close/cleanup
failures. This internal primitive performs no source acquisition or CLI work.
