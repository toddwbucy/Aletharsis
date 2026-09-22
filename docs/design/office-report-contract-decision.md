# Office report contract: decision record

Status: owner authorized proceeding with best judgment and reviewable PRs; no main merges.
Tracking: #40; prerequisite B1–B5 specification increment.
Inspected baseline: `5b1ca93168c8f2a51751a05cac995d8657960fc8`.

## Resolved direction

Proceed with a versioned Office report extension (proposed 4.0). The owner also
authorized continued work during review; no procedural wait is required unless
further work needs a merge. The complete specification is proposed in
[OI-001](../specs/office-cli-evidence.md). Comparator clearance remains independent.

## Original conflict

The goal requires Office evidence in report versions 1.0 and 2.0. The accepted
EC-002 contract (`docs/specs/report-v2-wire-and-import.md`) preserves the unchanged
1.0 schema. EC-003 likewise preserves 1.0/2.0 and explicitly excludes new source
formats from its migration. The following are concrete restrictions, not merely
missing implementation:

- `structure` in all three schemas permits only `{}` or
  `{inspection: "literal source text", byte_length: N}`. Those objects are closed.
  There is no package-part, relationship or embedded-object inventory variant.
- Native text evidence contains one boundary-offset array. Existing flat-file
  verification decodes the acquired source and checks that array against its
  bytes (`internal/identity/digest.go`, `VerifyText`).
- WA-001 findings already use scalar offsets and assembled UTF-8 byte offsets.
  Their separate scalar origins map back to XML lexical spans. An entity such as
  `&#x200B;` occupies eight part bytes and three assembled UTF-8 bytes. Cross-run
  text has discontinuous XML origins. Neither is a ZIP-container offset.
- OA-001/ODT projections can additionally contain generated controls. Treating a
  generated scalar as literal source text would manufacture evidence.

A schema probe against each checked-in `$defs/structure` accepted empty and
literal-text structures and rejected `{inspection: "office package", parts: []}`.
This demonstrates the missing structural variant; it does not establish that
all possible Office evidence representations have been exhaustively rejected.
Existing free-form prose is not a substitute for typed, independently addressable
package evidence.

## Proposed coordinate decision

Keep flat-file `Text.ByteOffsets` and `VerifyText` unchanged. For Office, retain
three explicit identities: acquired container bytes, decompressed part bytes,
and assembled analysis text bytes. Retain container/part digests and extraction
identity; reuse XP-001/XM-001 and WA-001/ODT origins for every scalar mapping.
Compressed ZIP spans identify the encoded part, not individual characters.
Discontinuous lexical regions remain separate; generated controls remain derived.
No envelope across XML markup is an edit range. Every finding identifies its
coordinate artifact and resolves through checked mappings to the source evidence.

This requires a typed Office representation; do not overload the flat-text array
or relax existing text verification to admit it.

## Historical options presented to the owner

1. **Recommended: authorize a versioned Office report extension.** Preserve
   accepted 1.0/2.0 semantics and importer behavior. Select the exact new version
   in the B1–B5 specification, accounting for the existing 3.0 adapter work.
   Older requested versions return a typed unsupported representation outcome,
   never a lossy success. This changes the goal's requirement for Office evidence
   in versions 1.0/2.0 and therefore needs explicit owner acceptance.
2. **Explicitly revise the frozen 1.0/2.0 contracts.** Add closed Office variants,
   precise coordinate identities and consumer dispatch, keeping existing text
   reports unchanged. Existing reports may remain valid, but older strict
   importers will reject new Office reports under the same version. This is not
   transparent wire compatibility; accept that consequence explicitly before
   specification work proceeds.

The owner subsequently authorized the recommended direction and implementation
on reviewable branches, with review continuing during work. No merge is authorized.
OI-001 is the proposed B1–B5 specification in PR #74; it remains subject to review.

## Independent comparator investigation

No upstream detector has been run, installed or imported. Static archive reads
identified:

- oletools revision `ec10260989dbc48b9109e3d05d22beee19cf2333`;
  codeload archive size 3,144,228 bytes;
  SHA-256 `8a5e4874f6393ed540c33a1a03367123a5054fd334de3cfca79cab1af1e3e045`.
- The root license contains BSD-2-Clause and MIT material but explicitly excludes
  third-party components. The archive contains GPL-licensed vendored material;
  the package declares `pcodedmp`, whose package metadata declares GPL licensing.
  Root licensing alone does not clear a full installation.
- A narrower `ooxml`/`oleobj` read-only surface is a candidate for assessment,
  not an approved replacement comparator. Its imported modules, optional imports,
  complete runtime closure, individual licenses, side effects and scope fidelity
  still require clearance. Deliberately excluding optional dependencies must be
  enforced by an isolated runtime, not assumed from the developer environment.

B0 remains unresolved. No registry clearance, adoption or gate disposition is
claimed. Python remains development-only; no comparator artifacts or comparison
results have been produced.

## Work retained after the decision

The spec must enumerate audit dispatch, native evidence, capability planning,
per-part outcomes, metadata and embedded-object producers, graph/anchors,
identity verification, both reporters, import/schema validators, subview filters,
reveal and directory reveal, corpus loops and format discovery. It must explicitly
state unchanged behavior as well as changes. Bad-part isolation must not weaken
container identity checks; unknown container structure may make safe part
identification impossible and must remain a visible typed failure.

The eventual comparison still covers all four targets, independent expected
identities, six reachable adjudication categories, original-byte preservation,
licensed fixtures, unsupported checks and the stated resource budgets. None of
those requirements is satisfied by this decision brief.
