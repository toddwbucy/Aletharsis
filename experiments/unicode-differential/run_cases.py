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
import sys
import time

from corpus import cases

LIMIT = 8 * 1024**2

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def namespace(upstream, probe, python, emoji, input_file, binary):
    args=['bwrap','--unshare-all','--die-with-parent','--new-session','--cap-drop','ALL','--clearenv']
    for path in ('/usr','/lib','/lib64'):
        args += ['--ro-bind',path,path]
    args += ['--symlink','usr/bin','/bin','--proc','/proc','--dev','/dev','--tmpfs','/tmp',
             '--ro-bind',str(upstream),'/upstream','--ro-bind',str(probe),'/probe',
             '--ro-bind',str(python),'/python','--ro-bind',str(emoji),'/emoji',
             '--ro-bind',str(input_file),'/input.txt','--ro-bind',str(binary),'/aletharsis']
    for key,value in {'PATH':'/usr/bin:/bin','PYTHONDONTWRITEBYTECODE':'1','PYTHONHASHSEED':'0',
                      'PYTHONNOUSERSITE':'1','HOME':'/nonexistent','GOMEMLIMIT':'384MiB'}.items():
        args += ['--setenv',key,value]
    return args


def execute(argv, payload, out, err):
    # Per-stream 4 MiB caps bound combined output to the 8 MiB issue ceiling.
    command=['prlimit','--cpu=25:25','--fsize=4194304:4194304','--nofile=128:128','--',*argv]
    start=time.monotonic()
    with out.open('xb') as stdout, err.open('xb') as stderr:
        proc=subprocess.Popen(command,stdin=subprocess.PIPE,stdout=stdout,stderr=stderr,start_new_session=True)
        timed_out=False
        try:
            proc.communicate(payload,timeout=25)
        except subprocess.TimeoutExpired:
            timed_out=True
            os.killpg(proc.pid,signal.SIGKILL)
            proc.communicate(timeout=5)
    if out.stat().st_size + err.stat().st_size > LIMIT:
        raise ValueError('combined output ceiling exceeded')
    return {'returncode':proc.returncode,'timed_out':timed_out,'wall_seconds':round(time.monotonic()-start,6)}


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('juriku','hiberius','emoji','python','binary','output'):
        p.add_argument('--'+name,type=Path,required=True)
    args=p.parse_args()
    for name in vars(args): setattr(args,name,getattr(args,name).resolve())
    # Every supplied upstream file must match the independently retained tree
    # inventory obtained from the hash-pinned archives, including emoji data.
    trees=json.loads((Path(__file__).parent/'input-trees.json').read_bytes())
    for name, expected in trees.items():
        root=getattr(args,name)
        actual={p.relative_to(root).as_posix():digest(p) for p in sorted(root.rglob('*')) if p.is_file()}
        if actual != expected or any(p.is_symlink() for p in root.rglob('*')):
            raise ValueError('pinned comparator tree mismatch: '+name)
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
            ('aletharsis',args.juriku,['/aletharsis','audit','/input.txt','--json']),
        ]:
            argv=namespace(upstream,probe,args.python,args.emoji,source,args.binary)+command
            commands.setdefault(tool,argv)
            out=directory/f'{tool}.stdout.json';err=directory/f'{tool}.stderr'
            run=execute(argv,payload,out,err)
            run.update(stdout_sha256=digest(out),stderr_sha256=digest(err),source_unchanged=digest(source)==before)
            record['tools'][tool]=run
            (args.output/'results.json').write_text(json.dumps(records+[record],indent=2)+'\n')
            allowed=range(5) if tool=='aletharsis' else [0]
            if run['timed_out'] or run['returncode'] not in allowed or not run['source_unchanged']:
                raise RuntimeError(f'partial study retained: {case["id"]}/{tool} failed')
            json.loads(out.read_bytes())
        records.append(record)
    used=sum(p.stat().st_size for p in args.output.rglob('*') if p.is_file())
    if used>2*1024**3: raise ValueError('retained output disk ceiling exceeded')
    usage=resource.getrusage(resource.RUSAGE_CHILDREN)
    environment={'python':platform.python_version(),'platform':platform.platform(),
        'binary_sha256':digest(args.binary),'probe_sha256':{p.name:digest(p) for p in probe.glob('*') if p.is_file()},
        'example_argv':commands,'child_cpu_seconds':usage.ru_utime+usage.ru_stime,'retained_bytes':used,
        'node':subprocess.check_output(['node','--version'],timeout=5,text=True).strip(),
        'node_binary_sha256':digest(Path('/usr/bin/node'))}
    (args.output/'environment.json').write_text(json.dumps(environment,indent=2)+'\n')
    print(f'{len(records)} cases completed; retained bytes={used}; child CPU seconds={usage.ru_utime+usage.ru_stime:.3f}')


if __name__=='__main__':main()
