"""Offline Linux probes. Run under the documented 512 MiB systemd cgroup."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import resource
import signal
import subprocess
import time
import tempfile

from corpus import cases

LIMIT = 8 * 1024**2
PROBE_FILES = ('corpus.py', 'adjudicate.py', 'run_cases.py', 'juriku_probe.py',
               'hiberius_probe.cjs', 'input-trees.json', 'provision.py')

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def base_namespace():
    args=['bwrap','--unshare-all','--die-with-parent','--new-session','--cap-drop','ALL','--clearenv']
    for path in ('/usr','/lib','/lib64'):
        args += ['--ro-bind',path,path]
    args += ['--symlink','usr/bin','/bin','--proc','/proc','--dev','/dev','--tmpfs','/tmp']
    for key,value in {'PATH':'/usr/bin:/bin','PYTHONDONTWRITEBYTECODE':'1','PYTHONHASHSEED':'0',
                      'PYTHONNOUSERSITE':'1','HOME':'/nonexistent','GOMEMLIMIT':'384MiB'}.items():
        args += ['--setenv',key,value]
    return args


def namespace(upstream, probe, python, emoji, input_file, binary):
    args=base_namespace()+['--ro-bind',str(input_file),'/input.txt','--ro-bind',str(binary),'/aletharsis']
    if upstream is not None:
        args += ['--ro-bind',str(upstream),'/upstream','--dir','/probe']
        for name in PROBE_FILES:
            args += ['--ro-bind',str(Path(probe)/name),'/probe/'+name]
        for root, target in ((python, '/python'), (emoji, '/emoji')):
            if root is not None:
                args += ['--ro-bind',str(root),target]
    return args


def runtime_version(argv):
    # Use the same bounded execution and offline namespace as study probes.
    with tempfile.TemporaryDirectory(prefix='aletharsis-runtime-') as directory:
        out, err = Path(directory)/'out', Path(directory)/'err'
        result = execute(argv, b'', out, err)
        if not result['reaped'] or result['timed_out'] or result['output_limit_exceeded'] or result['returncode'] != 0:
            raise ValueError('bounded runtime version probe failed')
        return out.read_text().strip()


def runtime_metadata(python):
    interpreter = python/'bin/python3.12'
    version = runtime_version(base_namespace()+['--ro-bind',str(python),'/python',
        '/python/bin/python3.12', '-I', '-c', 'import platform; print(platform.python_version())'])
    if version != '3.12.13':
        raise ValueError('Juriku runtime must be Python 3.12.13')
    return {'python':version, 'python_binary_sha256':digest(interpreter),
        'launcher_python':platform.python_version(),
        'node':runtime_version(base_namespace()+['/usr/bin/node','--version']),
        'node_binary_sha256':digest(Path('/usr/bin/node'))}


def validate_trees(args):
    trees=json.loads((Path(__file__).parent/'input-trees.json').read_bytes())
    if set(trees) != {'juriku','hiberius','emoji'}:
        raise ValueError('pinned comparator tree coverage mismatch')
    for name, expected in trees.items():
        root=getattr(args,name)
        actual={p.relative_to(root).as_posix():digest(p) for p in sorted(root.rglob('*')) if p.is_file()}
        if actual != expected or any(p.is_symlink() for p in root.rglob('*')):
            raise ValueError('pinned comparator tree mismatch: '+name)


def execute(argv, payload, out, err):
    # Per-stream 4 MiB caps bound combined output to the 8 MiB issue ceiling.
    command=['prlimit','--cpu=25:25','--fsize=4194304:4194304','--nofile=128:128','--',*argv]
    start=time.monotonic()
    with out.open('xb') as stdout, err.open('xb') as stderr:
        proc=subprocess.Popen(command,stdin=subprocess.PIPE,stdout=stdout,stderr=stderr,start_new_session=True)
        timed_out=False
        cleanup_timed_out=False
        try:
            proc.communicate(payload,timeout=25)
        except subprocess.TimeoutExpired:
            timed_out=True
            try:
                os.killpg(proc.pid,signal.SIGKILL)
            except ProcessLookupError:
                pass
            try:
                proc.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                cleanup_timed_out=True
    # Defensive check if per-stream limits change; equality is within budget.
    reaped = proc.returncode is not None
    output_limit_exceeded=(out.stat().st_size + err.stat().st_size > LIMIT) if reaped else None
    return {'reaped':reaped,'output_final':reaped,'returncode':proc.returncode,'timed_out':timed_out,'cleanup_timed_out':cleanup_timed_out,
            'output_limit_exceeded':output_limit_exceeded,'wall_seconds':round(time.monotonic()-start,6)}


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('juriku','hiberius','emoji','python','binary','output'):
        p.add_argument('--'+name,type=Path,required=True)
    args=p.parse_args()
    for name in vars(args): setattr(args,name,getattr(args,name).resolve())
    # Every supplied upstream file must match the independently retained tree
    # inventory obtained from the hash-pinned archives, including emoji data.
    validate_trees(args)
    runtimes=runtime_metadata(args.python)
    args.output.mkdir(exist_ok=False)
    probe=Path(__file__).resolve().parent
    corpus=cases()
    (args.output/'corpus.json').write_text(json.dumps(corpus,indent=2,ensure_ascii=True)+'\n')
    records=[]
    commands={}
    for case in corpus:
        directory=args.output/case['id'];directory.mkdir()
        source=directory/'source.bin';source.write_bytes(bytes.fromhex(case['raw_hex']))
        before=digest(source)
        payload=json.dumps({'text':case['decoded_text']},ensure_ascii=True).encode()
        assert len(payload)<=LIMIT and source.stat().st_size<=LIMIT
        record={'case':case['id'],'source_sha256':before,'tools':{}}
        for tool,upstream,command in [
            ('juriku',args.juriku,['/python/bin/python3.12','/probe/juriku_probe.py']),
            ('hiberius',args.hiberius,['/usr/bin/node','--max-old-space-size=128','/probe/hiberius_probe.cjs']),
            ('aletharsis',None,['/aletharsis','audit','/input.txt','--json']),
        ]:
            argv=namespace(upstream,probe,args.python if tool=='juriku' else None,
                args.emoji if tool=='juriku' else None,source,args.binary)+command
            commands.setdefault(tool,argv)
            out=directory/f'{tool}.stdout.json';err=directory/f'{tool}.stderr'
            run=execute(argv,payload,out,err)
            # Unreaped children may still write: no final digests/size claim.
            run.update(stdout_sha256=digest(out) if run['output_final'] else None,
                       stderr_sha256=digest(err) if run['output_final'] else None,
                       source_unchanged=digest(source)==before)
            record['tools'][tool]=run
            (args.output/'results.json').write_text(json.dumps(records+[record],indent=2)+'\n')
            allowed=range(5) if tool=='aletharsis' else [0]
            if not run['reaped'] or run['timed_out'] or run['output_limit_exceeded'] or run['returncode'] not in allowed or not run['source_unchanged']:
                raise RuntimeError(f'partial study retained: {case["id"]}/{tool} failed')
            json.loads(out.read_bytes())
        records.append(record)
    used=sum(p.stat().st_size for p in args.output.rglob('*') if p.is_file())
    if used>2*1024**3: raise ValueError('retained output disk ceiling exceeded')
    usage=resource.getrusage(resource.RUSAGE_CHILDREN)
    environment={**runtimes,'platform':platform.platform(),
        'binary_sha256':digest(args.binary),'probe_sha256':{name:digest(probe/name) for name in PROBE_FILES},
        'example_argv':commands,'child_cpu_seconds':usage.ru_utime+usage.ru_stime,'retained_bytes':used}
    (args.output/'environment.json').write_text(json.dumps(environment,indent=2)+'\n')
    print(f'{len(records)} cases completed; retained bytes={used}; child CPU seconds={usage.ru_utime+usage.ru_stime:.3f}')


if __name__=='__main__':main()
