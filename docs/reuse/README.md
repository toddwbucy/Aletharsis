# Detector reuse registry

This is the inert planning registry for [Epic #21](https://github.com/toddwbucy/Aletharsis/issues/21).
It is **not** a runtime plugin catalog, dependency installer, trust store or approval
to execute tools. See [ADR-0002](../adr/0002-detector-reuse-policy.md),
[assessment](assessment.md) and [bounded work plan](plan.md).

`candidates.json` conforms to [registry schema 1.0](../../schemas/reuse-registry.schema.json).
The G0 snapshot recorded eleven proposed candidates. Current states are in the
registry; no detector code or upstream vectors are shipped. See the first
[c2pa-text feasibility study](evaluations/c2pa-text.md) for an evaluating candidate.
The existing [JSON-schema utility dependency](../dependencies/README.md) has a
separate adopted dependency record; this detector inventory neither reapproves nor
replaces that record or `go.sum`.

## Contract

All objects are closed and all listed fields required. Unknown keys and unknown
role/status values fail validation. Null means explicitly unavailable/not yet
obtained, never an empty successful verification. Stable `id` identifies a scoped
component; IDs must be unique. Repository names do not define supported capability.

| Fields | Meaning |
| --- | --- |
| `repository`, `component`, `integration_class` | Origin, separately evaluated surface/purpose and one of four ADR roles |
| `adoption_status`, `decision` | Separate disposition; approve/reject/retire/defer requires an Aletharsis PR record, reviewer, exact scope, gate links and reason |
| `evaluation_issue`, ownership fields | Scoped work, accountable maintainer, implementer, technical reviewer and acceptance owner |
| `pin` | Immutable commit and Git tree SHA-1; optional descriptive tag and exact archive/package SHA-256, explicitly null until acquired |
| `license` | Root SPDX identification (or `NOASSERTION`), pinned source URL/path, retained exact-byte copy/digest, applicable scope and separate clearance state |
| `redistribution` | Whether executable upstream code or fixtures are actually redistributed; license-only inventory is neither |
| `runtime` | Verification state, transitive/runtime dependencies, network policy and known or suspected side effects |
| `supported_scope` | Proposed purpose versus explicitly accepted production scope; proposed/evaluating records cannot advertise production support |
| `vectors` | Not imported versus reviewed provenance, artifact kind/serialization/digest, generator and transformation lineage, independent expected evidence |
| `assessment` | Dated desk observations, pinned README, archive state, push/release observations, policy/build paths, reproducibility/security-review state and limits |
| `disable_or_replace` | How to remove the component without losing native auditing or historical evidence |

`approved` requires reviewed license and vulnerability state, verified runtime and
build evidence, plus a matching approval decision. This structural check cannot
establish that a review occurred; reviewers must inspect linked evidence. Production
scope may remain empty for an approved dev-only oracle/research component. No
source/vector redistribution while proposed/evaluating; isolated evaluation data
are not automatically accepted for redistribution in this repository.

Redistributed code or fixtures require reviewed license clearance in every status;
redistributed fixtures also require reviewed vector provenance. Calendar dates and
timestamps use JSON Schema formats, enforced by the test validator with FormatChecker.

A missing GitHub latest release is recorded as `github_latest_404`, not proof that
no tags, package releases or support exist. A policy-path inventory does not prove
security response effectiveness. `last_push` is mutable repository metadata at
observation time, not a commit timestamp. `release_tag` need not refer to the
selected commit; do not substitute it for the pin. The source tree listing was
checked for truncation. No upstream source was executed during G0.

## Validation and updates

Run the existing test-only Python environment:

```bash
python -m pytest -q tests/test_reuse_registry.py
```

The normal backend CI schema/reference step includes these tests. They check the
schema, invalid status/approval mutations, exact license hashes, pinned URL
identity, unique IDs, closure and complete initial inventory. No network is used.
The retained license files preserve upstream trailing spaces/blank lines; their
scoped Git whitespace attribute exempts only those exact-byte evidence copies.
The Python test runner is not a production runtime dependency.

To update: use a PR to record the new exact revision and license bytes/hash,
assessment date, changes in dependencies/data rights and side effects; link the
bounded issue and gate evidence. Re-run validation and relevant adoption-specific
conformance. Do not use inventory refreshes to silently approve an upstream update.

## Adapter contract review

[DA-001](../specs/detector-adapter-v1.md), accepted in PR #49, defines the bounded local exchange,
source/derived identities, raw-result retention, trust dimensions and worker
lifecycle under #36, informed by merged studies #47/#48. Its schema and offline
fixtures are design conformance tools. Production report schema 2.0 and dependency
approval remain unchanged; the report migration and G6 enforcement are separate
review gates.

The next #36 review unit is [EC-003 report migration design](../specs/report-v3-migration.md).
Its planning vectors do not replace the separately gated full wire fixtures and
Go marshal/import parity required before adapter implementation.

## Per-component adoption

The [G6 checklist and gap records](adoptions/README.md) bind each recommendation to
its exact pin, evidence, missing checks and owner acceptance. The first two C2PA
records recommend revision; neither approves a production dependency.
