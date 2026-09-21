# G6 component adoption and lifecycle gate

Tracking: [#45](https://github.com/toddwbucy/Aletharsis/issues/45), parent
[#21](https://github.com/toddwbucy/Aletharsis/issues/21), validation history
[#7](https://github.com/toddwbucy/Aletharsis/issues/7). This is a proposed reusable
gate and two **revise** gap records. It approves no dependency or release.
A closed validation tracker does not establish that a new component passed its
checks; link a scoped follow-up issue for new work rather than inheriting closure.

## Record and acceptance procedure

Create one JSON record per scoped registry component, conforming to
[adoption record 1.0](../../../schemas/adoption-record-v1.schema.json). The two
records in this directory demonstrate application to actual retained feasibility
evidence, not fictional production readiness. Do not copy their results to a new
component. Start new checks as `not_run` with explicit required evidence.

Bind the record to the exact registry commit, source archive and license digests,
requested scope, accountable owner, implementer, technical reviewer, evaluation
PR and tracking issues. Add exact-byte SHA-256 references to retained commands,
environment, results and artifact manifests. Evidence indices are local to this
record. A repository name, CI badge, study pass count or artifact URL alone is not
adoption evidence. Nested manifests must be verified by their owning experiment
checks; hashing a manifest alone does not validate the files it describes.

Every checklist field is required. Use `passed`, `partial`, `failed`, `not_run` or
`not_applicable`; the detail states what ran, what it proves and what it does not.
Passed/partial/failed entries require retained evidence. A not-applicable entry
requires an explicit scope-based justification in detail, reviewed by the owner;
it is not an escape hatch for missing platforms or an unavailable test system.
An `approve` recommendation requires all checks passed or justified not-applicable.
It is only a recommendation until `acceptance` names the owner, technical reviewer,
review record and matching disposition. CodeRabbit cannot grant owner acceptance.
Specialist review must be assigned where automated review lacks domain expertise.

Acceptance must inspect evidence, not merely validate JSON. Schema checks cannot
prove the truth of a pass assertion or that a linked reviewer accepted it. On
approval, update the existing registry decision/status and accepted production
scope in the same reviewed change. A dev-only oracle can be approved with empty
production scope, but its exact developer execution/redistribution scope still
needs acceptance. Rejection or revision does not silently close other gate issues.

## Required checklist

| Check | Evidence required before scoped adoption |
| --- | --- |
| `prerequisites` | Accepted G0, component G1 and applicable G2/G3/G4/G5; exact contract/schema versions, linked producer/consumer ownership and acceptance; no gate passed by implication |
| `licenses_notices` | Root and transitive component, fixture, dataset, model and redistribution rights; retained license/notice digests, per-file exceptions and reviewed packaging notices; unresolved rights stop adoption |
| `unit_contract` | Native unit and normalization tests, boundary coordinates, strict wire/import fixtures and reproducible commands; upstream suite alone is insufficient |
| `live_adapter` | Real pinned executable/library exchange under the accepted contract, absent/malformed/partial/timeout/canceled/failed results and preserved raw vendor semantics; fixture-only tests do not satisfy this |
| `source_integrity` | Exact input hashes and timestamps before/after, acquisition identity, raw/derived maps and exclusions; forged/stale source and report identities fail closed; no edit authority from detector output |
| `adversarial_fuzz` | Untrusted content, malformed lengths/frames, path/link tricks, prompt/instruction carriers, hostile output, crash/fuzz corpus and findings; licensed independent expected cases |
| `resources` | Reviewed execution and CI budget before running; wall/CPU/memory/output/storage/process limits, measurements, termination/reaping and no partial promotion after failure |
| `native_platforms` | Enumerated supported OS/architecture/library/toolchain targets, actual native executions and platform-specific omissions; cross-builds are not executions; unsupported targets report unavailable |
| `build_install` | Pinned offline inputs/transitives, reproducible build comparison, artifact hashes, installation and relocation on every accepted target; deliberate provisioning only, no audit-time downloads |
| `disable_rollback_missing` | Run audit with adapter disabled, removed, incompatible and rolled back; native auditing remains useful, coverage is unavailable rather than negative, historical reports remain readable; test worker cleanup and uninstall residue |
| `security_updates` | Named maintainer, advisory/transitive review with date and scope, controlled trust/telemetry/network defaults, vulnerability triage and response/retirement policy; unresolved vulnerabilities have an explicit disposition |
| `semantic_upgrade` | Versioned baseline and documented update procedure; new pins compare corpus findings, coverage, coordinates, raw semantics, scores, trust, resources and packaging; reviewed explanation of each change, rollback target and acceptance |

Initial adoption has no previous accepted version. Its semantic-upgrade evidence
must establish a retained baseline, comparison command and rollback/disable drill;
this is not automatically not-applicable. Actual upgrades rerun that comparison.

## Lifecycle

1. Propose a narrowly scoped component and accepted platforms. Link its separate
   reviewed provisioning/test/CI/resource budget **before** live work. G6's
   8-hour/1-CPU-hour/1-GiB design budget does not authorize a detector build,
   download, model run, private sample or network submission.
2. Collect exact evidence and run the checklist. At any ceiling, uncontrolled side
   effect, missing prerequisite or unresolved rights issue: stop, retain partial
   evidence and recommend revise/reject. Amend budgets through review, not silently.
3. Independent reviewer assesses technical sufficiency; the accountable owner
   records approve/revise/reject for that pin and scope. Update registry and epic;
   keep unrelated gates and unfinished requirements open.
4. For an upgrade, retain the previous record in version control, create a proposed
   replacement bound to the new pin and license/dependency/artifact digests, run
   semantic comparison and lifecycle drills, and obtain fresh acceptance. No
   floating version or registry refresh may silently change detector semantics.
5. For vulnerability or lost maintenance: disable affected capability, retain
   evidence and report unavailable, investigate with the recorded owner, then
   review a pinned fix, replacement or retirement. Revocation of approval is a
   recorded registry decision; it does not erase historical findings or credentials.

## Current applications

- [c2pa-text](encypherai--c2pa-text.json): merged #47 study informs a revise
  recommendation. HTML host-context mismatches and missing mappings remain; no
  production adapter, complete license/security clearance or native release gate.
- [encypher-c2pa](encypherai--encypher-c2pa.json): merged #48 study informs a revise
  recommendation. Bounded worker, packaging, independent verification and release
  checks remain. Matching local build hashes and eleven cases are scoped evidence.

Both remain `evaluating` in the registry and `acceptance: null` here. These records
apply the checklist now without manufacturing a production decision.

## Reproduction

```bash
python -m pytest -q tests/test_adoption_records.py tests/test_reuse_registry.py tests/test_carrier_feasibility.py tests/test_verifier_feasibility.py
```

Existing offline CI runs these tests. They verify closed record shapes, complete
checklists, pins/ownership against the registry, retained evidence digests, bounded
references and negative mutations. They neither contact upstream nor install/run
any detector. Study-specific tests validate the retained manifests and results.
