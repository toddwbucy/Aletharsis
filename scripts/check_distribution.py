"""Check native build/install and the current platform acquisition contract.

Only disposable generated inputs are audited. No Python auditor is imported.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile

import jsonschema

ROOT = Path(__file__).resolve().parents[1]


def run(command, *, timeout=10, env=None):
    return subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=timeout)


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    system = platform.system()
    require(system in ('Linux', 'Darwin', 'Windows'), 'Unvalidated test host')
    schema = json.loads((ROOT / 'schemas/report.schema.json').read_bytes())
    validator_type = jsonschema.validators.validator_for(schema)
    validator_type.check_schema(schema)
    validator = validator_type(schema)
    results = {'system': system, 'machine': platform.machine(), 'checks': [],
               'acquisition': 'supported' if system == 'Linux' else 'fails_closed'}
    with tempfile.TemporaryDirectory(prefix='aletharsis-distribution-') as directory:
        root = Path(directory)
        suffix = '.exe' if system == 'Windows' else ''
        built = root / ('aletharsis' + suffix)
        install = root / 'installed'
        # Absolute native paths also work with Windows GOBIN, without shell conversion.
        for command, env in [(['go', 'build', '-trimpath', '-o', str(built), './cmd/aletharsis'], None),
                             (['go', 'install', '-trimpath', './cmd/aletharsis'],
                              os.environ | {'GOBIN': str(install)})]:
            process = run(command, timeout=180, env=env)
            require(process.returncode == 0, process.stderr.decode(errors='replace'))
        results['go_version'] = run(['go', 'version']).stdout.decode().strip()
        clean = root / 'clean text.txt'
        clean.write_bytes(b'ordinary words\n')
        unicode_file = root / 'unicode.txt'
        unicode_file.write_bytes('A😀\u200bB\r\n'.encode())
        malformed = root / 'invalid.txt'
        malformed.write_bytes(b'\xff')
        sources = [clean, unicode_file, malformed]
        expected = {p: p.read_bytes() for p in sources}
        before = {p: p.stat() for p in sources}
        runtime_env = os.environ | {'PATH': ''}
        for binary in (built, install / ('aletharsis' + suffix)):
            for option, needle in [('--version', b'Aletharsis'), ('--help', b'audit')]:
                process = run([str(binary), option], env=runtime_env)
                require(process.returncode == 0 and needle.lower() in process.stdout.lower(),
                        f'{binary.name} {option} failed: {process.stderr!r}')
                require(not process.stderr, 'Unexpected help/version stderr')
            candidates = sources if system == 'Linux' else sources + [root / 'missing.txt', root]
            for source in candidates:
                for view in ('audit', 'unicode', 'metadata', 'structure'):
                    command = [str(binary), view, str(source), '--json']
                    first = run(command, env=runtime_env)
                    second = run(command, env=runtime_env)
                    require((first.returncode, first.stdout, first.stderr) ==
                            (second.returncode, second.stdout, second.stderr), 'Nondeterministic process output')
                    require(not first.stderr, 'Unexpected audit stderr')
                    report = json.loads(first.stdout)
                    validator.validate(report)
                    require(first.returncode == report['summary']['exit_code'], 'Exit/report mismatch')
                    if system != 'Linux':
                        require(first.returncode == 4 and report['status'] == 'failed', 'Unsupported host audited source')
                        require(all(report['file'][k] is None for k in ('sha256', 'size', 'parser')), 'Partial identity leaked')
                        require(report['evidence'] == {'texts': [], 'metadata': {}, 'structure': {}}, 'Partial evidence leaked')
                        require(len(report['findings']) == 1 and report['findings'][0]['id'] == 'parser.failure',
                                'Missing acquisition failure')
                        # A test of the current explicit diagnostic, not a new error-code protocol.
                        require('verified no-atime reader' in report['findings'][0]['evidence']['message'],
                                'Failure was unrelated to unsupported acquisition')
                    elif source == malformed:
                        require(first.returncode == 4 and report['status'] == 'failed', 'Malformed bytes accepted')
                    else:
                        require(report['status'] == 'completed', 'Linux acquisition failed')
                        require(report['file']['sha256'] == hashlib.sha256(expected[source]).hexdigest(), 'Wrong source digest')
                        text = report['evidence']['texts'][0]
                        require(text['text'].encode() == expected[source], 'Extracted text changed')
                        if source == unicode_file:
                            require(text['byte_offsets'] == [0, 1, 5, 8, 9, 10, 11], 'Scalar/byte coordinates changed')
                        else:
                            require(first.returncode == 0 and not report['findings'], 'Clean text reported findings')
                    results['checks'].append({'binary': 'built' if binary == built else 'installed',
                                              'source': source.name, 'view': view, 'exit_code': first.returncode})
        # Stat before validation reads: reading fixtures in the test can update atime.
        for source in sources:
            after = source.stat()
            require((after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns, after.st_size) ==
                    (before[source].st_atime_ns, before[source].st_mtime_ns,
                     before[source].st_ctime_ns, before[source].st_size), 'Source stat changed')
            require(source.read_bytes() == expected[source], 'Source bytes changed')
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(results, indent=2, sort_keys=True) + '\n')
    print(f"Validated {len(results['checks'])} native build/install audit cases on {system}/{platform.machine()}")


if __name__ == '__main__':
    main()
