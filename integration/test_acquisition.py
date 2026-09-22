"""Real kernel rejection paths supplement deterministic internal fault tests."""
import json
import os
import socket

import pytest


def assert_no_snapshot(result, validator):
    assert result.returncode == 4 and result.stderr == b''
    report = json.loads(result.stdout)
    validator.validate(report)
    assert report['status'] == 'failed' and report['summary']['exit_code'] == 4
    assert report['file']['sha256'] is None
    assert report['file']['size'] is None
    assert report['file']['parser'] is None
    assert report['evidence']['texts'] == []
    assert [f['id'] for f in report['findings']] == ['parser.failure']
    return report


@pytest.mark.parametrize('kind', ['fifo', 'socket', 'symlink'])
def test_nonregular_sources_fail_without_blocking(run_cli, validator, tmp_path, kind):
    path = tmp_path / 'source'
    sock = None
    target = None
    if kind == 'fifo':
        os.mkfifo(path)
    elif kind == 'socket':
        sock = socket.socket(socket.AF_UNIX)
        sock.bind(str(path))
    else:
        target = tmp_path / 'target.txt'
        target.write_bytes(b'unchanged\n')
        path.symlink_to(target)
    before = path.lstat()
    target_before = target.stat() if target else None
    try:
        # The shared runner kills a candidate that blocks opening a FIFO.
        assert_no_snapshot(run_cli('audit', path, '--json'), validator)
        assert path.lstat() == before
        if target:
            assert target.stat() == target_before
            assert target.read_bytes() == b'unchanged\n'
            assert path.is_symlink()
    finally:
        if sock:
            sock.close()


def test_unreadable_source_does_not_relax_acquisition(run_cli, validator, tmp_path):
    if os.geteuid() == 0:
        pytest.skip('Root can bypass mode permissions; injected EACCES/EPERM tests still run')
    path = tmp_path / 'denied.txt'
    path.write_bytes(b'unchanged\n')
    path.chmod(0)
    before = path.stat()
    try:
        report = assert_no_snapshot(run_cli('audit', path, '--json'), validator)
        assert report['findings'][0]['evidence']['error_type'] == 'PermissionError'
        assert path.stat() == before
    finally:
        path.chmod(0o600)
    assert path.read_bytes() == b'unchanged\n'
