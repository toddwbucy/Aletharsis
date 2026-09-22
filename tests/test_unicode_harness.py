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
    ('hiberius', 'U+1F600', 'unadjudicated'),
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
    ("document.getElementById('scanVerdict').textContent='';", 'empty written scan verdict'),
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
    assert [d['category'] for d in row['differences'] if d['tool']=='aletharsis'] == ['defect','unadjudicated']
    assert all(d.get('possible_offset_mismatch') for d in row['differences'] if d['tool']=='aletharsis')
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


@pytest.mark.parametrize('tool,known_case,cp,category', [
    ('hiberius', 'emoji_selector', 'U+FE0F', 'inventory'),
    ('hiberius', 'emoji_zwj', 'U+1F469', 'coverage'),
    ('juriku_inventory', 'arabic_letter_mark', 'U+061C', 'inventory'),
    ('juriku_word', 'tag_sequence', 'U+E0061', 'inventory'),
    ('juriku_inventory', 'boundary_offsets', 'U+1F600', 'coverage'),
    ('juriku_word', 'leading_bom', 'U+FEFF', 'policy'),
])
def test_comparator_exemptions_do_not_cover_new_contexts(monkeypatch, tool, known_case, cp, category):
    reason = load('adjudicate', monkeypatch).reason
    assert reason(tool, known_case, cp, 0, 'completed')[0] == category
    assert reason(tool, 'new-context', cp, 0, 'completed')[0] == 'unadjudicated'


@pytest.mark.parametrize('category_assignment,valid', [
    ("n.className='chip hidden';", True),
    ("n.setAttribute('class', 'chip hidden');", False),
    ("", False), ("n.className='';", False),
])
def test_probe_requires_observed_category(tmp_path, category_assignment, valid):
    node = shutil.which('node')
    if node is None: pytest.skip('Node required for synthetic facade checks')
    html = tmp_path/'index.html'
    html.write_text("<script>document.getElementById('btnScan').addEventListener('click',()=>{"
        "const n=document.createElement('span'); n.textContent='U+200B';" + category_assignment +
        "document.getElementById('scanViz').appendChild(n); document.getElementById('scanVerdict').textContent='found';});</script>")
    code = (PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'", json.dumps(str(html)))
    result = subprocess.run([node, '-e', code], input=json.dumps({'text': '\u200b'}), text=True, capture_output=True, timeout=5)
    if valid:
        assert result.returncode == 0, result.stderr
        assert json.loads(result.stdout)['observations'][0]['native_category'] == 'chip hidden'
    else:
        assert result.returncode != 0 and 'scan category contract drift' in result.stderr



def test_nonempty_blank_verdict_is_not_missing_coverage(tmp_path):
    node = shutil.which('node')
    if node is None: pytest.skip('Node required for synthetic facade checks')
    html = tmp_path/'index.html'
    html.write_text("<script>document.getElementById('btnScan').addEventListener('click',()=>{"
        "document.getElementById('scanViz').appendChild(document.createTextNode('a'));"
        "document.getElementById('scanVerdict').textContent='';});</script>")
    code = (PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'", json.dumps(str(html)))
    result = subprocess.run([node, '-e', code], input='{"text":"a"}', text=True, capture_output=True, timeout=5)
    assert result.returncode != 0 and 'empty written scan verdict' in result.stderr


def test_hiberius_escapes_untrusted_output(tmp_path):
    node = shutil.which('node')
    if node is None: pytest.skip('Node required')
    html = tmp_path/'index.html'
    html.write_text("<script>document.getElementById('btnScan').addEventListener('click',()=>{document.getElementById('scanVerdict').textContent='\\u202e\\u{1f600}';document.getElementById('scanSecret').textContent='\\u202e';});</script>")
    code = (PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'", json.dumps(str(html)))
    r = subprocess.run([node,'-e',code],input=b'{"text":""}',capture_output=True,timeout=5)
    assert r.returncode == 0, r.stderr
    assert r.stdout.isascii()
    assert json.loads(r.stdout)['secret_text'] == '\u202e'


@pytest.mark.parametrize('optimize', [False, True])
@pytest.mark.parametrize('returned,changed', [('original',False), ('mutated',False), ('original',True)])
def test_juriku_readonly_check_is_observed(tmp_path, optimize, returned, changed):
    fake = tmp_path/'target.py'
    fake.write_text("emoji_library_available=True\npathspec_library_available=False\nclass emoji: __version__='fake'\nclass SimpleLogger:\n def __init__(self, **kwargs): pass\nclass UnicodeMarkerDetector:\n def __init__(self, **kwargs): pass\n def _process_line(self, text, line): return "+repr((returned, [], changed))+"\n")
    code = (PROBE/'juriku_probe.py').read_text().replace('/upstream/hidden-characters-detector.py',str(fake))
    r = subprocess.run([sys.executable,*(['-O'] if optimize else []),'-c',code],input='{"text":"original"}',text=True,capture_output=True,timeout=5)
    if returned == 'original' and not changed:
        assert r.returncode == 0, r.stderr
        assert json.loads(r.stdout)['input_unchanged'] is True
    else:
        assert r.returncode != 0 and 'read-only detector changed input' in r.stderr
        assert not r.stdout


def test_reviewed_omission_survives_stray_same_codepoint(tmp_path, monkeypatch):
    adjudicate = load('adjudicate', monkeypatch)
    (tmp_path/'corpus.json').write_text(json.dumps([dict(id='hangul_fillers', expected_observations=[dict(scalar=1,code_point='U+115F')], interpretation='synthetic')]))
    case = tmp_path/'hangul_fillers'; case.mkdir()
    (case/'aletharsis.stdout.json').write_text(json.dumps(dict(status='completed', findings=[dict(id='unicode.zero_width',evidence=dict(code_point='U+115F'),location=dict(character_offsets=[9]))])))
    (case/'juriku.stdout.json').write_text(json.dumps(dict(results=dict(inventory=[],word_exclusions=[]))))
    (case/'hiberius.stdout.json').write_text(json.dumps(dict(scan_status='completed',observations=[])))
    rows = [d for d in adjudicate.summarize(tmp_path)[0]['differences'] if d['tool']=='aletharsis']
    assert [d['category'] for d in rows] == ['inventory', 'unadjudicated']
    assert all(d['possible_offset_mismatch'] for d in rows)


@pytest.mark.parametrize('fallback,mutation,succeeds', [(False,False,False),(True,False,True),(True,True,False)])
def test_repacked_source_requires_opt_in_and_exact_tree(tmp_path, monkeypatch, fallback, mutation, succeeds):
    import hashlib
    import io
    import tarfile
    provision = load('provision', monkeypatch)
    downloads = tmp_path/'downloads'; downloads.mkdir()
    archive = downloads/'source.tar.gz'
    with tarfile.open(archive,'w:gz') as out:
        data = b'changed' if mutation else b'original'
        entry = tarfile.TarInfo('root/file.txt'); entry.size = len(data)
        out.addfile(entry, io.BytesIO(data))
    (tmp_path/'input-trees.json').write_text(json.dumps({'juriku':{'file.txt':hashlib.sha256(b'original').hexdigest()}}))
    monkeypatch.setattr(provision,'__file__',str(tmp_path/'provision.py'))
    monkeypatch.setattr(provision,'PINS',{'juriku':('source.tar.gz','0'*64)})
    monkeypatch.setattr(sys,'argv',['provision','--downloads',str(downloads),'--output',str(tmp_path/'output'),*(['--allow-repacked-source'] if fallback else [])])
    if succeeds:
        provision.main()
        receipt=json.loads((tmp_path/'output/provisioning.json').read_bytes())['source.tar.gz']
        assert receipt['tree_verified'] and receipt['observed_sha256']==hashlib.sha256(archive.read_bytes()).hexdigest()
        assert receipt['historical_sha256'] != receipt['observed_sha256']
    else:
        with pytest.raises(ValueError,match='identity mismatch'): provision.main()
        assert not (tmp_path/'output/provisioning.json').exists()


@pytest.mark.parametrize('optimize', [False,True])
@pytest.mark.parametrize('emoji,pathspec', [(False,False),(True,True),(False,True)])
def test_juriku_library_preconditions_survive_optimization(tmp_path, optimize, emoji, pathspec):
    fake=tmp_path/'target.py'
    fake.write_text(f"emoji_library_available={emoji!r}\npathspec_library_available={pathspec!r}\n")
    code=(PROBE/'juriku_probe.py').read_text().replace('/upstream/hidden-characters-detector.py',str(fake))
    result=subprocess.run([sys.executable,*(['-O'] if optimize else []),'-c',code],input='{"text":"abc"}',text=True,capture_output=True,timeout=5)
    assert result.returncode != 0 and 'unexpected detector library availability' in result.stderr
    assert not result.stdout


@pytest.mark.parametrize('mode,diagnostic', [('mutate','read-only detector changed input'),('duplicate','duplicate event listener')])
def test_hiberius_mutation_and_duplicate_listener_fail(tmp_path, mode, diagnostic):
    node=shutil.which('node')
    if node is None: pytest.skip('Node required')
    registration="document.getElementById('btnScan').addEventListener('click',()=>{document.getElementById('scanViz').appendChild(document.createTextNode('a'));document.getElementById('scanVerdict').textContent='done';"+("document.getElementById('scanInput').value='MUTATED';" if mode=='mutate' else '')+"});"
    html=tmp_path/'index.html'; html.write_text('<script>'+registration*(2 if mode=='duplicate' else 1)+'</script>')
    code=(PROBE/'hiberius_probe.cjs').read_text().replace("'/upstream/index.html'",json.dumps(str(html)))
    result=subprocess.run([node,'-e',code],input='{"text":"a"}',text=True,capture_output=True,timeout=5)
    assert result.returncode != 0 and diagnostic in result.stderr
    assert not result.stdout
