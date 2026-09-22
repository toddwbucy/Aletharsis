"""Command construction and fail-closed behavior; actual cgroup probes are manual."""
import importlib.util
from pathlib import Path
import subprocess
from unittest.mock import patch

import pytest

spec = importlib.util.spec_from_file_location(
    "run_bounded", Path(__file__).parents[1] / "scripts/run_bounded.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def test_default_and_configured_limits():
    for supplied, expected in [([], 1073741824), (["--memory-mib", "256"], 268435456)]:
        with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run:
            assert runner.main([*supplied, "--", "/bin/true"]) == 0
        argv = run.call_args.args[0]
        assert f"--property=MemoryMax={expected}" in argv
        assert "--property=MemorySwapMax=0" in argv
        assert "--property=KillMode=control-group" in argv
        assert "--property=OOMPolicy=kill" in argv
        assert "--property=RuntimeMaxSec=60" in argv
        assert argv[-2:] == ["--", "/bin/true"]


def test_unavailable_enforcement_never_falls_back():
    with patch.object(runner.subprocess, "run", side_effect=FileNotFoundError) as run:
        assert runner.main(["--", "/bin/true"]) == 125
        assert run.call_count == 1
    with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 1)) as run:
        assert runner.main(["--", "/bin/true"]) == 1
        assert run.call_count == 1


@pytest.mark.parametrize("options", [["--memory-mib", "0"], ["--memory-mib", "-1"], ["--timeout-seconds", "no"], []])
def test_invalid_options_do_not_launch(options):
    with patch.object(runner.subprocess, "run") as run, pytest.raises(SystemExit):
        runner.main(options)
    run.assert_not_called()


def test_timeout_stops_service_and_reports_failed_cleanup():
    for cleanup, expected in [(0, 124), (1, 125)]:
        with patch.object(runner.subprocess, "run", side_effect=[
            subprocess.TimeoutExpired("systemd-run", 80),
            subprocess.CompletedProcess([], cleanup),
        ]) as run:
            assert runner.main(["--", "/bin/true"]) == expected
        launch = run.call_args_list[0].args[0]
        unit = next(x.removeprefix("--unit=") for x in launch if x.startswith("--unit="))
        assert run.call_args_list[1].args[0] == ["systemctl", "--user", "stop", unit]
