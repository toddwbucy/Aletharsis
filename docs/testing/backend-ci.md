# Backend CI foundation

This guide covers the CI foundation, focused unit contracts, and compiled-CLI integration workstreams of [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7). The acquisition fault tests are described below; fuzzing, performance, and platform validation remain separate workstreams.

## Checks and scope

`.github/workflows/backend.yml` runs for pull requests, pushes to main, and manual dispatch. Linux/amd64 jobs use Ubuntu 24.04 and exact Go versions:

- **Backend / Go 1.24.0** checks the minimum compiler declared in `go.mod`.
- **Backend / Go 1.27.1** checks the pinned development compiler.

Each job checks Go formatting, module integrity, `go vet`, uncached tests, combined internal-package statement coverage, race tests, and a built executable. Python 3.12.13 / Unicode 15.0.0 runs retained Python tests, 37 frozen report comparisons (including hashes and repeated-output determinism), and 210 seeded differential cases. The workflow installs pinned test dependencies and the retained Python application solely to exercise its installed-entry-point test; it never regenerates references. Go comparisons explicitly execute `bin/aletharsis`, avoiding ambiguity with the Python entry point. `GOTOOLCHAIN=local` prevents an automatic compiler upgrade from defeating minimum-version testing.

Python is a temporary testing oracle, not a dependency of the Go executable. Retirement of the Python application is separate work after these gates are established. Frozen reports, schemas, fixtures, and Unicode data remain useful to Go-only tests.

The executable integration suite validates every supported Go CLI view against the live `schemas/report.schema.json` contract. Frozen reference schemas remain immutable migration artifacts, not substitutes for the live contract. Neither parity nor statement coverage proves independent correctness; there is no percentage gate.

## Reproduction

Use a supported Linux filesystem and a user allowed to perform `O_NOATIME` reads of the test inputs. Missing no-atime capability is a failure, not a silent relaxed read. Install Go 1.24.0 or 1.27.1 and Python 3.12.13, then run from the repository root:

```bash
python3.12 -m venv .venv
. .venv/bin/activate
python -m pip install -r scripts/requirements-ci.txt
python -m pip install --no-deps --no-build-isolation -e .
export PYTHONPATH=src PYTHONHASHSEED=0 GOTOOLCHAIN=local
mkdir -p artifacts bin
git ls-files -z '*.go' | xargs -0 gofmt -l
go mod download
go mod verify
go vet ./...
go test -count=1 -timeout=2m -coverpkg=./internal/... -coverprofile=artifacts/coverage.out ./...
go tool cover -func=artifacts/coverage.out
go test -count=1 -race -timeout=2m ./...
go build -trimpath -o bin/aletharsis ./cmd/aletharsis
python -m pytest -q integration --aletharsis-binary=bin/aletharsis
python -m pytest -q
python scripts/check_parity.py bin/aletharsis
python scripts/check_differential.py bin/aletharsis
```

The formatting command must print no paths. CI enforces that requirement. CI additionally wraps long commands in GNU `timeout`, uses Bash pipeline failure propagation, and caps each job at 20 minutes. The comparison scripts also cap individual candidate executions at 30 seconds.

## Evidence and failures

Each job retains an artifact named `backend-go-<version>-<attempt>` for 14 days, including toolchain information, Go test/race JSON logs, coverage profile/summary, Python test results, and comparison diagnostics produced before a failure. Upload runs after ordinary failures; cancellation or abrupt runner termination can prevent collection. No audited user documents, application secrets, or release binaries are uploaded. The workflow uses read-only repository permissions and does not persist checkout credentials.

Tests may halt a job before later gates run. A red job is never evidence that unexecuted gates passed. Frozen-artifact integrity checks run after ordinary failures as well.

## Version and merge policy

Update development Go, Python dependency pins, and action commit pins through reviewed PRs. Retain the exact minimum Go job unless `go.mod` changes deliberately. The minimum compiler job is a compatibility check, not a recommendation to deploy an old toolchain. Updating Python or Unicode oracle versions requires a separate compatibility decision; never regenerate reference output simply to silence a failure.

Actions are pinned to immutable commits from the official [checkout](https://github.com/actions/checkout), [setup-go](https://github.com/actions/setup-go), [setup-python](https://github.com/actions/setup-python), and [upload-artifact](https://github.com/actions/upload-artifact) repositories. Review upstream release changes before refreshing pins.

After observing successful hosted runs, propose both job names above as required checks on main. On initial inspection, GitHub's classic branch-protection endpoint reported main as unprotected. This PR does not change branch protection or repository rulesets; enabling enforcement is a separate repository-policy action. If a merge queue is later enabled, add and validate its event trigger before requiring these checks there.

Windows/macOS readers remain unsupported, and cross-builds do not establish runtime integrity. Native platform validation, fuzz schedules, benchmarks, and resource budgets remain open in Issue #7.

## Direct unit contracts

The focused unit suite supplements migration parity with independently stated expectations:

| Tests | Contract exercised |
| --- | --- |
| `internal/parsers/text_test.go` | Literal UTF-8/16/32 bytes, BOM retention, exact scalar/byte/end coordinates, malformed encoding spans, no partial decoded evidence, literal markup and line endings |
| `internal/parsers/identify_test.go` | Signatures versus filename hints, ZIP/OOXML member requirements, supported MIME hints, binary-control density below/at/above the cutoff |
| `internal/analyzers/patterns_test.go` | Count, minority, adjacency, spacing and contiguous-run boundaries; context locations; legitimate-language/emoji negative controls with inventory retained |
| `internal/analyzers/evidence_test.go` | Explicit normalization results and independent SHA-256 values, unchanged source coordinates, UUID/Base64 boundaries, source-code emoji and keycaps |
| `internal/evidence/model_test.go` | Severity/failed-audit exit precedence, summary reset, end-boundary location mapping |
| `internal/reporters/report_test.go` | ASCII-safe lossless JSON, deterministic key ordering, serialization failures, escaped console evidence |

Run these with `go test ./internal/parsers ./internal/analyzers ./internal/evidence ./internal/reporters`, or use the full CI commands above. Expected decoding bytes and normalization strings are written explicitly; the new tests do not import Python, read golden reports, or generate expected output by calling the implementation under test. Pattern input generators construct counts/gaps, while expected detection boundaries are stated independently.

Negative controls establish behavior for those inputs, not a guarantee that all legitimate Unicode is free of suspicious patterns. Statement coverage is diagnostic, not proof of branch coverage or correctness. Fuzzing, resource characterization and platform validation remain separate Issue #7 workstreams.


## Compiled executable and live report contract

`integration/` launches only the explicit `--aletharsis-binary` candidate. There is no PATH fallback, no in-process `cli.Run` call, and no import of the Python auditor. Missing candidates fail setup. This test harness uses the existing pinned pytest/jsonschema test dependencies; it can remain after retiring the Python application.

The original executable-contract suite contains 188 Linux cases; acquisition checks add five more below:

- All 37 frozen inputs through audit, unicode, metadata, and structure: live-schema validation, complete retained evidence, expected finding subsets, recalculated summaries, process exit status, and byte-identical repeated output.
- Spaces, Unicode and leading-hyphen paths, explicit end-of-options, usage errors, help/version, and JSON versus human-readable output.
- Structured verbose logs confined to stderr; malformed/missing sources yield schema-valid failed reports instead of successful partial evidence.
- New reports match stdout JSON and use mode 0600 even under umask 000. Existing reports, sources, hardlinks and symlinks are preserved on rejected writes.
- Nonexistent output parents, stdout write failures via `/dev/full`, and deterministic partial-file cleanup using a child-only `RLIMIT_FSIZE` limit. Source bytes and nanosecond timestamps are checked around representative success/failure operations.

Every candidate process has a 10-second timeout; CI caps the suite at 180 seconds and retains its JUnit report and console diagnostics with the other artifacts. Run on Linux: native non-Linux acquisition remains unsupported. The forced file-size limit applies only to the child process, not the test runner. Close-only failures and acquisition races are not exercised by this suite and remain candidates for the fault-injection workstream.

Migration comparisons retain the documented implementation-version/finding-order/native-error-prose exceptions; strict-schema validation itself has no such exclusions. New path/permission/error tests use directly stated contracts independently of the Python reports. Do not relax the schema or rewrite frozen reports to make the executable pass.


## Acquisition integrity and deterministic faults

`internal/audit/faults_linux_test.go` uses small private, per-call dependency seams around opening a snapshot and invoking its reader. Production acquisition still uses the same syscall flags, bounded read, before/after stat checks, and failure reporting. No exported API or mutable global hook is introduced.

The tests verify:

- EACCES, EPERM, unsupported no-atime-style opening, and symlink-loop errors are propagated after exactly one open attempt. The required read-only/no-atime/no-follow/nonblocking flags are asserted; fallback opening is forbidden.
- First/second stat failures and a read that returns bytes plus EIO discard partial data. Every successfully opened descriptor is closed exactly once, including preflight failures.
- Size, modification-time and change-time differences independently reject a snapshot. Synthetic stat pairs avoid timestamp-resolution assumptions. A real-file hook also performs a deliberate same-size external edit during a read, with explicit distinct timestamps, to exercise rejection without sleeps.
- Oversized/nonregular preflight failures do not read content; accepted sizes and limit-plus-one enforcement are tested. A reader that grows beyond its initial size cannot bypass the read bound.
- Acquisition failures publish no source digest, size, parser, or extracted text, even if a failing reader returned partial bytes.

`internal/cli/concurrent_test.go` runs 16 independent input/output pairs, including malformed inputs, under the race detector. It checks each report against its own source identity/content, repeats the audit for deterministic output, and verifies source stat snapshots and bytes remain unchanged.

`integration/test_acquisition.py` exercises native directory, FIFO, Unix-domain socket, symlink, and denied-permission inputs through the compiled binary and live schema. Its child timeout makes a blocking FIFO open a failure. Run on Linux with local filesystem support for those fixtures. A sandbox that forbids Unix-domain sockets cannot validate that case; do not interpret such a setup failure as an auditor result. The mode-000 permission test explicitly skips root, which can bypass the mode; injected EACCES/EPERM cases still run. Hosted CI runs as its ordinary unprivileged runner user.

Denied `O_NOATIME`/unsupported filesystem behavior is injected deterministically; it is not a claim that all filesystem implementations have been tested. Actual source-byte/timestamp checks cover representative native successes and failures. The deliberate external-edit test changes its own disposable fixture and checks detection, not preservation of bytes changed by another actor. Close-only report-output failures remain an untested follow-up; read-side close calls are checked for resource cleanup. These tests do not promise detection of every possible concurrent modification.
