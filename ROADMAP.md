# Aletharsis roadmap

This is the delivery map for the approved [parent PRD](docs/product/aletharsis-PRD.md).
The PRD governs product scope and invariants; technical specifications define the
contracts; issues and PRs track implementation. This roadmap records dependencies
and acceptance gates, not promised dates or functionality available today.

Baseline: **Go CLI 0.2.0 / schema 1.0 by default, schema 2.0 by opt-in**,
following the reviewed P0 implementation in PRs #23–33. See the
[README](README.md) for supported commands and current limitations.

## Destination

```text
Select workspace → read-only detection → accounted-for review queue
    → inspect evidence → classify → save/associate rule
    → search → freeze exact matches → review proposed actions
    → dry run → explicit confirmation → validated new copies → re-audit
```

Detection, review and apply remain separate authority boundaries. Audits are
useful without a GUI or writer. Originals remain evidence. Rules do not authorize
removal, and new matches never inherit a previous frozen-set approval.

## Completed foundation

| Delivered | Evidence |
| --- | --- |
| Go implementation of the original M0/M1 framework and Unicode/text analysis | [Migration record](docs/migration/go-backend-migration.md), [frozen reference](reference/python-behavior/README.md) |
| Console/JSON CLI, strict schema 1.0, source identity and coordinate evidence | [Text audit reference](docs/text-audit-reference.md), [schema](schemas/report.schema.json) |
| Direct unit/property tests, compiled-CLI integration, acquisition fault/integrity checks and CI | Delivered work within open [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7), [CI guide](docs/testing/backend-ci.md) |
| Bounded fuzzing, resource characterization and platform verification | [Fuzzing](docs/testing/fuzzing.md), [performance](docs/testing/performance.md), [platforms](docs/testing/platforms.md) |
| Parent product architecture and revised frontend draft | [Parent PRD](docs/product/aletharsis-PRD.md), [frontend PRD](docs/product/frontend-PRD.md) |

Issue #7 remains open until its full validation scope is complete. The delivered
checks listed here do not mark that tracker complete.

Completed tests establish the current scope, not universal safety or unimplemented
end-to-end workflows. Acquisition is functional on supported Linux systems;
macOS/Windows have explicit refusal tests and cross-build checks, not supported
readers. The [Python application retirement](docs/migration/python-retirement.md)
preserves frozen evidence and replaces live comparisons with 210 captured cases;
Python test harnesses remain.

## Delivered: P0 evidence and capability contracts

[Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17) now has a reviewed
and merged implementation through PRs #23–33. The
[acceptance map](docs/testing/issue-17-acceptance.md) links the original criteria
to code, fixtures and validation. This delivers the mechanism/capability substrate;
it does not implement C2PA, statistical detection, profiles or a frontend.

- [ADR-0001](docs/adr/0001-evidence-capability-contract.md),
  [EC-001](docs/specs/evidence-contract-v2.md) and the
  [versioned wire/import contract](docs/specs/report-v2-wire-and-import.md)
  separate mechanism, capability, execution, result and finding semantics.
- [Identity primitives](docs/specs/go-v2-identity.md),
  [execution records](docs/specs/go-v2-records-registry.md),
  [graph validation](docs/specs/go-v2-evidence-graph.md) and
  [exact-byte import](docs/specs/go-v2-report-import.md) preserve source/report
  identity, typed locations and explicit unavailable/partial/failed coverage.
- The [native coordinator](docs/specs/go-v2-native-assembly.md) and
  [CLI](docs/specs/go-v2-cli.md) expose `--schema-version 2.0` with deterministic
  references, source-verified anchors and coverage before findings.
- [Consumer alignment](docs/specs/v2-consumer-alignment.md),
  [resource measurements](docs/testing/performance.md) and
  [native platform checks](docs/testing/platforms.md) document the tested scope.
  Report limits can reject inputs below the 8 MiB acquisition cap; rejection never
  means a completed audit or truncated evidence.

Schema **1.0 remains the default**. A future default-version switch requires a
separate release decision. Frozen findings, coordinates and migration artifacts
remain protected. The F0 Occurrence schema/component spike and concrete adapter
adoption continue under their own gates; P0 acceptance does not complete them.

The Python application is retired. Frozen reports, schemas, fixtures and Unicode
data remain as evidence, and Python utilities are test-only.

## Gated detector reuse program

G0 planning is tracked in [#35](https://github.com/toddwbucy/Aletharsis/issues/35):
[reuse policy](docs/adr/0002-detector-reuse-policy.md),
[versioned candidate registry](docs/reuse/README.md),
[maintenance assessment](docs/reuse/assessment.md) and
[bounded work breakdown #36–45](docs/reuse/plan.md).
These planning records do not approve any upstream detector.

[Epic #21](https://github.com/toddwbucy/Aletharsis/issues/21) coordinates upstream
reuse across Aletharsis, WeaverTools and isolated comparison/sidecar environments.
Its principle is **reuse detectors; own the evidence**. The epic is a plan, not
approval to install dependencies, change external repositories or claim new
capabilities. It complements #17's contract work and #7's ongoing validation.

| Gate | Decision or deliverable | Dependency / required evidence |
| --- | --- | --- |
| **G0 — Policy and inventory** | Reuse ADR, component registry, bounded child issues and named implementer/reviewer roles | Pinned revisions, license/data provenance, redistribution/runtime implications, budgets and separate adoption status |
| **G1 — Evidence and execution contracts** | Mechanism, capability, execution/result, input identity, coordinate and side-effect contracts | P0 / #17; reviewed schema migration, unavailable/partial states and contract fixtures |
| **G2 — C2PA feasibility** | Independently evaluate c2pa-text extraction and encypher-c2pa verification | Spikes after G0; production adoption requires G1, interoperability, normalized/original coordinate mapping, resource/platform evidence and control of trust/telemetry defaults |
| **G3 — Structural comparison corpora** | Unicode first; Office/PDF comparisons as parsers arrive | Independently expected and licensed fixtures; disagreements classified by inventory, parsing, offsets, policy and coverage; P2/P3 gates for structured formats |
| **G4 — Optional statistical sidecar** | Protocol first, then a selected configured detector | G1; pinned environment, actual score semantics, bounded process lifecycle and tests with runtime/access absent |
| **G5 — Weaver interoperability and research** | Portable baseline exporter/consumer contract and optional fingerprint experiment | Accepted corpus identities, linked producer/consumer issues and compatibility tests; blind/open-set validation before any fingerprint claim |
| **G6 — Release and lifecycle** | Per-component release, maintenance and rollback decision | #7 validation evidence, native supported-platform checks, conformance corpus, dependency notices and reviewed upgrade semantics |

The carrier/verifier studies are accepted in PRs #47/#48, and the
[DA-001 exchange contract](docs/specs/detector-adapter-v1.md) is accepted in PR #49.
The [report-3.0 migration design](docs/specs/report-v3-migration.md) is accepted in
PR #50. Under #36, the [wire/schema conformance gate](docs/specs/report-v3-wire-and-import.md)
now supplies complete reports and retained evidence for review. Go import/emit
parity and opt-in consumer acceptance follow before any production adapter.
Schema 1.0/2.0 behavior stays unchanged.

C2PA is the first adoption evaluation, not an already selected production
dependency. Feasibility can inform G1 without bypassing its acceptance gate.
Unicode comparisons can proceed independently; Office/PDF comparisons wait for
their parser contracts. Optional statistical and fingerprint work cannot block
validated local structural releases. G6 applies to each adopted component.

Aletharsis owns the evidence and consumer adapters. Weaver owns controlled
creation and eventual corpus export. Upstream tools remain pinned components or
isolated comparators; their outputs do not define Aletharsis' wire contract.
Cross-repository work requires linked issues, versioned compatibility fixtures,
producer/consumer validation and a rollout order before implementation. Repository
owners and independent reviewers must be identified at the relevant gate.

The epic holds the full candidate matrix, tests, gate decisions and completion
checklist. Each gate needs linked reproducible evidence and a reviewed go, revise,
reject or explicitly deferred disposition. Upstream updates cannot silently alter
result semantics. Neither a completed spike nor creation of child issues closes
the program; optional deferrals require an explicit product-owner decision.

## Product delivery tracks

All tracks below are **planned**, except for the foundation already listed. P6
includes optional research; it is not a commitment to ship a detector. Track
numbers P0–P7 follow the parent PRD and do not rename the historical backend M0–M4
milestones or frontend F0–F5 phases.

| Track | Intended outcome | Dependency and completion gate |
| --- | --- | --- |
| **P1 — Corpus and reveal CLI** | Bounded directory scans, JSONL, aggregate outcomes, revealed representations and diffs | P0 contracts; deterministic discovery, exclusions/cancellation accounting, source-to-reveal mappings, safe new-file output and measured resource limits. [Issue #15](https://github.com/toddwbucy/Aletharsis/issues/15) tracks reveal/diff work. |
| **P2 — Format profiles and office containers** | DOCX first, then a separately scoped ODT increment; contextual expected-artifact profiles; [PA-001 standalone evaluator](docs/specs/expected-artifact-profiles-v1.md) proposed | P0 identity/profile contracts; bounded parsing, exact package/object evidence, no external resource fetching, explicit extraction gaps and profile non-suppression tests. [Issue #14](https://github.com/toddwbucy/Aletharsis/issues/14) tracks profiles. |
| **P3 — PDF** | Born-digital structural/metadata evidence, with explicit OCR-derived limitations | P0 evidence contracts; bounded extraction, page/object locations, conservative visibility claims and declared unsupported checks. OCR is not implied. |
| **P4 — Review and rule backend** | Versioned rules, matching, review decisions and frozen result sets | P0 contracts; deterministic membership, source/query/rule identities, stale-source rejection, portable decisions and no inherited removal authority. Coordinate with frontend F0/F1. |
| **P5 — Provenance adapters** | C2PA inspection/validation; statistical watermark adapters when real access exists | P0 contracts; separate credential discovery, binding, signature and trust states; actual statistical result semantics, analyzed scope and explicit authorization for network analysis. |
| **P6 — Advanced analysis** | Vetted new steganographic/statistical/cross-format analysis; experimental intrinsic fingerprints | Detector-specific assumptions, positive/negative controls, held-out evaluation, measured error limits and unknown outcomes. Separate specification and validation before product integration. |
| **P7 — Controlled derivatives** | Reviewed text edits and later format-aware writers, dry runs, new files, manifests and re-audits | Accepted P4 review/plan contracts and relevant parsers/locations; source/plan revalidation, explicit confirmation, no overwrite, validation and independent integrity tests. |

These tracks can proceed independently when their prerequisites are met:

```text
P0 evidence/capability contracts
 ├── P1 corpus + reveal
 ├── P2 office + profiles
 ├── P3 PDF
 ├── P4 review + rules ───────────────┐
 └── P5 provenance adapters          │
                                    ↓
                          P7 controlled derivatives
                   (relevant parser/location/writer gates)

P6 research can proceed separately; integration requires accepted contracts.
```

C2PA feasibility does not wait for the GUI or cleanup. Vendor detector access cannot
block useful local structural analysis. Text-only derivative work need not wait for
PDF parsing; office derivatives require their own verified parsers, mappings and
writers. Successful parsing never establishes safe writing support.

## Frontend coordination

The [Evidence Review Workbench PRD](docs/product/frontend-PRD.md) is **draft v1.1**.
Changed requirements retain a separate product-review gate; the
[approved v1.0 archive](docs/product/frontend-PRD-v1.0.md) is historical.
No frontend phase is implemented by the current CLI.

| Phase | Planned scope | Backend/design prerequisites |
| --- | --- | --- |
| **F0 — Evidence viewer** | Offline report import, retained report SHA-256, typed evidence anchors, reveal/search and honest coverage | P0/legacy adapters and Occurrence contract; complete the large-report/Unicode-coordinate spike **before choosing a viewer/editor component** |
| **F1 — Review and rules** | Collection search, judgments, reusable rules, frozen match sets and sidecars | P4 matching/review contracts; no writer required |
| **F2 — Live workspace** | Local adapter, source verification, jobs, triage and language contexts | P1 workspace support and independently declared context parsers |
| **F3 — Reviewed text cleanup** | Exact plans, dry runs, confirmation, validated new-file output and re-audits | P7 text writer/validation services |
| **F4 — Office inspection** | DOCX/PDF and later ODT structural navigation | P2/P3 parsers and accurate location contracts; no implied office writer |
| **F5 — Provenance integration** | Credential panels and configured statistical results | P5 adapters and P0 result/coverage contracts; independent of F3/F4 |

Imported report hashes and original source hashes remain separate identities.
The UI cannot redefine byte/scalar/UTF-16 correspondence or turn a statistical
sample into an exact removable span. Resource budgets must reflect measurements,
including large JSON reports and complex Unicode, rather than nominal file size.

## Experimental fingerprint plan

Intrinsic model fingerprinting is an optional **P6 stretch goal**, distinct from
keyed watermark detection. The plan in [PRD §5.5](docs/product/aletharsis-PRD.md#55-experimental-stretch-goal-intrinsic-model-fingerprint-analysis)
uses [WeaverTools](https://github.com/toddwbucy/WeaverTools) as the first planned
controlled baseline producer:

1. Generate reference samples under recorded conditions and retain creation traces.
2. Export portable, versioned corpora binding artifact kind/stage, canonical byte
   serialization and hashes to conditions, trace references and transformation lineage.
3. Separate development, calibration and blind evaluation data; keep identifying
   labels, filenames and trace metadata out of the analyzer's evaluation features.
4. Vary model and harness conditions independently, test unseen models and
   generation setups, and report calibration, errors and unknown outcomes.

Weaver supplies controlled reference data; validation determines whether useful
calibration is possible. Independent corpus producers must satisfy the same
contract. A similarity result does not establish authorship, intent or a deliberate
watermark. Ordinary auditing requires neither Weaver nor fingerprint analysis.
No baseline exporter or fingerprint detector is claimed to exist yet.

## Release discipline

Each track proceeds through bounded specification and implementation PRs with
relevant validation. Update this roadmap when capabilities and their acceptance
evidence land; distinguish partial delivery from a completed track. No dates are
committed here.

Across releases:

- Preserve originals and evidence identities; fail explicitly when guarantees cannot be met.
- Keep missing, failed and incomplete coverage visible; no unqualified “clean” verdict.
- Keep inspected content inert and local by default; remote analysis requires authorization.
- Preserve strict versioned contracts and explicit migration paths.
- Retain reproducible fixtures and test the real executable, source integrity and
  applicable platform/resource behavior before advertising a capability.

Validated macOS/Windows acquisition remains separately gated work. Additional
formats, semantic rewriting and organizational provenance insertion require
separate product/specification decisions; they are not implied by the initial
writers or the research track.
