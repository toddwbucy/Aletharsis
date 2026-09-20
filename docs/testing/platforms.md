# Platform and distribution verification

Workstream 7 of [Issue #7](https://github.com/toddwbucy/Aletharsis/issues/7) distinguishes compilation, native execution and verified source acquisition. A successful build is not a forensic integrity guarantee.

## Validation matrix

| Target | Cross-build | Native validation | Auditing status |
| --- | --- | --- | --- |
| Linux amd64 | CGO disabled, Go 1.27.1 | Ubuntu 24.04; Go 1.24.0 and 1.27.1; full backend gates | Supported subject to file ownership, no-atime capability and filesystem semantics |
| Linux arm64 | CGO disabled, Go 1.27.1 | Ubuntu 24.04 arm64; Go 1.27.1; same full backend gates | Same Linux reader; native CI validates this architecture |
| Darwin arm64 | CGO disabled, Go 1.27.1 | macOS 15; portable Go tests and built/installed executable refusal checks | Source acquisition unsupported; exits 4 |
| Windows amd64 | CGO disabled, Go 1.27.1 | Windows Server 2025; portable Go tests and built/installed executable refusal checks | Source acquisition unsupported; exits 4 |

Other targets have no CI validation claim. These runner/filesystem samples do not establish behavior on every filesystem, mount configuration, network share or operating-system release. Runner labels follow the [GitHub-hosted runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners); native jobs assert Go's OS/architecture and retain the environment rather than inferring native execution from a cross-build.

The existing amd64 job names are preserved. The additional job is `Backend / Go 1.27.1 / arm64`. It runs the full unit/race/coverage suite, 193 Linux executable integration cases, schema/reference-tool checks, 37 frozen comparisons and 210 frozen seeded cases. No Python auditor is installed. Per-package Linux-only skips on macOS/Windows remain visible in JSON test logs; those skips are not counted as successful Linux acquisition tests.

`.github/workflows/platforms.yml` has separate cross-build and native refusal jobs. Cross-builds use `CGO_ENABLED=0`, record target/build information, file format and SHA-256, and never execute foreign binaries. They retain metadata rather than publishing binaries as supported releases. Native jobs run the portable Go suite and `scripts/check_distribution.py`; they do not run the Linux integration suite and report it as passing through skips.

## Build and install contract

From a fresh checkout, install the pinned development Go toolchain and test-only Python dependencies, then run:

```bash
python -m pip install -r scripts/requirements-ci.txt
python scripts/check_distribution.py --output artifacts/distribution.json
```

The script builds with `go build -trimpath` and installs with `go install -trimpath ./cmd/aletharsis` into a new temporary absolute `GOBIN`. It executes both resulting binaries directly with an empty PATH, checks help/version, and audits disposable generated fixtures twice in every view under both schema 1.0 and opt-in schema 2.0. It never selects an auditor from PATH or imports the Python auditor. All build/install invocations have a 180-second timeout; each candidate invocation has a 10-second timeout. Ordinary failures surface with diagnostics in the job log; the completion artifact is written only after all checks succeed.

On Linux, 48 built/installed cases cover clean ASCII, supplementary emoji/ZWSP/CRLF coordinates, invalid encoding, source digests, deterministic output, process exit statuses and live-schema validation. The larger independent integration/parity suites cover additional detection and failure contracts.

On macOS/Windows, 80 cases cover the same existing files plus missing paths and directories. Every audit/view must return exit 4, no source digest/size/parser and empty evidence. Schema 1.0 retains its `parser.failure` diagnostic. Schema 2.0 instead declares acquisition `not_run` with `integrity.no_atime_unavailable`, all downstream operations `not_run`, and no findings/results: an unavailable reader must not be reported as having executed. Both wire schemas and v2 semantic linkage are checked. Source bytes and nanosecond atime/mtime/ctime (as exposed by the host) are checked around candidate execution. Test validation reads occur after timestamp comparisons to avoid attributing the test's own reads to the auditor.

CI retains test logs and completed distribution evidence for 14 days. Workflow job limits bound the overall native runs; compilation and candidate invocations have their own limits. No user documents are used. macOS/Windows jobs use Python 3.14.7 only for schema/process validation, not the Python application or Unicode oracle. Python 3.12.13 has no matching macOS/Windows installers in the setup-python catalog; Linux parity retains that exact oracle version. Source dependencies and frozen evidence must remain unmodified. Git attributes also disable line-ending conversion for the embedded Go Unicode tables: category rows are byte-sensitive and a CR suffix would change classification on Windows checkouts.

## Manual compilation

On Linux, for each target pair `linux/amd64`, `linux/arm64`, `darwin/arm64`, `windows/amd64`:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o candidate ./cmd/aletharsis
go version -m candidate
file candidate
sha256sum candidate
```

Set `GOOS`/`GOARCH` for the desired pair; the example emits a Linux arm64 executable. Do not run it on an incompatible host. For native installation, `go install -trimpath ./cmd/aletharsis` writes to Go's default binary directory unless an absolute `GOBIN` is set. Put that directory on PATH for interactive use; use `aletharsis.exe` on Windows. Installation on macOS/Windows currently provides help/version and explicit failure reports, **not supported document auditing**.

## Remaining prerequisites (tracked in Issue #7)

macOS and Windows readers remain unimplemented. Separate implementation PRs must establish timestamp-preserving, read-only acquisition, regular-file checks, symlink/reparse-point handling, bounded reads, race/change detection and explicit permission/capability failures. They must document the actual filesystem/privilege assumptions; a silent ordinary-read fallback or restoration of timestamps by writing is unacceptable.

After a reader exists, replace that target's refusal expectation deliberately and run native parity, exact-coordinate, live-schema, permissions, no-overwrite, source-integrity, fault and executable tests on its supported filesystem combinations. Keep refusal tests for unavailable acquisition capabilities. Windows ACL/reparse semantics and macOS timestamp capabilities require native tests; POSIX mode bits or a Linux test pass are not substitutes.

This PR does not implement those readers, sign/package releases, add installers, alter branch protection, or close Issue #7. The tracker retains the reader-dependent acceptance items explicitly. Build/install verification here validates source-based distribution only.
