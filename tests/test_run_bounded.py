"""Offline guard, outcome, launch configuration and cleanup regressions."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import shlex
from unittest.mock import patch

import pytest

spec = importlib.util.spec_from_file_location("run_bounded", Path(__file__).parents[1] / "scripts/run_bounded.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


@pytest.mark.parametrize("memory,timeout", [(1024,60),(256,7)])
def test_launch_configuration(memory, timeout, tmp_path):
    with patch.dict(os.environ, {"PATH":"/chosen/bin", "GOCACHE":"/cache"}, clear=True):
        argv=runner.command_line(["go","test"],memory,timeout,"test.service",tmp_path,tmp_path/"guard.json")
    assert f"--property=MemoryMax={memory*1024*1024}" in argv
    assert f"--property=RuntimeMaxSec={timeout}" in argv
    assert "--property=MemorySwapMax=0" in argv
    assert "--property=KillMode=control-group" in argv
    assert "--property=OOMPolicy=kill" in argv
    assert "--working-directory="+str(tmp_path) in argv
    assert "--setenv=PATH" in argv and "--setenv=GOCACHE" in argv
    assert any(x.startswith("--property=UnsetEnvironment=") and "GOFLAGS" in x for x in argv)
    assert "--expand-environment=no" in argv
    assert "--guard" in argv and argv[-2:]==["go","test"]
    assert any(x.startswith("--property=ExecStopPost=:") for x in argv)


@pytest.mark.parametrize("memory,swap", [("max","0"),("100","0"),("33554432","max")])
def test_unapplied_limits_refuse_command(tmp_path,memory,swap):
    (tmp_path/"membership").write_text("0::/test\n")
    group=tmp_path/"test"; group.mkdir()
    (group/"memory.max").write_text(memory)
    (group/"memory.swap.max").write_text(swap)
    original=runner.verify_memory
    with patch.object(runner,"verify_memory",side_effect=lambda n:original(n,tmp_path/"membership",tmp_path)), patch.object(runner.os,"execvpe") as execute:
        assert runner.guard(33554432,tmp_path/"guard.json",["command"])==125
    execute.assert_not_called()
    assert json.loads((tmp_path/"guard.json").read_text())["verified"] is False


def test_applied_limits_and_missing_controller(tmp_path):
    (tmp_path/"membership").write_text("0::/test\n")
    group=tmp_path/"test"; group.mkdir()
    (group/"memory.max").write_text("33554432")
    (group/"memory.swap.max").write_text("0")
    assert runner.verify_memory(33554432,tmp_path/"membership",tmp_path)["memory_max"]==33554432
    (group/"memory.max").unlink()
    with pytest.raises(OSError): runner.verify_memory(33554432,tmp_path/"membership",tmp_path)


@pytest.mark.parametrize("result,code,status,ack,want", [
    ("success","1","0",True,"completed"),
    ("timeout","1","0",True,"supervisor_failed"),
    ("timeout","1","3",True,"supervisor_failed"),
    ("oom-kill","1","0",True,"supervisor_failed"),
    ("exit-code","1","1",True,"command_failed"),
    ("exit-code","1","0",True,"supervisor_failed"),
    ("oom-kill","2","9",False,"supervisor_memory_limit"),
    ("timeout","2","15",False,"supervisor_timeout"),
    ("exit-code","1","125",True,"command_failed"),
    ("oom-kill","2","9",True,"memory_limit"),
    ("timeout","2","15",True,"timeout"),
    ("signal","2","9",True,"command_signaled"),
    (None,None,None,False,"enforcement_unavailable"),
    ("exit-code","1","125",False,"enforcement_unavailable"),
])
def test_outcomes_not_inferred_from_launcher_exit(result,code,status,ack,want):
    assert runner.outcome({"Result":result,"ExecMainCode":code,"ExecMainStatus":status},{"verified":ack})==want


def test_launch_failure_has_no_unbounded_fallback(capsys):
    with patch.object(runner.subprocess,"run",side_effect=FileNotFoundError), patch.object(runner,"cleanup",return_value=False):
        assert runner.main(["--","/bin/true"])==1
    receipt=json.loads(capsys.readouterr().err.split("ALETHARSIS_VALIDATION_RESULT=")[1])
    assert receipt["status"]=="enforcement_unavailable" and not receipt["enforcement_verified"]


def test_timeout_argument_and_cleanup(capsys):
    with patch.object(runner.subprocess,"run",side_effect=subprocess.TimeoutExpired("launcher",27)) as run, patch.object(runner,"cleanup",return_value=True) as cleanup:
        assert runner.main(["--timeout-seconds","7","--","/bin/true"])==1
    assert run.call_args.kwargs["timeout"]==27
    cleanup.assert_called_once()
    assert '"status": "launcher_timeout"' in capsys.readouterr().err


@pytest.mark.parametrize("state", [{"LoadState":"not-found"},{"LoadState":"loaded","ActiveState":"inactive"},{"LoadState":"loaded","ActiveState":"failed"}])
def test_already_stopped_is_confirmed(state):
    with patch.object(runner,"inspect",return_value=state), patch.object(runner.subprocess,"run",return_value=subprocess.CompletedProcess([],0)) as run:
        assert runner.cleanup("test.service")
    assert not any("stop" in call.args[0] for call in run.call_args_list)


def test_cleanup_failure_is_not_confirmed():
    with patch.object(runner,"inspect",side_effect=RuntimeError):
        assert not runner.cleanup("test.service")


@pytest.mark.parametrize("options",[["--memory-mib","0"],["--memory-mib","-1"],["--timeout-seconds","no"],[]])
def test_invalid_options_do_not_launch(options):
    with patch.object(runner.subprocess,"run") as run, pytest.raises(SystemExit): runner.main(options)
    run.assert_not_called()


def test_receipt_path_preserved(tmp_path):
    receipt = tmp_path / "we%ird$dir space" / "guard.json"
    argv = runner.command_line(["/bin/true"], 1024, 60, "test.service", tmp_path, receipt)
    stop = next(arg.removeprefix("--property=ExecStopPost=:") for arg in argv if arg.startswith("--property=ExecStopPost=:"))
    assert shlex.split(stop)[-2:] == ["--service-result", str(receipt.with_name("service.json"))]
    assert str(receipt.with_name("service.json")) in stop


@pytest.mark.parametrize("present", [True, False])
def test_reproducibility_environment(present, tmp_path):
    names = ("TMPDIR", "XDG_CACHE_HOME", "PYTHONHASHSEED", "GOEXPERIMENT", "GOWORK", "GOENV")
    with patch.dict(os.environ, {name: "value" for name in names} if present else {}, clear=True):
        argv = runner.command_line(["/bin/true"], 1024, 60, "test.service", tmp_path, tmp_path / "guard.json")
    unset = next((arg.removeprefix("--property=UnsetEnvironment=").split() for arg in argv if arg.startswith("--property=UnsetEnvironment=")), [])
    for name in names:
        assert ("--setenv=" + name in argv) is present
        assert (name in unset) is not present


def test_active_cleanup_stops_rechecks_and_resets():
    states = [{"LoadState": "loaded", "ActiveState": "active"}, {"LoadState": "loaded", "ActiveState": "inactive"}]
    with patch.object(runner, "inspect", side_effect=states) as inspect, patch.object(runner.subprocess, "run") as run:
        assert runner.cleanup("test.service")
    assert inspect.call_count == 2
    assert [call.args[0][2] for call in run.call_args_list] == ["stop", "reset-failed"]


def test_second_interrupt_during_stop_retains_receipt(capsys):
    def run(argv, **kwargs):
        raise KeyboardInterrupt
    with patch.object(runner.subprocess, "run", side_effect=run), patch.object(runner, "inspect", return_value={"LoadState": "loaded", "ActiveState": "active"}):
        assert runner.main(["--", "/bin/true"]) == 1
    output = capsys.readouterr().err
    assert output.count("ALETHARSIS_VALIDATION_RESULT=") == 1
    receipt = json.loads(output.split("ALETHARSIS_VALIDATION_RESULT=")[1])
    assert receipt["status"] == "interrupted"
    assert receipt["cleanup_confirmed"] is False


def test_missing_swap_accounting_refuses_execution(tmp_path):
    (tmp_path / "membership").write_text("0::/test\n")
    group = tmp_path / "test"
    group.mkdir()
    (group / "memory.max").write_text("33554432")
    with pytest.raises(OSError):
        runner.verify_memory(33554432, tmp_path / "membership", tmp_path)


def test_systemd_c_escaping_of_receipt_path(tmp_path):
    receipt = tmp_path / r"bs\tdir\xdir" / "guard.json"
    argv = runner.command_line(["/bin/true"], 1024, 60, "test.service", tmp_path, receipt)
    stop = next(a for a in argv if a.startswith("--property=ExecStopPost=:"))
    assert str(receipt.with_name("service.json")).replace("\\", "\\\\") in stop
