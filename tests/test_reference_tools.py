"""Regression checks for the migration oracle, not the audited implementation."""
import importlib
import json
from pathlib import Path
import platform
import runpy
import subprocess
import sys

import pytest

ROOT = Path(__file__).resolve().parents[1]


@pytest.fixture
def parity(monkeypatch):
    monkeypatch.syspath_prepend(str(ROOT / 'scripts'))
    return importlib.import_module('check_parity')


@pytest.mark.parametrize(('expected', 'actual', 'equal'), [
    (1, 1.0, True), (0.0, 0, True), (True, True, True),
    (True, 1, False), (1, True, False), (False, 0.0, False),
    (0.0, False, False), (1, '1', False), ('1', 1, False),
    ([True], [1], False), ({'value': 0}, {'value': False}, False),
])
def test_numeric_equivalence_excludes_booleans(parity, expected, actual, equal):
    assert (not list(parity.differences(expected, actual))) is equal


@pytest.mark.parametrize('timeout_run', [1, 2])
def test_frozen_timeout_is_case_failure(parity, monkeypatch, capsys, timeout_run):
    monkeypatch.chdir(ROOT)
    monkeypatch.setattr(sys, 'argv', ['check_parity.py', '/unused/candidate'])
    root = ROOT / 'reference/python-behavior'
    cases = json.loads((root / 'manifest.json').read_text())['cases']
    reports = {c['input']: c for c in cases}
    calls = {}

    def run(command, **kwargs):
        assert kwargs['timeout'] == parity.AUDIT_TIMEOUT_SECONDS
        name = command[2]
        calls[name] = calls.get(name, 0) + 1
        if calls[name] == timeout_run:
            raise subprocess.TimeoutExpired(command, kwargs['timeout'])
        case = reports[name]
        return subprocess.CompletedProcess(command, case['exit_code'],
                                           (root / case['report']).read_text())

    monkeypatch.setattr(subprocess, 'run', run)
    assert parity.main() is True
    output = capsys.readouterr().out
    assert '0/37 frozen reports match; 37 failures.' in output
    assert output.count('audit timed out after 30 seconds') == 37
    assert all(count == timeout_run for count in calls.values())


def test_seeded_timeout_is_case_failure(parity, monkeypatch, capsys):
    monkeypatch.setattr(sys, 'argv', ['check_differential.py', '/unused/candidate'])
    calls = []

    def run(command, **kwargs):
        assert kwargs['timeout'] == parity.AUDIT_TIMEOUT_SECONDS
        calls.append(command)
        raise subprocess.TimeoutExpired(command, kwargs['timeout'])

    monkeypatch.setattr(subprocess, 'run', run)
    with pytest.raises(SystemExit) as exc:
        runpy.run_path(str(ROOT / 'scripts/check_differential.py'), run_name='__main__')
    assert exc.value.code == 1
    assert len(calls) == 210
    output = capsys.readouterr().out
    assert '0/210 seeded differential cases match.' in output
    assert output.count('audit timed out after 30 seconds') == 210


def test_capture_rejects_wrong_interpreter_before_writes(monkeypatch, tmp_path):
    monkeypatch.chdir(tmp_path)
    monkeypatch.setattr(platform, 'python_version', lambda: '3.12.12')
    with pytest.raises(RuntimeError, match='requires Python 3.12.13'):
        runpy.run_path(str(ROOT / 'scripts/freeze_python.py'), run_name='__main__')
    assert list(tmp_path.iterdir()) == []
