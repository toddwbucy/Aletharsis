# G0 work breakdown and gate order

Parent: [Epic #21](https://github.com/toddwbucy/Aletharsis/issues/21).
Policy/registry acceptance: [#35](https://github.com/toddwbucy/Aletharsis/issues/35).
This plan creates no upstream maintenance commitments and approves no detector.

For each issue: Codex is the Aletharsis implementer, CodeRabbit (`coderabbitai`)
the independent automated technical reviewer, and @toddwbucy the accountable
maintenance/product acceptance owner. Domain/legal review must be named before
acceptance when required; automation is not a substitute. These are workstream
roles, not a claim that review has already happened. Changes outside Aletharsis
require that repository's linked issue and producer/consumer review assignment.

Budgets below cap the first increment. G0 reserves up to three existing CI validation runs within
its compute ceiling; runner temporary build space is governed by the existing CI
jobs, while the 250 MiB G0 ceiling covers retained registry/evaluation evidence.
Each linked issue also specifies process
memory/time/output limits where execution is in scope, stopping rules, artifact
requirements and exclusions. Hours mean engineering effort, CPU hours aggregate
compute and GiB local disk ceiling. No GPU/API spending, live model calls or
private samples are authorized by G0. Amend a budget through review before
exceeding it. No automatic continuation into the next gate after a spike.

| Work | Dependency | Initial budget | Required output |
| --- | --- | --- | --- |
| [#35 G0 policy/registry](https://github.com/toddwbucy/Aletharsis/issues/35) | Native #17 delivered | 8 h / 2 CPU h / 250 MiB retained evidence | Reviewed ADR, validated registry, desk assessment, child issues and roadmap |
| [#36 G1 adapter contracts](https://github.com/toddwbucy/Aletharsis/issues/36) | G0; informed by G2 | 12 h / 1 CPU h / 1 GiB | Input/binding maps, bounded raw responses, trust snapshots, isolation/lifecycle and conformance fixtures |
| [#37 G2 carrier](https://github.com/toddwbucy/Aletharsis/issues/37) | G0; adoption also needs G1/G6 | 12 h / 2 CPU h / 2 GiB | Three claimed text carriers, normative/independent vectors, malformed limits, exact coordinates and adoption recommendation |
| [#38 G2 verifier](https://github.com/toddwbucy/Aletharsis/issues/38) | G0; adoption also needs G1/G6 | 16 h / 4 CPU h / 8 GiB | Byte API, ABI/platform/size/resources, telemetry/preference isolation, signature/binding/trust and placement recommendation |
| [#39 G3 Unicode](https://github.com/toddwbucy/Aletharsis/issues/39) | G0; coordinate #14/#15 | 16 h / 2 CPU h / 2 GiB | At least two independent comparators, independently expected licensed vectors and adjudicated discrepancies |
| [#40 G3 Office](https://github.com/toddwbucy/Aletharsis/issues/40) | G0 + Office parser contract and executable | 12 h / 2 CPU h / 2 GiB | Scoped DOCX parts/relationships/objects; legacy OLE not implicitly included |
| [#41 G3 PDF](https://github.com/toddwbucy/Aletharsis/issues/41) | G0 + PDF parser contract and executable | 12 h / 2 CPU h / 4 GiB | Born-digital object/stream/revision comparison; unsupported OCR/visibility explicit |
| [#42 G4 sidecar](https://github.com/toddwbucy/Aletharsis/issues/42) | G0 + accepted adapter G1 | 16 h / 4 CPU h / 8 GiB | Protocol first, then one scoped algorithm; failure/lifecycle and runtime-absent evidence |
| [#43 G5 corpus contract](https://github.com/toddwbucy/Aletharsis/issues/43) | G0 + G1 identities | 12 h / 1 CPU h / 1 GiB | Design/compatibility fixtures for Weaver and independent producers; linked repository issues before implementation |
| [#44 G5 fingerprint preregistration](https://github.com/toddwbucy/Aletharsis/issues/44) | G0 + validated baseline interchange | 8 h / 1 CPU h / 1 GiB | Rights, sample/metrics/splits/leakage/open-set and stopping plan; experiment execution separately budgeted |
| [#45 G6 lifecycle](https://github.com/toddwbucy/Aletharsis/issues/45) | Per-component G1 and relevant evaluation | 8 h / 1 CPU h / 1 GiB for checklist | Per-adoption evidence/upgrade/rollback checklist; actual release testing separately budgeted and linked to #7 |

## Recommended next review units

Accept G0, then run the carrier and verifier feasibility issues independently.
Use findings to finish adapter G1 before proposing production integrations. Unicode
comparison can proceed alongside them. Office/PDF wait for their parser tracks;
statistical sidecars wait for protocol acceptance; optional fingerprint research
cannot block useful local releases. G6 is per adoption, not one final all-or-nothing
release at the end of the epic.

Use separate PRs for policy, feasibility reports, contract changes, each adapter,
comparison harness and cross-repository producer/consumer implementation. Each
accepted result updates the registry and #21 with evidence, not just a checkbox.
Stop an evaluation with a partial/revise/reject report when its ceiling is reached.
A defer disposition needs an explicit owner decision and linked follow-up.

## G0 completion evidence

- Policy: [ADR-0002](../adr/0002-detector-reuse-policy.md).
- Registry contract/inventory: [README](README.md), [schema](../../schemas/reuse-registry.schema.json), [records](candidates.json).
- Maintenance, release, vulnerability, build and replacement assessment:
  [desk assessment](assessment.md), with unresolved adoption checks explicit.
- All epic workstreams: linked bounded issues above, owners and budgets in each.
- Validation: [offline registry contract tests](../../tests/test_reuse_registry.py)
  run by normal backend CI; retained exact license hashes and negative mutations.
- Acceptance: independent review and linked decision on #35. G0 remains proposed
  until accepted; this checklist itself is not approval. #21 and #7 stay open.
