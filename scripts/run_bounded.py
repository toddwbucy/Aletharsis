"""Run a trusted development command inside a memory-limited Linux user service.

This is validation infrastructure, not an audit-time subprocess adapter. No
fallback runs the command if systemd/cgroup enforcement is unavailable.
"""
import argparse
from pathlib import Path
import subprocess
import sys
import uuid


def positive(value):
    try:
        result = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a positive integer") from exc
    if result <= 0:
        raise argparse.ArgumentTypeError("must be a positive integer")
    return result


def command_line(command, memory_mib, timeout_seconds, unit, cwd):
    return [
        "systemd-run", "--user", "--wait", "--pipe", "--collect",
        "--unit=" + unit, "--working-directory=" + str(cwd),
        "--property=Type=exec",
        "--property=MemoryAccounting=yes",
        f"--property=MemoryMax={memory_mib * 1024 * 1024}",
        "--property=MemorySwapMax=0",
        "--property=OOMPolicy=kill",
        "--property=KillMode=control-group",
        f"--property=RuntimeMaxSec={timeout_seconds}",
        "--property=TimeoutStopSec=5",
        "--", *command,
    ]


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--memory-mib", type=positive, default=1024,
                        help="aggregate resident-memory ceiling, default 1024 MiB")
    parser.add_argument("--timeout-seconds", type=positive, default=60,
                        help="whole-command wall deadline, default 60 seconds")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    command = args.command
    if command[:1] == ["--"]:
        command = command[1:]
    if not command:
        parser.error("a command after -- is required")
    unit = "aletharsis-validation-" + uuid.uuid4().hex + ".service"
    print(f"Bounded validation: unit={unit} memory_mib={args.memory_mib} "
          f"timeout_seconds={args.timeout_seconds}", file=sys.stderr, flush=True)
    launch = command_line(command, args.memory_mib, args.timeout_seconds,
                          unit, Path.cwd())
    try:
        result = subprocess.run(launch, timeout=args.timeout_seconds + 20,
                                check=False)
        return result.returncode if result.returncode >= 0 else 128 - result.returncode
    except (subprocess.TimeoutExpired, KeyboardInterrupt) as exc:
        # Killing the launcher alone does not necessarily kill its service.
        try:
            stopped = subprocess.run(["systemctl", "--user", "stop", unit],
                                     timeout=10, check=False)
            if stopped.returncode:
                print("Service cleanup could not be confirmed", file=sys.stderr)
                return 125
        except (OSError, subprocess.TimeoutExpired):
            print("Service cleanup could not be confirmed", file=sys.stderr)
            return 125
        return 130 if isinstance(exc, KeyboardInterrupt) else 124
    except OSError as exc:
        print(f"Cannot start bounded validation: {exc}", file=sys.stderr)
        return 125


if __name__ == "__main__":
    raise SystemExit(main())
