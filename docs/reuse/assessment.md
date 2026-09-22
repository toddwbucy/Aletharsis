# Initial maintenance and feasibility desk assessment

Observed 2026-09-20. The [registry](candidates.json) records exact source commit/tree
identities, pinned README/license links, retained license byte hashes and
repository/release observations. This is source-document inspection, not an
executed build, vulnerability audit or compatibility finding. No candidate is
approved. Existing native functionality remains the fallback.

Method: GitHub repository metadata, `commits/HEAD`, then `license?ref=<commit>`,
`readme?ref=<commit>`, `git/trees/<commit>?recursive=1`, and `releases/latest`.
License contents were base64-decoded without rewriting line endings and hashed.
Tree listings were not truncated. README and license text were inspected as data;
no install/build scripts were run. The checked-in facts and exact license bytes
remain available offline; upstream historical links support independent review.
The queries pin source inspection even if the default branch subsequently moves.

| Candidate | Maintenance/release and security observations | Decision-relevant gap / next evaluation |
| --- | --- | --- |
| c2pa-text | README documents Go submodule; SECURITY.md and CI workflow present; latest-release endpoint 404 | Package tags may still exist. Verify Go closure, all claimed carriers, normative vectors, malformed limits and coordinates in #37 |
| encypher-c2pa | GitHub release v1.0.5 observed; Rust/Go surface, SECURITY.md and CI/release workflows | README documents interactive telemetry opt-in/preferences. Explicitly disable and test conflicting settings; measure ABI/build/native packaging in #38 |
| hidden-characters-detector | Pinned README declares deprecation and links juriku/untrace; CI present, no SECURITY.md found, latest-release 404 | Reference only. Verify successor separately; no assumption that Python behavior is current or correct (#39) |
| HIBERIUS | Security policy and CI/CodeQL paths; standalone browser tool claim; latest-release 404 | Claimed offline isolation and legitimate-use semantics need executed comparison; exclude sanitize/generate operations (#39) |
| invisible-character-detector | Browser deploy workflow; MIT root license read; no SECURITY.md found, latest-release 404 | Build closure, Unicode version and reveal semantics not established; compare independent cases (#39) |
| oletools | v0.60.2 GitHub release; SECURITY.md and test workflow | Root now identified as BSD-2-Clause AND MIT, excluding thirdparty/dependencies; [Office static assessment](evaluations/office.md) retains a candidate module inventory and separate notices. Runtime closure, vector rights and clearance remain pending (#40) |
| PDFScalpel | Python build metadata; no SECURITY.md found, latest-release 404 | No reproduced build; broad forensic/repair/generation claims do not prove supported inspection. Bound runtime and exclude mutations (#41) |
| MarkLLM | Python requirements; no SECURITY.md found, latest-release 404 | Per-algorithm environment, tokenizer/model rights, keys and downloads unresolved; optional protocol first (#42) |
| LLMmap | Python requirements; no SECURITY.md found, latest-release 404 | README describes remote-code-enabled model loading; inspect/isolate before execution. Active probing does not validate passive attribution (#44) |
| llm-fingerprint | v0.4.0 GitHub release; CI/publish workflows | README separates MIT code and CC-BY research data; dataset version/attribution conditions unresolved, no SECURITY.md found. Preregister comparison (#44) |
| StegZero | AGPL v3 license text; no SECURITY.md found, latest-release 404 | Exact grant and obligations need rights review before reuse; prior art/external fixture candidate only, not core code (#39) |

The SPDX field identifies root text only. StegZero uses `NOASSERTION` because the
observed AGPL v3 text/README alone is not a resolved project-wide only/or-later
expression. The [oletools candidate inventory](evaluations/office/static-inventory.json)
now records selected modules and notices; it does not clear the full distribution
or establish runtime closure. MIT/Apache labels
do not clear model weights, datasets, vectors or transitives. Retained license
copies are evidence, not a legal opinion or redistribution approval.

For all candidates, advisory history, response timeliness, signed release/build
provenance and two-build reproducibility remain **unverified**. The cited evaluation
must establish applicable properties or recommend revise/reject. Source workflows
are only leads for reproduction. No upstream promise of long-term maintenance is
assumed. Each record has a disable/replacement plan; native auditing and historical
reports do not depend on any candidate. These unresolved checks block adoption,
not acceptance of the G0 planning inventory.
