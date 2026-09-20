"""Measure synthetic CLI workloads on Linux; GNU time reports child peak RSS.

No auditor imports. Results retain corpus/build identities; output is a new
experiment directory. Timings characterize this machine, not a CI threshold.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import signal
import statistics
import sys
import subprocess
import threading
import time

ROOT = Path(__file__).resolve().parents[1]
MAX_BYTES = 8 * 1024 * 1024


def sha(data):
    return hashlib.sha256(data).hexdigest()


def command_output(args):
    return subprocess.check_output(args, cwd=ROOT, text=True, timeout=30).strip()


def generate(case, size):
    prefix, repeat, suffix = (case[key].encode('utf-8') for key in ('prefix', 'repeat', 'suffix'))
    remaining = size - len(prefix) - len(suffix)
    if remaining < 0 or not repeat:
        raise ValueError('Invalid corpus size or empty repeat unit')
    return prefix + repeat * (remaining // len(repeat)) + b' ' * (remaining % len(repeat)) + suffix


def validate_v2_report(report):
    # Validation is outside child timings. Use the independent test-only oracle;
    # neither the production importer nor an untrusted report supplies checks.
    from jsonschema import Draft202012Validator, ValidationError
    sys.path.insert(0, str(ROOT / "tests/contracts"))
    from support import schema, validate_semantics
    try:
        Draft202012Validator(schema("2.0")).validate(report)
        validate_semantics(report)
    except ValidationError as exc:
        raise ValueError("invalid v2 wire report") from exc


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--go', default='go')
    parser.add_argument('--time', default='/usr/bin/time')
    parser.add_argument('--sizes', default='65536,1048576,8388608')
    parser.add_argument('--repeats', type=int, default=3)
    parser.add_argument('--timeout', type=int, default=120)
    parser.add_argument('--workers', default='1,2,4')
    parser.add_argument('--concurrency-size', type=int, default=65536)
    parser.add_argument('--keep-reports', action='store_true')
    parser.add_argument('--schema-version', choices=('1.0', '2.0'), default='1.0')
    args = parser.parse_args()
    sizes = [int(s) for s in args.sizes.split(',')]
    workers = [int(s) for s in args.workers.split(',')]
    if platform.system() != 'Linux':
        parser.error('Linux and GNU time are required for this measurement harness')
    if any(s < 16 or s > MAX_BYTES for s in [*sizes, args.concurrency_size]):
        parser.error('Sizes must be between 16 bytes and the existing 8 MiB cap')
    if len(set(sizes)) != len(sizes) or len(set(workers)) != len(workers):
        parser.error('Sizes and worker counts must be unique')
    if not 1 <= args.repeats <= 10 or not 1 <= args.timeout <= 600 or any(w < 1 or w > 4 for w in workers):
        parser.error('Use 1–10 repeats, 1–600 seconds, and 1–4 workers')
    binary = args.binary.resolve()
    build = command_output([args.go, 'version', '-m', str(binary)])
    time_version = command_output([args.time, '--version'])
    if 'GNU' not in time_version:
        parser.error('--time must identify GNU time')
    spec_bytes = (ROOT / 'benchmarks/corpus.json').read_bytes()
    spec = json.loads(spec_bytes)
    if any(not re.fullmatch('[a-z0-9_]+', c['name']) for c in spec['cases']):
        parser.error('Corpus names must be simple filename components')
    if len({c['name'] for c in spec['cases']}) != len(spec['cases']):
        parser.error('Corpus names must be unique')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    inputs = output / 'inputs'
    reports = output / 'reports'
    inputs.mkdir(); reports.mkdir()
    cpu = next((line.split(':', 1)[1].strip() for line in Path('/proc/cpuinfo').read_text().splitlines()
                if line.startswith('model name')), 'unknown')
    result = {'schema_version': 1, 'environment': {
        'source_commit': command_output(['git', 'rev-parse', 'HEAD']),
        'worktree_dirty': bool(command_output(['git', 'status', '--porcelain'])),
        'binary_sha256': sha(binary.read_bytes()), 'go_build': build,
        'harness_sha256': sha(Path(__file__).read_bytes()), 'corpus_spec_sha256': sha(spec_bytes),
        'python': platform.python_version(), 'platform': platform.platform(), 'cpu_model': cpu,
        'logical_cpus': os.cpu_count(), 'gomaxprocs_per_child': 2, 'gnu_time': time_version,
        'timeout_seconds': args.timeout, 'repeats': args.repeats,
        'report_schema_version': args.schema_version,
        'sizes': sizes, 'workers': workers, 'concurrency_size': args.concurrency_size,
        'timing_scope': 'CLI process including GNU time wrapper, warm filesystem cache; JSON inspection excluded',
        'rss_scope': 'GNU time maximum resident set of each child, Linux KiB; excludes Python parent',
    }, 'corpus': [], 'runs': [], 'concurrency': []}
    files = {}
    for case in spec['cases']:
        for size in sorted(set(sizes + [args.concurrency_size])):
            data = generate(case, size)
            if size == 65536 and sha(data) != case['sha256_65536']:
                raise ValueError('Corpus hash differs from pinned 64 KiB identity')
            name = f'{case["name"]}-{size}.txt'
            path = inputs / name
            with path.open('xb') as f:
                f.write(data)
            identity = {'name': case['name'], 'filename': name, 'bytes': len(data),
                        'sha256': sha(data), 'characters': len(data.decode('utf-8'))}
            files[(case['name'], size)] = identity
            result['corpus'].append(identity)
    (output / 'corpus-manifest.json').write_text(json.dumps(result['corpus'], indent=2) + '\n')

    active = set()
    process_lock = threading.RLock()
    stopping = False

    def kill(child):
        if child.poll() is None:
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass

    def stop(signum, frame):
        nonlocal stopping
        with process_lock:
            stopping = True
            for child in active:
                kill(child)
        raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def measure(identity, label):
        report_path = reports / (label + '.json')
        stats_path = reports / (label + '.time')
        stderr_path = reports / (label + '.stderr')
        command = [args.time, '-q', '-f', '%e %U %S %M', '-o', str(stats_path),
                   str(binary), 'audit', identity['filename'], '--json']
        if args.schema_version == '2.0':
            command += ['--schema-version=2.0']
        started = time.perf_counter()
        timed_out = False
        with report_path.open('xb') as report_file, stderr_path.open('xb') as stderr:
            with process_lock:
                if stopping:
                    raise RuntimeError('Measurement interrupted')
                child = subprocess.Popen(command, cwd=inputs, stdout=report_file, stderr=stderr,
                                         env=os.environ | {'GOMAXPROCS': '2'}, start_new_session=True)
                active.add(child)
            def expire():
                nonlocal timed_out
                if child.poll() is None:
                    timed_out = True
                    kill(child)
            timer = threading.Timer(args.timeout, expire)
            timer.start()
            try:
                # Blocking wait avoids Popen.wait(timeout)'s polling delay
                # contaminating measurements of small inputs.
                code = child.wait()
            finally:
                timer.cancel()
                timer.join()
                kill(child)
                child.wait()
                with process_lock:
                    active.discard(child)
        wall = time.perf_counter() - started
        row = {'case': identity['name'], 'input_bytes': identity['bytes'], 'label': label,
               'wall_seconds': wall, 'exit_code': code, 'timed_out': timed_out,
               'output_bytes': report_path.stat().st_size, 'peak_rss_kib': None,
               'cpu_user_seconds': None, 'cpu_system_seconds': None, 'valid_report': False}
        if stats_path.exists():
            fields = stats_path.read_text().strip().split()
            if len(fields) == 4:
                row.update(gnu_elapsed_seconds=float(fields[0]), cpu_user_seconds=float(fields[1]),
                           cpu_system_seconds=float(fields[2]), peak_rss_kib=int(fields[3]))
        if not timed_out and code in (0, 1, 2, 3):
            try:
                with report_path.open() as f:
                    report = json.load(f)
                row['valid_report'] = (report['status'] == 'completed'
                    and report['file']['sha256'] == identity['sha256']
                    and report['file']['size'] == identity['bytes']
                    and report['summary']['exit_code'] == code)
                if args.schema_version == '2.0':
                    validate_v2_report(report)
                row['findings'] = report['summary']['findings']
                row['text_offset_entries'] = sum(len(t['byte_offsets']) for t in report['evidence']['texts'])
                row['finding_offset_entries'] = sum(len(f['location'].get('byte_offsets', [])) for f in report['findings'])
            except (ValueError, KeyError, TypeError):
                row['valid_report'] = False
        row['stderr'] = stderr_path.read_text(errors='backslashreplace')
        row['resource_rejected'] = (args.schema_version == '2.0' and not timed_out
            and code == 4 and row['output_bytes'] == 0
            and row['stderr'] == 'aletharsis: execution.resource_limit: could not assemble a complete report\n')
        row['measurement_valid'] = ((row['valid_report'] or row['resource_rejected'])
            and row['peak_rss_kib'] is not None)
        if row['valid_report'] and not args.keep_reports:
            report_path.unlink()
        return row

    def save():
        (output / 'results.json').write_text(json.dumps(result, indent=2, sort_keys=True) + '\n')

    save()
    for size in sizes:
        for case in spec['cases']:
            identity = files[(case['name'], size)]
            for repeat in range(args.repeats):
                row = measure(identity, f'{case["name"]}-{size}-{repeat}')
                result['runs'].append(row)
                save()
                print(json.dumps(row), flush=True)
    for count in workers:
        jobs = [(files[(case['name'], args.concurrency_size)], f'concurrent-{count}-{i}-{case["name"]}')
                for i in range(2) for case in spec['cases']]
        started = time.perf_counter()
        with ThreadPoolExecutor(max_workers=count) as pool:
            rows = list(pool.map(lambda job: measure(*job), jobs))
        batch_wall = time.perf_counter() - started
        peaks = [r['peak_rss_kib'] for r in rows if r['peak_rss_kib'] is not None]
        result['concurrency'].append({'workers': count, 'jobs': len(jobs), 'input_bytes': args.concurrency_size,
            'batch_wall_seconds': batch_wall, 'jobs_per_second': len(jobs) / batch_wall,
            'median_process_wall_seconds': statistics.median(r['wall_seconds'] for r in rows),
            'max_process_peak_rss_kib': max(peaks, default=None),
            'overlapping_peak_upper_bound_kib': sum(sorted(peaks, reverse=True)[:count]) if len(peaks) == len(rows) else None,
            'memory_note': 'Sum of largest N individual peaks is a conservative envelope, not measured simultaneous RSS',
            'batch_note': 'Batch time includes parent report validation and scheduling', 'runs': rows})
        save()
        print(f'Concurrency {count}: {batch_wall:.3f}s for {len(jobs)} jobs', flush=True)
    return int(any(not r['measurement_valid'] for r in result['runs']) or
               any(not r['measurement_valid'] for b in result['concurrency'] for r in b['runs']))


if __name__ == '__main__':
    raise SystemExit(main())
