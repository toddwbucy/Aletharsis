# G6 component adoption and lifecycle gate

Tracking: [#45](https://github.com/toddwbucy/Aletharsis/issues/45), parent
[#21](https://github.com/toddwbucy/Aletharsis/issues/21), validation history
[#7](https://github.com/toddwbucy/Aletharsis/issues/7). This is a proposed reusable
gate and two **revise** gap records. It approves no dependency or release.
A closed validation tracker does not establish that a new component passed its
checks; link a scoped follow-up issue for new work rather than inheriting closure.

## Record and acceptance procedure

Create one `<component-id>.json` record per scoped registry component, conforming to
[adoption record 1.0](../../../schemas/adoption-record-v1.schema.json). The two
records in this directory demonstrate application to actual retained feasibility
evidence, not fictional production readiness. Do not copy their results to a new
component. This directory reserves `.json` files for records; keep indexes or
other JSON sidecars elsewhere. Registry-approved components must have a record.
Start new checks as `not_run` with explicit required evidence.

Bind the record to the exact registry commit, source archive and license digests,
requested scope, accountable owner, implementer, technical reviewer, evaluation
PR and tracking issues. Add exact-byte SHA-256 references to retained commands,
environment, results and artifact manifests. The `evidence` inventory is an object
keyed by retained path; each value contains its `sha256` and `purpose`. Checks cite
those keys directly. This gives each path one digest and makes inventory ordering
irrelevant. Consumers must reject duplicate JSON object keys before schema
validation (the repository loader does so), rather than accepting a parser's
first/last-wins interpretation. The schema rejects the former array representation.
A repository name, CI badge, study pass count or artifact URL alone is not adoption
evidence. Nested manifests must be verified by their owning experiment checks;
hashing a manifest alone does not validate the files it describes.

Every checklist field is required. Use `passed`, `partial`, `failed`, `not_run` or
`not_applicable`; the detail states what ran, what it proves and what it does not.
Every status, including `not_run`, requires at least one retained citation. CI
checks path membership and byte digests, not whether the document supports the
claim. For `not_run`, citations identify the study/context used to declare the
gap; they are not execution evidence. Human review must assess whether that basis
supports the stated gap. Referencing a feasibility study does not turn an unrun
adoption gate into an executed one. A
not-applicable entry also requires a nonblank `scope_justification`, reviewed by
the owner against that evidence;
it is not an escape hatch for missing platforms or an unavailable test system.
For approval, every check except `live_adapter` must pass, including native
platform execution, security review, disable/rollback and an initial semantic
baseline. `live_adapter` alone may be not-applicable for an explicitly scoped
component that ships no adapter (for example a development-only fixture oracle).
Every scope justification must contain at least 20 non-whitespace characters;
that is a placeholder guard, not proof of a sound justification.
A pending `approve` recommendation with null acceptance is valid and grants no
adoption authority; the registry cannot be approved in that state.
It is only a recommendation until `acceptance` names the owner, technical reviewer,
review record and matching disposition. The schema itself requires any acceptance
disposition to equal the recommendation, so an accepted approval cannot bypass
the approval checklist by retaining a `revise` recommendation.
`acceptance.record` must be a bare
Aletharsis PR URL, matching the registry decision contract; issue URLs and comment
anchors can be retained in tracking/evidence but cannot replace that canonical URL.
Known `coderabbitai` and `*[bot]` identifiers are rejected as acceptance owners
or accepted technical reviewers, and as registry decision reviewers for accepted
approval, rejection or withdrawal. Pending records may name automated reviewers
that supplied evaluation feedback. Before recording acceptance in any disposition,
assign an independent human technical reviewer and update `technical_reviewer` in
both the adoption record and registry in the same reviewed change. The acceptance
must name that reviewer. Preserve the original automated feedback in its linked
evaluation review; reassignment does not erase that history or prove a human has
accepted it. This is a limited guard, not identity authentication.
The technical reviewer identifier must differ from the implementer (ignoring
case and surrounding whitespace). An owner may implement work but cannot thereby
self-review it. Specialist review must be assigned where automated review lacks
domain expertise; schema validation cannot authenticate a person, detect alternate
accounts, or establish reviewer competence.

Acceptance may be recorded in the evaluation PR if that same reviewed change
contains explicit owner acceptance and independent technical review. A separate PR
URL is not required and would not establish independence by itself. Pending records
naming an automated reviewer are not evidence of completed specialist acceptance.

Acceptance must inspect evidence, not merely validate JSON. Schema checks cannot
prove the truth of a pass assertion or that a linked reviewer accepted it. For accepted approval or rejection, record a complete registry decision (reviewer,
scope, gates, reason, disposition and canonical PR URL) and matching status. On
approval, update the accepted production
scope in the same reviewed change. A dev-only oracle can be approved with empty
production scope, but its exact developer execution/redistribution scope still
needs acceptance. Rejection or revision does not silently close other gate issues.
A previously accepted record remains historical when the registry becomes
`retired`, `deferred` or `rejected`. The registry must carry a complete matching
`retire`, `defer` or `reject` decision with a distinct later PR reference; the
original acceptance is retained and grants no current authority. A rejected,
retired or deferred component cannot retain a pending `approve` recommendation
with null acceptance; only a retained historical approval may accompany its
recorded withdrawal. CI checks the
recorded transition, not the chronology or authorization of the linked review.

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

An accepted rejection requires a matching rejected registry status and rejection
decision/reference; accepted revision can remain evaluating. `scope_justification`
is allowed only while a check is `not_applicable`; remove it when that status changes.
