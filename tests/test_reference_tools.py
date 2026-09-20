"""Regression checks for the migration oracle, not the audited implementation."""
import importlib
import json
from pathlib import Path
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
    monkeypatch.setattr(sys, 'argv', ['check_seeded.py', '/unused/candidate'])
    calls = []

    def run(command, **kwargs):
        assert kwargs['timeout'] == parity.AUDIT_TIMEOUT_SECONDS
        calls.append(command)
        raise subprocess.TimeoutExpired(command, kwargs['timeout'])

    monkeypatch.setattr(subprocess, 'run', run)
    with pytest.raises(SystemExit) as exc:
        runpy.run_path(str(ROOT / 'scripts/check_seeded.py'), run_name='__main__')
    assert exc.value.code == 1
    assert len(calls) == 210
    output = capsys.readouterr().out
    assert '0/210 frozen seeded cases match.' in output
    assert output.count('audit timed out after 30 seconds') == 210


def test_seeded_mismatch_is_failure(parity, monkeypatch, capsys):
    monkeypatch.chdir(ROOT)
    monkeypatch.setattr(sys, 'argv', ['check_seeded.py', '/unused/candidate'])
    cases = [json.loads(line) for line in (ROOT / 'reference/seeded-python/cases.jsonl').read_text().splitlines()]

    def run(command, **kwargs):
        index = int(Path(command[2]).stem)
        case = cases[index]
        report = json.loads(json.dumps(case['report']))
        assert (Path(kwargs['cwd']) / command[2]).read_bytes() == bytes.fromhex(case['input_hex'])
        if index == 0:
            report['file']['sha256'] = '0' * 64
        return subprocess.CompletedProcess(command, report['summary']['exit_code'], json.dumps(report))

    monkeypatch.setattr(subprocess, 'run', run)
    with pytest.raises(SystemExit) as exc:
        runpy.run_path(str(ROOT / 'scripts/check_seeded.py'), run_name='__main__')
    assert exc.value.code == 1
    assert '209/210 frozen seeded cases match.' in capsys.readouterr().out


@pytest.mark.parametrize('target', [
    'tests/fixtures/mixed_endings.csv',
    'reference/python-behavior/reports/000.json',
    'reference/python-behavior/manifest.json',
    'reference/seeded-python/cases.jsonl',
    'tests/schema-cases.json',
])
@pytest.mark.parametrize('missing', [False, True])
def test_reference_integrity_rejects_missing_or_changed_evidence(parity, monkeypatch, target, missing):
    monkeypatch.chdir(ROOT)
    read_bytes = Path.read_bytes

    def read(path):
        if path == Path(target):
            if missing:
                raise FileNotFoundError(target)
            return read_bytes(path) + b'\n'
        return read_bytes(path)

    monkeypatch.setattr(Path, 'read_bytes', read)
    with pytest.raises((AssertionError, FileNotFoundError)):
        parity.verify_references()


def test_retired_source_inventory_is_not_a_general_missing_file_exception(parity, monkeypatch):
    monkeypatch.chdir(ROOT)
    read_bytes = Path.read_bytes

    def read(path):
        raw = read_bytes(path)
        if path == Path('reference/python-retirement.json'):
            record = json.loads(raw)
            record['historical_source_files']['tests/fixtures/mixed_endings.csv'] = '0' * 64
            return json.dumps(record).encode()
        return raw

    monkeypatch.setattr(Path, 'read_bytes', read)
    with pytest.raises(AssertionError, match='Historical source inventory changed'):
        parity.verify_references()
