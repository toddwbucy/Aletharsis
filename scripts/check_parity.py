"""Compare Go CLI output to immutable Python reports; no Python auditor import.

Only the implementation version, finding ordering, JSON formatting, and backend
failure message wording may differ. All other report values must be identical.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

AUDIT_TIMEOUT_SECONDS = 30


def canonical(report):
    report = json.loads(json.dumps(report))
    report.pop('aletharsis_version')
    for finding in report['findings']:
        if finding['id'] == 'parser.failure':
            finding['evidence'].pop('message')
    report['findings'].sort(key=lambda f: json.dumps(f, sort_keys=True))
    return report


def differences(expected, actual, path=''):
    numeric_pair = type(expected) in (int, float) and type(actual) in (int, float)
    if type(expected) is not type(actual) and not numeric_pair:
        yield f'{path}: {type(expected).__name__} != {type(actual).__name__}'
    elif isinstance(expected, dict):
        for k in sorted(expected.keys() | actual.keys()):
            if k not in expected or k not in actual:
                yield f'{path}/{k}: missing/extra field'
            else:
                yield from differences(expected[k], actual[k], f'{path}/{k}')
    elif isinstance(expected, list):
        if len(expected) != len(actual):
            yield f'{path}: length {len(expected)} != {len(actual)}'
        else:
            for i, (a, b) in enumerate(zip(expected, actual)):
                yield from differences(a, b, f'{path}/{i}')
    elif expected != actual:
        yield f'{path}: {expected!r} != {actual!r}'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('binary', type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve()
    root = Path('reference/python-behavior')
    manifest = json.loads((root/'manifest.json').read_text())
    for name, key in [('report.schema.json', 'schema_sha256'), ('unicode.json', 'unicode_oracle_sha256')]:
        assert hashlib.sha256((root/name).read_bytes()).hexdigest() == manifest[key], f'Reference changed: {name}'
    for name, digest in manifest['files'].items():
        assert hashlib.sha256(Path(name).read_bytes()).hexdigest() == digest, f'Python reference changed: {name}'
    failures = []
    for case in manifest['cases']:
        assert hashlib.sha256(Path(case['input']).read_bytes()).hexdigest() == case['sha256']
        assert hashlib.sha256((root/case['report']).read_bytes()).hexdigest() == case['report_sha256']
        expected = json.loads((root/case['report']).read_text())
        errors = []
        try:
            run = subprocess.run([str(binary), 'audit', case['input'], '--json'], capture_output=True, text=True, timeout=AUDIT_TIMEOUT_SECONDS)
            actual = json.loads(run.stdout)
            errors = list(differences(canonical(expected), canonical(actual)))
            if run.returncode != case['exit_code']:
                errors.append(f'exit: {case["exit_code"]} != {run.returncode}')
            second = subprocess.run([str(binary), 'audit', case['input'], '--json'], capture_output=True, text=True, timeout=AUDIT_TIMEOUT_SECONDS)
            if second.stdout != run.stdout or second.returncode != run.returncode:
                errors.append('nondeterministic output')
        except subprocess.TimeoutExpired:
            errors.append(f'audit timed out after {AUDIT_TIMEOUT_SECONDS} seconds')
        if errors:
            failures.append(case['input'])
            print(case['input'], *errors[:10], sep='\n  ')
    print(f'{len(manifest["cases"])-len(failures)}/{len(manifest["cases"])} frozen reports match; {len(failures)} failures.')
    return bool(failures)


if __name__ == '__main__':
    raise SystemExit(main())
