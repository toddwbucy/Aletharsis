# ODT package identification — OI-001

Status: proposed #14 increment, stacked on DI-001 (#67) but implemented independently
of OPC. `internal/odtidentify` joins the mimetype item, ODF manifest and content XML.
It does not render text, decrypt content, verify signatures, assess profiles or enable
ODT in the CLI. Package source bytes are acquired by the caller and never rewritten.

## Identity chain and format scope

`Inspect` verifies the expected source SHA-256 through DP-001 and retains the full
package inventory. Its result preserves mimetype/manifest/content digests, parsed XML,
located manifest entries, per-file membership, issues and selected root/content entry
indices. Each manifest anchor names the exact part/hash, element index and part-byte
span. Stored-part identities are distinct from claimed plaintext sizes or content.

Identification requires the exact ODT mimetype bytes, an uncompressed first ZIP item
named `mimetype` with no local-header extra field, matching manifest package identity,
a usable unencrypted `content.xml` entry, and matching
content XML with one `office:body` containing exactly one `office:text` element. The
DP-001 ZIP admission guarantees make mimetype payload offset 38 an exact layout check.
Supported manifest/content versions in this increment are 1.2 and 1.3; newer/other
versions remain explicit limitations. An absent manifest version may coexist with
recognized content/body identity (including older content carrying version 1.1);
this yields partial coverage, not a claim of full support for that ODF version.
Conflicting declared versions still block format identification. No filename
extension participates.

The result identifies `odt` only after the whole supported chain succeeds. A known
format can coexist with partial manifest coverage or an encrypted auxiliary entry.
The selected content XML retains XP-001 token mappings: transformed XML values do not
become exact original character offsets. Document rendering and content interpretation
remain separate from format identification.

## Manifest semantics and retained evidence

Manifest names match ZIP names exactly and case-sensitively. No OPC URI resolution,
ASCII case equivalence, Unicode normalization or percent decoding occurs. Unicode and
literal percent-containing filenames remain distinct. Root `/` is the package entry;
other supported names are relative paths without traversal or unsafe separators.
Directory declarations can refer to explicit ZIP directories or an existing descendant
prefix; an implicit directory receives no invented byte digest.

Every file-entry preserves full path, media type, version, declared-size text, size
presence, encryption presence and an exact anchor. Duplicate paths invalidate every
occurrence. Namespace/structure ambiguity, missing required fields, forbidden self or
mimetype entries and unknown file-entry constructs prevent those entries from
providing membership authority while preserving their XML. Unrelated defective
entries, unknown root attributes and unsupported/absent manifest versions mark
coverage partial without stopping content-root/body inspection. Required root or
content entries must still resolve unambiguously; invalid roots and entry-budget
exhaustion stop inspection. Declared sizes are compared only for resolved plaintext
entries, so a missing target retains its missing state and declared-size evidence.
This is a conservative
admitted subset, not a complete ODF manifest schema validator.

Each retained file is linked to a manifest entry or marked unlisted or manifest-exempt.
The mimetype item and `META-INF/` files have the latter state when no entry is needed.
It means only that manifest membership is not required by this operation. Signature
files remain unverified bytes; exemptions do not imply trust, expectedness, coverage
by another analyzer or omission from the forensic inventory. Missing targets and
unlisted files are explicit diagnostics. Plaintext declared sizes, when present, must
match stored decompressed bytes; empty, invalid or mismatched declarations are not
silently ignored. Raw size and media values remain available for later interpretation.

## Encryption boundary

A namespace-qualified `manifest:encryption-data` child is an observed declaration,
not proof that the payload is valid ciphertext. Its subtree is retained as opaque XML;
no algorithm, key derivation, password prompt, decryption or plaintext expansion runs.
The declared original size never controls an allocation. The exact stored-part digest
identifies the ZIP-extracted payload, which may be ciphertext, not decrypted content.

Encrypted content stays unavailable even if the stored bytes happen to look like valid
XML. `content.xml` declared encrypted therefore never enters the XML content parser.
Encrypted auxiliary files make the result partial without erasing an independently
established plaintext main-document identity. Missing encrypted payloads retain both
the missing-target and unavailable-encryption diagnostics. Encryption declared for a
package/directory is unsupported rather than treated as a usable plaintext object.

## States, limits and validation

`not_applicable` means neither exact ODF entry point was present. `completed` means the
supported identification/membership checks completed without recorded gaps; it says
nothing about every file's semantic contents. `partial` records uncertainty or failure;
format is empty when identification itself is unproven. Selected indices start at `-1`.
Package errors return no result; later failures retain the source package and typed
issue codes. Context cancellation is cooperative at parser/operation boundaries.

DP-001 source/container limits apply. Manifest and content each use XP-001 limits:
4 MiB XML, 100,000 tokens, 50,000 elements, depth 128, 128 attributes per element and
8 MiB retained name/value data. At most 4,096 manifest entries are emitted; overflow
retains the parsed XML and an explicit limitation. File membership is bounded by the
package's entry limit; implicit directory lookup uses the sorted part inventory.
These bounds add to retained source/package storage and are not hard RSS/time promises.

Tests cover exact mimetype layout/content, versions, source/part/entry identities,
case-sensitive Unicode/percent names, implicit directories, encrypted content versus
stored plaintext, missing encrypted targets, partial auxiliary coverage, unverified
signature exemptions, duplicate/forbidden/traversing entries, namespace spoofing,
DTD rejection, missing/case-mismatched content, mixed body types, invalid sizes,
retained DOCX/ODT fixtures, determinism, immutable input, cancellation and entry limits.

```sh
go test ./internal/odtidentify
go test -race ./internal/odtidentify
go test ./...
go vet ./...
go test ./internal/odtidentify -run '^$' -fuzz FuzzODTManifest -fuzztime 2s -parallel 2
```

Next: format-specific text, metadata and hidden-content observations; exact extraction
maps; reviewed context producers/profiles; report/import and CLI integration. #14 stays
open. Independent Office oracle validation remains a separate #40 gate. No additional
runtime dependency or external service was introduced.

Reference: [OASIS OpenDocument 1.3 package specification](https://docs.oasis-open.org/office/OpenDocument/v1.3/os/part2-packages/OpenDocument-v1.3-os-part2-packages.html).
The admitted version range, conservative failure handling, bounded evidence objects
and absence of decryption are Aletharsis implementation policies. Passing these checks
does not certify ODF conformance or validate cryptographic assertions.

## Increment validation record

Linux amd64 / Go 1.27.1: full Go tests, vet and package race checks passed. The 265
offline contract tests passed. A two-second, two-worker fuzz configuration completed
75,332 manifest inputs without failure (about three seconds including shutdown).
The retained DOCX/ODT packages are distinguished without filenames and preserve
original bytes. These are bounded native checks, not full ODF conformance, cryptographic
validation or independent Office-oracle comparison. No decryption, external resource
access, model call or source-file mutation occurred.

Manifest entries with absent/empty full paths retain `odt.manifest_entry_invalid`;
multiple missing paths are not a duplicate-path assertion. For a genuinely duplicated
nonempty path, all entry anchors remain in `Entries`. `Membership.Entry` references
the last declaration in manifest order as a deterministic diagnostic anchor only;
its ambiguous state/code remains authoritative and the reference grants no membership
or extraction authority. Consumers can inspect all entries sharing that path.

Entries retain expanded element namespace and local kind. Only manifest-namespace
file-entry elements participate in path collision and membership lookup; a foreign
element or different local kind remains unsupported evidence without claiming a
real entry's path. Genuine defective/duplicate manifest declarations still block
membership authority for their own paths.

## Proposed Office coordinator boundary

The existing API remains unchanged; proposed [OC-001 §4](office-cli-evidence.md#4-producers-and-orchestration-b1b2) defines the separate outcome-based coordinator and budget policy.
