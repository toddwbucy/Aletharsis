# Native v2 audit assembly

This increment implements fresh native audit assembly for [Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17), following the [report and import boundary](go-v2-report-import.md). It does not change the CLI's schema 1.0 default or implement external detectors.

## Service boundary

`audit.RunV2(ctx, path, options)` returns a typed report and its validated canonical JSON snapshot. It uses the same acquisition, identification, literal-text parser and six analyzers as the legacy audit. An internal trace captures each operation's actual completion and findings. Imported reports cannot execute operations or acquire invented execution history.

The compiled catalog declares eleven capabilities. C2PA extraction, C2PA verification and Anthropic statistical detection remain disabled and unavailable, with no results, endpoints, keys or network access. A clean structural scan still reports those coverage gaps. On platforms without the no-atime reader, acquisition is unavailable and is not invoked.

Acquisition/parsing failures retain their typed diagnostic and existing parser finding, extended with `failure_code`. Context cancellation and deadlines are checked between native operations. Completed operations and their results survive interruption; downstream operations are explicitly not run. Interruption produces an execution diagnostic, not an invented parser failure. This is cooperative cancellation, not preemption of an analyzer or a hard runtime limit.

## Identity and ordering

The source artifact records acquired byte identity even when parsing fails. Successful parsing adds a literal UTF-8 text artifact, its digest, decode transform and exact scalar-to-source-byte mapping. `identity.VerifyText` verifies the entire native decoding and boundary table once; its owned, immutable mapping supplies subsequent selection hashes.

Capabilities sort by ID/revision, executions by stage and capability ID. Native anchors sort by execution order and canonical locator; their other contract ordering keys are constant for the single literal text artifact. Findings retain legacy severity, rule, location and title ordering, with canonical payload and execution order as deterministic tie breakers. References are assigned after ordering, without clocks or shared counters.

Occurrence anchors preserve every reported offset. Adjacent spans coalesce while gaps remain explicit. UUIDs, encoded-string candidates, mixed-script tokens and provenance markers use their full verified observed extent. Escaped tokens are compared as inert character representations. Normalization and line-ending aggregates do not acquire fabricated locations. These spans describe evidence, not permission to delete it.

Every completed analyzer produces a structural result independently of the selected finding view. Filtering preserves source evidence, capabilities, executions, anchors, results and stable references. It cannot turn a positive analyzer result into a negative result.

## Compiled data and resource limits

Catalog `aletharsis.native-text/2` gives native descriptors revision 2 and digests of the exact embedded Unicode category, emoji, message and limitation data. Native configuration identity includes a digest of this ordered data manifest. This identifies these data inputs; it is not a complete compiler, module or reproducible-build attestation.

Defaults allow at most 8 MiB of source and 16 MiB of serialized report, with one million report nodes and depth 64. Callers may lower the source bound and set independent report budgets. Invalid options fail before acquisition. Existing v2 per-record budgets also apply to ordering keys, anchors and selections: 1 MiB, 32,768 nodes and depth 32. A source accepted by the legacy pipeline may exceed v2 evidence budgets. Such failures return an error and no purported complete report; evidence is not silently truncated.

These are validation/serialization budgets, not hard process-memory ceilings. Native evidence and marshaled ordering keys are allocated before some checks. Large-report measurement and CLI failure handling remain required before release acceptance.

## Validation and remaining scope

Tests project emitted reports back onto frozen native findings, file identities, evidence, summaries and exits. They cover malformed input, unavailable acquisition, cancellation/deadline boundaries, retained partial results, required/optional unavailable operations, filtered positive evidence, exact token extents, shuffled catalog/finding order and concurrent determinism. Linux tests verify source bytes and timestamps through the public service. Bounded fuzzing checks arbitrary byte inputs through assembly, strict import and source identity.

This service is not yet the opt-in CLI emitter or human coverage renderer. Compiled CLI v2 integration, consumer/resource validation, third-party review and release acceptance remain open under #17. No default-version switch, C2PA adoption, statistical detector, profile suppression or remediation is included.
