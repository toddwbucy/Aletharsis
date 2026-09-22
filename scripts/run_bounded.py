"""Run a trusted development command with verified Linux cgroup memory limits."""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import shlex
import signal
import sys
import tempfile
import uuid

ENVIRONMENT = ("PATH", "HOME", "TMPDIR", "XDG_CACHE_HOME", "PYTHONHASHSEED", "GOEXPERIMENT", "GOWORK", "GOENV", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "GOMAXPROCS", "GOMEMLIMIT", "GOPATH", "GOROOT", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "CGO_ENABLED")


def positive(value):
    try:
        result = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a positive integer") from exc
    if result <= 0:
        raise argparse.ArgumentTypeError("must be a positive integer")
    return result


def verify_memory(expected, membership=Path("/proc/self/cgroup"), root=Path("/sys/fs/cgroup")):
    lines = membership.read_text().splitlines()
    unified = [line[3:] for line in lines if line.startswith("0::")]
    if len(unified) != 1:
        raise ValueError("unified cgroup membership unavailable")
    name = PurePosixPath(unified[0])
    if not name.is_absolute() or ".." in name.parts:
        raise ValueError("invalid cgroup path")
    group = root.joinpath(*name.parts[1:])
    memory = (group / "memory.max").read_text().strip()
    swap = (group / "memory.swap.max").read_text().strip()
    if memory != str(expected) or swap != "0":
        raise ValueError("requested memory/swap limits not applied")
    return {"memory_max": int(memory), "memory_swap_max": 0}


def guard(expected, receipt, command):
    try:
        applied = verify_memory(expected)
    except (OSError, ValueError) as exc:
        receipt.write_text(json.dumps({"verified": False, "reason": str(exc)}))
        return 125
    receipt.write_text(json.dumps({"verified": True, **applied}))
    try:
        os.execvpe(command[0], command, os.environ)
    except OSError as exc:
        receipt.write_text(json.dumps({"verified": True, **applied, "exec_error": str(exc)}))
        return 125


def command_line(command, memory_mib, timeout_seconds, unit, cwd, receipt):
    argv = ["systemd-run", "--user", "--wait", "--pipe", "--expand-environment=no", "--unit=" + unit,
            "--working-directory=" + str(cwd), "--property=Type=exec",
            "--property=MemoryAccounting=yes",
            f"--property=MemoryMax={memory_mib * 1024 * 1024}",
            "--property=MemorySwapMax=0", "--property=OOMPolicy=kill",
            "--property=KillMode=control-group", f"--property=RuntimeMaxSec={timeout_seconds}",
            "--property=TimeoutStopSec=5"]
    # ExecStopPost receives systemd's outcome even if the unit is subsequently GC'd.
    stop = [sys.executable, str(Path(__file__).resolve()), "--service-result", str(receipt.with_name("service.json"))]
    argv += ["--property=ExecStopPost=:" + shlex.join(stop)]
    absent = []
    for name in ENVIRONMENT:
        if name in os.environ:
            argv += ["--setenv=" + name]
        else:
            absent.append(name)
    if absent:
        argv += ["--property=UnsetEnvironment=" + " ".join(absent)]
    return argv + ["--", sys.executable, str(Path(__file__).resolve()), "--guard",
                   str(memory_mib * 1024 * 1024), str(receipt), *command]


def inspect(unit):
    result = subprocess.run(["systemctl", "--user", "show", unit,
                             "--property=LoadState,ActiveState,Result,ExecMainCode,ExecMainStatus"],
                            capture_output=True, text=True, timeout=10, check=False)
    values = dict(line.split("=", 1) for line in result.stdout.splitlines() if "=" in line)
    if result.returncode and values.get("LoadState") != "not-found":
        raise RuntimeError("service state unavailable")
    if "LoadState" not in values:
        raise RuntimeError("service state missing")
    return values


def cleanup(unit):
    try:
        state = inspect(unit)
        if state.get("LoadState") == "not-found":
            return True
        if state.get("ActiveState") not in ("inactive", "failed"):
            subprocess.run(["systemctl", "--user", "stop", unit], timeout=10, check=False,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        state = inspect(unit)
        stopped = state.get("LoadState") == "not-found" or state.get("ActiveState") in ("inactive", "failed")
        if stopped and state.get("LoadState") != "not-found":
            subprocess.run(["systemctl", "--user", "reset-failed", unit], timeout=10,
                           check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return stopped
    except (OSError, subprocess.TimeoutExpired, RuntimeError, KeyboardInterrupt):
        return False


def outcome(state, acknowledgment):
    result = state.get("Result")
    if result == "oom-kill":
        return "memory_limit" if acknowledgment.get("verified") else "supervisor_memory_limit"
    if result == "timeout":
        return "timeout" if acknowledgment.get("verified") else "supervisor_timeout"
    if not acknowledgment.get("verified"):
        return "enforcement_unavailable"
    if acknowledgment.get("exec_error"):
        return "command_start_failed"
    if result == "signal" or result == "core-dump":
        return "command_signaled"
    if result == "success" and state.get("ExecMainCode") == "1" and state.get("ExecMainStatus") == "0":
        return "completed"
    if result == "exit-code" and state.get("ExecMainCode") == "1" and state.get("ExecMainStatus") not in (None, "", "0"):
        return "command_failed"
    return "supervisor_failed"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--memory-mib", type=positive, default=1024)
    parser.add_argument("--timeout-seconds", type=positive, default=60)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("a command after -- is required")
    unit = "aletharsis-validation-" + uuid.uuid4().hex + ".service"
    report = {"unit": unit, "memory_mib": args.memory_mib, "timeout_seconds": args.timeout_seconds,
              "status": "supervisor_failed", "enforcement_verified": False, "cleanup_confirmed": False,
              "service_result": None, "command_exit_code": None, "command_signal": None,
              "launcher_returncode": None}
    state, acknowledgment = {}, {}
    with tempfile.TemporaryDirectory(prefix="aletharsis-limit-") as directory:
        receipt = Path(directory) / "guard.json"
        try:
            run = subprocess.run(command_line(command, args.memory_mib, args.timeout_seconds,
                                             unit, Path.cwd(), receipt),
                                 timeout=args.timeout_seconds + 20, check=False)
            report["launcher_returncode"] = run.returncode
            service_receipt = receipt.with_name("service.json")
            state = json.loads(service_receipt.read_text()) if service_receipt.exists() else inspect(unit)
            if receipt.exists():
                acknowledgment = json.loads(receipt.read_text())
            report["status"] = outcome(state, acknowledgment)
        except subprocess.TimeoutExpired:
            report["status"] = "supervisor_timeout"
        except KeyboardInterrupt:
            report["status"] = "interrupted"
        except (OSError, RuntimeError, ValueError):
            report["status"] = "enforcement_unavailable" if not state else "supervisor_failed"
        finally:
            report["cleanup_confirmed"] = cleanup(unit)
        report["enforcement_verified"] = acknowledgment.get("verified") is True
        report["service_result"] = state.get("Result")
        if state.get("ExecMainCode") == "1" and report["enforcement_verified"] and not acknowledgment.get("exec_error"):
            report["command_exit_code"] = int(state.get("ExecMainStatus", "0"))
        elif state.get("ExecMainCode") in ("2", "3") and report["enforcement_verified"]:
            report["command_signal"] = int(state.get("ExecMainStatus", "0"))
    print("ALETHARSIS_VALIDATION_RESULT=" + json.dumps(report, sort_keys=True), file=sys.stderr, flush=True)
    return 0 if report["status"] == "completed" and report["cleanup_confirmed"] else 1


def service_result(path):
    code = os.environ.get("EXIT_CODE", "")
    status = os.environ.get("EXIT_STATUS", "")
    if code in ("killed", "dumped") and not status.isdecimal():
        status = str(int(getattr(signal, "SIG" + status, 0)))
    path.write_text(json.dumps({"Result": os.environ.get("SERVICE_RESULT", ""),
                               "ExecMainCode": {"exited": "1", "killed": "2", "dumped": "3"}.get(code, "0"),
                               "ExecMainStatus": status}))


if __name__ == "__main__":
    if sys.argv[1:2] == ["--service-result"]:
        service_result(Path(sys.argv[2]))
        raise SystemExit(0)
    if sys.argv[1:2] == ["--guard"]:
        raise SystemExit(guard(int(sys.argv[2]), Path(sys.argv[3]), sys.argv[4:]))
    raise SystemExit(main())
