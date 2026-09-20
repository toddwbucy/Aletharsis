# ADR-0002: Reuse detectors; own the evidence

| Attribute | Value |
| --- | --- |
| Status | Proposed for G0 acceptance through independent PR review |
| Scope | [Epic #21](https://github.com/toddwbucy/Aletharsis/issues/21), [G0 #35](https://github.com/toddwbucy/Aletharsis/issues/35) |
| Prerequisite | Accepted native evidence contracts under #17; adapter-specific G1 remains open |
| Artifacts | [Registry contract and inventory](../reuse/README.md), [work breakdown](../reuse/plan.md) |

## Decision

Aletharsis owns acquisition, immutable source identity, representations and location
maps, evidence, coverage, profile context, interpretation, review state and apply
authority. An upstream library contributes a bounded capability behind an adapter.
It cannot define our wire schema, reopen source paths, authorize edits, or turn an
unavailable check into a negative result. Its reported assertions remain distinct
from Aletharsis interpretation and human judgment.

Classify each separately evaluated component/capability using exactly one role:

| Role | Meaning | Required boundary |
| --- | --- | --- |
| `direct_dependency` | Linked implementation behind a narrow internal API | Pinned build/transitives, format/scope and resource evidence, offline operation, native platform tests |
| `external_adapter` | Optional isolated process/service | Versioned bounded protocol, explicit provisioning/consent, immutable acquired input, lifecycle/child cleanup, missing-runtime behavior |
| `reference_oracle` | Development-only comparator or fixture reference | Independent expectations, pinned environment and licensed vectors; upstream output is not ground truth |
| `research_prior_art` | Methodology, active probing or adversarial reference | Preregistered scope and rights review; no advertised production attribution or automatic code reuse |

Separate role from adoption status: `proposed`, `evaluating`, `approved`, `rejected`,
`retired`, `deferred`. Proposed is not approved. Evaluation authorizes only the
bounded issue scope. Approving extraction does not approve verification, signing,
cleanup or another language binding. Split registry records when scopes or roles
differ; changing role requires a new reviewed decision.

An approval identifies exact pin, role, formats/platforms, supported API, owner,
reviewer, evidence and limitations. Rejection, retirement and explicit deferral
also require a linked decision and reason. Re-evaluation starts a new reviewed
revision; preserve previous records in Git and never overwrite historical evidence.
A reviewed PR decision is authoritative; setting a JSON field is not authorization.
No candidate in this G0 inventory is approved.

## Gates and review authority

G0 accepts this policy, registry schema and work decomposition. Bounded G2 spikes
can then inform remaining G1 contracts. Production adapters require accepted G1,
a capability-specific adoption decision, and G6 release evidence. G3 comparator
work has its own provenance/conformance gate. G4 runtime integration and G5
cross-repository research cannot bypass those boundaries.

Codex implements the scoped Aletharsis PRs; CodeRabbit (`coderabbitai`) provides
independent automated technical review; @toddwbucy owns maintenance, product and
gate acceptance. Automated review is not legal clearance or cryptographic
certification. Where specialist expertise is needed, or review is unavailable,
@toddwbucy names an appropriate replacement before accepting that gate. Upstream
maintainers are not assigned work by this policy. Cross-repository implementation
requires linked local issues, named producer/consumer reviewers and owner approval;
this ADR authorizes no WeaverTools changes or new repositories.

Every gate records scope, implementer/reviewer, exact revisions, reproducible
commands, artifact hashes, measured results, residual limits, budget use and a
reviewed go/revise/reject or explicit defer disposition. A green check or issue
creation alone is insufficient. G0 acceptance does not close #21 or #7.

## Supply chain and evidence

Before execution, pin source plus environment and review applicable code,
transitive, fixture, model-weight and dataset licenses. Before redistribution,
record reviewed obligations and notices for the actual shipped surface. Root
license identification alone is insufficient. Do not copy copyleft implementation
code into the core without explicit rights review; out-of-process use is not a
blanket exemption from obligations. Unknown rights stop that workstream.

Registry pins use immutable Git commit and tree object identities. Git SHA-1 tree
identity is not a SHA-256 archive digest or signature. The optional artifact digest
is SHA-256 over exact downloaded package/archive bytes; it must be filled before
that artifact is used and retained with the download/provisioning record. A release
tag is descriptive, never the sole identity. Each retained license uses SHA-256 of
its exact original bytes. No line-ending normalization of these evidence copies.

Fixtures record origin, applicable rights, artifact kind, byte serialization,
exact bytes/digest, generator revision and all transformations between raw,
rendered and post-processed stages. Expected observations must be independently
reasoned, not copied from majority detector votes. This applies equally to upstream,
Weaver-produced and independently produced corpora. No live secrets/private
samples in default tests. Upstream text and outputs are untrusted review data.

Evaluate maintenance and release cadence, security-reporting route, known
advisories, reproducible build/install, disable/replacement and update cost. A
README claim, security file, recent push, release tag or reference-implementation
label does not establish correctness or support commitments. The initial
[desk assessment](../reuse/assessment.md) states what was checked and what was not.

## Execution, budgets and lifecycle

Each evaluation has time, compute, storage and process limits before execution.
Stop at a ceiling, missing prerequisite, unresolved rights or uncontrolled side
effect; publish partial evidence and request a reviewed scope/budget revision.
Do not convert failure into a passed gate. Network provisioning is explicit,
separate from audits; no ambient telemetry, preferences, model downloads or
unapproved remote submission. An adapter receives acquired bytes or a verified
controlled snapshot, never authority over originals. Raw response retention,
secret exclusion, deadlines, process-tree cleanup and output/memory limits belong
in G1 and must be tested with malformed and hostile inputs before adoption.

For each adoption G6 requires contract and real-adapter tests, independent vectors,
source integrity, bounded fuzz/adversarial tests, resource measurements and native
checks on each supported platform. Publish omissions, notices, reproduction steps
and missing-dependency behavior. Exercise disable/rollback before release; preserve
raw evidence and report unavailability when a component is disabled. Never replace
an old report with a newly interpreted result in place.

The maintainer tracks advisories and reviews pinned upgrades before merging them.
A suspected vulnerability pauses affected adapter use pending triage; sensitive
reports use the upstream's verified private reporting route where available.
Upgrades require a pin/license/environment diff, result-semantic comparison on the
accepted corpus, relevant regression/native checks and a new decision. No floating
runtime updates, automatic trust-list updates or silent result-semantic changes.

## Alternatives and consequences

Reimplementing standards ourselves adds interoperability risk. Shelling out to
unrelated tools and concatenating their output loses identity, scope and semantics.
Letting a universal plugin object define our report schema makes upstream changes
our public contract. We instead accept the cost of narrow adapters and independent
conformance evidence. Optional dependencies can be declined without losing native
structural auditing. An inconclusive experiment is a valid result, not a reason to
advertise unvalidated capability.
