"""Compiled directory commands use corpus envelopes, not file reports."""
import hashlib
import json

import pytest

from scripts.check_distribution import corpus_validator, validate_directory


@pytest.mark.parametrize('version', ['1.0', '2.0'])
@pytest.mark.parametrize('output_format', ['--json', '--jsonl'])
@pytest.mark.parametrize('populated', [False, True])
def test_directory_corpus_contract(run_cli, tmp_path, version, output_format, populated):
    source = tmp_path / 'corpus'
    source.mkdir()
    payload = b'ordinary words\n'
    child = source / 'clean.txt'
    if populated:
        child.write_bytes(payload)
    before = source.stat()
    child_before = child.stat() if populated else None
    args = ('audit', source, output_format, '--schema-version=' + version)
    first, second = run_cli(*args), run_cli(*args)
    assert (first.returncode, first.stdout, first.stderr) == (second.returncode, second.stdout, second.stderr)
    assert first.stderr == b''
    if output_format == '--jsonl':
        records = [json.loads(line) for line in first.stdout.splitlines()]
        report = {'header': records[0], 'entries': records[1:-1], 'summary': records[-1]}
    else:
        report = json.loads(first.stdout)
    validate_directory(report, first.returncode, 'Linux', version, corpus_validator())
    assert len(report['entries']) == int(populated)
    if populated:
        entry = report['entries'][0]
        assert entry['relative_path'] == 'clean.txt'
        assert entry['report']['file']['sha256'] == hashlib.sha256(payload).hexdigest()
        assert child.stat() == child_before
        assert child.read_bytes() == payload
    assert source.stat() == before
