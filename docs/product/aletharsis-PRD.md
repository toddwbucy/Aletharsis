# Aletharsis — Forensic Document Signal Auditor

| Attribute | Value |
| --- | --- |
| Status | Product architecture approved; review clarifications incorporated; further detail belongs in specifications |
| Document version | 0.1 |
| Date | 2026-09-19 |
| Product owner | Todd W. Bucy |
| Implementation direction | Go backend/CLI; lightweight local web review interface |
| Deployment | Local-first, offline-capable; remote analysis opt-in only |
| Core scope | Detection, evidence preservation, classification, human review and reporting of hidden, unusual, provenance-bearing and covert document signals |
| Modification policy | Originals are never modified; any future remediation produces derivatives only |
| Implementation baseline | Go CLI 0.2.0, report schema 1.0, main commit `20d5bdc`; see section 3 for limits |
| Child product document | [Evidence Review Workbench, draft v1.1](frontend-PRD.md); approved v1.0 retained as history |

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

## 1. Product definition and document authority

Aletharsis is a forensic instrument for exposing and characterizing information channels in documents. It detects and preserves evidence that may not be apparent from ordinary human viewing, including explicit provenance and visible artifacts that still require contextual review. Watermarks are one use case; hidden Unicode, document objects, cryptographic credentials, instruction carriers and configured statistical channels require different evidence and interpretation.

**Detection establishes what is present. Evidence review establishes what it means in context. Explicit apply determines what, if anything, should change in a new derivative.**

This document is the approved parent product definition. It governs common scope, invariants and release gates. The frontend PRD specifies the review experience within those boundaries; backend and frontend technical specifications define implementation contracts. Neither a technical specification nor an implementation PR may silently relax a product invariant.

The frontend PRD is being rewritten as draft v1.1 to align with this product definition. Its previously approved v1.0 baseline, derived from v0.4 requirements, is preserved byte-for-byte in [frontend-PRD-v1.0.md](frontend-PRD-v1.0.md). That historical document describes the former Python baseline; current implementation status is recorded here and in the revised child. The parent architecture and parent/child relationship are approved following product review. The revised child does not inherit approval from v1.0; its changed requirements remain subject to explicit review before technical specifications treat them as accepted.

## 2. Problem, users and desired outcome

Documents carry information in visible text, encodings, formatting, metadata, relationships, signed assertions and generation-time choices. Ordinary viewers can conceal these layers; editors may alter them. A reviewer needs to distinguish normal format mechanics from evidence requiring investigation without treating everything unusual as malicious.

The central question is:

> What information-bearing mechanisms were observed, where is the evidence, what examined it, what is expected in context, and what could not be assessed?

Primary users are security/forensic reviewers investigating document signals, developers reviewing code and agent instructions, provenance researchers, and organizations triaging document corpora. CLI automation and interactive review are equally important. These users need reproducible evidence, transparent coverage, independently reviewable judgments and preservation of originals.

Success is a defensible explanation, not a universal “watermark found” or “clean” verdict. Absence of structural findings says nothing about an unavailable statistical detector. A detector result does not establish intent, ownership or a specific user's identity.

## 3. Current capabilities versus target scope

The production backend is already Go; this is not a proposal to restart the port. The Python application is retained as a temporary migration oracle. Its retirement requires preserving useful reference artifacts and replacing any remaining live-oracle dependency deliberately; it is not permission to discard validation evidence.

| Capability | Current baseline | Target / release dependency |
| --- | --- | --- |
| Acquisition | One regular file, up to 8 MiB; strict Linux no-atime reader | Bounded workspace acquisition; other OS readers require native integrity validation |
| Platforms | Native Linux amd64/arm64 checks; macOS/Windows explicitly refuse acquisition | Verified readers before advertising functional non-Linux auditing |
| Text | Literal TXT/MD/RST/CSV/JSON/XML/HTML and source-like text; UTF-8 and BOM-marked UTF-16/32 | Preserve exact representations; language/format context only when supported |
| Analysis | Unicode/emoji inventories, normalization hashes, pattern candidates, identifiers, provenance labels, line endings | Mechanism/capability model, context profiles and additional validated analyzers |
| Structured formats | DOCX/PDF recognized but parsing unsupported; ODT not implemented | Incremental DOCX, ODT and PDF parsers with explicit coverage |
| Reports | Deterministic console/JSON, current categories/severities, strict schema 1.0 | Versioned capability/result reporting, batch summaries and JSONL |
| Cryptographic/statistical | No C2PA verifier or vendor statistical detector | Separate adapters; unavailable access must remain explicit |
| Reveal/review/apply | No CLI reveal/diff, workspace UI, saved rules or writer | Individually gated delivery tracks below |

Current validation includes direct unit/property tests, bounded fuzzing, compiled CLI integration, frozen-reference comparisons, resource characterization, native Linux checks, cross-builds and non-Linux refusal tests. This is not an end-to-end test of an unimplemented GUI/remediation workflow. See the [backend CI](../testing/backend-ci.md), [platform](../testing/platforms.md), [performance](../testing/performance.md), and [migration](../migration/go-backend-migration.md) records.

## 4. Product invariants

| ID | Requirement |
| --- | --- |
| INV-01 | Source bytes and the supported acquisition timestamp guarantees remain unchanged during auditing, review and derivative production. Fail closed when the required acquisition capability is unavailable; never restore timestamps by writing them. |
| INV-02 | Observations, detector interpretations, profile assessments and reviewer decisions remain distinguishable. No finding, rule or profile carries edit authority. |
| INV-03 | All supported evidence is traceable to an exact source identity and an honest location or analyzed scope. No synthetic precision where a byte mapping does not exist. |
| INV-04 | Unknown, unavailable, unsupported, failed, skipped, canceled and incomplete coverage cannot become an unqualified clean result. |
| INV-05 | Expected-artifact classification can reduce default queue noise but cannot destroy evidence or prevent independent analyzers from examining it. |
| INV-06 | Local auditing performs no telemetry, upload, external resource retrieval or remote analysis by default. Network operations require explicit configuration and scope authorization. |
| INV-07 | Inspected content, imported reports, rules, profiles and detector responses are untrusted data. They cannot execute instructions through parsing, rendering or tooling. |
| INV-08 | Deterministic local results retain tool, parser, detector, profile and configuration identities. Time-dependent trust or remote results preserve their evaluation context rather than claiming timeless reproducibility. |
| INV-09 | Reveal artifacts are presentations, review decisions are judgments, and derivatives are new assets. None replaces the original evidence or authoritative report. |

“Immutable originals” describes Aletharsis behavior and verification, not a guarantee against another process changing a live file. Detect relevant source changes, invalidate stale evidence/actions and disclose snapshot limitations; stronger assurance may require a forensic image or read-only mount.

## 5. Signal and mechanism classes

Mechanism is independent of severity, existing finding category and intent. More than one mechanism can coexist in the same asset. Statistical provenance watermarks and statistical covert channels share a mechanism family but have different detector purposes and interpretation requirements.

### 5.1 Structural and representational

Examples include invisible Unicode, bidi controls, variation selectors, tags, unusual whitespace, soft hyphens, combining/homoglyph candidates, hidden runs, comments, revisions, text boxes, invisible/off-page PDF content, metadata, custom XML, relationships, embedded objects and repeated overlays. Headers, comments, emoji and identifiers have legitimate uses and are not automatically watermarks.

Locations may be original text byte/scalar spans, package part + node/run identifiers, or page/object/rendering locations. OOXML extracted-text coordinates are not ZIP compressed-byte edit instructions. A parser must declare exact, derived, approximate or unavailable correspondence where applicable. Future writers need a verified format-specific mapping, not a guessed conversion from a displayed selection.

Structured-format technical specifications must define identities and digests for extracted package parts, streams or objects where practical, retaining their relationship to the source identity and structural locator. Evidence must remain independently identifiable across parsing stages; the specification must state which representation is identified or hashed and disclose unavailable identity guarantees.

### 5.2 Cryptographic provenance

C2PA/Content Credentials is a planned priority. Anthropic describes credentials for supported generated file types separately from its text watermark; this does not establish credentials in any particular file or promise support for every format. [Anthropic description](https://www.anthropic.com/news/claude-text-watermark).

Separate credential discovery, parsing, signature verification, asset binding, signer/chain evaluation, trust policy and unavailable checks. Presence is not successful validation. A valid signature does not prove every assertion or the content true, and absent credentials do not establish human authorship. Record assertions, ingredient relationships and validation context. Local validation must disclose unavailable trust/revocation information; remote manifests or trust services cannot be fetched implicitly. The [C2PA specification](https://spec.c2pa.org/specifications/specifications/2.3/specs/C2PA_Specification.html) supplies the validation/trust concepts; verifier and supported-spec choices require a technical decision.

### 5.3 Statistical generative watermarks

Anthropic describes keyed SynthID-Text-family sampling without added hidden characters; its article describes private-preview detector access and a staged rollout. Detection is weaker on small samples and on text where the model made relatively few token choices, such as factual passages, proofreading, light editing of human-written text, and much code. Conversely, light subsequent editing of already watermarked generated text may leave the watermark detectable; extensive rewriting can reduce or eliminate detectability. These statements describe the announced mechanism, not verified watermark status of a locally inspected file. [Anthropic description, updated September 1, 2026](https://www.anthropic.com/news/claude-text-watermark).

Google describes SynthID text watermarking through generation-time token probability adjustments. [Google DeepMind](https://deepmind.google/models/synthid/).

A generic algorithm implementation is not possession of a vendor's detector key/configuration. Do not substitute prose-style heuristics or generic AI classifiers. An adapter must retain actual vendor result semantics, detector/configuration version, exact analyzed sample identity, coverage and limitations. Do not invent probability fields, equate vendor scores with generic finding confidence, or invent byte spans for a sample-level result.

Initially, the proposed Anthropic capability is `unavailable_without_detector_access`; it is not a finding or negative result. Availability assertions must be rechecked when implementing an adapter. Ordinary auditing cannot upload documents merely because an integration has credentials.

### 5.4 Statistical steganographic/covert channels

Controlled-environment reviewers may investigate suspected information transfer through representational choices or keyed token selection. This is a research and configured-detection use case, not a claim that arbitrary covert channels can be inferred from prose.

Distinguish a validated known-watermark match, an explicitly justified anomaly, a known keyed-channel match and insufficient evidence. Every proposed analyzer needs stated assumptions, suitable positive/negative controls and measured false-positive limits. Unusual vocabulary alone is not evidence of steganography. Aletharsis may decode a known observed carrier when necessary to characterize evidence, such as extracting the ASCII projection of Unicode tags, but does not provide tooling to construct or operationalize covert channels. Channel generation and exploitation are outside this detection scope.

### 5.5 Experimental stretch goal: intrinsic model fingerprint analysis

Aletharsis may experimentally investigate whether outputs without a deliberate watermark exhibit reproducible, model-associated generative signatures under controlled conditions. This is a research hypothesis to validate, not an assumed stable or unique identifier. Intrinsic fingerprint analysis is separate from keyed watermark detection, cryptographic provenance and evidence of an intentionally encoded covert channel.

Any experiment must use documented, versioned baselines and independently held-out samples, with controls for prompts, topic, language, model/version, sampling configuration and subsequent editing. Evaluation must test unseen models and changed conditions rather than force every sample into a known-model label. Sample size and scope must be explicit; minimum usable sample sizes, error rates and decision thresholds require empirical validation. Shared training data, model families, prompt effects and distribution shifts are potential confounders, not evidence of identity.

The eventual experiment plan uses [WeaverTools](https://github.com/toddwbucy/WeaverTools) as the first planned reference-corpus producer. Weaver owns controlled generation and creation-trace capture; Aletharsis owns artifact inspection and experimental comparison. A separately specified, portable baseline package must bind exact output SHA-256 hashes to recorded generation conditions, trace references, experiment/configuration versions and dataset splits. Record unavailable generation details explicitly. This integration is planned, not an existing export capability; independently produced corpora must also be admissible under the same contract. Ordinary Aletharsis auditing does not require Weaver or a creation trace.

Controlled reference data is the input to calibration, not proof of a calibrated detector. Vary harness conditions with the model fixed, then vary the model with harness conditions fixed, accounting for prompts, memory, tools, sampling and post-processing. Evaluate whether any apparent signature belongs to the model, the surrounding agent system or their interaction. Split development, calibration and blind evaluation data to prevent shared prompts, runs or derived samples from leaking across partitions. During blind fingerprint evaluation, model labels, revealing filenames and creation-trace metadata remain outside the analyzer's input features; retain them separately for scoring and provenance. Distinguish raw model output from any rendered or post-processed artifact being tested.

The research sequence is controlled Weaver runs → versioned, hash-bound corpus → separate development/calibration/blind evaluation sets → Aletharsis analysis → measured discrimination, calibration, error rates and unknown outcomes. Validation must include unseen models and independently varied generation setups before claiming applicability beyond the reference environment.

Results remain probabilistic and scoped to the tested baselines. Report method/version, baseline identity, analyzed sample identity, evaluation conditions, uncertainty and limitations; do not present an uncalibrated similarity score as an attribution probability. Unknown, inconclusive, insufficient-sample and out-of-scope outcomes must be supported. A fingerprint result cannot establish authorship, intent, a particular user or organization, or the presence of a deliberate watermark; it supplies no exact watermark bytes to reveal or remove.

This is an optional P6 research stretch goal, not a committed detector or a dependency for P0–P5, the workbench or remediation. Product integration requires a separately reviewed specification and reproducible validation showing useful discrimination and acceptable false-positive limits. The current schema and detector contracts remain unchanged; Issue #17 should preserve room for distinct experimental result semantics without inventing them now.

## 6. Primary use cases

| ID | Scenario | Required outcome |
| --- | --- | --- |
| UC-01 | Hidden Unicode sequence | Inventory characters, detect supported sequence patterns, preserve positions and show mapped reveal markers |
| UC-02 | DOCX/PDF hidden or watermark-like object | Identify the supported part/object and rendering evidence; distinguish ordinary structure and unsupported visibility |
| UC-03 | Agent-instruction carrier | Expose concealed content; retain context for judging legitimate instructions versus suspected injection; do not execute it |
| UC-04 | Signed provenance credential | Discover and evaluate supported credentials independently of suspicion; expose incomplete validation |
| UC-05 | Configured statistical watermark | Record the real detector result and analyzed scope; unavailable access remains untested |
| UC-06 | Suspected covert channel | Report observable/configured-detector evidence without claiming arbitrary-channel detection or intent |
| UC-07 | Workspace triage | Account for every selected source and move from queue status to evidence, limitations and review |
| UC-08 | Repeated reviewed pattern | Search source/code/document evidence, freeze exact matches and bulk-classify a reviewed set without global removal authorization |

Instructions can also be ordinary visible comments, docstrings or Markdown. Source-language classification is a capability, not something implied by syntax highlighting. Reviewer judgments about legitimacy must retain scope and must not alter original detector findings.

## 7. Known-good format profiles

Versioned, immutable expected-artifact profiles provide contextual assessments. Planned targets include plain text, source contexts, DOCX/OOXML, ODT/ODF, HTML/XML and separately scoped born-digital/OCR-derived PDF extraction. A profile cannot substitute for a parser or imply semantic support the parser lacks.

Expected does not mean trusted: an expected document structure can contain malicious content. Cryptographic trust is a separate assessment and does not establish the truth of credential assertions.

Rules may reference format/version, parser identity, package location, character/object type, relationships, surrounding structure, frequency/placement, rationale and rule revision. They must be inert data, not executable hooks. Do not implement blanket character whitelists or promise an unmeasured percentage of expected-format coverage.

**All relevant analyzers receive the preserved evidence regardless of profile assessment.** A legitimate isolated joiner can participate in a suspicious larger pattern; sequence evidence must not be lost because a character matched an expected-use rule. Profiles can explain expected/noteworthy/suspicious/unknown assessments and default queue treatment, with visible disagreements and exclusions. They cannot suppress independent cryptographic/statistical execution or conceal its coverage state.

Every report records exact profile/revision/configuration identity and retains the underlying observations and assessment rationale. A new revision creates a new identity; old audits remain interpretable with their recorded rules. See [Issue #14](https://github.com/toddwbucy/Aletharsis/issues/14).

## 8. Detection pipeline and evidence authority

```text
Read-only acquisition → identity/hash → bounded format parser → normalized evidence
                                                                  ├→ profile annotations
                                                                  ├→ structural analyzers
                                                                  ├→ credential adapters
                                                                  └→ configured statistical detectors
                                                                              ↓
                                                   findings + assessments + coverage
                                                                              ↓
                                                             report / reveal / review
```

Parsers establish what they extracted and what they could not interpret. Analyzers report evidence-based interpretations. Profiles add context. Human decisions remain separate records. None of these stages discards raw evidence because another stage considers it expected.

The proposed Go contract must distinguish:

| Concept | Responsibility |
| --- | --- |
| Source identity | Original byte SHA-256, acquisition outcome, supplied path and verified identity |
| Report artifact identity | SHA-256 of exact imported/retained report bytes, separate from the source digest |
| Observation/location | Source segment or structural object, coordinate space, offsets and mapping quality |
| Finding | Stable detector/rule identity, existing category, mechanism, severity, explanation and supporting evidence |
| Profile assessment | Rule/revision, rationale, expectedness and queue treatment; original finding retained |
| Detector execution | Capability, enabled state, execution outcome, supported/analyzed scope and exclusions |
| Typed result | Detector-specific result and documented score semantics, if actually supplied |
| Review decision | Reviewer assessment, exact evidence references, scope and decision history |

Retain the existing INFO/LOW/MEDIUM/HIGH severity scale: review priority, not maliciousness. Confidence describes support for a stated finding, not intent or a universal probability of AI authorship. Do not fabricate confidence for an unavailable detector.

Existing classification values (`observed_fact`, `suspicious_pattern`, `likely_mechanism`, `undetermined`) are current wire contracts. The draft concepts “expected artifact,” “known mechanism” and “statistical result” need deliberate modeling; they must not be silently added as enum values or collapse profile/result state into one classification field. Exact field names/types are technical-specification decisions under [Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17).

## 9. Capabilities, coverage and failures

Capability availability, execution outcome and substantive detector result are separate. In particular, “unavailable,” “disabled,” “not run,” “unsupported input,” “failed,” “partial” and “completed” must remain distinguishable where applicable. A completed detector may report a negative, positive or inconclusive result according to its actual contract.

Queue presentation distinguishes completed with no reported findings, requires review, expected-only findings, failed, unsupported, skipped and canceled. Completion/review disposition and coverage must remain independent dimensions: a completed structural scan can still have no statistical analysis. An expected-only queue entry must not conceal unexamined mechanisms.

Define stable machine-readable backend failure codes and an explicit legacy adapter. Human diagnostic prose is not a protocol. Current schema 1.0 is strict; new fields or enum variants require a version/compatibility decision and reviewed migration. Do not rewrite frozen reference artifacts to conceal intentional changes. Current CLI exit codes remain until a reviewed extension specifies batch, partial and configured-detector failure behavior.

Capability discovery must show supported formats, modes, resource limits and optional adapter prerequisites. Missing vendor access must be useful information, not a guessed negative or a reason to make all local findings unusable.

## 10. CLI and workspace scans

The CLI remains a first-class product. Current single-file audit/unicode/metadata/structure views continue to work. Target forms include directory auditing, JSONL, aggregate summaries and explicit reveal/diff destinations. Proposed provenance/statistical views and capability commands require syntax review rather than being advertised as installed commands.

Workspace scans need explicit recursion, inclusion/exclusion rules, deterministic relative-path identities, symlink policy, cancellation and bounded concurrency. Record all discovered/selected items and exclusions without double-counting outcome totals. Unsupported, failed and skipped sources remain visible. A changing source invalidates affected results; a partial scan cannot present complete coverage.

Bound bytes, expanded archive content, object counts, recursion, parser work, output size and elapsed time. State limit-triggered omissions precisely. Large-file concurrency must follow measured memory costs, not CPU count alone. Output destinations must not feed recursive scans, alias sources, flatten paths into collisions or overwrite existing artifacts silently.

## 11. Revealed forensic derivatives and diffs

A revealed derivative is an inert presentation of evidence, not cleanup. For a source containing `Hello` + U+200B + `world`, it may show:

```text
Hello⟦U+200B ZERO WIDTH SPACE⟧world
```

Markers must be deterministic, distinguishable from identical-looking literal source text, and mapped to source/report identities, finding IDs, exact coordinates and code-point details. Reuse analyzer evidence instead of creating a divergent detector in the renderer. Preserve visible text and line endings where the representation supports them; do not normalize the source. Any necessary display escaping must be explicit and mapped.

Provide ordinary unified diffs plus structured mapping/JSON or JSONL. A faithful diff's original side retains the original character; a sanitized visual comparison must be labeled as a presentation, not a byte-faithful patch. Reveal diffs must not be accepted as approved cleanup plans or executable source files.

Statistical sample results have no fabricated hidden-character marker. Cryptographic results use credential/validation presentation. Directory reveal output preserves relative source organization beneath a safe, new destination. The report and location mapping remain authoritative. See [Issue #15](https://github.com/toddwbucy/Aletharsis/issues/15).

## 12. Evidence Review Workbench

The primary eventual workflow is:

```text
Select workspace → detect → review queue → source-mapped evidence → classify/tag
    → recognize pattern → save/associate rule → search scope → freeze result set
    → review context/exclusions → approve interpretation or propose an exact action
```

F0 report import is an implementation step toward workspace triage. Retain the child PRD's requirements to hash exact imported bytes as `report_artifact_sha256`, distinguish imported reports from verified source matches, and approve the Occurrence/location-adapter contract before UI implementation.

Run the mandatory large-report/Unicode-coordinate spike before choosing a viewer/editor. It must cover source-byte/scalar/UTF-16 mappings, combining sequences, emoji/ZWJ, bidi, collapsed invisibles, report hashing and retained raw evidence. A visually convincing highlight is insufficient without verified correspondence. A sample-level statistical result maps to its analyzed scope rather than a false removable span.

Backend logic owns acquisition, parsers, matching semantics, source verification and future writers. The frontend owns presentation/interactions and consumes explicit capabilities; it must not infer semantic contexts or edit coordinates from rendered text.

## 13. Reusable rules and frozen result sets

Rules are versioned, portable declarative data describing matching semantics, applicable formats/contexts, profile constraints and suggested presentation/assessment. Support exact selected patterns first; additional matching modes require bounded behavior and explicit normalization semantics. Matching must preserve original coordinates even when a comparison view is transformed.

A saved rule never grants blanket removal authorization. Bulk review is allowed after showing rule revision, query fingerprint, scope, source/occurrence counts, source hashes, representative context, context coverage and exclusions.

An approval binds to a deterministic frozen set or explicit subset: rule revision + query/scope + source identities + exact occurrences + result-set digest. New files, changed source bytes, changed membership, rule revision or query create a different set. Later matches do not inherit previous approval. Record reviewer actions independently from detector classifications and retain the chosen subset exactly.

## 14. Remediation authority boundary

Detection and review must remain useful without any writer. Future apply requires:

```text
Reviewed exact evidence → change plan → dry run → explicit plan confirmation
    → reverify sources/destinations → create new derivatives → validate → re-audit
```

Bind confirmation to the plan digest, source hashes, exact operations and output destinations. Reject stale plans and source/output aliases. Preserve originals and existing destinations; failures cannot publish an apparently complete success. Record derivative SHA-256, report/decision/rule references, applied operations, omissions and validation results in a change manifest.

Initial writers may support exact text edits only at verified encoding/code-point boundaries. Arbitrary byte deletion can corrupt UTF-16/32 or split a character and is not sufficient. Structured formats require format-aware writers with independent validation. Reversibility comes from retained originals and history, not a promise that deleted content can be recovered from the derivative alone.

## 15. Statistical transformations and future provenance insertion

There is no general exact-byte deletion operation for a token-selection watermark. Semantic rewriting is a separate potential capability, outside core detection and ordinary structural cleanup. It must be explicitly authorized, preserve an independent derivative, disclose semantic change and avoid promises of eliminating unknown watermarks. Re-analysis may retain before/after detector results under their actual limitations.

Organizational provenance insertion/replacement is a future separately scoped subsystem. It cannot be smuggled into cleanup, impersonate another issuer, or destroy the source evidence. Managed provenance must remain inspectable; “ours” does not mean unconditionally trusted or omitted. Credential changes may invalidate prior bindings and need deliberate verification/manifests. Neither rewriting nor insertion is committed for the near-term delivery tracks.

## 16. Threat model and non-goals

Consider concealed content, tracking/provenance identifiers, agent-instruction carriers and suspected covert exfiltration channels. An observation is not proof of an attack. Visible instructions can also be unauthorized; hidden instructions can be legitimate format/application data.

Threat surfaces include hostile files/archives, malformed parser inputs, resource exhaustion, path aliases, external relationships/URLs, malicious report strings, imported rules and misleading provenance assertions. Mitigations require inert handling, bounded work, source verification, distinct trust decisions and explicit coverage when inspection fails.

Aletharsis is not a general text editor, malware sandbox, universal covert-channel detector, general-purpose AI-authorship classifier, malicious-intent judge, in-place sanitizer, OCR platform, or automatic arbitrary-document rewriter. It cannot prove a document safe, guarantee a downstream agent will resist injection, or automatically trust instructions discovered in a repository.

## 17. Security, privacy and operational requirements

HTML/JavaScript/SVG, Markdown resources, office links, PDF actions and terminal controls must not execute or fetch resources during inspection or display. Imported profiles/rules are validated inert data; untrusted findings cannot invoke shell commands. A future local service needs explicit local access/origin controls rather than assuming that binding to localhost makes any browser request safe.

External detectors require content/scope consent, credential isolation, timeouts, bounded retries and a visible disclosure/retention policy. C2PA external references are not permission to perform arbitrary network requests. Do not record API secrets or watermark keys in reports. Air-gapped use remains meaningful with optional capabilities visibly unavailable.

Reports, revealed files and manifests can contain sensitive source content. Use restrictive permissions where supported, document platform equivalents, and avoid content-bearing logs by default. User documents are not silently uploaded as CI fixtures, analytics or public issue attachments. Synthetic fixtures and redacted/reviewed examples support testing.

## 18. Determinism, evidence retention and resource limits

For identical input bytes and declared local versions/configuration, findings, ordering, locations, match sets and reveal artifacts must be deterministic. Preserve Unicode database, parser, analyzer, profile and matching identities. Paths and other contextual inputs affecting identity must be explicitly defined.

Keep exact source and report hashes distinct. Retain analyzed-text hashes and extraction/preprocessing identity for statistical samples. Cryptographic evaluation may depend on trust-store versions, time and network availability; retain that context and the observed response rather than asserting that later evaluation must agree.

The [initial Linux resource baseline](../../benchmarks/baselines/linux-amd64-go1.27.1/README.md) observed approximately 101–266 MiB JSON and 1.1–2.7 GiB child peak RSS for six 8 MiB inputs. Those measurements are not worst-case bounds or cross-platform guarantees. The frontend spike must measure additional browser memory and import/search responsiveness. Publish supported input/report limits based on measurements before release; do not silently truncate evidence to achieve a UI target.

## 19. Technology and component boundaries

Go remains the production backend/CLI, with embedded Unicode reference data and small parser/analyzer interfaces. A thin TypeScript/web frontend remains a direction, not a selected viewer/framework. Python may remain a test harness after application retirement; no Python runtime is required by the Go executable.

Logical components are acquisition, identification, parsers, evidence, profiles, analyzers/adapters, reporting, reveal/diff, workspace, rules/matching, review and eventually apply/writers. These are responsibilities, not a mandate to reorganize existing packages. Prefer incremental changes to current boundaries. External utilities and crypto/vendor integrations require adapters with declared availability and constraints.

## 20. Delivery tracks and release gates

Use **P0–P7** for this parent roadmap to avoid renaming historical M0–M4 backend milestones or the revised child PRD's F0–F5 milestones. Track numbers describe product scope; independent tracks can proceed once their dependencies are met.

| Track | Deliverable | Exit gate / dependency |
| --- | --- | --- |
| P0 — Evidence/capability foundation | Accept the tested Go baseline; define mechanism, capability, failure and location contracts; deliberate Python retirement plan | Issue #17 design, versioned schema compatibility, preserved fixtures/tests; no invented detector results |
| P1 — Corpus/reveal CLI | Bounded directory scans, JSONL/aggregate coverage, mapped reveal artifacts and faithful diffs | Issue #15 tests, deterministic paths, output safety, cancellation/accounting and unchanged sources |
| P2 — Profiles and office containers | Context profiles; DOCX first, ODT in a separately scoped increment | Issue #14; bounded package parsing, exact part/object evidence, profile non-suppression and unsupported coverage tests |
| P3 — PDF | Born-digital structural/metadata evidence and separately declared OCR-derived limits | Bounded extraction, object/page locations and conservative visibility claims; no implicit OCR service |
| P4 — Review/rule backend | Review decisions, declarative rules, scope matching, frozen sets | Accepted adapter contracts, stale-source rejection, bulk approval identity and F0/F1 alignment |
| P5 — Provenance adapters | Prioritized C2PA feasibility/verification; statistical adapters when access exists | P0 contracts; tested trust/coverage semantics; actual vendor contract and consent before remote analysis |
| P6 — Advanced analysis | Vetted new steganographic/statistical/cross-format detectors; optional intrinsic model fingerprint research (§5.5) | Stated detection assumptions, positive/negative controls, measured error limits and honest unsupported cases; fingerprint integration requires separate specification and held-out validation |
| P7 — Controlled derivatives | Text and later format-aware writers, dry run, confirmed plans, manifests/re-audit | Independent writer/integrity tests; full source/plan revalidation and reviewed failure handling |

P5 C2PA discovery/design can proceed after P0 without waiting for all PDF/UI work. Vendor API availability cannot block local structural functionality. P4/F0 inspection and review do not require P7. No office writer is implied by successful office parsing. Non-Linux acquisition remains a separate capability gate, not an automatic consequence of cross-compilation.

Historical M0/M1 framework/text work is implemented in Go; original DOCX/PDF/batch milestones remain unimplemented capabilities reflected in P1–P3. Acceptance of Go parity permits a reviewed Python application retirement; it does not authorize deletion of immutable migration evidence or weaken CI.

## 21. Acceptance and success measures

| ID | Acceptance scenario | Required evidence |
| --- | --- | --- |
| A-01 | Ordinary and adversarial text in supported encodings | Exact bytes/scalars/hashes, independently expected findings and meaningful false-positive controls |
| A-02 | Invisible binary sequence or bidi/emoji/combining input | Inventory and supported sequence evidence; source-to-reveal and display-coordinate round trips |
| A-03 | Unsupported/failed/limited parser or detector | Explicit scoped failure/coverage, never an unqualified clean verdict |
| A-04 | Expected character inside suspicious sequence | Pattern evidence survives profile assessment; suppressed queue items remain inspectable |
| A-05 | No structural findings, unavailable statistical detector | Report states that statistical analysis was not performed; no vendor-negative implication |
| A-06 | Credential found, invalid binding or untrusted signer | Distinct discovery, integrity and trust states with verifier/context identity |
| A-07 | Directory scan with failures/cancellation/exclusions | Complete accounting for selected items, retained completed results and honest incomplete scope |
| A-08 | Literal marker text and overlapping findings | No marker/source ambiguity or duplicate mutation; authoritative structured mapping |
| A-09 | Frozen bulk approval followed by new/changed source | Original set remains identifiable; new/stale matches receive no inherited authorization |
| A-10 | Hostile markup, relationships, rules or report text | No execution, network access or instruction following by default |
| A-11 | Confirmed derivative plan, destination conflict or changed input | Safe rejection or separately validated derivatives; source preservation, manifests and re-audit |
| A-12 | Large reports at published supported limits | Measured resource use and responsive bounded review with coordinate fidelity; no silent evidence loss |

Release evidence combines unit/property/fuzz tests, real executable tests, deterministic snapshots, schema/semantic validation, source integrity checks and native platform tests. Synthetic positive fixtures must establish the mechanism under test. A document merely believed to have been generated by Claude is not a known-positive statistical fixture. Future detector claims need independently verified results and documented applicability.

Track detection/error rates against labeled corpora per detector/profile; unexplained classifications, hidden omissions and unmapped actionable locations must be zero in the applicable acceptance suite. Every selected corpus item must have an accounted-for outcome. Measure import/search latency, memory and time-to-reviewed-set on stated workloads; establish numerical release budgets in the relevant spec after benchmarking rather than inventing universal SLAs here.

Product success means another reviewer can identify the evidence and its limitations, distinguish machine observation from human interpretation, reproduce supported local results, locate recurring patterns and preserve original evidence throughout any later apply operation. No single detection rate represents all mechanism classes.

## 22. Next specifications and open decisions

The next specification focus is the **Go detection/evidence engine and mechanism model**, not additional CLI flags or remediation. Review in bounded increments:

1. Mechanism/capability/execution/result contract and schema-version/legacy-report strategy; stable failure codes; relationship to current findings and confidence.
2. Source/segment/location identity, extraction maps and statistical sample scope; reconcile with the child Occurrence contract and exact imported-report hashing.
3. Expected-artifact assessment contract and analyzer non-suppression rules, followed by corpus/reveal contracts with resource and output boundaries.
4. C2PA adapter feasibility and trust-policy design, independently of private detector access.
5. Approve the revised child PRD for the new evidence classes; F0 large-report/Unicode spike before viewer choice; finalize F0/F1 specs only after required contracts and evidence exist.

The parent product architecture is approved with the review clarifications incorporated. Further detail belongs in specification files, starting with the mechanism/capability/execution/result model and schema migration under Issue #17. The revised child's requirements, delivery priorities, resource budgets and Python retirement timing retain their separate review gates. Technical review must resolve precise wire fields, optional-detector failure/exit policy, profile conflict handling and structured-format location fidelity; architecture approval does not imply those contracts are implemented or approved.

Related tracking: [#14 expected-artifact profiles](https://github.com/toddwbucy/Aletharsis/issues/14), [#15 reveal/diff](https://github.com/toddwbucy/Aletharsis/issues/15), [#17 mechanism classes](https://github.com/toddwbucy/Aletharsis/issues/17). The [draft frontend PRD](frontend-PRD.md) is the proposed aligned UI definition; its approved v1.0 predecessor remains archived.
