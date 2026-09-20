# Issue #17 acceptance evidence

This checklist maps the issue's original acceptance criteria to inspectable implementation and tests. It covers the opt-in schema 2.0 candidate stack, not a claim that all of it is merged or release-approved. The default remains schema 1.0. Review/merge acceptance must be checked against live PR state before closing #17 or Epic #21 G1.

| #17 requirement | Evidence in the candidate | Scope / remaining gate |
| --- | --- | --- |
| Independent mechanism, finding, capability and execution/result concepts | ADR-0001, EC-001, EC-002; accepted PRs #23–24 | Design accepted; implementation review follows |
| Go/JSON coexisting mechanism classes preserve categories/evidence | `internal/capability/registry.go`, `internal/evidence/v2`, `TestNativeV2DeclaresUnavailableMechanisms`, frozen projection tests | All three mechanism declarations; only native structural results implemented |
| Unavailable/unrun/failed/partial differs from completed negative | Eight complete contract fixtures; `TestAcceptedGraphsAndAggregates`, `TestV2ConsoleCoverageFixtures`; compiled v2 CLI cases | Disabled declarations do not imply a detector result; partial/failed status retains exit 4 |
| Anthropic unavailable without access, no heuristics/network | Compiled declaration and `TestNativeV2DeclaresUnavailableMechanisms`; native dispatcher only contains six local analyzers | No endpoint, credentials or statistical execution implementation |
| Statistical identity/scope without fabricated watermark location/score | Typed sample artifacts/locators; `TestAllLocatorVariants`, `TestResultRejectsInventedVendorSemantics`, `test_unreviewed_result_contracts_rejected` | Reserved locator contract is implemented; concrete vendor result schema waits for real API access |
| C2PA discovery/binding/signature/trust distinction | EC-001 credential design; separate carrier and verifier capabilities; typed manifest/credential identities | No supported credential formats or verifier yet; adoption remains Epic #21/P5 work |
| Clean, structural-plus-unavailable, partial/failed, mixed mechanisms | Native coordinator tests; complete synthetic reports; independent Python oracle; native CLI coverage tests | Mixed capabilities are implemented; fixtures do not claim actual C2PA or vendor-positive detection |
| Synthetic fixture provenance, no private inputs/live credentials | `tests/contracts/fixtures`, portable design vectors and frozen public inputs | No cryptographic credential or provider response is presented as real detector evidence |
| Profiles cannot suppress other mechanisms; no statistical reveal bytes | Registry/dispatcher independent of presentation; filtered views preserve full graph/results; closed profile/result contracts | Profile execution (#14) and reveal (#15) are not implemented; their consumers must preserve EC-001 boundaries |
| Original bytes/timestamps, findings/coordinates, frozen evidence | Linux source-integrity tests, compiled integration, 37 frozen reports, 210 seeded cases; `TestNativeReportConsumerCoordinates` | Actual acquisition supported only on Linux; other native targets must refuse safely |
| No uploads without authorization; reviewable adapter/config identities | Code-owned registry; configuration/data identities; offline bundled schema loader; inert import tests | No remote adapter/sidecar exists, so there is no upload path; future adapters have their own policy gate |
| Honest no-findings/coverage documentation | README, v2 CLI spec, human coverage renderer, consumer alignment note | Legacy reports remain explicitly unknown coverage when imported |

## Supporting validation

- Identity and semantic tests validate exact report bytes separately from source bytes and domain-separated selected spans. Source, scalar and UTF-16 boundary expectations are independent in the consumer smoke test.
- `scripts/check_distribution.py` now checks native built and installed binaries with empty PATH under **both** schemas. Linux has 48 cases; macOS/Windows have 80 refusal cases. Hosted CI execution is required before claiming those native results.
- Go 1.24 and development-toolchain tests, race/vet, compiled CLI tests, frozen/reference contracts and bounded fuzzing remain applicable. Review test names and assertions, not only aggregate green status.
- The resource probes at `097b35e` and `96d2201` preserve exact candidate/corpus/harness identities and both valid-report and rejection outcomes. Corrected budgets accept all 64 KiB corpus cases and four 1 MiB cases. Remaining report-budget rejection is explicit and does not claim complete analysis. Measurements are local single samples, not universal latency/RSS guarantees.
- Consumer alignment is documented in [v2-consumer-alignment.md](../specs/v2-consumer-alignment.md). In particular, the backend's selected-span digest cannot be copied into a raw-text-hash field. The eventual F0 schema and viewer spike are separate deliverables; no component has been selected or UI implemented here.

## Closure gates

1. All candidate implementation, acceptance and corrective PRs receive review, pass their applicable CI and merge in dependency order. Confirm exact reviewed commits and unresolved comments; a skipped bot review is not approval.
2. Retain schema 1.0 as the default. A future default-version switch is a separate release decision; completing opt-in support does not authorize it.
3. Confirm native v2 platform evidence and disposition of measured resource limits. Preserve documented limits rather than claiming complete support for all 8 MiB sources.
4. Update #17 and Epic #21 G1 only with accepted evidence. Keep #7, concrete detector adoption, profiles, reveal, frontend design/component work and remediation gates open where their scopes remain incomplete.

This document is an acceptance map. It does not replace any missing implementation, test, third-party review or user merge decision.
