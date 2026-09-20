# ADR-0001: Separate evidence, capability, execution and result

| Attribute | Value |
| --- | --- |
| Status | Design direction accepted in PR #23; schema and production changes require separate review |
| Decision scope | Issue [#17](https://github.com/toddwbucy/Aletharsis/issues/17), parent P0; prerequisite for Epic #21 G1 |
| Baseline | Main `8035414`, Go 0.2.0, report schema 1.0 |
| Companion specification | [Evidence contract design v0.1](../specs/evidence-contract-v2.md) |

## Context

The current Go model records source identity, extracted evidence and findings.
Analyzer interfaces return findings alone. A successful structural scan cannot
express that cryptographic or statistical analysis was unavailable. Empty findings
therefore cannot serve as universal negative evidence.

Schema 1.0 closes objects and discriminates findings by stable rule ID. Adding
mechanisms, execution state or failure fields without a new schema would violate
that contract. The frontend requires exact imported-report identity, source
coordinates and typed non-text anchors. Detector reuse adds upstream identities,
partial results and side effects that the current interface cannot represent.

## Decision

1. Introduce report schema **2.0** through a separate reviewed implementation;
   preserve the original schema 1.0 and all frozen references unchanged.
2. Separate registered capability, local availability, requested participation,
   actual execution, analyzed scope, detector-specific result, finding and profile
   assessment. Review decisions remain outside audit reports.
3. Use `structural`, `cryptographic` and `statistical` as mechanism classes;
   algorithm purpose distinguishes statistical watermarking, channel research and
   future intrinsic fingerprinting. Operational acquisition/parser failures have
   no signal mechanism. Mechanism does not imply intent, expectedness or severity.
4. Keep native finding IDs, categories, classification vocabulary, severity,
   confidence, evidence and locations. Add report-local provenance links in v2;
   introduce stable `parser.failure.evidence.failure_code` at structured failure
   boundaries. New detector result variants need a reviewed schema, not arbitrary
   dictionaries or fabricated vendor probability fields.
5. Maintain a fixed, versioned catalog of built-in capabilities plus explicitly
   configured adapters. The initial Anthropic entry is an unavailable declaration,
   not an implementation, endpoint, detector result or claim about a file.
6. Bind executions and typed anchors to immutable representations and declared
   scope. Original-byte, extracted-text, binding-representation, manifest and
   analyzed-sample identities remain distinct. Coordinate precision never grants
   edit authority.
7. Preserve schema-1.0 imports as original artifacts; adapt into a separate
   consumer model with unknown historical coverage, not fabricated executions.
   Prefer an explicit v2 output opt-in during rollout, followed by a separately
   reviewed default switch. No lossy downgrade of v2-only results.
8. Keep production implementation in Go. Bound adapters behind acquired bytes or
   verified controlled snapshots; default auditing neither uploads nor invokes
   absent runtimes. Optional sidecars have independently reviewed protocols and
   deployment gates under Epic #21.

## Alternatives considered

| Alternative | Reason not selected |
| --- | --- |
| Add optional fields to schema 1.0 | Strict existing validators reject them; a nominally unchanged version would hide an incompatible change |
| Represent missing detectors as INFO findings | Confuses execution coverage with evidence and changes finding counts/exit priority for capability declarations |
| Add `expected_artifact`/`statistical_result` to classification | Conflates independent profile or result state with the existing finding interpretation vocabulary |
| Treat all results as text findings | Invents removable spans for sample-level results and obscures credential validation dimensions |
| Expose upstream result objects directly | Lets upstream updates change the public protocol and can leak secrets or introduce unbounded data |
| Replace the engine with a plugin framework now | Adds deployment and discovery complexity before the contracts or first adapter have been validated |

## Consequences

Consumers need explicit version dispatch and semantic reference/coordinate checks
in addition to JSON Schema validation. Report-local links add size, so large-report
measurements remain a release gate. A capability catalog states only known,
registered coverage; it cannot enumerate every possible detector or certify the
absence of all information channels.

Local deterministic reports remain reproducible for the same source/path, registry,
configuration and pinned data. Remote/trust-dependent results preserve evaluation
context instead of claiming timeless reproducibility. Hashes bind representations,
not authenticity. The report-artifact digest is computed outside the report to
avoid self-reference.

This decision does not select a C2PA SDK, add a statistical detector, change profile
policy, authorize remote analysis or implement the frontend/apply pipeline. The
separate [reuse epic](https://github.com/toddwbucy/Aletharsis/issues/21) controls
adoption; [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7) remains open.

## Acceptance

Approve this ADR and the companion contract together, resolving the explicit review
decisions in the specification. Approval authorizes the next bounded schema/fixture
PR, not a silent default change. Subsequent Go implementation must pass independent
contract tests and the preserved regression suite before advertising v2 support.
