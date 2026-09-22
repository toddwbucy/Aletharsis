# Bounded local validation

`scripts/run_bounded.py` is the shared launcher for new trusted development
commands. It requires Linux, a user systemd manager and cgroup v2 memory controls.
It is not an audit-time detector adapter or a sandbox for adversarial commands.
Existing pinned C2PA study launchers are historical reproduction artifacts; this
change does not rewrite their recipes or retroactively strengthen their evidence.

The default hard limit is **1024 MiB (1 GiB)** for the command and descendants,
with swap disabled. `--memory-mib` changes it explicitly. Before executing the
command, an in-service guard reads its own `memory.max` and `memory.swap.max` and
requires the exact requested byte limit and zero swap. Missing/unapplied controls
refuse execution. There is no unbounded fallback. A receipt records whether this
check actually completed; an exit code alone proves nothing about enforcement.

The default whole-service deadline is 60 seconds (`--timeout-seconds`); shutdown
allows five seconds. Both flags require positive integers. Changing them does not
implicitly authorize exceeding a study budget. CPUQuota throttles throughput,
not total CPU consumption, so cumulative CPU remains the study ledger's duty.
This helper also does not enforce study-wide disk, input/output or engineering
budgets. It inherits command output rather than buffering it.

```sh
python3 scripts/run_bounded.py -- /bin/true
python3 scripts/run_bounded.py --memory-mib 512 --timeout-seconds 60 -- /path/to/command
python3 scripts/run_bounded.py -- /usr/bin/env GOMAXPROCS=2 GOTOOLCHAIN=local go test -race -count=1 -p 1 ./internal/officemetadata
```

The working directory and an explicit environment allowlist (`ENVIRONMENT` in
the script: PATH, HOME and named Go/toolchain/cache variables) come from the caller.
Allowlisted variables absent in the caller are explicitly unset in the service,
not inherited from stale manager configuration. Other environment settings still
come from the user manager; this is not a hermetic build environment. Supply other
required settings using `/usr/bin/env`; do not put secrets in command arguments.
Use absolute tool paths where tool identity must be pinned.

## Machine-readable outcome

Capture stderr as well as stdout. Exactly one supervisor line starts with
`ALETHARSIS_VALIDATION_RESULT=` followed by JSON. The process exit is 0 only for
completed command plus confirmed cleanup, otherwise 1. Do not infer the failure
class from that exit code or parse systemd's human prose.

The JSON separates `status`, `service_result`, `enforcement_verified`,
`cleanup_confirmed`, `launcher_returncode`, `command_exit_code` and
`command_signal`, alongside unit name and requested limits. Status values include
completed, command_failed, command_signaled, command_start_failed, memory_limit,
timeout, enforcement_unavailable, supervisor_failed, supervisor_timeout and
interrupted. A command exiting 125 is command_failed with command_exit_code 125,
not evidence of unavailable enforcement. An unconfirmed cleanup remains explicit
and always makes the wrapper fail, regardless of its primary status.

An ExecStopPost helper retains systemd's SERVICE_RESULT/EXIT_CODE/EXIT_STATUS in
private temporary data before the service can be garbage-collected; these values
are inspected before cleanup. The unit is stopped if still active and its state
is rechecked; already inactive/failed/not-found counts as stopped. Failed-unit
state is reset after inspection. On launcher timeout/interrupt, cleanup addresses
the service group, not merely the launcher process. Missing outcome information
never becomes successful validation. Temporary receipts are summarized into the
final line; retain that output with test logs. This protocol assumes trusted
commands: it is not tamper-proof against the same user deliberately altering it.

## Validation observations

The revised runner passes offline tests for limit application/refusal, distinct
outcomes, environment/cwd/timeout configuration and cleanup. Live probes on
2026-09-22 distinguished a 32 MiB touched-page OOM (`memory_limit`), one-second
sleep deadline (`timeout`), command exit 125 (`command_failed`) and successful
commands. Each applied-limit probe reported enforcement_verified and confirmed
cleanup. Reserving zero-filled pages alone is not an OOM test; the probe touches
each page. Historical PR #76's breach remains recorded and is not erased.
