"""Compiled v2 CLI acceptance, using the independent contract oracle."""
import json
from pathlib import Path
import sys

import pytest
from jsonschema import Draft202012Validator

from conftest import ROOT
sys.path.insert(0, str(ROOT / 'tests/contracts'))
from support import schema, strict_json, validate_semantics

INPUTS = sorted((ROOT / 'reference/python-behavior/inputs').glob('*'))
assert len(INPUTS) == 16


def validate(raw):
    report = strict_json(raw)
    Draft202012Validator(schema('2.0')).validate(report)
    validate_semantics(report)
    return report


@pytest.mark.parametrize('source', INPUTS, ids=lambda p: p.name)
def test_v2_corpus(run_cli, source):
    first = run_cli('audit', source, '--schema-version', '2.0', '--json')
    second = run_cli('audit', source, '--schema-version=2.0', '--json')
    assert first.stdout == second.stdout
    assert first.returncode == second.returncode
    assert first.stderr == second.stderr == b''
    report = validate(first.stdout)
    assert report['summary']['exit_code'] == first.returncode
    assert not first.stdout.endswith(b'\n')
    legacy = run_cli('audit', source, '--json')
    old = json.loads(legacy.stdout)
    for key in ('file', 'evidence', 'summary', 'limitations'):
        assert report[key] == old[key]
    findings = []
    for finding in report['findings']:
        item = {k: v for k, v in finding.items()
                if k not in ('finding_ref', 'execution_ref', 'mechanism', 'anchor_refs')}
        if item['id'] == 'parser.failure':
            item['evidence'].pop('failure_code')
        findings.append(item)
    assert findings == old['findings']


def test_v2_views_and_source_integrity(run_cli, tmp_path):
    source = tmp_path / 'source.py'
    payload = '# a\u200bb 😀\n'.encode()
    source.write_bytes(payload)
    before = source.stat()
    full = validate(run_cli('audit', source, '--schema-version=2.0', '--json').stdout)
    for view in ('unicode', 'metadata', 'structure'):
        result = run_cli(view, source, '--schema-version=2.0', '--json', '--verbose')
        report = validate(result.stdout)
        assert report['view']['name'] == view
        for key in ('capabilities', 'executions', 'artifacts', 'anchors', 'results', 'evidence'):
            assert report[key] == full[key]
        assert json.loads(result.stderr)['schema_version'] == '2.0'
    after = source.stat()
    assert (before.st_atime_ns, before.st_mtime_ns, before.st_ctime_ns) == (
        after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns)
    assert source.read_bytes() == payload


def test_v2_output_safety(run_cli, tmp_path):
    source = tmp_path / 'source.txt'
    source.write_text('ordinary ASCII\n')
    dest = tmp_path / 'report.json'
    result = run_cli('audit', source, '--schema-version=2.0', '--output', dest)
    assert result.returncode == 0 and result.stdout == b'' and result.stderr == b''
    original = dest.read_bytes()
    validate(original)
    assert dest.stat().st_mode & 0o777 == 0o600
    linked = tmp_path / 'linked'
    linked.hardlink_to(source)
    symlink = tmp_path / 'symlink'
    symlink.symlink_to(source)
    for path in (dest, source, linked, symlink):
        result = run_cli('audit', source, '--schema-version=2.0', '--output', path)
        assert result.returncode == 4 and result.stdout == b''
        assert b'output.create_failed' in result.stderr
    assert dest.read_bytes() == original
    assert source.read_text() == 'ordinary ASCII\n'


def test_v2_clean_and_failed_coverage(run_cli, tmp_path):
    source = tmp_path / 'clean.txt'
    source.write_text('ordinary ASCII\n')
    result = run_cli('audit', source, '--schema-version=2.0')
    assert result.returncode == 0
    text = result.stdout.decode()
    assert 'Availability: unavailable' in text
    assert 'anthropic.claude_text_watermark' in text
    assert 'execution: not_run' in text
    assert 'no negative result' in text
    missing = run_cli('audit', tmp_path / 'missing', '--schema-version=2.0', '--json')
    report = validate(missing.stdout)
    assert missing.returncode == 4 and report['status'] == 'failed'
    assert report['diagnostics'][0]['code'] == 'file.not_found'
    assert report['results'] == []


def test_explicit_legacy_unchanged(run_cli, tmp_path):
    source = tmp_path / 'source.txt'
    source.write_text('a\u200bb')
    for flags in ([], ['--json']):
        default = run_cli('audit', source, *flags)
        explicit = run_cli('audit', source, '--schema-version=1.0', *flags)
        assert (default.stdout, default.stderr, default.returncode) == (
            explicit.stdout, explicit.stderr, explicit.returncode)


def test_v2_resource_failure_has_no_partial_report(run_cli, tmp_path):
    source = tmp_path / 'dense.txt'
    source.write_bytes(b'A' * (8 * 1024 * 1024))
    dest = tmp_path / 'report.json'
    result = run_cli('audit', source, '--schema-version=2.0', '--output', dest)
    assert result.returncode == 4
    assert result.stdout == b'' and not dest.exists()
    assert b'execution.resource_limit' in result.stderr
