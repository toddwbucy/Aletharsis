# Bounded document package parts — DP-001

Status: proposed implementation increment under #14, stacked on PA-001 (#55).
This internal package supplies exact archive-part identities for future DOCX/ODT
parsers and context producers. It does not enable those formats in the CLI,
interpret XML, assess profiles, or change an existing report schema.

## Acquisition and identities

`packageparts.Read` accepts an already acquired source snapshot, its expected
SHA-256, a context and explicit limits. It never opens a path, extracts to disk,
resolves a relationship, fetches a resource or executes embedded content. The
caller retains the original snapshot and must not mutate it during analysis.

The result records the source SHA-256, implementation version `zip-parts/1` and
parts sorted by exact archive name. Each part retains:

- Exact name and directory status, compression method and declared CRC32.
- A half-open compressed-payload span in original source bytes and its SHA-256.
- Owned decompressed bytes and their separate SHA-256, preserving BOMs, whitespace,
  line endings and binary content without normalization.

Part identity is the tuple `(source SHA-256, exact name, decompressed SHA-256)`.
Equal content hashes in different parts do not collapse part identity. Directory
entries remain explicit. Names, comments, XML and all extracted bytes are untrusted.

A compressed-payload span is **not** a map from extracted XML/text offsets back to
ZIP source offsets. Subsequent parsers must locate observations in identified part
bytes and define their own extraction maps. The retained source remains necessary
for central/local headers, extras, archive comments and all container-level evidence;
the part inventory is not a substitute for the original container.

## Admission and resource bounds

The admitted subset is single-disk ZIP32 using Store or Deflate. ZIP64, encryption,
unsupported compression/flag modes and special file types fail explicitly. No
self-extracting prefix, trailing bytes, padding/gaps, local-record overlaps or
unindexed local records are silently skipped. Empty archives are valid ZIP evidence,
not evidence of a valid Office document. Format identification belongs downstream.

A bounded central-directory preflight verifies actual records and advertised counts
before `archive/zip.NewReader` can allocate its header collection. Local and central
names, flags, compression, CRC/size declarations and descriptor identities must
agree. ZIP32 descriptors with or without signatures are supported. Local records
must occupy the complete pre-central region without gaps or overlaps. CRC checking
and declared decompressed length checking complete before a part is returned.
Standard decompressors are registered per reader rather than inheriting replacements
from application-global registrations. This is a conservative admission policy,
not a claim to implement every legal ZIP dialect.

Names must be valid UTF-8, relative slash-separated paths without backslashes,
colons, dot/dot-dot/empty components or ASCII control characters. Duplicate names,
file/directory aliases and file-as-parent conflicts fail. Names are not case-folded,
URI-decoded or Unicode-normalized. Format-specific URI aliases and relationship
resolution require separate OPC/ODF rules; filesystem-export collision policy is
also separate because this layer performs no extraction.

Default and maximum bounds are 8 MiB source, 4,096 entries, 32 MiB per decompressed
part, 64 MiB total decompressed content, 1 MiB total name bytes and 4,096 bytes per
name. Callers can lower the configurable limits. Advertised expansion is checked
before opening streams; actual reads are capped at declared size plus one byte.
Memory includes source, archive headers and temporary buffers in addition to retained
part bytes. These bounds are not an RSS or hard CPU-time guarantee.

Cancellation is cooperative at operation/read boundaries, with decompressed reads
limited to 32 KiB chunks. Native ZIP/Deflate work within a read is not preempted.
All failures return no partial package. Stable sentinel errors distinguish resource
limits, invalid packages, unsupported features, ambiguous names and source identity
mismatch; cancellation preserves the context error. Future hosts must translate
these explicitly to capability/coverage diagnostics, never a clean result.

## Evidence fixtures and validation

`internal/packageparts/testdata` retains four original synthetic packages: minimal
DOCX/ODT plus hidden-content counterparts. They contain multilingual text and an
emoji joiner sequence; hidden counterparts add 64 alternating ZWSP/ZWNJ characters
inside a hidden Word run or ODF hidden-text element. These are actual ZIP/XML
containers with conventional package markers and relationships, not normalized
profile observations. They have not been certified by an Office application or an
OPC/ODF conformance validator. Their presence alone does not satisfy #14's eventual
legitimate-format/parser/profile acceptance gate.

The explicit test-only generator uses fixed ZIP metadata and stored bytes. A committed
manifest records source and decompressed-part digests independently of the Go reader;
Git attributes preserve exact fixture bytes. Regeneration is never an audit-time action.

```sh
python3 internal/packageparts/testdata/generate.py
go test ./internal/packageparts
go test -race ./internal/packageparts
go test ./...
go vet ./...
go test ./internal/packageparts -run '^$' -fuzz FuzzRead -fuzztime 2s -parallel 2
```

Tests assert exact content/identity, ownership and determinism, both descriptor forms,
central-order independence, unsafe/duplicate names, special files, corrupt payloads,
conflicting headers, truncated input, prefix/trailer rejection, dishonest counts,
encryption/ZIP64/unsupported methods, expansion and name budgets, cancellation and
retained fixture manifests. Fuzzing accepts arbitrary bounded byte snapshots.

## Remaining #14 work

Next: bounded XML token/part locations, OPC/ODF package identification and relationship
rules, format-specific body/header/hidden-content/metadata observations, and independent
context producers. Unknown parts remain evidence; embedded containers are never
recursively expanded implicitly. Then real profile bundles, pattern override links,
report/import migration and CLI/forensic views require their own reviews and tests.
PDF is a separate parser path. No external forensic oracle is adopted or executed by
this increment; #40/#41 still require their own gates. #14 remains open.

Implementation references: Go's [archive/zip documentation](https://pkg.go.dev/archive/zip)
and pinned toolchain source informed the ZIP API boundary. Header and payload identity
checks here add product constraints; successful extraction does not imply semantic
format validity, trust or absence of hidden content.

## Increment validation record

On Linux amd64 with Go 1.27.1, the full Go suite, `go vet ./...` and package race
checks passed. A two-worker, two-second fuzz run executed 253,116 inputs without
failure; this is a bounded check, not exhaustive assurance. The 265 offline contract
tests also passed. Explicit fixture regeneration retained all five package/manifest
byte identities; an independent Python ZIP CRC check and XML well-formedness parse
passed for all four packages and their XML/relationship parts. No Office application,
external provider, network audit, model, or format conformance validator ran.

## XML evidence layer

[XP-001](xml-part-evidence.md) defines the next bounded layer: namespace-aware
structure and exact part-byte token spans, with conservative mapping for transformed
text. It preserves package/part identity boundaries and does not enable format-specific
parsers or profile suppression by itself.

## Proposed Office coordinator boundary

[OI-001](office-cli-evidence.md) adds an outcome-based package path and
verified-package inspector entry points for the 4.0 coordinator. Existing
`Read`/`Inspect` contracts here remain strict and unchanged. New entry points
consume one shared, bounded package outcome, preserving unassessed/failed parts
and incomplete prerequisites without re-running the strict reader or
decompressing again. OI-001's staged identification-priority budget applies to
that new path; it does not silently change this legacy API's failure semantics.
