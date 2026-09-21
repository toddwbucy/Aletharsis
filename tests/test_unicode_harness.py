"""Harness regressions using synthetic handlers/processes, never upstream code."""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys

import pytest

PROBE = Path(__file__).resolve().parents[1] / 'experiments/unicode-differential'


def load(name, monkeypatch):
    monkeypatch.syspath_prepend(str(PROBE))
    spec = importlib.util.spec_from_file_location(name, PROBE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.mark.parametrize('tool,cp,expected', [
    ('juriku_inventory', 'U+2026', 'inventory'),
    ('juriku_word', 'U+2026', 'policy'),
    ('aletharsis', 'U+2026', 'policy'),
    ('hiberius', 'U+2026', 'policy'),
    ('aletharsis', 'U+1F600', 'inventory'),
    ('hiberius', 'U+1F600', 'coverage'),
])
def test_tool_specific_adjudication(monkeypatch, tool, cp, expected):
    category, detail = load('adjudicate', monkeypatch).reason(tool, 'word_typography', cp, 0, 'completed')
    assert category == expected
    if tool == 'aletharsis' and cp == 'U+1F600':
        assert 'Aletharsis deliberately does' not in detail


def test_native_namespace_excludes_upstream(monkeypatch):
    runner = load('run_cases', monkeypatch)
    native = runner.namespace(None, '/probe', '/python', '/emoji', '/input', '/binary')
    assert not {'/upstream', '/probe', '/python', '/emoji'} & set(native)
    hiberius = runner.namespace('/tree', '/probe', None, None, '/input', '/binary')
    assert not {'/python', '/emoji'} & set(hiberius)
    assert '/upstream' in runner.namespace('/tree', '/probe', '/python', '/emoji', '/input', '/binary')


@pytest.mark.parametrize('cleanup_timeout', [False, True])
def test_timeout_retains_cleanup_outcome(tmp_path, monkeypatch, cleanup_timeout):
    runner = load('run_cases', monkeypatch)
    class Process:
        pid = 123
        returncode = None
        calls = 0
        def communicate(self, *args, **kwargs):
            self.calls += 1
            if self.calls == 1 or cleanup_timeout:
                raise subprocess.TimeoutExpired('synthetic', kwargs['timeout'])
            self.returncode = -9
    proc = Process()
    monkeypatch.setattr(runner.subprocess, 'Popen', lambda *a, **k: proc)
    monkeypatch.setattr(runner.os, 'killpg', lambda *a: None)
    result = runner.execute(['synthetic'], b'', tmp_path/'out', tmp_path/'err')
    assert result['timed_out']
    assert result['cleanup_timed_out'] == cleanup_timeout
    assert proc.calls == 2


@pytest.mark.parametrize('size,exceeded', [(8*1024**2, False), (8*1024**2+1, True)])
def test_defensive_output_budget_retains_result(tmp_path, monkeypatch, size, exceeded):
    runner = load('run_cases', monkeypatch)
    class Process:
        returncode = 0
        def communicate(self, *a, **k):
            with (tmp_path/'out').open('r+b') as stream:
                stream.truncate(size)
    monkeypatch.setattr(runner.subprocess, 'Popen', lambda *a, **k: Process())
    result = runner.execute(['synthetic'], b'', tmp_path/'out', tmp_path/'err')
    assert result['output_limit_exceeded'] == exceeded


@pytest.mark.parametrize('handler,expected', [
    ("", 'no_verdict_emitted'),
    ("document.getElementById('scanVerdict').textContent='scanned empty'; document.getElementById('scanSecret').hidden=true;", 'completed'),
    ("const n=document.createElement('span'); n.textContent='U+200B'; document.getElementById('scanViz').appendChild(n);", 'visual map overran input'),
    ("while(true) {}", 'Script execution timed out'),
])
def test_probe_observes_handler_and_bounds_scan(tmp_path, handler, expected):
    node = shutil.which('node')
    if node is None:
        pytest.skip('Node is required for synthetic DOM probe tests')
    html = tmp_path/'index.html'
    html.write_text("<script>document.getElementById('btnScan').addEventListener('click',()=>{" + handler + "});</script>")
    code = (PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'", json.dumps(str(html)))
    result = subprocess.run([node, '-e', code], input='{"text":""}', text=True, capture_output=True, timeout=5)
    if expected in ('completed', 'no_verdict_emitted'):
        assert result.returncode == 0, result.stderr
        observation = json.loads(result.stdout)
        assert observation['scan_status'] == expected
        assert observation['secret_visible'] is (False if expected == 'completed' else None)
    else:
        assert result.returncode != 0 and expected in result.stderr


@pytest.mark.parametrize('failure', ['cleanup_timeout', 'output_limit'])
def test_failing_case_is_retained_before_abort(tmp_path, monkeypatch, failure):
    runner = load('run_cases', monkeypatch)
    (tmp_path/'input-trees.json').write_text('{}')
    monkeypatch.setattr(runner, '__file__', str(tmp_path/'run_cases.py'))
    monkeypatch.setattr(runner, 'runtime_metadata', lambda *a: {})
    monkeypatch.setattr(runner, 'cases', lambda: [{'id': 'synthetic', 'raw_hex': '61', 'decoded_text': 'a'}])
    args = ['run_cases']
    for name in ('juriku', 'hiberius', 'emoji', 'python', 'binary'):
        args += ['--'+name, str(tmp_path)]
    args += ['--output', str(tmp_path/'output')]
    monkeypatch.setattr(sys, 'argv', args)
    def failed(argv, payload, out, err):
        out.write_bytes(b'partial')
        err.write_bytes(b'')
        return dict(returncode=None, timed_out=failure == 'cleanup_timeout',
                    cleanup_timed_out=failure == 'cleanup_timeout',
                    output_limit_exceeded=failure == 'output_limit', wall_seconds=30)
    monkeypatch.setattr(runner, 'execute', failed)
    with pytest.raises(RuntimeError, match='partial study retained'):
        runner.main()
    records = json.loads((tmp_path/'output/results.json').read_bytes())
    assert records[0]['case'] == 'synthetic'
    result = records[0]['tools']['juriku']
    assert result['source_unchanged']
    assert result['cleanup_timed_out'] if failure == 'cleanup_timeout' else result['output_limit_exceeded']


@pytest.mark.parametrize('version', ['3.12.13', '3.13.0'])
def test_runtime_identity_uses_probe_binaries(monkeypatch, version):
    runner = load('run_cases', monkeypatch)
    commands = []
    def output(command, **kwargs):
        commands.append(command)
        assert kwargs == dict(timeout=5, text=True)
        return version if command[0] == '/supplied/python/bin/python3.12' else 'v26.8.1'
    monkeypatch.setattr(runner.subprocess, 'check_output', output)
    monkeypatch.setattr(runner.platform, 'python_version', lambda: 'different-launcher')
    monkeypatch.setattr(runner, 'digest', lambda path: str(path))
    if version != '3.12.13':
        with pytest.raises(ValueError, match='Juriku runtime'):
            runner.runtime_metadata(Path('/supplied/python'))
        return
    result = runner.runtime_metadata(Path('/supplied/python'))
    assert result['python'] == '3.12.13'
    assert result['launcher_python'] == 'different-launcher'
    assert result['python_binary_sha256'] == commands[0][0]
    assert result['node_binary_sha256'] == commands[1][0] == '/usr/bin/node'
    assert commands[1] == ['/usr/bin/node', '--version']
