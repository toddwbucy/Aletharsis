"""Executable behavior, strict report shapes, and source/output protections."""
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat

import pytest

ROOT = Path(__file__).resolve().parents[1]
REFERENCE = ROOT / 'reference/python-behavior'
CASES = json.loads((REFERENCE / 'manifest.json').read_bytes())['cases']
VIEWS = {
    'audit': None,
    'unicode': {'unicode', 'possible_steganography', 'parser'},
    'metadata': {'metadata', 'identifier', 'provenance', 'parser'},
    'structure': {'document_structure', 'embedded_content', 'hidden_content', 'visual_watermark', 'parser'},
}


def assert_report(result, validator):
    report = json.loads(result.stdout)
    validator.validate(report)
    counts = Counter(f['severity'].lower() for f in report['findings'])
    ranks = {'INFO': 1, 'LOW': 1, 'MEDIUM': 2, 'HIGH': 3}
    code = 4 if report['status'] == 'failed' else max(
        (ranks[f['severity']] for f in report['findings']), default=0)
    assert report['summary'] == {
        'findings': len(report['findings']), 'exit_code': code,
        **{severity: counts[severity] for severity in ('info', 'low', 'medium', 'high')},
    }
    assert result.returncode == code
    return report


def canonical_findings(findings):
    # Match only the documented migration exclusion for native diagnostic prose.
    value = json.loads(json.dumps(findings))
    for finding in value:
        if finding['id'] == 'parser.failure':
            finding['evidence'].pop('message')
    return sorted(value, key=lambda f: json.dumps(f, sort_keys=True))


@pytest.mark.parametrize('view', VIEWS)
@pytest.mark.parametrize('case', CASES, ids=lambda case: case['input'])
def test_all_reports_and_views(run_cli, validator, case, view):
    result = run_cli(view, case['input'], '--json')
    assert result.stderr == b''
    report = assert_report(result, validator)
    expected = json.loads((REFERENCE / case['report']).read_bytes())
    assert report['file'] == expected['file']
    assert report['status'] == expected['status']
    assert report['evidence'] == expected['evidence']  # filtering retains evidence
    selected = expected['findings'] if VIEWS[view] is None else [
        f for f in expected['findings'] if f['category'] in VIEWS[view]]
    assert canonical_findings(report['findings']) == canonical_findings(selected)
    if view == 'audit':
        assert result.returncode == case['exit_code']
    else:
        assert any(f'This is the {view} finding view' in s for s in report['limitations'])
    again = run_cli(view, case['input'], '--json')
    assert (again.stdout, again.stderr, again.returncode) == (
        result.stdout, result.stderr, result.returncode)


@pytest.mark.parametrize('name', ['notes with spaces.txt', 'évidence 😀.txt', '-h'])
def test_paths_and_exact_coordinates(run_cli, validator, tmp_path, name):
    raw = 'é😀\u200b\r\n'.encode()
    source = tmp_path / name
    source.write_bytes(raw)
    before = source.stat()
    result = run_cli('audit', '--json', '--', name, cwd=tmp_path)
    after = source.stat()
    report = assert_report(result, validator)
    assert report['file']['path'] == name
    assert report['file']['sha256'] == hashlib.sha256(raw).hexdigest()
    assert report['evidence']['texts'][0]['byte_offsets'] == [0, 2, 6, 9, 10, 11]
    assert (before.st_atime_ns, before.st_mtime_ns, before.st_ctime_ns) == (
        after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns)
    assert source.read_bytes() == raw


@pytest.mark.parametrize('args', [[], ['audit'], ['unknown'], ['audit', 'a', 'b'],
    ['audit', 'unused', '--bad'], ['audit', 'unused', '--output'],
    ['audit', 'unused', '--output', ''], ['audit', 'unused', '--output', '-h'],
    ['audit', 'unused', '--output=']])
def test_usage_errors_are_not_reports(run_cli, tmp_path, args):
    result = run_cli(*args, cwd=tmp_path)
    assert result.returncode == 4
    assert result.stdout == b''
    assert result.stderr.startswith(b'aletharsis: ')
    assert list(tmp_path.iterdir()) == []


@pytest.mark.parametrize('args', [['--help'], ['-h'], ['audit', '--help'], ['--version']])
def test_help_and_version(run_cli, args):
    result = run_cli(*args)
    assert result.returncode == 0
    assert result.stderr == b''
    if args == ['--version']:
        assert re.fullmatch(rb'aletharsis [0-9]+\.[0-9]+\.[0-9]+\n', result.stdout)
    else:
        assert b'Usage:' in result.stdout


@pytest.mark.parametrize('view', VIEWS)
def test_verbose_json_keeps_logging_on_stderr(run_cli, validator, tmp_path, view):
    source = tmp_path / 'source.txt'
    source.write_bytes(b'ordinary text\n')
    result = run_cli(view, source, '--json', '--verbose')
    report = assert_report(result, validator)
    log = json.loads(result.stderr)
    assert log == {'event': 'audit_completed', 'path': str(source),
                   'status': report['status'], 'exit_code': result.returncode}
    console = run_cli(view, source)
    assert console.returncode == result.returncode
    assert console.stdout.startswith(b'ALETHARSIS FORENSIC AUDIT')
    assert console.stderr == b''


@pytest.mark.parametrize('kind', ['source', 'hardlink', 'symlink', 'existing'])
def test_output_rejects_existing_targets(run_cli, tmp_path, kind):
    source = tmp_path / 'source.txt'
    raw = b'original source\n'
    source.write_bytes(raw)
    target = source if kind == 'source' else tmp_path / 'report.json'
    if kind == 'hardlink':
        os.link(source, target)
    elif kind == 'symlink':
        target.symlink_to(source)
    elif kind == 'existing':
        target.write_bytes(b'previous report')
    before = source.stat()
    target_before = target.lstat()
    result = run_cli('audit', source, '--output', target)
    assert result.returncode == 4
    assert result.stdout == b''
    assert b'could not create report:' in result.stderr
    after = source.stat()
    assert (before.st_atime_ns, before.st_mtime_ns, before.st_ctime_ns) == (
        after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns)
    assert target.lstat() == target_before
    assert source.read_bytes() == raw
    if kind == 'symlink':
        assert target.is_symlink() and target.readlink() == source
    elif kind == 'existing':
        assert target.read_bytes() == b'previous report'


@pytest.mark.parametrize('output_name', ['report.json', '-h'])
def test_private_output_matches_stdout(run_cli, validator, tmp_path, output_name):
    source = tmp_path / 'source.txt'
    source.write_bytes(b'ordinary text\n')
    expected = run_cli('audit', source, '--json')
    assert_report(expected, validator)
    result = run_cli('audit', source, f'--output={output_name}', cwd=tmp_path, umask=0)
    assert result.returncode == expected.returncode
    assert result.stdout == result.stderr == b''
    output = tmp_path / output_name
    assert stat.S_IMODE(output.stat().st_mode) == 0o600
    assert output.read_bytes() == expected.stdout


@pytest.mark.parametrize('source_name', ['missing.txt', 'bad.txt'])
@pytest.mark.parametrize('view', VIEWS)
def test_failed_audit_is_a_valid_report(run_cli, validator, tmp_path, source_name, view):
    if source_name == 'bad.txt':
        (tmp_path / source_name).write_bytes(b'\xff')
    result = run_cli(view, source_name, '--json', cwd=tmp_path)
    report = assert_report(result, validator)
    assert report['status'] == 'failed'
    assert result.returncode == 4
    assert result.stderr == b''
    assert [f['id'] for f in report['findings']] == ['parser.failure']
    assert report['evidence']['texts'] == []


def test_output_creation_failure(run_cli, tmp_path):
    source = tmp_path / 'source.txt'
    source.write_bytes(b'ordinary text\n')
    result = run_cli('audit', source, '--output', tmp_path / 'absent' / 'report.json')
    assert result.returncode == 4 and result.stdout == b''
    assert b'could not create report:' in result.stderr


def test_failed_write_removes_partial_report(run_cli, tmp_path):
    import resource  # Linux-only test; defer import until after the platform fixture.

    source = tmp_path / 'source.txt'
    raw = b'ordinary text\n'
    source.write_bytes(raw)
    output = tmp_path / 'partial.json'

    def restrict_output():
        signal.signal(signal.SIGXFSZ, signal.SIG_IGN)
        resource.setrlimit(resource.RLIMIT_FSIZE, (64, 64))

    before = source.stat()
    result = run_cli('audit', source, '--output', output, preexec_fn=restrict_output)
    after = source.stat()
    assert result.returncode == 4 and result.stdout == b''
    assert b'could not write report:' in result.stderr
    assert not output.exists()
    assert (before.st_atime_ns, before.st_mtime_ns, before.st_ctime_ns) == (
        after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns)
    assert source.read_bytes() == raw


@pytest.mark.parametrize('args', [['--help'], ['--version'], ['audit', '--help'],
    ['audit', 'tests/fixtures/clean_ascii.txt', '--json']])
def test_stdout_write_failure(run_cli, args):
    with open('/dev/full', 'wb', buffering=0) as sink:
        result = run_cli(*args, stdout=sink)
    assert result.returncode == 4
