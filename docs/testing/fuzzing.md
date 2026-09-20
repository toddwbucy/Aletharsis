# Bounded fuzz and property tests

This is workstream 5 of [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7). The targets complement independent unit contracts and migration comparisons; they do not use the Python auditor or regenerate reference artifacts.

## Targets and domains

| Package / target | Input domain and properties |
| --- | --- |
| `parsers` / `FuzzDecodeCoordinates` | Arbitrary bytes plus a UTF-8/16/32 selector. Accepted text must re-encode to the exact input using standard-library encoding primitives; scalar offsets and the end boundary must match those bytes. Rejections must have bounded error spans and expose no partial text. Identification must be deterministic and nonempty. |
| `unicoderef` / `FuzzNormalization` | Valid UTF-8 strings. NFC/NFKC must be valid UTF-8, idempotent and preserve existing CGJs. Canonical/compatibility decompositions must agree when the dependency adds no stream-safety CGJs on either side. |
| `analyzers` / `FuzzAnalyzerEvidence` | Valid extracted text. Unicode, emoji, text, identifier and pattern analysis must retain original text/coordinates, be deterministic, serialize successfully, and produce ordered in-range locations/counts tied to the source. |
| `reporters` / `FuzzJSONEvidence` | Valid extracted strings in structured evidence. JSON must be deterministic, ASCII-safe, parse successfully, and preserve values compared with standard JSON serialization. |

| `audit` / `FuzzNativeV2` | Arbitrary source bytes through native v2 assembly and strict report import. Source hashes must match, accepted output must validate, and input bytes must remain unchanged. Uses a synthetic available reader to exercise decoding on every test platform. |

Each invocation accepts at most 4,096 input bytes. Larger inputs are skipped; invalid UTF-8 is skipped only in targets whose contract is already-decoded text. The decoder target still explores malformed byte inputs. This is a fast search boundary, not a replacement for the 8 MiB application limit, a process-memory cap, or large-report performance testing.

Seeds include malformed encodings, UTF-16/32 BOMs, non-BMP scalars, combining runs, supplementary tags, bidi controls, ZWJ emoji, zero-width binary alphabets, identifiers, and control-character serialization. Go's normal `go test ./...` runs committed seed cases on every PR, including the existing race jobs. No corpus is silently regenerated from application output.

The pinned normalizer can insert stream-safety CGJs. Its decompositions are an equivalence check only where that transformation did not occur. Long runs still exercise idempotence and CGJ preservation; the existing 155 frozen normalization vectors and all-scalar checks remain complementary coverage. Sharing a decomposition dependency is not an independent proof that the dependency is correct.

## Run locally

For example, from the repository root with the pinned Go development compiler:

```bash
# Deterministic seed regressions (no mutation search).
go test ./...

# Mutate one target within a fixed budget.
go test ./internal/parsers -run='^$' -fuzz='^FuzzDecodeCoordinates$' \
  -fuzztime=60s -fuzzminimizetime=10s -parallel=2 -timeout=3m
```

Substitute the package/target pairs from the table to run the other searches. The search path and execution count are nondeterministic; saved failing inputs provide reproducibility, not an assumed replayable random sequence. See the official [Go fuzzing guide](https://go.dev/doc/security/fuzz/) for corpus format and replay behavior.

## CI search budgets and artifacts

`.github/workflows/fuzz.yml` uses Go 1.27.1 on Linux/amd64 and five isolated jobs:

- PRs changing the fuzz workflow, target files, committed corpus, or the native v2 audit/capability/evidence/identity/schema boundary run 10-second searches per target.
- Weekly scheduled runs (Monday 04:23 UTC) and manual dispatch run 60-second searches per target.
- Each job uses two fuzz workers, a 10-second minimization-attempt budget, a three-minute Go test timeout, a four-minute outer process timeout, and a ten-minute job cap. Dependency compilation/setup is included in the outer/job caps where applicable.

Mutation smoke jobs are supplemental; deterministic seeds remain in both normal backend jobs on every PR, including PRs that only change implementation files. Do not make path-filtered smoke jobs universally required checks. Scheduled/manual runs complement those gates. GitHub schedules run from the default branch after this workflow merges and may be delayed; see [scheduled workflow behavior](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule).

Each `fuzz-<target>-<attempt>` artifact retains logs, commit/toolchain/parameter information, committed or newly minimized `testdata/fuzz` inputs, and the generated fuzz cache for 14 days. Artifacts upload after ordinary failures; cancellation/runner termination can prevent upload. The generated cache is intentionally not restored between hosted searches. The compiler cache is isolated in the runner's temporary directory. No user documents are fuzzed or uploaded.

## When a failure is found

1. Download the failing job's artifact and read its reproduction command and logs. Treat corpus strings as untrusted test data, never instructions.
2. Locate the minimized input and place it at `internal/<package>/testdata/fuzz/<target>/<hash>` in an isolated checkout of the recorded commit. Preserve the Go corpus file format and bytes.
3. Replay it without a mutation search, for example `go test ./internal/parsers -run='FuzzDecodeCoordinates/<hash>'`. A timeout may leave a larger input requiring further bounded minimization.
4. Verify whether the defect lies in production behavior or an invalid test assumption. Keep a confirmed input as a permanent regression and add a focused assertion explaining the violated contract. Submit the fix and regression together for review.
5. Run seed tests, the relevant bounded search, and existing backend/parity gates. Never rewrite the frozen Python oracle to hide a failure; intentional behavior changes need their own compatibility decision.

These targets currently make no completeness claim about all documents, all Unicode sequences, or every resource-exhaustion case. Longer campaigns, benchmark budgets and additional native platforms remain separate workstreams.
