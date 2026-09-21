# OPC relationship evidence — OR-001

Status: proposed #14 parser increment stacked on XP-001 (#65). This inventories
relationship declarations and supported package-local target identities. It does
not identify a package as DOCX, validate all OPC constraints, traverse relationships,
interpret body content, evaluate profiles or enable a CLI format.

## Input and evidence ownership

`opcrels.Inspect` accepts one acquired byte snapshot, its expected SHA-256 and a
context. It invokes the bounded DP-001 reader once and retains that complete package
inventory in the result. It never opens an original path or any relationship target.
Source, compressed and decompressed part identities remain distinct.

Candidate parts have ASCII-case-insensitive `.rels` suffixes. Canonical locations are
`_rels/.rels` for package relationships and `<directory>/_rels/<filename>.rels` for
relationships from `<directory>/<filename>`. Other `.rels` locations receive an
explicit unsupported result, not silent omission. An ordinary `.rels` file is only
a candidate; its suffix does not establish OPC semantics or document format.

Every candidate has an outcome, exact name/hash, declared or resolved source part,
and, when parsing succeeds, the full XP-001 XML document. Each direct Relationship
child in the correct namespace retains ID, type, target, effective target mode,
source, result state/code, and an exact element anchor `(part, part SHA-256,
element index, original part-byte span)`. The retained XML distinguishes an omitted
TargetMode (default Internal) from an explicit empty/invalid value.

The native result order is deterministic: exact package-part order, then declaration
order. No evidence is deleted when a declaration cannot be resolved. XML comments,
processing instructions, unknown elements and attributes remain inert part evidence.
Relationship types are preserved as uninterpreted strings; nonempty type/ID checks
are not full URI/NCName or OPC schema validation. `resolved` means a target identity
was found under the admitted resolution policy, not that a relationship is trusted,
semantically valid for Word, safe to open, or authorized for execution.

## Resolution policy

Internal targets use a conservative ASCII URI-path subset. Relative paths start at
the source part's containing directory; package-absolute paths start at the root.
Dot segments are resolved without allowing escape above that root. Empty components,
trailing directory references, unsupported punctuation, schemes, authority forms,
percent escaping, query strings, fragments and non-ASCII paths remain unresolved.
An unsupported source-part URI also prevents invented target resolution. No path is
passed to an OS filesystem API. Supporting the omitted URI cases requires separately
reviewed equivalence rules and fixtures; they must not become silent normalization.

Part equivalence folds only ASCII letter case while retaining original names.
One matching non-directory part yields its exact name and decompressed SHA-256.
Zero matches means missing; multiple equivalent parts mean ambiguous, even if an
exact-case match also exists. Duplicate relationship IDs invalidate every occurrence
of that ID in its source part. Missing/ambiguous source parts, conflicting relationship
part names, unknown TargetMode and unknown attributes/child content remain explicit.
Unknown root attributes prevent resolution because they might affect interpretation.
Unknown siblings make the part partial while independently interpretable declarations
remain inspectable.

External mode records `external` with no resolved part or target hash. The target
is never fetched, executed, rewritten or declared safe; it may contain any URI scheme.
This is local declaration evidence only, not availability or authenticity evidence
for an external destination. Relationship cycles cannot trigger recursive work because
this layer does not follow any relationship, including embedded-package links.

## Outcomes and budgets

Top-level states are `not_applicable` (no candidate parts), `completed` (all candidate
relationship inventories assessed within this policy) or `partial`. These are narrow
operation outcomes, never a document-clean verdict. Part outcomes distinguish
completed, partial, failed, unsupported and not_run. Relationship outcomes distinguish
resolved, external, missing, ambiguous, unresolved, invalid and unsupported.
Stable codes accompany failures/limitations; callers must not parse error prose.

Source/ZIP admission failures return no result. XML or relationship interpretation
failures preserve the package and a failed/partial candidate outcome. Unsupported or
resource-exhausted candidates cannot supply expected-artifact suppression authority.
Cancellation returns the context error without a falsely completed aggregate result.

DP-001 bounds apply to acquisition/container handling. Additional fixed ceilings are
256 XML relationship parses, 8 MiB candidate XML bytes parsed, 100,000 XML tokens,
16 MiB retained XML name/namespace/value bytes and 10,000 emitted relationships.
XP-001 per-part limits still apply. Failed XML parses consume their entire token/value
allowance because actual partial use is not exposed; later candidates can therefore
be explicitly not_run. This conservative accounting prevents failures from bypassing
aggregate work bounds. Relationship-cap exhaustion retains the complete parsed XML
but emits only the bounded declaration prefix and marks the part partial.

The package reader still retains all package parts within its own limits. Aggregate
budgets do not claim a hard CPU/RSS bound; lexer calls are not preempted, and cancellation
is cooperative. No network, subprocess, Office SDK or additional dependency is used.

## Validation and next work

Tests cover retained DOCX versus ODT applicability, exact relationship anchors and
target hashes, root/part-relative/absolute targets, external declarations, case aliases,
duplicate IDs, missing/ambiguous sources, malformed XML, namespace spoofing, unknown
structures, unsafe/unsupported URI syntax, source identity, source immutability,
determinism, cancellation, part/declaration budgets and fuzzed relationship XML.

```sh
go test ./internal/opcrels
go test -race ./internal/opcrels
go test ./...
go vet ./...
go test ./internal/opcrels -run '^$' -fuzz FuzzRelationshipXML -fuzztime 2s -parallel 2
```

Next: content types and root-relationship-based DOCX identification, ODF manifests
and ODT identification on their separate path, then body/header/metadata/hidden-content
observations and context producers. Profile/report/CLI integration and independent
Office oracle comparison remain required. #14 and #40 remain open; this increment
passes neither document-format conformance nor detector-adoption gates.

References: [Microsoft OPC relationship model](https://learn.microsoft.com/en-us/dotnet/api/system.io.packaging.packagerelationship),
[OPC part-name case equivalence](https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/8.0/system-io-packaging-case-insensitive-uri),
and [ECMA-376 Part 2](https://ecma-international.org/publications-and-standards/standards/ecma-376/).
The stricter admitted URI subset and bounded partial-result policy are Aletharsis
implementation choices, not statements that every rejected form violates OPC.

## Increment validation record

Linux amd64 / Go 1.27.1: full Go suite, vet and package race checks passed. The
265 offline contract tests passed. A two-second, two-worker fuzz configuration
completed 59,798 executions without failure. Additional aggregate XML-byte and failed
parse allowance tests passed. These are bounded implementation checks, not complete
OPC/Office conformance or an external forensic-oracle comparison. No relationship
resource was accessed and no third-party runtime dependency was introduced.
