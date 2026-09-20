# Go v2 execution records, native catalog and failure boundaries

This increment follows the [identity primitives](go-v2-identity.md) accepted in
PR #25 and implements the next bounded part of [#17](https://github.com/toddwbucy/Aletharsis/issues/17).
The [accepted schema/import contract](report-v2-wire-and-import.md) remains the
wire authority. Production CLI output is still schema 1.0.

## Implemented records

`internal/evidence/v2` defines capability/implementation identities, availability,
participation, explicit limits, native/unknown configuration variants, scopes,
exclusions, execution states and stage-specific diagnostics. These are the
execution-model records, not a complete report envelope. Artifact, anchor, finding
and result assembly and cross-record reference assignment remain coordinator work.
No arbitrary upstream payload map or statistical/credential result is introduced.

Producer `MarshalJSON` methods validate records before serialization. The bounded
`DecodeCapability`, `DecodeConfig`, `DecodeExecution` and `DecodeDiagnostic` functions
reject unknown properties, missing required fields (including nullable ones),
invalid UTF-8/surrogates, duplicate decoded keys, wrong-case aliases, invalid enums,
unsafe integers and invalid variant combinations. Use these functions rather than
plain `json.Unmarshal` at an input boundary. The record decoder accepts integral
JSON spellings such as `1.0`; booleans are not integers. Strings are never normalized.

The per-record import budget is 1 MiB input/output, 32,768 JSON nodes and depth 32.
This budget covers small planning records; it is not a whole-report or workbench
import budget. Producer records are constructed by trusted Go code; caller-side
budgets remain necessary before constructing large in-memory structures.

Native configurations are hashed with the accepted identity domain and checked
against their serialized settings. Unknown configuration identity remains explicit.
No secret material, secret digest or unreviewed settings dictionary is accepted.
Scopes preserve ordered half-open regions. Execution validation rejects overlapping
coverage/exclusions, false completed coverage, partial states without exclusions
and diagnostics, and unrun/failed/canceled states claiming analyzed scope.

Record validation cannot establish that a referenced artifact exists, that a region
is within its actual extent, that exclusions account for all omitted evidence, or
that a result/diagnostic reference resolves. Those checks need the complete report
and remain mandatory in the coordinator. It also cannot authenticate imported
producer claims. Unknown diagnostic codes remain failure information, not success.

## Native capability catalog

`internal/capability.Native` returns fresh descriptors sorted by stable ID:

- Acquisition and literal-text parsing.
- Unicode inventory, emoji inventory, text analysis, pattern analysis, identifier
  analysis and metadata analysis.
- Disabled/unavailable declarations for C2PA carrier extraction, C2PA verification
  and the Claude statistical watermark detector.

The native audit's six analyzer registrations share catalog IDs; a test prevents
missing, duplicate or unused native analyzer declarations. Catalog revision is
`aletharsis.native-text/2`; implementation version is supplied by the owning service.
Revision 2 native descriptors record digests for the embedded Unicode category,
emoji, message and limitation data. Declaration-only descriptors remain revision 1.
Returned slices, pointers and nested structures do not share mutable catalog state.

Availability describes implementation readiness, not success on a particular input.
Linux has the compiled no-atime reader; other platforms explicitly declare that
reader unavailable. Parsing/analyzers can still operate on already acquired bytes
or evidence, so their implementation availability is independent of the reader.
Acquisition uses the `*` format marker to mean an otherwise policy-eligible regular
file of any identified format; parsing support remains limited to literal text.
Unimplemented declarations publish empty supported-scope lists rather than inventing
supported formats. There are no model endpoints, heuristic detectors or SDK calls.

The source byte bound is declared for acquisition and parsing. Analyzer limit fields
are null because these operations do not independently enforce an input byte limit;
they depend on bounded acquisition. A UTF-16/32 source's decoded UTF-8 byte length
is a distinct quantity. Null does not assert unlimited safe operation or complete
resource isolation. The [native coordinator](go-v2-native-assembly.md) adds report
budgets and compiled data identities. Hard time/heap limits, expanded-container
budgets and per-adapter resource accounting remain separate gates.

Catalog construction reads no PATH entries, preferences, credentials or files and
contacts no services. `NonExecutionReason` takes explicit prerequisite, support and
policy decisions and applies the accepted precedence: disabled, unavailable,
prerequisite failure, unsupported input, policy denial. A nil reason means only that
these checks permit consideration; it does not authorize execution of an imported
capability or install a detector. The later coordinator must dispatch only its
code-owned registered operations.

## Typed native failures

`internal/failure.Error` carries a stable code, stage and wrapped cause. Error text
and `errors.Is/As` remain intact. Classification uses typed OS/decoder conditions
or conditions established at the native boundary, never matching human prose.

| Boundary | Code |
| --- | --- |
| Missing source / denied access | `file.not_found` / `file.permission_denied` |
| Verified nonregular source | `file.not_regular` |
| Pre-read or bounded-read limit exceeded | `file.too_large` |
| Snapshot changed during acquisition | `file.changed_during_read` |
| Other open/stat/read error | `file.io_failed` |
| Compiled platform lacks the verified reader | `integrity.no_atime_unavailable` |
| Identified format has no supported parser | `format.unsupported` |
| Typed strict decoder error | `text.decode_failed` |
| Other parser failure without a more specific typed condition | `audit.failed` |

`EPERM` remains permission denial. `ELOOP` does not prove a final-component symlink;
unsupported-operation errno alone does not establish why no-atime failed. Both
remain generic I/O failures without stronger evidence. No fallback read or new
path-following probe is introduced. Existing source open flags, limit enforcement,
partial-read discard and timestamp checks are unchanged.

`audit.Inspect(path, limit)` returns an internal `Outcome` containing the legacy
report and typed failure, if any. `Outcome.Diagnostic` converts the retained cause
to a validated native v2 diagnostic using coordinator-supplied references/scope;
it does not derive a code from the legacy report message. A successful audit has
no failure diagnostic. The existing `Run`/`WithLimit` entry points still return the
same schema-1 report. No `failure_code` is silently inserted into schema 1.0.

CLI output-delivery diagnostics, aggregate v2 state/exit behavior and complete
execution histories are deferred to the coordinator/emitter increment. This code
is not a general panic recovery or external-sidecar failure handler.

## Validation and next gate

Go tests decode/re-encode all 40 capability records, 40 execution records and three
diagnostics in the eight accepted fixtures, preserving their JSON values. Mutation
tests reject unsafe or contradictory records. Native tests verify catalog isolation,
platform availability, nonexecution precedence and analyzer/catalog correspondence.
Fault injections assert stable codes while retaining existing no-retry, bounded-read,
source-identity and error-chain checks. The record decoder has a bounded fuzz target.

```bash
go test -race ./internal/evidence/v2 ./internal/capability ./internal/failure ./internal/audit
go test -run '^$' -fuzz FuzzRecordDecode -fuzztime=10s -parallel=2 ./internal/evidence/v2
```

The next increment must assemble artifacts/anchors/results, assign deterministic
references, validate cross-record coverage and identity, and expose opt-in v2 JSON
and human coverage reporting. Emitted-report schema/semantic integration and
resource/consumer checks remain release gates. #17, Epic #21 G1 and #7 remain open;
no C2PA/statistical integration or default output-version switch is implied.
