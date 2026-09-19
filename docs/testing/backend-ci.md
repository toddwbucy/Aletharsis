# Backend CI foundation

This implements the first workstream of [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7). It automates existing correctness and migration tests; it does not complete the remaining parser, executable-contract, fault-injection, fuzz, performance, or platform workstreams.

## Checks and scope

`.github/workflows/backend.yml` runs for pull requests, pushes to main, and manual dispatch. Linux/amd64 jobs use Ubuntu 24.04 and exact Go versions:

- **Backend / Go 1.24.0** checks the minimum compiler declared in `go.mod`.
- **Backend / Go 1.27.1** checks the pinned development compiler.

Each job checks Go formatting, module integrity, `go vet`, uncached tests, combined internal-package statement coverage, race tests, and a built executable. Python 3.12.13 / Unicode 15.0.0 runs retained Python tests, 37 frozen report comparisons (including hashes and repeated-output determinism), and 210 seeded differential cases. The workflow installs pinned test dependencies and the retained Python application solely to exercise its installed-entry-point test; it never regenerates references. Go comparisons explicitly execute `bin/aletharsis`, avoiding ambiguity with the Python entry point. `GOTOOLCHAIN=local` prevents an automatic compiler upgrade from defeating minimum-version testing.

Python is a temporary testing oracle, not a dependency of the Go executable. Retirement of the Python application is separate work after these gates are established. Frozen reports, schemas, fixtures, and Unicode data remain useful to Go-only tests.

The existing Python suite validates the report schema, but this first CI PR does not yet automate strict-schema validation of every Go CLI view. That belongs to workstream 3. Neither parity nor statement coverage proves independent correctness; there is no percentage gate in this initial workflow.

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
