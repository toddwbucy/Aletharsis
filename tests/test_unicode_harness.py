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
    ('aletharsis', 'U+1F600', 'defect'),
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
    assert result['reaped'] == result['output_final'] == (not cleanup_timeout)
    if cleanup_timeout: assert result['output_limit_exceeded'] is None


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
    ("document.getElementById('scanSecret').hidden='';", 'non-boolean hidden assignment'),
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
    monkeypatch.setattr(runner, 'validate_trees', lambda *a: None)
    monkeypatch.setattr(runner, 'cases', lambda: [{'id': 'synthetic', 'raw_hex': '61', 'decoded_text': 'a'}])
    args = ['run_cases']
    for name in ('juriku', 'hiberius', 'emoji', 'python', 'binary'):
        args += ['--'+name, str(tmp_path)]
    args += ['--output', str(tmp_path/'output')]
    monkeypatch.setattr(sys, 'argv', args)
    def failed(argv, payload, out, err):
        out.write_bytes(b'partial')
        err.write_bytes(b'')
        return dict(reaped=failure != 'cleanup_timeout', output_final=failure != 'cleanup_timeout', returncode=None, timed_out=failure == 'cleanup_timeout',
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
    def output(command):
        commands.append(command)
        assert command[0] == 'bwrap' and '--unshare-all' in command and '--clearenv' in command
        return version if '/python/bin/python3.12' in command else 'v26.8.2'
    monkeypatch.setattr(runner, 'runtime_version', output)
    monkeypatch.setattr(runner.platform, 'python_version', lambda: 'different-launcher')
    monkeypatch.setattr(runner, 'digest', lambda path: str(path))
    if version != '3.12.13':
        with pytest.raises(ValueError, match='Juriku runtime'):
            runner.runtime_metadata(Path('/supplied/python'))
        return
    result = runner.runtime_metadata(Path('/supplied/python'))
    assert result['python'] == '3.12.13'
    assert result['launcher_python'] == 'different-launcher'
    assert result['python_binary_sha256'] == '/supplied/python/bin/python3.12'
    assert '/supplied/python' in commands[0]
    assert result['node_binary_sha256'] == '/usr/bin/node'
    assert commands[1][-2:] == ['/usr/bin/node', '--version']


def test_incomplete_hiberius_does_not_infer_absence(monkeypatch):
    adjudicate = load('adjudicate', monkeypatch)
    assert adjudicate.reason('hiberius','nonempty','U+200B',0,'completed','no_verdict_emitted')[0] == 'coverage'
    assert adjudicate.reason('hiberius','new-case','U+200B',0,'completed')[0] == 'unadjudicated'


def test_wrong_scalar_is_offset_difference(tmp_path, monkeypatch):
    adjudicate = load('adjudicate', monkeypatch)
    (tmp_path/'corpus.json').write_text(json.dumps([dict(id='shift', expected_observations=[dict(scalar=1,code_point='U+200B')], interpretation='synthetic')]))
    case = tmp_path/'shift'; case.mkdir()
    (case/'aletharsis.stdout.json').write_text(json.dumps(dict(status='completed', findings=[dict(id='unicode.zero_width',evidence=dict(code_point='U+200B'),location=dict(character_offsets=[2]))])))
    (case/'juriku.stdout.json').write_text(json.dumps(dict(results=dict(inventory=[],word_exclusions=[]))))
    (case/'hiberius.stdout.json').write_text(json.dumps(dict(scan_status='no_verdict_emitted',observations=[])))
    row = adjudicate.summarize(tmp_path)[0]
    assert [d['category'] for d in row['differences'] if d['tool']=='aletharsis'] == ['offset','offset']
    assert [d['category'] for d in row['differences'] if d['tool']=='hiberius'] == ['coverage']


@pytest.mark.parametrize('script', [
    "document.addEventListener('DOMContentLoaded',()=>{});",
    "document.getElementById('btnScan').addEventListener('click',()=>{document.getElementById('renamedViz').textContent='a';document.getElementById('renamedVerdict').textContent='done';});",
])
def test_probe_rejects_facade_drift(tmp_path, script):
    node = shutil.which('node')
    if node is None: pytest.skip('Node required for synthetic facade checks')
    html = tmp_path/'index.html'; html.write_text('<script>'+script+'</script>')
    code = (PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'", json.dumps(str(html)))
    result = subprocess.run([node,'-e',code],input='{"text":"a"}',text=True,capture_output=True,timeout=5)
    assert result.returncode != 0
    assert 'unsupported document event' in result.stderr or 'scan output contract drift' in result.stderr


def test_probe_identity_excludes_docs(monkeypatch):
    runner = load('run_cases', monkeypatch)
    assert set(runner.PROBE_FILES) == {'corpus.py','adjudicate.py','run_cases.py','juriku_probe.py','hiberius_probe.cjs','input-trees.json','provision.py'}


@pytest.mark.parametrize('case,cp,expected', [('hangul_fillers','U+200B','defect'),('hangul_fillers','U+115F','inventory'),('new_typography_case','U+2026','defect')])
def test_native_exemptions_are_scoped(monkeypatch, case, cp, expected):
    assert load('adjudicate',monkeypatch).reason('aletharsis',case,cp,0,'completed')[0] == expected


def test_only_hashed_probe_files_are_mounted(monkeypatch):
    runner=load('run_cases',monkeypatch)
    argv=runner.namespace('/upstream-tree','/local-probe',None,None,'/input','/binary')
    mounts=[argv[i+1:i+3] for i,a in enumerate(argv) if a=='--ro-bind']
    assert ['/local-probe','/probe'] not in mounts
    assert {target for _,target in mounts if target.startswith('/probe/')} == {'/probe/'+p for p in runner.PROBE_FILES}


@pytest.mark.parametrize('keys', [[], ['juriku','hiberius'], ['juriku','hiberius','emoji','unexpected']])
def test_tree_pin_coverage_is_exact(tmp_path,monkeypatch,keys):
    runner=load('run_cases',monkeypatch)
    (tmp_path/'input-trees.json').write_text(json.dumps(dict.fromkeys(keys,{})))
    monkeypatch.setattr(runner,'__file__',str(tmp_path/'run_cases.py'))
    with pytest.raises(ValueError,match='tree coverage'): runner.validate_trees(object())


def test_runtime_probe_uses_bounded_executor(monkeypatch):
    runner=load('run_cases',monkeypatch)
    def execute(argv,payload,out,err):
        assert argv==['synthetic'] and payload==b''
        out.write_text('3.12.13\n');err.write_text('')
        return dict(reaped=True,timed_out=False,output_limit_exceeded=False,returncode=0)
    monkeypatch.setattr(runner,'execute',execute)
    assert runner.runtime_version(['synthetic'])=='3.12.13'
