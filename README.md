# Aletharsis

**A local-first forensic auditor for document signals.** Aletharsis inspects files for
information that ordinary viewing can miss — invisible Unicode, structured character
patterns, identifiers and provenance-bearing content — and keeps observable evidence
separate from interpretation.

> A suspicious artifact is not necessarily a watermark. Aletharsis reports observable
> evidence and structural patterns; intent and provenance may require additional
> investigation.

## What it does

It reads a file without modifying it and produces a report describing what is actually
there: exact characters and their positions, structural facts about the document, and
findings classified by what kind of claim they support. A finding is an observation, not a
verdict. The tool never calls a file clean, watermark-free or AI-generated, and an absence
of findings is never evidence that nothing is present.

Three principles shape the design:

- **Evidence is preserved, not summarised.** Locations are exact, original bytes are
  untouched, and output is deterministic and reproducible.
- **Missing coverage stays visible.** What could not be analysed is declared rather than
  silently omitted, so an empty result cannot be mistaken for a clean one.
- **Detection, review and modification are separate authorities.** Auditing is read-only.
  Any future change to a document is a distinct, explicitly confirmed operation producing
  a new file.

## Current state

Pre-1.0 and under active development. The shipped interface is a Go command-line tool;
auditing currently requires Linux, because acquisition depends on timestamp-preserving
reads and fails closed rather than falling back.

Capabilities are changing frequently enough that listing them here would be misleading.
**[ROADMAP.md](ROADMAP.md) is the current statement of what is implemented, what is
planned, and what is deliberately out of scope.** It also records the acceptance gates
each capability has to clear.

```bash
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
./bin/aletharsis --help
```

Audit exits 0–3 summarize finding severity; exit 4 indicates an operational failure.
Report 1.0 has no coverage state; judged from report status alone, its audit exits 4 only on failed status.
Report 2.0 exits 4 on any non-completed status. Inspect report status and coverage as well as the exit code. `--help`
documents the current commands and flags.

## Documentation

This file is an introduction and is descriptive only. Authoritative documentation lives in
the product requirements and specifications:

| Document | Purpose |
| --- | --- |
| [Parent PRD](docs/product/aletharsis-PRD.md) | Product scope, invariants and the phase boundaries between detection, review and modification |
| [Frontend PRD](docs/product/frontend-PRD.md) | The planned evidence review workbench |
| [ROADMAP.md](ROADMAP.md) | Delivery tracks, reuse gates and current status |
| [docs/specs/](docs/specs/) | Normative contracts for evidence, reports, formats and the CLI |
| [docs/adr/](docs/adr/) | Accepted architectural decisions |
| [docs/text-audit-reference.md](docs/text-audit-reference.md) | Detector thresholds and coordinate semantics |
| [docs/testing/](docs/testing/) | CI, fuzzing, performance and platform validation |

Where this README and a specification disagree, the specification is correct.

## Scope and limits

Aletharsis performs structural analysis of what a file contains. It does not determine
authorship, intent or provenance, and it does not detect statistical model-output
watermarks — that would require a known algorithm and usually a key. Absence of structural
findings says nothing about whether such a watermark exists.

Audits make no network requests, run no external programs, and never execute inspected
content. Reports can contain the full source text, so treat them with the same sensitivity
as the documents they describe.

## Contributing

Work lands as bounded pull requests carrying the validation evidence for the relevant
gate. Specifications and architectural decisions are accepted before implementation.
[CLAUDE.md](CLAUDE.md) describes the repository layout, build and test commands, and the
invariants that must not be eroded.

## License

[Apache License 2.0](LICENSE). Bundled Unicode data carries its
[Unicode license](internal/analyzers/data/UNICODE-LICENSE.txt).
