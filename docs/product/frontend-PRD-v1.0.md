# Aletharsis Frontend — Evidence Review Workbench

| Attribute | Value |
| --- | --- |
| Status | Approved and frozen; baseline for F0 technical design |
| Document version | 1.0 |
| Date | 2026-09-19 |
| Backend baseline | Aletharsis 0.1.0, M0/M1 implementation; report schema 1.0 |
| Product owner | Todd W. Bucy |
| Approval | Product owner approved the freeze of the v0.4 requirements as baseline v1.0 |
| Scope | Inspection, search, human review, reusable patterns, and controlled derivative-file cleanup |

## 1. Product definition

This is the frozen product requirements baseline. Technical specifications define implementations within these requirements; any material requirement change must be recorded explicitly in a new PRD revision rather than introduced silently through a spec. Freezing this PRD does not freeze the backend report schema or declare its outstanding failure-code dependency complete.

Aletharsis will provide a local evidence review workbench for examining hidden or unusual content in text, source code, Markdown, and, as backend support becomes available, office documents. Users will reveal artifacts, select content, search for related occurrences, record whether instructions are legitimate in context, reuse matching patterns, and preview explicit changes to a new copy.

The frontend builds on the existing read-only forensic auditor. Detection, reviewer judgment, and modification remain separate operations. The audit command remains read-only; cleanup is a new capability with its own backend contract and release gate.

The central invariant is that every interpretation must remain traceable to exact source evidence. A viewer component, normalized preview, saved rule, or human decision must never become a substitute for that evidence or its coordinates.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable evidence and structural patterns; intent and provenance may require additional investigation.

This warning must be accessible from every report. The product must not call a file “safe,” “watermark-free,” or “AI-generated” merely because of its findings or their absence.

### 1.1 Problem

The current CLI can identify Unicode artifacts and patterns but leaves users to interpret large lists of positions. Users cannot easily inspect the exact text, find all instances of a selected sequence, distinguish a legitimate agent instruction from an unauthorized one, preserve review decisions, or reuse a pattern across files. Automatic stripping would risk damaging language, code, test fixtures, and document structure.

### 1.2 Desired outcome

A user can move through this workflow without losing the link between an interpretation and its source evidence:

**Detect → reveal → select → search → inspect context → classify → save/reuse a rule → dry run → confirm → write and re-audit a new copy.**

The first frontend release delivers inspection without waiting for an editing engine or office-format parsers. Later releases enable review and cleanup only when the required backend guarantees exist.

### 1.3 Three-phase product pipeline

**Detection establishes what is present. Evidence review establishes what it means in context. Explicit apply determines what, if anything, should be changed.**

| Product phase | Responsibility | Durable output | Source-file boundary |
| --- | --- | --- | --- |
| 1. Detection | Scan supported documents read-only for Unicode artifacts, watermark-like patterns, identifiers, metadata and structure as parser capabilities allow | An evidence report with source identity, exact locations, findings and limitations | Never edits, normalizes, sanitizes or otherwise rewrites source files |
| 2. Evidence review and tagging | Present evidence in a lightweight local web GUI; inspect source-mapped occurrences and representative context, apply human judgment individually or to a frozen result set, and save reusable rules | Separate review decisions and versioned declarative rules linked to the retained report and exact source evidence | Tagging, approving an interpretation, and saving/applying a matching rule do not change document content |
| 3. Explicit apply | Produce a dry run from selected reviewed changes, obtain confirmation for that exact plan, then generate and validate new copies | Derivative files, change manifest, validation results and re-audit reports | Never overwrites originals or existing destinations; no derivative is published before confirmation |

Phase boundaries are product invariants, not merely separate screens. A detection result cannot itself authorize an edit; a human classification or reusable rule cannot bypass the apply phase. A user may stop after detection or review and still retain useful evidence. Decisions can be promoted into reusable rules with explicit matching scope and suggested action. A reviewer may approve an exact frozen set of matches across documents after reviewing representative occurrences and the set's scope; separate manual approval of every occurrence is not required. Later matches never inherit that approval from the rule.

Reversibility comes from preserving original files, retaining decision/plan history, and generating separate derivatives. The product does not promise that deleted content can be reconstructed from a cleaned copy alone. These three product phases are distinct from the incremental F0–F4 delivery milestones in section 13.

### 1.4 North-star workflow — workspace triage

The eventual primary interaction is workspace triage, not opening isolated report files. A user selects a directory or workspace, receives an evidence-backed review queue, establishes the meaning of recurring patterns through representative context, and carries that judgment across a precisely bounded result set without hundreds of redundant approval clicks.

```text
Select workspace
      ↓
Detect across included documents
      ↓
Review queue (illustrative counts)
  82 documents
  21 require review
   5 failed
   3 unsupported
  53 no reported findings
      ↓
Open a document with evidence already highlighted
      ↓
Review representative occurrences: “This is a watermark”
      ↓
Save or associate a versioned rule
      ↓
Search the workspace for that exact pattern
      ↓
Freeze results: 17 documents, 684 matching occurrences
      ↓
Review the rule, scope, counts, exclusions and representative context
      ↓
Approve all 684 matches, or an explicit subset, for proposed removal
      ↓
Dry run → confirm exact plan → write reviewed copies
      ↓
Manifest, validation results and re-audit
```

Queue counts must reconcile without double-counting unsupported files as other failures. Skipped and canceled files, when present, must also remain visible; “no reported findings” is not a safety certification. A file discovered tomorrow can match the same rule, but its occurrences appear as **new matches — not previously approved**.

F0's `import report.json` workflow is an implementation stepping stone that proves evidence identity, coordinates and inspection. F1 brings collection review and frozen-set decisions; F2 supplies live workspace discovery, scanning and triage. Component and contract choices must support that progression rather than making single-report import the permanent product boundary.

## 2. Users and primary jobs

| User | Job | Required result |
| --- | --- | --- |
| Document reviewer | Understand hidden characters, identifiers, and provenance artifacts | Explainable findings linked to exact evidence |
| Codebase maintainer | Review comments, docstrings, Markdown, and other text that may guide agents | Contextual instruction decisions without treating source content as executable authority |
| Security reviewer | Investigate repeated suspicious sequences across files | Reproducible searches, explicit scopes, and a reusable pattern library |
| Editor or repository owner | Remove specifically approved artifacts | Previewed changes, validation, a separate output copy, and a change manifest |
| Independent reviewer | Assess another person's findings and proposed changes | Portable original report, decisions, rules, and a reviewable diff |

No account or cloud service is required for these jobs.

## 3. Goals and non-goals

### 3.1 Goals

- Make invisible and unusual content visible and selectable without changing it.
- Search an entire document for a selection, including selections containing only invisible characters.
- Extend searches to an explicitly selected collection or workspace when supported.
- Support source code as ordinary Unicode text immediately; introduce reliable language-context boundaries incrementally.
- Record legitimate, unauthorized, quoted, and uncertain instruction judgments separately from machine findings.
- Save portable, versioned patterns for use on other documents.
- Reduce repeated manual review through representative context and explicit approval of deterministic frozen match sets, while preserving exact evidence anchors for every member.
- Make every proposed removal traceable to exact original spans and explicit review.
- Preserve original files and audit evidence throughout inspection and cleanup.
- Present unsupported operations and incomplete coverage honestly.

### 3.2 Non-goals

- General-purpose IDE, collaborative document editor, or office-suite replacement.
- Automatic determination of author intent, maliciousness, AI authorship, or statistical watermark provenance.
- Automatically trusting instructions found in inspected files, rule descriptions, or imported reports.
- Automatically deleting matches because of severity, a saved rule, or a previous decision on another document.
- In-place source cleanup, automatic execution of inspected code, or blanket removal of invisible Unicode.
- Cloud synchronization, accounts, shared live editing, and organization identity verification in the initial releases.
- Universal encoding support, complete language parsing, OCR, or lossless DOCX/PDF rewriting in the initial releases.

## 4. Verified backend baseline

This table describes the checked implementation, not a future capability promise. The implementation was submitted in backend PRs [#1](https://github.com/toddwbucy/Aletharsis/pull/1), [#2](https://github.com/toddwbucy/Aletharsis/pull/2), and [#3](https://github.com/toddwbucy/Aletharsis/pull/3); this document does not assume their merge status.

| Area | Available in M0/M1 | Frontend implication or missing capability |
| --- | --- | --- |
| Reports | Deterministic JSON, schema 1.0, console output | Start with existing report import; no new detector is necessary |
| Failure identity | `parser.failure`, exit code 4, exception type/message, and decode offsets when applicable | No stable machine-readable failure-reason code exists yet; add it before schema 1.0 freezes as specified in section 9.3 |
| Identity | Supplied path, extension, format/MIME and basis, size, SHA-256, parser | Show identity; a report's recorded hash is not independent proof of its authenticity |
| Text | Literal source, encoding, BOM, line endings, full character-to-byte offset map | Text/source-code inspection is possible now |
| Encodings | UTF-8; BOM-marked UTF-16/32, both byte orders | Display encoding; preserve it in future edits |
| Unicode | Inventories, positions, context samples, normalization indicators and hashes | Reveal characters and navigate all offsets, not just sampled contexts |
| Patterns | ZWSP/ZWNJ candidates, periodic insertion, long tags/selectors | Explain the supplied heuristic; do not turn it into a confirmed watermark |
| Other findings | UUID-shaped identifiers, labels, Base64 candidates, mixed scripts, line endings | Inspect content without assigning unproven intent |
| Source code | Accepted as Unicode text regardless of extension when identification permits | No syntax tree, comment/docstring boundaries, or instruction-authority analysis exists |
| Markdown/HTML/XML/JSON | Inspected as literal source | Rendered content, escapes, entities, and semantic regions are not interpreted |
| Metadata/structure | Common model exists; text structure is populated | Structured document metadata extraction is not implemented |
| DOCX/PDF | Content identification only; auditing returns unsupported-format failure | Do not present an empty report as completed analysis |
| File access | Single regular file, maximum 8 MiB; Linux no-atime reads; symlinks rejected | Live frontend auditing must retain this backend path and its failures |
| Directories | Not implemented | Workspace discovery, recursion, exclusions, and aggregate coverage require backend work |
| Review/editing | Not implemented | Decisions, custom matching, saved rules, cleanup and validation require new contracts |
| API | CLI and Python modules; no frontend service or desktop bridge | A live local adapter is future work, not an existing endpoint |

Authoritative local references: [models](../../src/aletharsis/models.py), [audit pipeline](../../src/aletharsis/audit.py), [text parser](../../src/aletharsis/parsers/text.py), [report schema](../../schemas/report.schema.json), and [current limitations](../../README.md).

## 5. Delivery model and priorities

Requirements use **P0** for required functionality within the assigned phase, **P1** for follow-up functionality, and **P2** for deferred exploration. P0 does not mean every feature ships in the first viewer release.

The initial viewer is a bundled local web application that opens existing JSON reports. It requires no server, network connection, or external fonts/scripts. A self-contained HTML export is desirable in F0; it must be generated with an inert data-embedding strategy. The authoritative evidence remains the JSON report.

Live auditing and cleanup later use an explicitly launched local Python adapter. The browser does not acquire arbitrary filesystem access and is not the authority for source validation or patch application. Framework and packaging choices belong in a separate implementation design; this PRD does not select a UI framework or prescribe an HTTP API.

Opening a source directly through a browser cannot inherit the backend's timestamp guarantees. F0 therefore imports reports rather than claiming to perform forensic acquisition. Future acquisition must go through the backend's integrity-preserving reader. Importing an existing report does not touch the source document.

## 6. Information architecture

### 6.1 Primary surfaces

| Surface | Purpose |
| --- | --- |
| Documents | Imported reports or selected workspace files, analysis state, coverage, and review progress |
| Inspector | Findings, revealed content, and exact evidence in linked panels |
| Search | Query definition, scope, matches, exclusions, and progress |
| Review | Human decisions, rationale, conflicts, stale decisions, and history |
| Pattern library | Create, test, version, import, export, and disable reusable rules |
| Change review | Proposed edits, before/after comparison, conflicts, validation, and output destination |

### 6.2 Inspector layout

```text
Document identity | audit status | coverage | source verification state
------------------------------------------------------------------------
Findings / search results | Revealed source            | Evidence / review
Filters and counts       | Line and byte navigation   | Code points
Grouped occurrences      | Selectable markers         | Exact spans
Next / previous result   | Optional context coloring  | Explanation
                         |                            | Decision + rationale
------------------------------------------------------------------------
Search selection | Save pattern | Review selected occurrences | Plan change
```

Smaller windows may use tabs or drawers without dropping evidence fields. No critical action may require a hover interaction.

### 6.3 Terminology

- **Finding:** a machine-reported observation or pattern, often grouping multiple positions.
- **Occurrence:** a particular matched span or set of spans in a source snapshot.
- **Decision:** a human judgment about an occurrence, with scope and rationale.
- **Rule:** a reusable definition of what to match and where to match it.
- **Change plan:** an explicit collection of exact edits proposed for a verified source snapshot.
- **Reviewed copy:** an exported derivative; the phrase does not imply that all artifacts were removed or that the content is safe.

## 7. User journeys

### J1 — Investigate a revealed sequence

The reviewer imports an audit report, selects a zero-width finding, and sees every occurrence highlighted. They select a sequence of revealed characters, choose exact search, and inspect the result set and representative matches in the document. They save the query as a named rule, retaining the literal code-point sequence rather than the visible marker labels. Every match remains individually inspectable without requiring a separate approval click.

### J2 — Review agent-facing instructions in source

A maintainer opens Python source or Markdown. They select an instruction even if no analyzer flagged it, search its exact wording, and inspect each context. One occurrence is approved guidance, one is a quoted attack example, and another is unauthorized. The maintainer records different labels and reasons. Only the unauthorized occurrence is added to a removal plan. A parser-derived context label helps locate it but does not establish its authority.

### J3 — Reuse a saved pattern across a workspace

A reviewer imports a versioned rule, previews its scope and match mode, and runs it over selected files. Results show completed, failed, excluded, and unsupported files separately. Matches in comments, docstrings, and prose can be filtered where semantic coverage exists. After reviewing representative occurrences and the displayed scope, the reviewer freezes 684 matches across 17 documents and approves that entire set, or an explicit subset, for proposed removal. The approval references the exact rule revision, query, source identities and set membership. Future matches do not inherit it; no files change until a dry-run plan is separately confirmed in the apply phase.

### J4 — Produce a reviewed text copy

The reviewer opens a change plan, compares original and proposed content, and resolves overlapping edits. The backend verifies the original hash, checks exact bytes, runs available non-executing validation, and writes a new destination without overwriting an existing path. The copy is re-audited. The reviewer receives the derivative, a change manifest, and before/after findings.

### J5 — Encounter a changed or unsupported document

A report is imported without its source: it remains inspectable but is labeled “source not verified,” and applying changes is unavailable. If the source later has a different hash, decisions are shown as stale and require reconciliation. A current DOCX/PDF failure shows unsupported coverage; it never becomes a “no findings” success state.

## 8. Functional requirements

### 8.1 Import, identity, and coverage

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| IMP-01 | P0 / F0 | Import a schema-1.0 JSON report; validate structure and defensively validate ranges, counts, offset monotonicity, and references before navigation. Present malformed data as an import error. |
| IMP-02 | P0 / F0 | Show tool/schema versions, file identity, MIME identification basis, audit status, parser, hashes, and limitations. Never interpret exit codes 1–3 as parser failures; use the report's status and failure evidence. |
| IMP-03 | P0 / F0 | Distinguish report loaded, source unverified, source verified, source changed, audit failed, and unsupported format. Report import alone never establishes source verification. |
| IMP-04 | P0 / F0 | Reject unsupported major schema versions with an explanation. Preserve the imported report unchanged and do not silently coerce unknown findings into a known detector. |
| IMP-05 | P0 / F1 | Open a collection of imported reports, preserving independent source identities and review states. This is collection inspection, not recursive filesystem auditing. |
| IMP-06 | P0 / F2 | For a live workspace, expose selected roots, inclusion/exclusion patterns, symlink policy, discovered files, completed files, failures, cancellations, and skipped files. Never equate skipped files with clean files. |
| IMP-07 | P0 / F0 | Compute and retain `report_artifact_sha256` from the exact complete imported report bytes before decoding, parsing, or reserialization. Bind the loaded report, occurrence anchors, and subsequent exports to that digest. Preserve original report bytes for the active session or an explicitly saved evidence bundle; do not silently persist them in browser storage. |
| IMP-08 | P0 / F0 | Interpret backend failures using the stable `failure_code` contract in section 9.3, not exception classes or message text. Missing legacy codes or unfamiliar codes produce a generic failed-audit state with the original diagnostic available, without guessing a more specific cause. |

Current filtered CLI reports do not include a dedicated machine-readable scope field; their limitations contain a view notice. F0 must preserve that notice, label counts as “findings in this report,” and never claim complete analyzer coverage. F2 requires a versioned scope/capability field for reliable coverage reporting.

### 8.2 Evidence visualization and selection

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| VIS-01 | P0 / F0 | Provide source, revealed-character, and escaped views. Labels such as `⟦U+200B ZERO WIDTH SPACE⟧` represent source characters; they are not inserted into the evidence text. |
| VIS-02 | P0 / F0 | Selecting a finding navigates all its occurrences. Selecting an occurrence highlights the corresponding source positions and exposes code point, Unicode name, byte offsets, and context. |
| VIS-03 | P0 / F0 | Distinguish character counts, occurrence counts, finding counts, and visible result counts. A grouped finding with 64 character positions is not automatically 64 instructions or one contiguous edit. |
| VIS-04 | P0 / F0 | Collapse long repetitive runs with an exact count and expandable content. Collapsing must not lose searchable characters or imply that unreviewed matches were inspected. |
| VIS-05 | P0 / F0 | Support text selection across visible and invisible content; selections crossing marker labels resolve to original code points. Let users copy original text, escaped text, or a code-point list as distinct actions. |
| VIS-06 | P0 / F0 | Show normalization changes and available hashes without replacing source content. Differentiate raw-byte SHA-256 from UTF-8 hashes of extracted/comparison text. |
| VIS-07 | P0 / F0 | Preserve logical order in the evidence view and visibly expose bidi controls. A reading-oriented preview must not replace the authoritative escaped/offset view. |
| VIS-08 | P1 / F2 | Offer an original-byte view only when bytes have been acquired and verified. Do not claim an imported JSON string is a verified byte dump of the original file. |

User-facing line/column navigation may be one-based, but the evidence inspector must identify all coordinate units. Python code-point offsets, JavaScript UTF-16 indices, grapheme clusters, byte offsets, and displayed marker positions are different coordinate systems. The implementation must maintain explicit mappings; emoji and combining characters must not shift selections.

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
| RULE-09 | P0 / F1 | Allow a reviewer to bulk-classify or propose an action for all or an explicit subset of a deterministic frozen rule-match result set. Before approval, display the rule revision, query fingerprint, scope, source identities/count, occurrence count, exclusions and context coverage. New or changed matches are not included in the previous approval. |

A rule is not a document-specific approval. The library may contain an organization's recognized guidance, but any proposal to make a rule an automatic trust policy requires a separate product decision and is outside this scope.

**Bulk review of a frozen result set:** A reviewer may approve an assessment, retain, investigate, or propose removal for all or a selected subset of matches after reviewing the rule, scope, count, representative context, exclusions and source identities. The UI must support representative review without requiring every occurrence to be opened or individually approved. Every occurrence remains accessible for closer inspection, and incomplete scan/context coverage must be visible.

An action such as **“Approve all 684 matches in this result set for removal”** records a human approval for change planning, not an immediate write. The approval is bound to the immutable rule revision, query fingerprint and scope, source hashes, report-artifact hashes, exact occurrence membership, and frozen-set digest. A selected subset is recorded explicitly rather than as a live filter or “whatever matches later.” Record the representative occurrences reviewed, reviewer attribution and rationale alongside the decision; inspecting representatives must not be represented as individually inspecting every member.

Later matching documents or revised rules require a new frozen result set and approval. A saved rule never carries universal removal authorization. Invalid mappings, unresolved decision conflicts and stale inputs still block dependent change plans; bulk approval does not waive those checks or the separate dry-run confirmation required to write copies.

### 8.6 Change planning and reviewed copies

Cleanup first supports explicit deletions of selected spans in supported text encodings. General rewriting, automatic normalization, and office-object removal are later work.

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| EDIT-01 | P0 / F3 | Create a plan bound to the exact source hash, encoding, parser version, exact byte ranges/expected bytes, decisions, and rule revisions. Classification/search actions never apply edits. |
| EDIT-02 | P0 / F3 | Show readable and escaped before/after views, exact removed bytes, affected lines, and proposed output name. Retain the full surrounding context needed to understand code changes. |
| EDIT-03 | P0 / F3 | Deduplicate identical edits. Block overlapping or conflicting edits until resolved. Apply original-coordinate changes without position drift and reject ambiguous transformed-match mappings. |
| EDIT-04 | P0 / F3 | Verify the acquired snapshot hash and every expected byte range before applying. If the source changed, reject the plan and require re-audit. Browser-supplied offsets alone are never trusted. |
| EDIT-05 | P0 / F3 | Preserve all untargeted bytes, including encoding, BOM, line endings and surrounding whitespace. Editing a comparison view must not silently normalize an entire file. |
| EDIT-06 | P0 / F3 | Never overwrite the original or any existing destination, including aliases through symlinks/hard links. Publish a completed derivative without leaving an apparently successful partial output after failure. |
| EDIT-07 | P0 / F3 | Perform available non-executing syntax/format validation. New parse errors block normal export; a separately acknowledged unvalidated derivative may be exported with the failure recorded. Unsupported validators must be shown as unavailable. |
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

## 9. Data and integration contracts

### 9.1 Preserve the current audit report

The current schema rejects unknown fields in structured objects. Do not insert frontend decisions, rules, or UI state into schema-1.0 reports. Store separately versioned sidecar documents or use a deliberately versioned report migration.

| Entity | Minimum contract |
| --- | --- |
| Audit reference | Exact report-artifact digest, recorded source SHA-256, schema/tool versions, parser, and scope/verification state |
| Occurrence anchor | Audit reference, segment identity/index, zero-based half-open code-point ranges, byte ranges where available, selected-content digest, and optional detector/rule references |
| Review decision | Decision ID/revision, individual occurrence anchor or explicit frozen-set/subset reference, artifact assessment and/or instruction-authority assessment, proposed action, scope, rationale, reviewer attribution, timestamp, superseded-decision reference, and stale/conflict state |
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
- Current finding locations are flexible dictionaries and often contain position lists, not start/end spans. A frontend adapter must interpret known evidence shapes. Unknown shapes remain inspectable but non-editable.
- A grouped zero-width pattern can consist of scattered positions. Selecting its finding must not propose deleting everything between its first and last positions.
- Normalization can map multiple original code points to one displayed character. Store mappings to all contributing original spans; never guess an edit range from string lengths.
- Office locators require part/page/object-aware coordinates. Extracted offsets must not be represented as original-file byte offsets when no such mapping exists.

### 9.3 Stable backend failure codes — pre-freeze dependency

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| FAIL-01 | P0 / before schema 1.0 freeze | Add required `evidence.failure_code` to each `parser.failure` finding and update the backend model/emitter, schema variants, fixtures, and contract tests together. `parser.failure` remains the detector ID; `failure_code` identifies the reason for failure. |
| FAIL-02 | P0 / before schema 1.0 freeze | Maintain a documented stable code registry. Codes must be selected at the failure boundary using structured conditions, not inferred by parsing exception messages. Preserve existing human-readable diagnostics and decode byte ranges as supplementary evidence. |
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

This is a requested backend extension, not current behavior. Land it before schema 1.0 freezes if possible; if the schema freezes first, introduce the required field under a deliberate schema revision and retain legacy import support. Do not silently change the required fields of a frozen schema or synthesize a backend code into retained original evidence. The F0 design must document which contract version it targets and its migration path.

### 9.4 Frontend Occurrence and location-adapter contract

| ID | Priority / phase | Requirement |
| --- | --- | --- |
| LOC-01 | P0 / before UI implementation | Define and review a versioned frontend `Occurrence` contract and location-adapter interface, including machine-readable types/schema, deterministic identity rules, and executable examples. UI components consume this contract rather than interpreting arbitrary backend location dictionaries themselves. |
| LOC-02 | P0 / F0 | Adapt supported finding locations, user selections and search matches into exact original-source anchors. Preserve every contributing span; distinguish exact, unmapped and invalid locations explicitly. |
| LOC-03 | P0 / F0 | Validate source/segment references, integer bounds, ordering, paired coordinate consistency, and supported evidence shapes before producing an exact occurrence. Invalid or ambiguous mappings must not result in guessed highlights or editable targets. |
| LOC-04 | P0 / before component selection | Prove that a candidate viewer can map source coordinates to display selections and back through the component-independent adapter. Component offsets, DOM positions, line numbers and marker labels must not become authoritative evidence coordinates. |

Minimum `Occurrence` fields to formalize in the F0 technical design:

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
5. Preserve non-text findings without fabricating text anchors. Metadata-only or unsupported locations can remain `unmapped` while their evidence is still reviewable.

Grouped findings may create multiple occurrences or one occurrence with disjoint spans according to a documented per-rule policy. The policy must preserve membership and distinguish finding, occurrence, and code-point counts. The accepted contract and tests are prerequisites to UI implementation; prototype components may be evaluated during the spike but must not define the contract implicitly.

### 9.5 Required backend extensions

| Dependency | Required by | Outcome |
| --- | --- | --- |
| Stable failure-code registry and emitted field | Before schema 1.0 freeze; consumed by F0 | Specific failure states without parsing messages; explicit legacy compatibility if the schema must be revised |
| Versioned capability/scope descriptor | F2 | Reliable parser, analyzer, language-context, writer, and validation availability |
| Declarative matching service and rule schema | F1/F2 | Consistent match semantics, deterministic original spans, cancellation and bounded resource use; F1 report-only matching can run locally against the same contract |
| Review/rule persistence schemas | F1 | Portable files separate from original reports; no source mutation |
| Local adapter | F2 | Read-only audit, source verification, workspace jobs, and explicit file-access boundaries |
| Context parser adapters | F2 | Python/Markdown first, additional languages independently advertised |
| Bounded workspace jobs | F2 | Discovery, exclusions, per-file results, progress, cancellation, and completeness reporting |
| Plan/apply/validate service | F3 | Backend-enforced source checks, exact edits, no-overwrite output, manifests, re-audits |
| DOCX/PDF parser and locator extensions | F4 | Structural evidence suitable for accurate navigation |
| Format-specific writers | Later than inspection | Separately reviewed office cleanup; never inferred from parsing support |

No endpoints or CLI commands in this table exist yet. Implementation designs must specify their schemas and lifecycle before coding the corresponding frontend controls.

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

Targets below are release acceptance targets, not measured current performance. Record browser/build and reference hardware in benchmark results; use a reference Linux machine with at least four logical cores and 8 GiB RAM.

| Operation | Target |
| --- | --- |
| Open a normal report from a 1 MiB source | First useful content within 2 seconds |
| Open an 8 MiB source report within the supported report-size budget | First useful content within 10 seconds, with progress/cancel controls |
| Navigate to a loaded occurrence | Within 100 ms for the benchmark corpus |
| Cancel a running search | Acknowledge cancellation within 500 ms |
| Exact search across one 8 MiB text segment | Complete within 3 seconds on the benchmark corpus |

Rendering must use windowed/paged views rather than one DOM element per character or occurrence. Search and decoding work must not monopolize the UI thread. Full reports can be much larger than their source because of offset arrays and evidence; measure worst-case fixtures before setting and documenting the F0 report-byte/memory cap. F0 cannot ship with an unbounded report import or a silent truncation policy. Inputs over the declared cap receive an explicit unsupported-size state.

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

Reuse [current fixtures](../../tests/fixtures/) and [example report](../../examples/suspicious-report.json) as regression inputs. Add deterministic instruction/context, malicious rendering, overlapping-match, stale-source, and multilingual coordinate fixtures. DOCX/PDF fixture suites become mandatory when F4 is enabled.

## 13. Milestones and release gates

Frontend phases are deliberately distinct from backend milestones M0–M4.

The three-phase product pipeline spans these milestones: backend audit capabilities supply detection; F0–F2 supply evidence review, tagging and reusable rules; F3 introduces explicit apply for text. F4 adds office inspection to the first two product phases, with office apply gated separately. None of the milestone boundaries weakens the detection/review/apply separation.

| Phase | Deliverable | Backend dependencies | Exit gate |
| --- | --- | --- | --- |
| F0 design gate | Versioned Occurrence/location-adapter contract, failure-code dependency plan, and completed large-report/Unicode-coordinate spike | Existing reports and explicit pre-freeze backend contract work | Adapter contract accepted before UI implementation; spike results accepted before viewer/editor component selection |
| F0 — Evidence viewer | Offline report import with retained artifact SHA-256, three-panel inspection, revealed/escaped text, exact selection/search, clear failure/coverage states | M0/M1 reports plus failure-code contract or explicitly versioned migration; frontend compatibility adapter | A01–A05, A07, A14–A16, A19–A24 applicable to read-only scope; import/rendering limits published |
| F1 — Review and rules | Collection search, individual/bulk artifact and instruction judgments, frozen match sets, portable sidecars, reusable rules and rule testing | Versioned review/rule/matcher and frozen-set contracts; no live writer required | A06, A08–A10, A14, A25, A27–A28 plus F0 regressions; no implied semantic coverage when unavailable |
| F2 — Live workspace and contexts | Local adapter, source verification, discovery/jobs, workspace triage queue, Python and Markdown context parsing | Backend batch support and context adapters; capability/scope contract | A08, A10, A16–A17, A28–A29; source integrity and local-service access tests |
| F3 — Reviewed text cleanup | Exact deletion plans, mandatory dry run, explicit confirmation, conflict checks, validation, new-file output, manifests, re-audit | Separately reviewed writer/validation services | A11–A14, A18, A26; originals unchanged; stale/ambiguous or unconfirmed plans blocked |
| F4 — Office inspection | DOCX/PDF structural navigation and format-aware search; optional validated visual mapping | Backend M2/M3 parser evidence and locator contracts | Deterministic office fixtures; correct locations and disclosed extraction gaps |
| Later — Office cleanup and advanced matching | Format-specific editing, additional languages, optional bounded regex | Separate designs and writer-specific tests | Independent approval of behavior and integrity evidence |

F4 inspection may proceed independently once its parsers are ready; it does not require waiting for or enabling text cleanup. F0–F2 must remain useful without a writer. Review each phase in focused PRs; do not merge an editable UI whose backend integrity checks are placeholders.

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
| Failure messages become an accidental API | Add stable backend codes before schema 1.0 freeze; if already frozen, use an explicit revision and legacy adapter, never message-text inference |
| Cross-platform forensic acquisition | Retain Linux restrictions until a separately tested backend design can provide equivalent guarantees |
| Repository test execution after cleanup | Keep non-executing validation default; define explicit command approval and isolation before adding automated project test runs |
| Office cleanup mistaken for simple text replacement | Separate inspection capability from writer capability and maintain format-specific release gates |

## 16. Definition of done for implementation planning

This revision incorporates the four product-review changes and is ready to enter **F0 technical design**. That status authorizes design work, not a claim that the spike, backend failure-code change, or frontend implementation is complete.

Before production UI implementation, the F0 design must supply and review:

1. The stable backend failure-code contract and a pre-schema-freeze delivery plan, or an explicit schema revision and legacy import path if necessary.
2. Exact imported-report SHA-256 computation, raw-byte retention, export binding, and the associated memory/resource budget.
3. The versioned Occurrence/location-adapter schema, concrete interface, deterministic identity rules and executable coordinate examples from section 9.4.
4. Completed large-report/Unicode-coordinate spike results and a justified viewer/editor component decision made after those results.
5. The inert-rendering approach, bounded import/search strategy, keyboard interaction design, and acceptance tests against current backend fixtures.

No DOCX/PDF parser, cleanup engine, or statistical detector is implied by accepting this PRD. Every subsequent design decision remains subject to the invariant that interpretations, selections and actions map back to exact retained source evidence.
