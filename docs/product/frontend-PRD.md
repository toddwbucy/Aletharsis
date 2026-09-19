# Aletharsis Frontend — Evidence Review Workbench

| Attribute | Value |
| --- | --- |
| Status | Draft rewrite for product review; not frozen |
| Document version | 1.1 |
| Date | 2026-09-19 |
| Parent | [Aletharsis — Forensic Document Signal Auditor, draft v0.1](aletharsis-PRD.md) |
| Backend baseline | Go CLI 0.2.0, report schema 1.0; main `20d5bdc` |
| Product owner | Todd W. Bucy |
| Prior baseline | [Approved v1.0](frontend-PRD-v1.0.md), preserved unchanged; new requirements need approval |
| Scope | Workspace triage, mechanism-aware evidence review, contextual tagging, reusable rules, and separately gated derivative apply |

## 1. Product definition and authority

The Evidence Review Workbench is the local human-review interface for Aletharsis. Its central job is to make evidence, interpretation and analysis coverage understandable together. Some evidence has exact characters to reveal; some has a signed credential to inspect; some is a detector result over a sample. The workbench must present each honestly without forcing every mechanism into a text highlight or deletion range.

This v1.1 draft rewrites the product framing against the proposed parent PRD. It retains v1.0's identity, coordinate, review and apply safeguards while extending the requirements for profiles, capability coverage and cryptographic/statistical evidence. It is not approved merely because v1.0 was frozen. The archived baseline remains historical; parent/child requirements and technical contracts must be reconciled through review before implementation.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

The workbench must never equate no reported findings with no watermark, successful import with authentic evidence, or an unrun detector with a negative result. Every interpretation remains traceable to retained source evidence or an explicitly described analyzed scope.

### 1.1 Three phases and authority

**Detection establishes what is present. Evidence review establishes what it means in context. Explicit apply determines what, if anything, should be changed.**

| Phase | Workbench responsibility | Durable record |
| --- | --- | --- |
| Detection | Import or request supported backend analysis, display capability/execution coverage and preserve original reports | Exact report bytes/digest, source identity, findings, profile assessments and detector results |
| Evidence review and tagging | Navigate evidence, interpret context, search, record decisions, version rules and freeze reviewed result sets | Separate attributed decisions, rule revisions, exact evidence anchors and set membership |
| Explicit apply | Present a backend dry run, collect confirmation for that exact plan and display validation/re-audit results | New derivatives, plan/confirmation identity and change manifests |

Phase boundaries are product invariants, not merely separate screens. Neither a finding, profile, credential, vendor result, reviewer label nor saved rule can bypass an explicit apply plan. Review and inspection remain useful without a writer. Original files are never rewritten; reversibility depends on preserved originals and history.

### 1.2 North-star workflow: workspace triage

```text
Select workspace → scoped detection → reconciled review queue
    → inspect evidence using the appropriate mechanism view
    → distinguish detector result, profile assessment and human judgment
    → select/search a supported pattern → review representative contexts
    → freeze exact matches → classify/propose action for set or subset
    → when a writer is available: dry run → confirm → new copies + re-audit
```

A queue can show 82 documents with 21 requiring review, 5 failed, 3 unsupported and 53 completed with no reported findings. Those illustrative primary outcome counts reconcile; mechanism coverage is a separate dimension. Any of the completed documents might have statistical analysis unavailable. Expected-only, skipped and canceled outcomes remain visible when present. A matching document discovered tomorrow receives no prior approval.

F0 report import is a stepping stone. F1 enables collection review and frozen decisions; F2 introduces backend-mediated live workspace scanning. Design for this progression without pretending an imported report is a live verified source.

### 1.3 Mechanism-aware interpretation

| Mechanism | Reviewer sees | Prohibited inference |
| --- | --- | --- |
| Structural/representational | Source characters, encoded spans, package parts, objects, metadata and exact/derived location quality | An invisible character, header or identifier is necessarily a watermark |
| Cryptographic provenance | Credential discovery, assertions, signature/content-binding results and trust context | A discovered credential is verified, or a valid signature proves all content true |
| Statistical | Named detector, actual result semantics, analyzed sample identity/scope, coverage and limitations | A score is a hidden substring, exact removable bytes, proven human/AI authorship or user identity |

A single asset may contain all three. Finding severity, profile expectedness, detector confidence and reviewer judgment remain independent. Statistical watermark versus suspected covert-channel purpose must come from an explicit detector contract, not stylistic guesses by the UI.

## 2. Users and primary jobs

| User | Job | Required outcome |
| --- | --- | --- |
| Forensic/security reviewer | Triage unusual mechanisms and incomplete analysis | Inspectable evidence, honest coverage and reproducible decisions |
| Codebase maintainer | Assess comments, docstrings, Markdown and other agent-facing text | Contextual instruction judgments without executing inspected instructions |
| Provenance reviewer | Inspect credentials or configured detector results | Distinct integrity, trust, result and uncertainty states |
| Repository/document owner | Carry reviewed judgments across recurring patterns | Versioned matching and frozen-set approval without automatic editing |
| Independent reviewer | Reopen another person's review and proposed changes | Exact report identity, declared source verification, portable decisions and manifests |

No account or cloud service is required for local review. Optional remote analysis has a separate disclosure and authorization flow.

## 3. Goals and non-goals

### 3.1 Goals

- Make evidence navigable in a presentation appropriate to its mechanism, including exact reveal where it exists.
- Explain what was assessed and what was not, even when a report has no findings.
- Search original text/code/declared segments from a selection and retain verified mappings.
- Separate expected-artifact assessments, original detector claims and attributed human decisions.
- Reuse rules and representative review across exact frozen result sets without one mandatory click per occurrence.
- Retain report/source identity, source integrity and explicit apply authorization through later derivatives.

### 3.2 Non-goals

No general-purpose editor, automatic intent/AI-authorship judge, browser watermark detector, blanket Unicode removal, arbitrary statistical-channel inference, in-place sanitizer or implicit command execution. The browser does not implement cryptographic validation or vendor scoring as a presentation shortcut. No universal language parsing, OCR, cloud synchronization or lossless office rewriting is promised. Semantic rewriting and organizational watermark insertion are separate product proposals, not controls in this initial workbench.

## 4. Current backend and dependencies

| Area | Current Go baseline | UI consequence |
| --- | --- | --- |
| Reports/identity | Deterministic schema-1.0 JSON, source digest and current finding categories | Import through a versioned adapter; preserve exact report bytes |
| Text | Literal source, UTF-8 or BOM-marked UTF-16/32, exact scalar/byte offsets and EOF boundary | Text inspection is available; rendered markup/semantic contexts are not |
| Detection | Unicode/emoji, normalization, pattern candidates, identifiers and provenance labels | Explain actual evidence; these are not statistical or C2PA tests |
| Failure | `parser.failure`, status/exit 4 and diagnostics; no stable reason code yet | Generic legacy failure handling; explicit versioned failure-code extension |
| Scope/capability | No complete machine-readable execution/coverage descriptor yet | Unknown coverage stays unknown; Issue #17 contract required for reliable mechanism status |
| Files/platforms | Single regular file ≤8 MiB; Linux amd64/arm64 native checks; non-Linux acquisition fails closed | Browser access cannot bypass backend acquisition policy |
| DOCX/PDF/ODT | No structured parsers; DOCX/PDF recognized as unsupported | No office inspection or false empty success |
| C2PA/statistical | No implemented verifier or vendor adapter | Display unavailable/unassessed capability, no guessed negative |
| Workspace, rules, review, reveal/apply | Backend capabilities not implemented | Separate contracts and release gates; current CLI is not a UI service |

References: [Go evidence](../../internal/evidence/model.go), [audit](../../internal/audit/audit.go), [schema](../../schemas/report.schema.json), [platform validation](../testing/platforms.md), [parent PRD](aletharsis-PRD.md). Python is a temporary migration oracle, not the frontend adapter or production engine.

## 5. Delivery model and priorities

Priority **P0** means required within its assigned frontend phase; P1 is follow-up and P2 exploration. These priority labels are not the parent roadmap's P0–P7 track names.

F0 is a bundled offline viewer for imported reports, with no required server, remote assets or account. A self-contained HTML export is desirable if inert data embedding and disclosure are demonstrated. Later live functions use an explicitly launched local **Go** adapter with authenticated session/path boundaries. Packaging, framework and transport remain technical decisions.

F0 imports report bytes and computes their digest; it does not read the original source or inherit forensic acquisition guarantees through a browser file picker. Exact in-report mappings can support inspection before independent source verification, but cannot authorize a write. Unsupported legacy coverage stays explicit. Future backend schema versions require declared adapters rather than permissive coercion.

## 6. Information architecture

### 6.1 Primary surfaces

| Surface | Purpose |
| --- | --- |
| Workspace/documents | Scope, primary outcomes, independent per-mechanism coverage and review progress |
| Inspector | Selected source evidence, credential or statistical sample with identity and limitations |
| Capability/coverage | Available, enabled, executed, failed, partial and unavailable analyses; exclusions |
| Search/results | Exact query/mode, included segments, matches, exclusions and frozen-set controls |
| Review/history | Human artifact/instruction assessments, rationale, conflicts and stale decisions |
| Rules/profiles | Distinct versioned rule suggestions and expected-artifact assessments; neither is edit authority |
| Change review | Supported exact plans, dry-run comparison, confirmation and derivative validation |

### 6.2 Inspector layout

```text
Source/report identity | verification | primary outcome | mechanism coverage
-------------------------------------------------------------------------
Evidence/results tree | Mechanism-specific view       | Facts and review
Structural findings   | Revealed/logical source       | Detector explanation
Credentials           | Manifest/validation details   | Profile assessment
Statistical results   | Sample/result/coverage        | Limits + human judgment
-------------------------------------------------------------------------
Context actions: copy/search supported source | classify | plan eligible edit
```

Keep source context available for sample-level results without highlighting fictitious watermark tokens. A missing exact location is not malformed when the contract intentionally describes a whole asset/sample. Responsive layouts may use drawers/tabs; essential information and actions cannot depend on hover or color alone.

### 6.3 Terminology

A **finding** is a machine observation/interpretation; a **profile assessment** supplies expected-format context; a **detector execution** records what ran and its coverage; a **result** carries that detector's documented outcome. An **evidence anchor** can identify exact text occurrences, structural objects, credentials or statistical samples. A **decision** is attributed human judgment. A **rule** defines matching; a **frozen set** records exact membership. A **change plan** proposes supported exact operations. A **reviewed copy** is a derivative, not a safety certification.

## 7. User journeys

### J1 — Reveal and reuse an invisible sequence

Import a report, navigate all inventory positions, select the actual hidden sequence through its markers, search the full included text scope, inspect representative matches and save a literal rule. Marker labels never become the query. Every occurrence remains inspectable without mandatory individual approval clicks.

### J2 — Review instructions in source code

Select visible or hidden instruction text even without a detector finding. Compare legitimate project guidance, quoted examples and unauthorized instructions in their contexts. Record artifact and authority assessments separately. A docstring does not automatically become repository policy, and context coloring does not prove a parser recognized it.

### J3 — Freeze bulk review

Preview a rule against selected reports/workspace scope. After reviewing counts, exclusions, source identities and representative context, freeze 684 exact matches across 17 documents. Approve an assessment or propose removal for that exact set/subset. New files, altered bytes, changed rules or queries create a new set without inherited approval.

### J4 — Inspect a statistical result or unavailable detector

A report with ordinary ASCII and no structural findings shows statistical analysis as unperformed when no execution exists. A future configured detector result shows its actual categories, sample hash, declared length/units and limitations. There is no hidden-character reveal or “remove watermark bytes” action. Short/inconclusive samples remain inconclusive.

### J5 — Inspect a provenance credential

Open a manifest, distinguish discovered from validated and trusted, inspect assertions/bindings and disclose missing checks. An absent credential or untrusted signer does not become a verdict about authorship. External references do not trigger fetching when clicked as evidence; any network retrieval needs a separate authorized operation.

### J6 — Review expected artifacts without losing signal

The default queue deemphasizes profile-matched typography. The reviewer can inspect those observations and rule versions, including any sequence-level finding overriding isolated expectedness. Credential/statistical coverage remains visible regardless of structural filters.

### J7 — Produce a reviewed derivative, or stop at a missing capability

For a supported exact text operation, inspect the backend dry run, resolve conflicts, confirm the immutable plan, and receive a new validated/re-audited copy plus manifest. Stale sources, unavailable writers, unsupported object mappings or changed destinations block apply. Import-only evidence remains reviewable. No statistical sample becomes a deletion range.

## 8. Functional requirements

### 8.1 Import, identity, and coverage

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| IMP-01 | P0 / F0 | Import schema-1.0 reports and explicitly supported new versions through declared adapters; validate structure and defensively validate ranges, counts, offset monotonicity, and references before navigation. Present malformed data as an import error. |
| IMP-02 | P0 / F0 | Show tool/schema versions, file identity, MIME identification basis, audit status, parser, hashes, and limitations. Never interpret exit codes 1–3 as parser failures; use the report's status and failure evidence. |
| IMP-03 | P0 / F0 | Distinguish report loaded, source unverified, source verified, source changed, audit failed, and unsupported format. Report import alone never establishes source verification. |
| IMP-04 | P0 / F0 | Reject unsupported major schema versions with an explanation. Preserve the imported report unchanged and do not silently coerce unknown findings into a known detector. |
| IMP-05 | P0 / F1 | Open a collection of imported reports, preserving independent source identities and review states. This is collection inspection, not recursive filesystem auditing. |
| IMP-06 | P0 / F2 | For a live workspace, expose selected roots, inclusion/exclusion patterns, symlink policy, discovered files, completed files, failures, cancellations, and skipped files. Never equate skipped files with clean files. |
| IMP-07 | P0 / F0 | Compute and retain `report_artifact_sha256` from the exact complete imported report bytes before decoding, parsing, or reserialization. Bind the loaded report, occurrence anchors, and subsequent exports to that digest. Preserve original report bytes for the active session or an explicitly saved evidence bundle; do not silently persist them in browser storage. |
| IMP-08 | P0 / F0 | Interpret backend failures using the stable `failure_code` contract in section 9.3, not exception classes or message text. Missing legacy codes or unfamiliar codes produce a generic failed-audit state with the original diagnostic available, without guessing a more specific cause. |

Legacy filtered CLI reports do not include complete machine-readable scope; retain their view notice, label counts as “findings in this report,” and mark unspecified coverage unknown. F0 must define this legacy behavior and consume the new mechanism/capability contract when supported. Never infer a negative detector result from missing fields or an empty finding list.

### 8.2 Evidence visualization and selection

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| VIS-01 | P0 / F0 | Provide source, revealed-character, and escaped views. Labels such as `⟦U+200B ZERO WIDTH SPACE⟧` represent source characters; they are not inserted into the evidence text. |
| VIS-02 | P0 / F0 | Selecting a structural text finding navigates all its mapped occurrences. Other mechanisms navigate their typed evidence scope. Selecting a text occurrence highlights the corresponding source positions and exposes code point, Unicode name, byte offsets, and context. |
| VIS-03 | P0 / F0 | Distinguish character counts, occurrence counts, finding counts, and visible result counts. A grouped finding with 64 character positions is not automatically 64 instructions or one contiguous edit. |
| VIS-04 | P0 / F0 | Collapse long repetitive runs with an exact count and expandable content. Collapsing must not lose searchable characters or imply that unreviewed matches were inspected. |
| VIS-05 | P0 / F0 | Support text selection across visible and invisible content; selections crossing marker labels resolve to original code points. Let users copy original text, escaped text, or a code-point list as distinct actions. |
| VIS-06 | P0 / F0 | Show normalization changes and available hashes without replacing source content. Differentiate raw-byte SHA-256 from UTF-8 hashes of extracted/comparison text. |
| VIS-07 | P0 / F0 | Preserve logical order in the evidence view and visibly expose bidi controls. A reading-oriented preview must not replace the authoritative escaped/offset view. |
| VIS-08 | P1 / F2 | Offer an original-byte view only when bytes have been acquired and verified. Do not claim an imported JSON string is a verified byte dump of the original file. |

User-facing line/column navigation may be one-based, but the evidence inspector must identify all coordinate units. Unicode scalar offsets, JavaScript UTF-16 indices, grapheme clusters, byte offsets, and displayed marker positions are different coordinate systems. The implementation must maintain explicit mappings; emoji and combining characters must not shift selections.

### 8.3 Search from a selection

| Mode | Behavior | Editing consequence |
| --- | --- | --- |
| Exact literal | Case-sensitive original Unicode sequence; includes invisible characters and actual line endings | Can propose changes after exact source-range review |
| Exact invisible sequence | Selected code-point sequence in original text, without inventing a payload interpretation | Same requirements as literal matching |
| Formatting projection | User explicitly selects a character set to extract and search while ignoring other characters | Results identify every contributing original span and intervening content; never delete the enclosing interval implicitly |
| Comparison search | Explicit NFC, NFKC, case-folding, or whitespace transformation | Discovery only until exact original spans are selected and reviewed; ambiguous mappings block direct edits |
| Regular expression | Optional bounded, non-executing expression mode | P2; requires engine limits, dialect/version disclosure, and safe handling of zero-length matches |

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| SRCH-01 | P0 / F0 | “Find this selection” populates an exact query from source characters, shows an escaped preview, and searches the entire current text segment or selected document scope. |
| SRCH-02 | P0 / F0 | Show total matches, current result, source context, original ranges, and next/previous navigation. Search can be canceled. Display truncation or result caps explicitly. |
| SRCH-03 | P0 / F0 | Return overlapping exact matches deterministically; reject empty queries. Do not match across independent text segments by silently concatenating them. |
| SRCH-04 | P0 / F1 | Add formatting projection and comparison modes with visible settings and original-span mappings. Show the actual matching mode on every result set. |
| SRCH-05 | P0 / F1 | Search a selected collection of reports. Let users restrict filenames/types and include unknown semantic contexts explicitly. |
| SRCH-06 | P0 / F2 | Search a selected workspace through the backend. New files, changed source hashes or changed queries produce a new result-set identity and cannot inherit prior bulk approval. Retain the old approval as a historical record; changed source anchors and dependent plans become stale. |
| SRCH-07 | P0 / F1 | Labeling or planning changes for “all matches” must show the exact count and scope, including whether selection covers a page, current filter, or the full frozen result set. |

“Whole document” means all extractable, included segments supplied by the parser. It does not promise OCR, complete rendering coverage, or content the parser could not inspect.

### 8.4 Source code, Markdown, and instruction review

Source extensions must not be restricted to a small document allowlist. Python, JavaScript, TypeScript, shell, configuration files, and other supported Unicode files must remain inspectable as raw text. Language-sensitive capabilities are separate from text eligibility.

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| CODE-01 | P0 / F0 | Inspect source code without evaluating it, resolving imports, running macros, or executing repository scripts. Language coloring must not alter source mappings. |
| CODE-02 | P0 / F2 | Establish parser-derived Python boundaries for comments, recognized docstrings, other strings, and executable syntax. A triple-quoted string is not automatically a docstring. |
| CODE-03 | P0 / F2 | Establish Markdown boundaries for prose, fenced code, inline code, and quotations using a documented parser/dialect. Rendered Markdown remains optional. |
| CODE-04 | P1 / F2 | Add JavaScript/TypeScript and other language adapters. Advertise context coverage per language and parser version rather than implying universal semantic support. |
| CODE-05 | P0 / F2 | If parsing fails or a language is unsupported, fall back to raw text with context `unknown`. Rules requiring a semantic context must report ineligibility instead of treating all text as comments. |
| REV-01 | P0 / F1 | Let reviewers label any selected span, including text with no machine finding. |
| REV-02 | P0 / F1 | Keep reviewer classification, proposed action, severity, detector confidence, and detector classification separate. A human label must not rewrite the original finding. |
| REV-03 | P0 / F1 | Record reviewer identity as an attribution string, rationale, scope, source hash, spans, and decision history. Local identity is self-declared, not authenticated proof. |
| REV-04 | P0 / F1 | Store review decisions outside source files. Undo/revise decisions through history without modifying original evidence. |
| REV-05 | P0 / F1 | A changed hash or parser coordinate model makes approvals stale. Suggestions for relocating them require explicit re-review before approval is restored. |
| REV-06 | P0 / F1 | Conflicting imported decisions remain visible and unresolved until a reviewer resolves them. No silent last-writer-wins rule may authorize removal. |
| REV-07 | P0 / F1 | Support artifact judgments including `watermark`, `benign artifact`, and `needs investigation`, with reviewer attribution, rationale and exact evidence anchors. Preserve these as human assessments alongside the unchanged detector finding; they do not authorize removal. |

Artifact review must let a reviewer express “this is a watermark” without rewriting a machine finding into a confirmed detection. Display that assessment as “Watermark — reviewer assessment,” with its author and rationale. Artifact assessment and instruction-authority assessment are separate dimensions: content can be instruction-bearing as well as carrying a suspected identifying artifact.

Instruction review labels (with shared benign/uncertain review states):

| Label | Meaning |
| --- | --- |
| Unreviewed | No reviewer decision exists |
| Legitimate instruction | Approved guidance within the recorded file/workspace and purpose |
| Unauthorized instruction | Reviewer considers this guidance inappropriate for that context |
| Quoted/example content | Instruction-like text is an example, quotation, fixture, or discussion |
| Needs investigation | Authority, purpose, or impact remains uncertain |
| Benign artifact | Reviewed non-instruction artifact worth retaining |

Action choices are separately `retain`, `investigate`, or `propose removal`. “Unauthorized” alone does not execute removal. An approved occurrence does not make identical text universally legitimate. In particular, guidance in an arbitrary docstring does not automatically acquire repository-wide authority.

### 8.5 Reusable pattern library

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| RULE-01 | P0 / F1 | Save a selection or search as a named, versioned rule with a literal sequence, readable escaped/code-point preview, match mode, scope, rationale, and suggested action. |
| RULE-02 | P0 / F1 | Support portable JSON import/export independent of any one report. Rules are inert declarative data, never scripts, executable plugins, or shell commands. |
| RULE-03 | P0 / F1 | Preview a rule against the current report/collection before enabling it. Show positive matches, exclusions, and any context requirements that cannot be evaluated. |
| RULE-04 | P0 / F1 | Rule scope can include file globs, formats, workspace association, explicit case/normalization options, and allowed/excluded contexts. Path-pattern semantics must be documented and consistent across frontend/backend. |
| RULE-05 | P0 / F1 | Preserve immutable revisions. A revision changes the rule fingerprint and requires a fresh match result set; it cannot silently change an approved plan. |
| RULE-06 | P0 / F1 | Imported rules begin disabled for future scans until reviewed and enabled. Enabled rules may flag matches or suggest labels/actions; they never carry removal authorization. |
| RULE-07 | P0 / F1 | Allow test examples, negative examples, descriptions, origin, author attribution, and revision notes. Warn before exporting captured source snippets or sensitive patterns. |
| RULE-08 | P0 / F1 | Retain multiple matching rule references for one occurrence. Conflicting suggestions require explicit review, not priority-based automatic deletion. |
| RULE-09 | P0 / F1 | Allow a reviewer to bulk-classify or propose an action for all or an explicit subset of a deterministic frozen rule-match result set whose members have supported evidence anchors. Before approval, display the rule revision, query fingerprint, scope, source identities/count, occurrence count, exclusions and context coverage. New or changed matches are not included in the previous approval. |

A rule is not a document-specific approval. The library may contain an organization's recognized guidance, but any proposal to make a rule an automatic trust policy requires a separate product decision and is outside this scope.

**Bulk review of a frozen result set:** A reviewer may approve an assessment, retain, investigate, or propose removal for all or a selected subset of matches after reviewing the rule, scope, count, representative context, exclusions and source identities. The UI must support representative review without requiring every occurrence to be opened or individually approved. Every occurrence remains accessible for closer inspection, and incomplete scan/context coverage must be visible.

An action such as **“Approve all 684 matches in this result set for removal”** records a human approval for change planning, not an immediate write. The approval is bound to the immutable rule revision, query fingerprint and scope, source hashes, report-artifact hashes, exact occurrence membership, and frozen-set digest. A selected subset is recorded explicitly rather than as a live filter or “whatever matches later.” Record the representative occurrences reviewed, reviewer attribution and rationale alongside the decision; inspecting representatives must not be represented as individually inspecting every member.

Later matching documents or revised rules require a new frozen result set and approval. A saved rule never carries universal removal authorization. Invalid mappings, unresolved decision conflicts and stale inputs still block dependent change plans; bulk approval does not waive those checks or the separate dry-run confirmation required to write copies.

### 8.6 Change planning and reviewed copies

Cleanup first supports explicit deletions at verified character/encoding boundaries in supported text encodings. General rewriting, automatic normalization, and office-object removal are later work.

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| EDIT-01 | P0 / F3 | Create a plan bound to the exact source hash, encoding, parser version, exact byte ranges/expected bytes, decisions, and rule revisions. Classification/search actions never apply edits. |
| EDIT-02 | P0 / F3 | Show readable and escaped before/after views, exact removed bytes, affected lines, and proposed output name. Retain the full surrounding context needed to understand code changes. |
| EDIT-03 | P0 / F3 | Deduplicate identical edits. Block overlapping or conflicting edits until resolved. Apply original-coordinate changes without position drift and reject ambiguous transformed-match mappings. |
| EDIT-04 | P0 / F3 | Verify the acquired snapshot hash and every expected byte range before applying. If the source changed, reject the plan and require re-audit. Browser-supplied offsets alone are never trusted. |
| EDIT-05 | P0 / F3 | Preserve all untargeted bytes, including encoding, BOM, line endings and surrounding whitespace. Editing a comparison view must not silently normalize an entire file. |
| EDIT-06 | P0 / F3 | Never overwrite the original or any existing destination, including aliases through symlinks/hard links. Publish a completed derivative without leaving an apparently successful partial output after failure. |
| EDIT-07 | P0 / F3 | Perform available non-executing syntax/format validation. New parse errors block export. Unsupported validators must be shown as unavailable; each writer must declare and satisfy its minimum validation gate before export is enabled. Any unvalidated-export exception requires a separate product decision. |
| EDIT-08 | P0 / F3 | Re-audit the derivative with recorded analyzer/rule versions. Show resolved, persistent, new, and unassessable findings. “No findings” is not a safety certification. |
| EDIT-09 | P0 / F3 | Export the copy and a change manifest with original/output hashes, edits, decisions, rule revisions, application/tool versions, validation outcomes and the re-audit reference. |
| EDIT-10 | P0 / F3 | Keep the apply operation unavailable without a verified source and supported writer. An imported report alone may support a proposal, but not claims of a validated edited original. |
| EDIT-11 | P0 / F3 | Require a dry run before publishing derivative files. Show the exact proposed changes, affected files, destinations, available validation and unresolved limitations; planning may use memory or temporary working data but must not publish cleaned outputs. |
| EDIT-12 | P0 / F3 | Require an explicit “Write reviewed copies” confirmation bound to the dry-run plan digest, input hashes, exact edits, rule revisions and destinations. Changed inputs, edits, rules or destinations invalidate confirmation and require a refreshed dry run. Canceling creates no derivative files. |

For code, syntax validity is not semantic equivalence. Removing a docstring can change introspection behavior; removing text can join tokens or break indentation. The UI must show these implications where detectable. Running repository tests or commands requires a separate explicit user action and a displayed command; inspecting a file or applying a rule must never trigger execution implicitly.

“Undo” during planning restores the plan. After export, the original remains available; the application does not undo by rewriting it or silently deleting exported artifacts.

### 8.7 DOCX, PDF, and additional formats

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| DOC-01 | P0 / F4 | Enable format inspection only when the corresponding backend advertises parser capabilities. An unsupported format remains an explicit state until then. |
| DOC-02 | P0 / F4 | For DOCX, navigate by package part and structural location, distinguishing body, headers, footers, comments, deleted/hidden text, properties, relationships, and drawing evidence as provided. |
| DOC-03 | P0 / F4 | For PDF, navigate by page and object/text location where available, exposing metadata, layers, annotations, attachments, and rendering evidence. Disclose extraction/reading-order limitations. |
| DOC-04 | P0 / F4 | Search the declared extracted segments with scope controls. Include/exclude metadata and hidden/deleted content explicitly. Do not concatenate unrelated objects into fictitious matches. |
| DOC-05 | P0 / F4 | Show thumbnails or visual overlays only with a trustworthy page/object mapping. Unmapped findings remain navigable as structural evidence rather than approximate edit targets. |
| DOC-06 | P0 / F4 | Keep office cleanup disabled until a separate writer, structural mapping, integrity tests, and format-specific validation are reviewed. Rendered selection alone does not authorize object deletion. |

Future ODT/RTF/XLSX/PPTX/EPUB support must reuse these capability boundaries. No single plain-text byte-offset assumption may become the universal location model.

### 8.8 Capability and execution coverage

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| COV-01 | P0 / F0 | Present mechanism, capability availability, enabled state, execution outcome, analyzed scope and substantive result as distinct concepts. Legacy unspecified coverage is unknown, not a negative result. |
| COV-02 | P0 / F0 | Show unavailable/disabled/not-run/unsupported/failed/partial/completed states according to the declared backend contract. A zero-finding report still exposes its coverage and limitations. |
| COV-03 | P0 / F0 | Label a missing statistical execution “not analyzed” or the actual unavailable reason, never “no Claude watermark.” Do not infer coverage from an AI/vendor name in metadata. |
| COV-04 | P0 / F2 | Keep primary file outcomes and per-mechanism coverage separate, reconcile selected-file counts, retain completed work on cancellation and expose exclusions/result caps. |
| COV-05 | P0 / F0 | Display unknown detector/result variants as safely inspectable unsupported evidence without coercing them into another category, score or edit target. |

Default structural filtering must not hide failed or unperformed cryptographic/statistical analysis. Color alone cannot express uncertainty; every state needs plain language and an explanation of its scope.

### 8.9 Cryptographic and statistical evidence

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| CRED-01 | P0 / F5 | Display credential/manifest identity, assertions, ingredient references and separately supplied discovery, signature, content-binding, chain/trust and incomplete-check outcomes. |
| CRED-02 | P0 / F5 | Show verifier/spec version, trust policy and evaluation context; distinguish asserted signer names from verified identity. No credential badge may imply content truth or safety. |
| CRED-03 | P0 / F5 | External manifest/ingredient references remain inert. Missing offline checks are visible; fetching requires an explicitly authorized backend operation. |
| STAT-01 | P0 / F5 | Display actual detector identity/version/configuration reference, result semantics, exact sample digest, extraction/preprocessing identity, analyzed scope and limitations. Never expose secret keys. |
| STAT-02 | P0 / F5 | Display characters/tokens only with declared coordinate/tokenizer definitions or actual provider-reported counts. Do not invent scores, confidence conversions or attribution certainty. |
| STAT-03 | P0 / F0 and F5 | A sample-level result can navigate its sample/source context but cannot create hidden-character markers, inferred token contributions or a removable byte span. Unsupported sample mapping remains explicit. |
| STAT-04 | P0 / F5 | Before remote analysis, disclose provider, exact selected content/scope, data being sent and configured retention implications. Require explicit authorization; possession of credentials is not permission to upload. No automatic retry may expand the authorized scope. |
| STAT-05 | P0 / F5 | Preserve separately identified before/after detector observations when requested for an authorized derivative; disclose differing detector/configuration context. A result change is not proof that all watermarks were removed. |

F0 must explain unavailable capabilities and preserve supported imported evidence; it does not invent C2PA verification or a live statistical integration. A future local generic detector requires the same honest identity/configuration/coverage semantics. Statistical-channel research results need their declared method and limitations, not prose-style suspicion inferred by the viewer.

### 8.10 Expected-artifact profiles

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| PROF-01 | P0 / when profile reports are supported | Show immutable profile/rule revisions, rationale, assessed context, default queue treatment and all retained underlying observations. |
| PROF-02 | P0 / same gate | Allow expected artifacts to be deemphasized without deleting evidence. Expose both a profile assessment and a conflicting sequence-level finding; the viewer cannot prevent analysis of expected characters. |
| PROF-03 | P0 / same gate | Structural expectedness has no effect on independent credential/statistical result visibility or execution coverage. “Expected-only” is a queue disposition, not a complete safety or coverage verdict. |
| PROF-04 | P0 / F1 | Keep user-saved match rules distinct from shipped/selected format profiles. Neither grants automatic trust or edit authorization. A changed profile revision is explicit and does not rewrite historic findings/decisions. |

Profile presentation may ship with its backend contract independently of office parsing. Parent P0/P2 and Issue #14 supply those contracts; absence of a profile remains visible rather than causing invented expectedness.

## 9. Data and integration contracts

### 9.1 Preserve the current audit report

The current schema rejects unknown fields in structured objects. Do not insert frontend decisions, rules, or UI state into schema-1.0 reports. Store separately versioned sidecar documents or use a deliberately versioned report migration.

| Entity | Minimum contract |
| --- | --- |
| Audit reference | Exact report-artifact digest, recorded source SHA-256, schema/tool versions, parser, mechanism coverage and source-verification state |
| Evidence anchor | Discriminated exact text occurrence, structural object, credential or statistical sample reference, with report/source identity and mapping/scope quality |
| Review decision | Decision ID/revision, typed evidence anchor or explicit frozen-set/subset reference, artifact assessment and/or instruction-authority assessment, proposed action, scope, rationale, reviewer attribution, timestamp, superseded-decision reference, and stale/conflict state |
| Pattern rule | Rule ID/revision/schema version, name, rationale, matcher configuration, literal value, scopes, suggestions, examples, origin/author, and fingerprint |
| Match result | Rule revision or query fingerprint, source snapshot, original spans, matching mode, context coverage, and scan completion state |
| Frozen match set | Set ID/version/digest, rule revision, query fingerprint and matcher version, explicit scope, source/report hashes, deterministically ordered occurrence membership, counts, exclusions, and scan/context coverage snapshot |
| Bulk approval | Decision ID/revision, frozen-set digest, whole-set or explicit selected-member references, assessment/action, representative-occurrence references, rationale, reviewer attribution, timestamp, and stale/conflict state |
| Change plan | Plan ID/version/digest, verified inputs, frozen exact edits/expected bytes, decision/rule references, conflicts, validation requirements, destinations, dry-run result and confirmation state |
| Change manifest | Original/output hashes, applied plan reference, exact changes, versions, validation results, re-audit reference, and completion status |

Review timestamps are intentional human-history data and must not be added to otherwise deterministic audit outputs. Exporting the same unchanged review state should produce stable serialized content and ordering.

The F1 technical design must specify canonical serialization and digest derivation for frozen sets, including exact membership and all approval-bound inputs. A bulk decision must resolve to each member's original occurrence anchor without duplicating hundreds of manual decisions. Identical match text alone does not establish identical source identity or approval. Neither pagination nor a later filter change may silently alter a saved set or subset.

F0 itself must compute the audit reference's `report_artifact_sha256`; the value must not be trusted from an imported field or deferred until cleanup exists. It identifies the exact report file, including whitespace, escaping and any BOM. Reformatting the same JSON values therefore produces a different report-artifact digest, while importing identical bytes under another filename produces the same digest. It is distinct from `file.sha256` and the extracted-text hashes. Neither the report digest nor its claimed source hash proves authorship, authenticity, or possession of the original source.

An import that is incomplete, canceled, or over the supported budget must not publish a completed digest or create usable occurrence anchors. F0's import/spike design must account for hashing and retaining original report bytes alongside parsed data, indexes, and presentation mappings.

### 9.2 Identity and offsets

- `Finding.id` currently identifies a rule, not a unique occurrence. It cannot serve alone as a UI key, decision anchor, or deduplication key.
- Current text segments contain a `source` label and an offset array with an EOF entry. Use segment identity plus source hash and exact ranges; do not anchor by a displayed line number alone.
- Current finding locations are Go maps with schema-constrained per-rule shapes and often contain position lists, not start/end spans. A frontend adapter must interpret known evidence shapes. Unknown shapes remain inspectable but non-editable.
- A grouped zero-width pattern can consist of scattered positions. Selecting its finding must not propose deleting everything between its first and last positions.
- Normalization can map multiple original code points to one displayed character. Store mappings to all contributing original spans; never guess an edit range from string lengths.
- Office locators require part/page/object-aware coordinates. Extracted offsets must not be represented as original-file byte offsets when no such mapping exists.

### 9.3 Stable backend failure codes — versioned contract dependency

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| FAIL-01 | P0 / versioned backend contract before F0 implementation | Add required `evidence.failure_code` to each `parser.failure` finding and update the backend model/emitter, schema variants, fixtures, and contract tests together. `parser.failure` remains the detector ID; `failure_code` identifies the reason for failure. |
| FAIL-02 | P0 / versioned backend contract before F0 implementation | Maintain a documented stable code registry. Codes must be selected at the failure boundary using structured conditions, not inferred by parsing exception messages. Preserve existing human-readable diagnostics and decode byte ranges as supplementary evidence. |
| FAIL-03 | P0 / F0 | Keep failure handling deterministic across diagnostic wording changes. Legacy reports without a code remain identifiable as failed reports through an explicit compatibility adapter; retain their original bytes and mark the specific reason as unavailable. Unknown code values display a generic failure and are never treated as success. |

Minimum proposed registry for the technical design:

| Code | Meaning |
| --- | --- |
| `file.not_found` | The requested source does not exist |
| `file.permission_denied` | Source acquisition was denied, including insufficient permission for the required read mode |
| `file.symlink_not_supported` | The input is a symlink rejected by the source policy |
| `file.not_regular` | The input is a directory or another unsupported nonregular file |
| `file.too_large` | The source exceeds the configured acquisition/analysis limit |
| `file.changed_during_read` | The source changed during snapshot acquisition |
| `integrity.no_atime_unavailable` | The platform cannot provide the required no-atime acquisition mechanism |
| `format.unsupported` | Identification completed, but no supported parser exists for the detected format |
| `text.decode_failed` | Strict decoding of the acquired text failed |
| `audit.failed` | A failure has no more specific registered classification; diagnostics remain available |

The exact registry and error-to-code mapping must be ratified in the backend contract change before implementation. It must cover the current failure paths and must not claim that an OS error establishes a more specific cause than the available evidence supports. Once published, code meanings must not be repurposed. The field is a stable namespaced string; consumers must support an unfamiliar value without inferring meaning from its spelling. Future cancellation, adapter, and output failures require their own scoped contracts rather than being mislabeled as parse failures.

This is a requested backend extension, not current behavior. Current schema 1.0 rejects unknown fields. Introduce this through an explicitly reviewed schema revision/compatibility plan and retain legacy import support. Do not silently change the required fields of a frozen schema or synthesize a backend code into retained original evidence. The F0 design must document which contract version it targets and its migration path.

### 9.4 Typed evidence anchors and Occurrence/location-adapter contract

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| LOC-01 | P0 / before UI implementation | Define and review a versioned `EvidenceAnchor` union, its exact-text `Occurrence` variant, and location-adapter interface, including machine-readable types/schema, deterministic identity rules, and executable examples. UI components consume this contract rather than interpreting arbitrary backend location dictionaries themselves. |
| LOC-02 | P0 / F0 | Adapt supported finding locations, user selections and search matches into exact original-source anchors. Preserve every contributing span; distinguish exact, unmapped and invalid locations explicitly. |
| LOC-03 | P0 / F0 | Validate source/segment references, integer bounds, ordering, paired coordinate consistency, and supported evidence shapes before producing an exact occurrence. Invalid or ambiguous mappings must not result in guessed highlights or editable targets. |
| LOC-04 | P0 / before component selection | Prove that a candidate viewer can map source coordinates to display selections and back through the component-independent adapter. Component offsets, DOM positions, line numbers and marker labels must not become authoritative evidence coordinates. |

`EvidenceAnchor` is a discriminated union whose exact schema must be approved before UI implementation. Proposed variants are:

| Variant | Identity and scope | Action eligibility |
| --- | --- | --- |
| Text occurrence | Exact report/segment, original scalar/byte spans and selected-text digest | Inspection/search; future apply only after source/writer verification |
| Structural object | Report/source identity, package part or page/object locator, parser/mapping quality | Structural navigation; no inferred raw-byte deletion |
| Credential | Asset/report identity, manifest/assertion reference and verifier result context | Inspect/evaluate; no automatic stripping action |
| Statistical sample | Detector execution/result reference, exact analyzed-text digest, preprocessing identity and declared sample/source scope | Context navigation and review; no inferred watermark-removal range |
| Legacy unknown | Original finding reference and adapter diagnostics | Inert inspection only |

Do not call a correctly scoped credential or statistical sample an invalid text occurrence. Review decisions may reference any supported variant; text matching produces text occurrences, not substitute statistical detections. Source verification, location exactness and edit eligibility are independent properties.

Minimum `Occurrence` fields for the **exact-text variant** to formalize in the F0 technical design:

| Field | Contract |
| --- | --- |
| `contract_version` | Version of the frontend occurrence/adapter contract, independent of the backend report version |
| `occurrence_id` | Deterministic identity scoped to the report artifact, adapter contract, segment, exact spans and originating finding/selection/query reference; the design specifies canonical serialization and ID derivation |
| `report_artifact_sha256` | F0-computed digest of the exact imported report bytes; mandatory |
| `source_sha256` | Recorded original-source hash or explicit null when unavailable; it is not proof that source bytes were independently verified |
| `origin` | Discriminated reference to a finding, user selection or search result; a finding reference includes report-array index and stable detector ID because the ID alone is not unique |
| `segment` | Report text-segment index and source label for text locations; null only when there is no text anchor. Future office locators must use a distinct contract variant |
| `mapping_status` | `exact`, `unmapped`, or `invalid`; unavailable and malformed mappings must not be conflated |
| `spans` | Ordered list of zero-based, half-open original code-point ranges and corresponding original byte ranges when available. Each entry has explicit coordinate units; byte ranges remain null when no valid original-byte mapping exists |
| `selected_text_sha256` | For exact text mappings, SHA-256 of the unnormalized selected source text encoded as UTF-8; null otherwise. The canonical identity retains span boundaries so different disjoint selections cannot be conflated |
| `diagnostics` | Structured adapter reason codes and safe explanatory text; unknown evidence shapes remain inspectable through their original finding reference |

An exact text occurrence requires a valid segment and at least one nonempty source span. Report-only exactness means an exact mapping within the reported evidence, not independently verified source bytes or edit authorization. `unmapped` and `invalid` occurrences cannot provide a replacement range or an exact-text digest.

The adapter must expose these behaviors, with concrete method signatures settled in the technical design:

1. Accept a validated report reference plus a known finding, selection, or query result and return occurrences with diagnostics; never modify the original report.
2. Interpret each known finding shape explicitly. An inventory position at `p` maps to `[p, p + 1)`. Scattered positions remain separate spans. A token-start location alone cannot justify a token-length highlight; deriving a longer span requires an adapter rule that verifies the corresponding original text.
3. Map between original code points, original bytes, JavaScript UTF-16 indices and display positions without mixing their units. Out-of-range positions and selections inside a surrogate pair or synthetic marker label require an explicit, tested boundary policy.
4. Resolve a display selection back to exact original spans and return their original source text for search/copy. Presentation labels and normalization output must not enter the selected source value.
5. Preserve non-text findings through typed structural/credential/sample anchors without fabricating text spans. Legacy unknown locations can remain `unmapped`; an intentional whole-sample scope is valid evidence, not an invalid text occurrence.

Grouped findings may create multiple occurrences or one occurrence with disjoint spans according to a documented per-rule policy. The policy must preserve membership and distinguish finding, occurrence, and code-point counts. The accepted contract and tests are prerequisites to UI implementation; prototype components may be evaluated during the spike but must not define the contract implicitly.

### 9.5 Required backend extensions

| Dependency | Required by | Outcome |
| --- | --- | --- |
| Stable failure-code registry and emitted field | Versioned contract before F0 implementation | Specific failure states without parsing messages; explicit legacy compatibility if the schema must be revised |
| Versioned mechanism/capability/scope/result contract | F0 design/import; F2 live use | Honest per-mechanism availability/execution/results plus parser, context, writer and validation coverage |
| Declarative matching service and rule schema | F1/F2 | Consistent match semantics, deterministic original spans, cancellation and bounded resource use; F1 report-only matching can run locally against the same contract |
| Review/rule persistence schemas | F1 | Portable files separate from original reports; no source mutation |
| Local adapter | F2 | Read-only audit, source verification, workspace jobs, and explicit file-access boundaries |
| Context parser adapters | F2 | Python/Markdown first, additional languages independently advertised |
| Bounded workspace jobs | F2 | Discovery, exclusions, per-file results, progress, cancellation, and completeness reporting |
| Plan/apply/validate service | F3 | Backend-enforced source checks, exact edits, no-overwrite output, manifests, re-audits |
| DOCX/PDF/ODT parser and locator extensions | F4 | Structural evidence suitable for accurate navigation |
| Profile assessment schema | Profile presentation | Immutable contextual rationale and retained evidence without analyzer suppression |
| C2PA and statistical adapter contracts | F5 | Typed validation/sample results, honest availability and explicit network authorization |
| Format-specific writers | Later than inspection | Separately reviewed office cleanup; never inferred from parsing support |

This table specifies target integration contracts, not currently available service endpoints. Implementation designs must specify their schemas and lifecycle before coding the corresponding frontend controls.

## 10. Security, privacy, and evidence integrity

| ID | Requirement |
| --- | --- |
| SEC-01 | Treat inspected text, filenames, reports, patterns, notes, and explanations as untrusted data. Do not interpret embedded instructions as application or agent authority. |
| SEC-02 | Render source content inertly. HTML, Markdown links/images, script tags, SVG, event handlers, terminal escapes, PDF actions, and office relationships must not execute or fetch resources on inspection. |
| SEC-03 | No telemetry, external models, remote assets, or network requests by default. Imports/exports are user-visible. Do not silently persist full source text in browser storage. |
| SEC-04 | If a local service is used, restrict it to the local session, validate origins, require a session capability/token, and reject arbitrary path access, traversal, and shell commands. Loopback binding alone is insufficient authorization. |
| SEC-05 | Acquisition and writing remain backend responsibilities. Retain the source-byte/timestamp guarantees and document platform limits; never bypass no-atime failures with a browser read advertised as equivalent. |
| SEC-06 | Imported rules cannot enable commands, elevate trust, or execute cleanup. Future regex support requires execution budgets and cancellation rather than unrestricted patterns. |
| SEC-07 | Share/export flows disclose included source text, paths, snippets, reviewer names, and rule literals. Redacted presentation exports must be labeled incomplete; they cannot replace complete forensic evidence for applying changes. |
| SEC-08 | Future agent assistance may suggest classifications only within an explicit opt-in design. Sending source to a remote model requires separate authorization; model output cannot itself authorize modification. |

## 11. Nonfunctional requirements

### 11.1 Accessibility and usability

- All essential navigation, occurrence selection, review labeling, rule editing, and plan approval must be keyboard-operable with visible focus.
- Severity and review state must use text/icons as well as color. Screen readers must announce revealed characters by name/code point without reading a collapsed run thousands of times.
- Large type, zoom, high contrast, multilingual text, and right-to-left prose must remain usable. The evidence view must preserve its explicit logical-order convention.
- UI language must distinguish “flagged,” “reviewed,” “approved for this scope,” and “proposed for removal.” Avoid “clean” as a synonym for verified safety.

### 11.2 Performance and resource limits

Targets below are provisional goals retained for the component spike, not measured performance or approved release budgets. Ratify or explicitly revise them against the measured report/memory envelope before release. Record browser/build and reference hardware in benchmark results; use a reference Linux machine with at least four logical cores and 8 GiB RAM.

| Operation | Target |
| --- | --- |
| Open a normal report from a 1 MiB source | First useful content within 2 seconds |
| Open an 8 MiB source report within the supported report-size budget | First useful content within 10 seconds, with progress/cancel controls |
| Navigate to a loaded occurrence | Within 100 ms for the benchmark corpus |
| Cancel a running search | Acknowledge cancellation within 500 ms |
| Exact search across one 8 MiB text segment | Complete within 3 seconds on the benchmark corpus |

The backend baseline observed 101–266 MiB JSON and roughly 1.1–2.7 GiB child peak RSS for six 8 MiB inputs. Browser memory is additional, and these are observations rather than worst-case bounds; see the [performance guide](../testing/performance.md). Rendering must use windowed/paged views rather than one DOM element per character or occurrence. Search and decoding work must not monopolize the UI thread. Full reports can be much larger than their source because of offset arrays and evidence; measure worst-case fixtures before setting and documenting the F0 report-byte/memory cap. F0 cannot ship with an unbounded report import or a silent truncation policy. Inputs over the declared cap receive an explicit unsupported-size state.

### 11.3 Reliability and compatibility

- Same report bytes, rule revision, scope, and matching-engine version must produce the same ordered matches.
- Report-only inspection should work in the tested desktop browsers without depending on backend OS support. Live acquisition initially inherits the Linux-only limitation.
- Saving and reopening review files must preserve anchors and history. An interrupted save must not corrupt the last explicitly saved review file.
- On cancellation or partial failure, retain completed evidence while visibly marking incomplete coverage. Never publish a success state from a partial cleanup.

### 11.4 Mandatory large-report and Unicode-coordinate spike

Run this spike during F0 technical design **before selecting or committing to a viewer/editor component**. A prototype comparison is allowed; a component decision based only on visual appearance or feature lists is not sufficient. Approve the Occurrence/location-adapter contract before production UI implementation, and use that contract to evaluate candidates.

The spike must cover:

- Real backend reports from 1 MiB and 8 MiB sources plus worst-case supported Unicode inventories, large offset arrays, repeated emoji, and long collapsed runs. Record both source bytes and expanded report bytes.
- End-to-end import costs: raw-byte SHA-256, retention of original report bytes, JSON parsing, model/index construction, windowed rendering, search and cancellation. Measure peak memory and main-thread responsiveness, not only first paint.
- Round-trip selection fidelity for UTF-8/16/32 evidence, BOMs, CRLF, non-BMP emoji, joiners, combining sequences, bidi controls, escaped labels, disjoint spans and ambiguous normalized selections.
- Navigation and selection across viewport boundaries, collapsed content and partial surrogate/marker selections, with the same original coordinates before and after rendering.
- Keyboard operation, inert rendering and bounded work/cancellation for each shortlisted component.
- Typed credential/sample anchors, absent detector coverage and mixed-mechanism reports: no false text selection or edit eligibility when there is no exact text occurrence.

Deliver a reproducible fixture/benchmark harness, coordinate assertions, measured results with browser/build/hardware, an explicit report-size and memory budget, and a component decision record. A candidate with fast rendering but incorrect coordinate round-trips fails the gate. If no candidate meets the integrity and resource requirements, revise the presentation architecture or documented limits and repeat the relevant measurements before selecting a component.

This PRD requires the spike; it does not claim the spike has run or that any component has been selected.

## 12. Acceptance scenarios and test plan

These scenarios are required in addition to existing backend tests. Frontend tests must assert evidence and outcomes, not only that a page renders.

| Test ID | Scenario | Acceptance result |
| --- | --- | --- |
| A01 | Import clean ASCII and normal UTF-8 fixture reports | No fabricated findings; identity, limitations, text, and zero finding counts remain available |
| A02 | Select the deliberate zero-width fixture | 64 selected code points map to exact original bytes; detector facts and the possible-encoding interpretation remain distinct |
| A03 | Select `U+200B` from a marker label | Query contains the source character, not the label text; exact matches and original offsets are correct |
| A04 | UTF-16 input containing BOM, emoji, combining marks and CRLF | Browser selection, code-point spans and original byte spans agree; copying source preserves exact selected text |
| A05 | Search `aba` in `ababa` and search an empty query | Two overlapping matches are reported; empty query is rejected; overlapping removals require resolution |
| A06 | Match across an invisible-character projection and an NFC contraction | All contributing original spans are shown; enclosing visible text is not implicitly targeted; ambiguous edits are blocked |
| A07 | Show emoji, Persian joiners, variation selectors and bidi fixtures | Evidence is visible without calling legitimate uses confirmed watermarks; individual and bulk decisions resolve to exact occurrence anchors |
| A08 | Review the same instruction in a Python docstring, ordinary string and quoted Markdown example | Contexts are accurate where supported, unknown otherwise; distinct labels and actions survive save/reopen |
| A09 | Save a selected literal as a rule and apply it to another report | Match semantics and revision survive export/import; no inherited removal approval |
| A10 | Import two conflicting review decisions, then change source bytes | Conflict is visible; changed-source approvals are stale and cannot authorize edits |
| A11 | Plan removal of selected hidden characters from UTF-8/16/32 source | Only selected bytes change; untouched BOM/line endings remain; original bytes/timestamps are unchanged |
| A12 | Apply against a stale hash, alias destination, existing path or overlapping edits | Application is blocked with a specific reason and no source modification |
| A13 | Proposed code deletion breaks syntax or changes a docstring | Relevant validation/warnings appear; syntax success is not presented as semantic equivalence; no code runs automatically |
| A14 | Import malicious HTML/script text, links, hostile rule descriptions or fake agent instructions | No script, network request, tool action or instruction adoption occurs |
| A15 | Import malformed schema, out-of-range offsets, unknown location shapes or failed audits | Errors/limitations are explicit; unsafe navigation/edit actions are unavailable |
| A16 | Inspect current PDF/DOCX failures or unsupported language contexts | Unsupported coverage is visible; no false “clean” result or context-scoped auto-match |
| A17 | Search a workspace with failed, excluded and canceled files | Coverage counts reconcile; bulk approval binds only the displayed frozen result set |
| A18 | Export a reviewed copy and reopen its manifest | Input/output hashes, exact edits, reviewer decisions, validation, and re-audit results can be traced |
| A19 | Navigate/review using only keyboard and screen reader | Core workflow works with announced states and without reliance on color or hover |
| A20 | Load worst-case supported reports and cancel a search | Published performance/resource budgets are met; no silent truncation or unresponsive cancellation |
| A21 | Import identical report bytes under different filenames, then import a whitespace-reformatted equivalent | Identical bytes yield identical retained `report_artifact_sha256`; reformatting changes that digest; neither operation changes or verifies the recorded source hash |
| A22 | Import coded failures with changed diagnostic wording, then legacy failures without codes and an unfamiliar code | Known-code UI states are unchanged by wording; legacy/unfamiliar codes remain generic failures with diagnostics and no guessed cause |
| A23 | Adapt inventory points, disjoint patterns, token-start locations, non-text findings and malformed coordinates | Exact spans follow the documented adapter policy; no envelope deletion or invented token span; unmapped/invalid results cannot become exact edit targets |
| A24 | Evaluate viewer candidates against the large-report/Unicode-coordinate spike | Reproducible results include import hashing/retention costs and exact round-trips; no component is selected before the integrity and resource gate passes |
| A25 | Tag evidence as a watermark, save a matching rule, then run that rule on another document | The reviewer judgment remains attributed and separate from detector classification; source files do not change; new matches do not inherit removal approval |
| A26 | Dry-run a plan, cancel it, then confirm another plan and change a destination or source hash | Cancellation publishes no derivatives; any plan/input/destination change invalidates confirmation; only an unchanged confirmed plan can write new copies |
| A27 | Review representative occurrences from 684 exact matches across 17 documents and approve the whole set or a subset | One bulk decision binds the displayed rule/query/scope, source/report hashes and exact selected membership; opening or clicking every occurrence is not required; no source files change |
| A28 | Save bulk approval, add a matching file, change a source or rule, and change pagination/filters | New or changed result sets do not inherit approval; stale anchors cannot authorize changes; old membership and decision history remain immutable regardless of the displayed page/filter |
| A29 | Open a scanned workspace with mixed analysis outcomes | A reconciled review queue leads directly to contextual evidence, rule search and frozen-set review; failed, unsupported, skipped and canceled inputs remain visible |
| A30 | Import no structural findings with unavailable or absent statistical execution | UI reports unassessed/unavailable coverage, never a negative vendor watermark result |
| A31 | Inspect a known-good character inside a suspicious sequence and a mixed-mechanism report | Underlying evidence and conflicting assessments remain inspectable; profiles cannot hide independent coverage/results |
| A32 | Inspect discovered credentials with bad binding, untrusted signer and unavailable revocation checks | Discovery, integrity, trust and incomplete checks remain distinct; no universal authenticity verdict |
| A33 | Review a statistical sample result without exact token/byte contributions | Actual result semantics and sample identity displayed; no fabricated score, reveal markers or removal span |
| A34 | Enable a remote detector, then change selected scope or cancel authorization | No content leaves before explicit scope authorization; changed scope needs fresh authorization; reports/logs contain no secrets |
| A35 | Import unknown result/anchor variants and legacy reports without coverage | Safe inspectable unsupported state, preserved raw bytes and no guessed capabilities or editable ranges |
| A36 | Review the same sample under different detector/configuration/trust contexts | Observations retain separate identities/context; UI does not promise identical remote/trust-dependent results |

Reuse [current fixtures](../../tests/fixtures/) and [example report](../../examples/suspicious-report.json) as regression inputs. Add deterministic instruction/context, malicious rendering, overlapping-match, stale-source, and multilingual coordinate fixtures. DOCX/PDF fixture suites become mandatory when F4 is enabled.

## 13. Milestones and release gates

Frontend phases F0–F5 are distinct from historical backend milestones M0–M4 and parent roadmap tracks P0–P7. F0 supports mechanism-aware absence/coverage and declared imported variants; F5 enables actual new adapter integrations when available.

The three-phase product pipeline spans these milestones: backend audit capabilities supply detection; F0–F2 supply evidence review, tagging and reusable rules; F3 introduces explicit apply for text. F4 adds office inspection to the first two product phases, with office apply gated separately. None of the milestone boundaries weakens the detection/review/apply separation.

| Phase | Deliverable | Backend dependencies | Exit gate |
| --- | --- | --- | --- |
| F0 design gate | Versioned mechanism/capability and evidence-anchor/Occurrence contracts, failure-code migration, and completed large-report/Unicode-coordinate spike | Parent P0 / Issue #17 plus legacy report adapter | Adapter contract accepted before UI implementation; spike results accepted before viewer/editor component selection |
| F0 — Evidence viewer | Offline import with retained report digest, typed evidence inspection, text reveal/search and honest per-mechanism failure/coverage | Current Go reports plus supported contract revision; explicit legacy adapter | A01–A05, A07, A14–A16, A19–A24, A30, A33, A35 applicable to imported read-only scope; import/rendering limits published |
| F1 — Review and rules | Collection search, individual/bulk artifact and instruction judgments, frozen match sets, portable sidecars, reusable rules and rule testing | Versioned review/rule/matcher and frozen-set contracts; no live writer required | A06, A08–A10, A14, A25, A27–A28 plus F0 regressions; no implied semantic coverage when unavailable |
| F2 — Live workspace and contexts | Local adapter, source verification, discovery/jobs, workspace triage queue, Python and Markdown context parsing | Backend batch support and context adapters; capability/scope contract | A08, A10, A16–A17, A28–A29; source integrity and local-service access tests |
| F3 — Reviewed text cleanup | Exact deletion plans, mandatory dry run, explicit confirmation, conflict checks, validation, new-file output, manifests, re-audit | Separately reviewed writer/validation services | A11–A14, A18, A26; originals unchanged; stale/ambiguous or unconfirmed plans blocked |
| F4 — Office inspection | DOCX/PDF and later ODT structural navigation/search; optional validated visual mapping | Parent P2/P3 parser evidence and locator contracts | Deterministic office fixtures; correct locations and disclosed extraction gaps |
| F5 — Provenance integrations | Credential validation panels and actual statistical detector results; explicit remote consent | Parent P5 adapters + P0 result/coverage contracts; independent of F3/F4 | A30–A36 and integrity/privacy regressions; no invented scores or removable sample spans |
| Later — Office cleanup and advanced matching | Format-specific editing, additional languages, optional bounded regex | Separate designs and writer-specific tests | Independent approval of behavior and integrity evidence |

F4 inspection and F5 integrations may proceed independently once their backend capabilities are ready; it does not require waiting for or enabling text cleanup. F0–F2 must remain useful without a writer. Review each phase in focused PRs; do not merge an editable UI whose backend integrity checks are placeholders.

## 14. Success measures

Measure through local development tests and explicit usability sessions; these goals do not introduce production telemetry.

- All seeded Unicode/instruction scenarios navigate to the intended original spans, including non-BMP and combining text.
- Reviewers can distinguish detector evidence, reviewer decisions, and proposed actions in every usability scenario.
- A reviewer can select a hidden pattern, find every exact occurrence, and save/reapply it without manually entering offsets.
- A saved pattern replays deterministically across documents and never implies prior removal authorization.
- A reviewer can approve a frozen set of hundreds of exact matches after representative review without hundreds of mandatory per-occurrence interactions; the approval still resolves to every selected original span.
- All cleanup integrity tests preserve original bytes/timestamps and reject stale inputs; all successful exports include a traceable manifest and re-audit outcome.
- Unsupported and skipped content is visible in every coverage summary; no tested failure path becomes a zero-findings success.

## 15. Risks and decisions still required

| Risk or decision | Proposed disposition |
| --- | --- |
| Large reports expand far beyond source size | Run the section 11.4 spike before selecting a viewer/editor, including report hashing and raw-byte retention; publish measured byte/memory caps |
| Visual selection differs from original coordinates | Approve the section 9.4 Occurrence/location-adapter contract before UI implementation and verify round-trips in the component-selection spike |
| Overbroad rules remove legitimate language or examples | Show representative context, coverage and scope; separate rule suggestions from explicit approval of an exact frozen set or subset and from confirmation of the apply plan |
| Syntax highlighting is mistaken for semantic parsing | Mark context capabilities; parser failures fall back to unknown, not guessed comments/docstrings |
| Stale review history creates false trust | Bind decisions to source snapshots and immutable rule revisions; require reconciliation |
| Standalone viewer versus packaged desktop experience | Ship offline report inspection first; select bridge/packaging in an implementation design before F2 |
| Frontend framework and editor component | Select the viewer/editor only after the mandatory spike demonstrates coordinate fidelity, inert rendering, keyboard access and bounded large-report behavior; no framework is mandated here |
| Failure messages become an accidental API | Use a versioned backend failure registry and explicit schema revision/legacy adapter, never message-text inference |
| Cross-platform forensic acquisition | Retain Linux restrictions until a separately tested backend design can provide equivalent guarantees |
| Repository test execution after cleanup | Keep non-executing validation default; define explicit command approval and isolation before adding automated project test runs |
| Office cleanup mistaken for simple text replacement | Separate inspection capability from writer capability and maintain format-specific release gates |

## 16. Definition of done for implementation planning

This is a **draft rewrite requiring product review** alongside the parent PRD. It identifies the F0 design work but does not claim those contracts, the component spike or frontend implementation are complete. Preserve existing accepted safeguards while reviewing the new mechanism/coverage requirements.

Before production UI implementation, the F0 design must supply and review:

1. The stable backend failure-code and mechanism/capability/result contracts, with an explicit schema revision and legacy import path.
2. Exact imported-report SHA-256 computation, raw-byte retention, export binding, and the associated memory/resource budget.
3. The versioned evidence-anchor/Occurrence adapter schema, concrete interface, deterministic identity rules, exact coordinate examples and valid sample/credential/object anchors from section 9.4.
4. Completed large-report/Unicode-coordinate spike results and a justified viewer/editor component decision made after those results.
5. The inert-rendering approach, bounded import/search strategy, keyboard interaction design, and acceptance tests against current backend fixtures.

No DOCX/PDF/ODT parser, credential verifier, cleanup engine or statistical detector is implied by accepting this PRD. Every subsequent design decision must preserve traceability to exact retained evidence and declared scope; only supported exact mappings can authorize future structural edits.

## 17. Revision record

Version 1.1 is a draft product rewrite requested after introducing the parent signal-auditor PRD. It replaces the Python-adapter assumption with the current Go baseline, makes mechanism/coverage distinctions first-class, adds credential/statistical/profile journeys and requirements, and expands the location contract into typed evidence anchors. C2PA and statistical integrations have an independently gated F5 track. Performance targets require ratification against the measured resource envelope. New validation failures block export; the former optional unvalidated-export exception is deferred to a separate product decision.

The exact v1.0 bytes are archived in [frontend-PRD-v1.0.md](frontend-PRD-v1.0.md), SHA-256 `9dd5e6aa1966795c31439f1d456ac4933ccb654f7b589e1cdfac6c54a28671f0`. Existing requirement IDs and safeguards are retained where applicable; A30–A36 and COV/CRED/STAT/PROF requirements add the new coverage. V1.0 approval does not imply approval of this draft or completion of its implementation gates.
