# Bounded workspace discovery

This is the directory-discovery boundary for #15/P1. It is an internal Go API,
not a new CLI command or report wire schema. Content acquisition, corpus reports,
source-relative publication and JSONL remain subsequent integration work.

`workspace.Discover(ctx, directory, options)` returns sorted relative candidate
paths and explicit skipped/failed entries. Candidate means only an observed
regular-file directory entry; it does not mean supported, analyzed, safe or clean.
Unknown extensions, dotfiles and source-code names are not filtered out. The
existing content-based audit engine must decide support and findings later.

## Enumeration policy

- The root must be an existing real directory, not a symlink.
- Recursion is explicit in the API. With it disabled, child directories appear
  as `skipped / recursion_disabled` rather than disappearing.
- Symlinks (including directory cycles) are `skipped / symlink_not_followed`.
  FIFOs and other special entries are `skipped / not_regular`; their contents
  are never opened.
- Missing or inaccessible entries produce `failed / entry_unavailable`; child
  directories that cannot be enumerated produce `failed / directory_unavailable`.
- `complete` concerns enumeration only. Failed entries set it false. Explicit
  policy skips remain in a complete enumeration; content coverage is a separate
  future audit result, not this boolean.
- Successful results sort paths by Go string byte ordering. POSIX `/` separates
  components. Case and Unicode are not normalized. Source names remain untrusted
  data: consumers must escape them in presentation and assess publication-path
  collisions on the destination filesystem.

Invalid UTF-8 names fail the entire discovery instead of becoming lossy JSON
paths. There is no raw-filename interchange adapter in this increment. Literal
backslashes or other valid UTF-8 filename characters are retained as observations;
they are not approval to use those names unchanged on another platform.

## Bounds and failures

Callers supply positive entry and cumulative relative-path-byte budgets, and a
nonnegative recursion-depth bound (root depth zero). Hard ceilings are 10,000
encountered entries, 64 nested levels, 4 MiB cumulative path bytes and 4,096 bytes
per relative path. Counts include directories and skipped entries, not just files
selected for later audits. No unbounded directory listing is allocated: entries
are read in batches of 128 with budget checks before retention.

Limits, invalid paths, cancellation and detected directory changes return an
error with **no usable partial candidate set**. The caller must record the entire
discovery as failed/canceled and must not represent omitted content as reviewed.
Child permission/read failures can instead yield explicit failure entries and a
partial enumeration. Cancellation is cooperative at traversal boundaries, not a
hard wall-clock deadline for a blocked filesystem operation.

## Source integrity and authority

Linux directory reads use `O_NOATIME`, `O_NOFOLLOW`, `O_DIRECTORY` and
`O_NONBLOCK` under pinned `os.Root` handles. There is no atime-changing fallback.
Other platforms return `ErrUnavailable`. Root and child directory identities are
checked against non-following stat observations before listing; size, modification
and change timestamps are checked across listing/traversal. Discovered links are
not traversed and root-relative operations cannot escape the selected root.

These checks detect observed changes; they do not create a filesystem-wide atomic
snapshot or protect against an administrator controlling the filesystem. Returned
paths are observations only, not descriptors or durable acquisition authority.
The subsequent corpus executor must independently acquire beneath the root, reject
links/changed sources, and preserve source hashes and per-file failures. It must
also prevent scanning its output tree; discovery itself never creates output.

## Validation and remaining delivery

Tests cover deterministic nested order, case-distinct and unknown-extension names,
non-recursive skips, external links and cycles, empty roots, FIFOs, invalid names,
entry/depth/path budgets, canceled calls, permission-denied child directories,
mutation detection and exact native directory/file timestamp preservation.

Remaining #15 work: a bounded corpus executor with secure source acquisition,
complete outcome accounting and JSONL; source-relative output layout with
collision/alias handling; CLI integration; and nested-corpus demonstrations with
unchanged source hashes. This package does not close #15.
