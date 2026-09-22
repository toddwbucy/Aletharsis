# G6 checklist validation evidence

Scope: checklist/schema, two current C2PA gap records, offline validation only.
No component adoption, download, live detector or cross-repository change.

Recorded 2026-09-21, Linux/amd64, Python 3.12.13 in the existing test environment.
The initial implementation/test interval was 04:25:40–04:29:18 UTC (218 seconds);
subsequent PR preparation/review polling is outside that measured interval.
No claim of total engineering-time precision is made. Work remains within #45's
8-hour design ceiling; stop and revise the budget before exceeding it.

Commands from repository root:

```bash
python -m pytest -q tests/test_adoption_records.py tests/test_reuse_registry.py tests/test_carrier_feasibility.py tests/test_verifier_feasibility.py
/usr/bin/time -f 'wall=%e cpu_user=%U cpu_system=%S max_rss_kib=%M' python -m pytest -q
git diff --check
```

Historical measurement for the initial pre-main-merge implementation retained in
commit `9707176` (not the current PR head): 74 focused tests passed; 283 complete
offline tests passed. Full-suite time for that implementation:
2.28 seconds wall, 2.23 seconds user CPU, 0.06 seconds system CPU, maximum resident
set 91,868 KiB. These are observed test costs, not enforced process ceilings or
worst-case guarantees. Retained adoption directory/schema/test data occupied
48 KiB before this note; total new retained evidence remains below 1 MiB, within
the 1 GiB design ceiling. No live-detector compute was incurred.

The full suite includes owning feasibility-manifest validators; this complements
G6's exact digest checks on the referenced study documents and manifests. No Go
runtime code changed. CI independently runs the normal backend/platform checks.
Reviewer acceptance remains pending and is not inferred from these results.

## Review remediation validation

Re-run on 2026-09-21 against the review-remediation tree based on `07289e6`,
using the same commands and Python 3.12.13 Linux/amd64 environment: 87 focused
tests and 366 full offline tests passed. Full-suite process measurements were
2.98 seconds wall, 3.20 seconds user CPU, 0.07 seconds system CPU and 91,756 KiB
maximum resident set. These measurements supersede the historical counts above
for this revision, without rewriting the original observation.

Coverage now includes new registered components, accepted approve/revise/reject
transitions, pending recommendations without authority, missing registry decisions,
non-file evidence paths and supported versus unsupported scope exclusions.
Both retained studies remain revise recommendations with no owner acceptance.

## Second review remediation validation

Re-run on 2026-09-21 against the second remediation tree based on `1b0c15c`,
using the commands and Python 3.12.13 Linux/amd64 environment above: 114 focused
tests and 393 full offline tests passed (58 adoption checks). Full-suite process
measurements: 3.09 seconds wall, 3.33 seconds user CPU, 0.08 seconds system CPU,
and 92,272 KiB maximum resident set. Earlier measurements remain historical.

New checks exercise the ten-waiver scenario and each mandatory approval gate,
matching acceptance/registry URL forms, schema-only traversal rejection, short
justifications, and matching-registry self-review attempts. No component was
approved, no retained experiment bytes changed, and no live detector ran.

## Third review remediation validation

Re-run on 2026-09-21 against the remediation tree based on `128a7e3`, using
Python 3.12.13: 432 full offline tests passed, including 97 adoption checks.
New regressions cover retained historical approval with recorded withdrawal,
byte-exact identity patterns, nonblank substantive fields, bare evaluation PRs,
not-run evidence references and rejection of known automated acceptance identities.
No experiment bytes changed and neither component received adoption approval.

Fourth remediation pass (2026-09-21, based on `7f78a5d`): 440 offline tests passed,
including 105 adoption checks. Accepted rejection now requires matching registry
status/decision/reference; stale scope waivers are rejected after status changes.
Wording distinguishes pending automated review from accepted review and distinct
withdrawal references from verified chronology. Neither component is approved.

## Fifth review remediation validation

Re-run on 2026-09-21 against the remediation tree based on `d19b4d1`, using
Python 3.12.13 and the pinned CI requirements (including `rfc3339-validator`):
456 full offline tests passed, including 121 adoption checks; the four focused
files listed above passed 177 tests. Full-suite process measurements: 3.07 seconds
wall, 3.01 seconds user CPU, 0.04 seconds system CPU, 94,616 KiB maximum RSS.

New regressions start with both shipped pending records and exercise explicit
human-reviewer handoff, complete approval/rejection decisions, duplicate registry
IDs before indexing, exact URL termination, and path-keyed evidence citations
that survive insertion/reordering and reject missing or positional references.
CI checks citation presence, membership and hashes; human review still evaluates
whether a cited document supports an observation or a declared unrun gap.
The six retained evidence identities, study bytes, check statuses and pending
acceptance are unchanged. No dependency is approved and no runtime code changed.

## Sixth review remediation validation

Re-run on 2026-09-21 against the remediation tree based on `64ddbdd`, using
Python 3.12.13 and pinned CI requirements: 492 full offline tests passed, including
157 adoption checks; the four focused files above passed 213 tests. Full-suite
measurements: 3.09 seconds wall, 3.05 seconds user CPU, 0.05 seconds system CPU,
93,816 KiB maximum RSS. `git diff --check` passed.

New checks reject known automated registry decision reviewers, pending approval
recommendations after a registry rejection/retirement/deferral, schema-only
acceptance/recommendation mismatches, ambiguous legacy evidence lists, duplicate
raw JSON keys, gate URLs with trailing newlines, and whitespace-padded scope
justifications. Historical accepted approvals with recorded withdrawal still pass.
The draft evidence inventory is now an object keyed by retained path; its values
preserve all six existing digest/purpose pairs. Citations, check statuses/details,
recommendations and null acceptance are unchanged. The loader's duplicate-key
rejection happens before schema validation; schema validation alone cannot recover
keys already discarded by a permissive JSON parser. No runtime or experiment
artifact changed and no component was approved.
