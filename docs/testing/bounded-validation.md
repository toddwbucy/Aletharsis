# Bounded local validation

Use `scripts/run_bounded.py` for trusted development commands on Linux with a
working user systemd manager and cgroup memory controller. This does not execute
inspected document content and is not the future external-detector adapter.

The default is **1024 MiB (1 GiB)** for the entire command/service, including child
processes. This is stronger than a per-process ceiling. Swap is disabled.
`MemoryMax` is enforced by the kernel; `GOMEMLIMIT` and observed peak RSS alone
are not hard memory limits. The default wall timeout is 60 seconds. Memory OOM
kills the service group; timeout stops the group, with five seconds allowed for
shutdown. Missing service-manager support fails closed, without rerunning the
command unbounded. Output is inherited, not buffered in the Python supervisor.

```sh
python3 scripts/run_bounded.py -- /bin/true
python3 scripts/run_bounded.py --memory-mib 512 --timeout-seconds 60 -- /path/to/command
python3 scripts/run_bounded.py -- /usr/bin/env GOMAXPROCS=2 GOTOOLCHAIN=local go test -race -p 1 ./internal/officemetadata
```

`--memory-mib` and `--timeout-seconds` must be positive integers. Their defaults
are in one argparse declaration each. Changing the flag permits future budgets
without changing implementation; it does not override a study's authorized
budget. Each invocation prints the selected limits and unique service unit.
The service manager prints completion state, CPU and peak memory information.
Capture both stdout and stderr when retaining receipts. The working directory is
preserved; environment variables needed by tests should be supplied explicitly
with `/usr/bin/env` as above. Do not pass secrets on the command line.

Run Go packages in small batches with `-p 1`; use a small `GOMAXPROCS` where
appropriate. A test that exceeds the ceiling is a failed bounded attempt, never
permission to rerun unbounded. Preserve earlier breaches and failed outcomes.
Memory and wall-time enforcement do not enforce cumulative study CPU, disk,
input/output or engineering budgets. Those still require the study supervisor
and ledger. This helper alone does not clear B0 or authorize comparator execution.

## Validation observations (2026-09-22)

- Default-limit `/bin/true`: success; reported 2.7 MiB peak.
- Configured 32 MiB limit with 128 MiB of allocated **and touched** pages:
  `oom-kill`, nonzero exit, reported 32 MiB peak, zero swap.
- One-second timeout with `/bin/sleep 20`: `timeout`, nonzero exit.
- Seven regression tests ran inside the default limited service and passed.
  They cover limit construction, validation, unavailable enforcement, timeout
  cleanup and failed-cleanup reporting.

Reserving a zero-filled bytearray without touching its pages did not exceed
resident memory; that preliminary probe succeeded and is not OOM evidence.
The touched-page probe above established enforcement. These observations are
local Linux validation, not a portability claim. The historical Office race-test
breach in PR #76 remains recorded; this helper does not retroactively fix it.
