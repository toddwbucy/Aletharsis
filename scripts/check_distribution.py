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
import sys
import tempfile

import jsonschema
from referencing import Registry, Resource

ROOT = Path(__file__).resolve().parents[1]


def run(command, *, timeout=10, env=None):
    return subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=timeout)


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def corpus_validator():
    """Resolve corpus and embedded-report contracts exclusively from local schemas."""
    names = ('corpus-document-v1.schema.json', 'corpus-v1.schema.json',
             'report.schema.json', 'report-v2.schema.json')
    schemas = {name: json.loads((ROOT / 'schemas' / name).read_bytes()) for name in names}
    registry = Registry().with_resources(
        ('https://aletharsis.local/schemas/' + name, Resource.from_contents(schema))
        for name, schema in schemas.items())
    return jsonschema.Draft202012Validator(schemas[names[0]], registry=registry)


def validate_directory(report, code, system, version, validator):
    """Check directory outcomes without accepting a file-report envelope."""
    validator.validate(report)
    require(report['header']['report_schema'] == version, 'Wrong corpus report version')
    summary = report['summary']
    require(code == summary['exit_code'], 'Corpus exit/report mismatch')
    if system != 'Linux':
        require(code == 4 and summary['state'] == 'failed', 'Unsupported host scanned directory')
        require(summary['reason'] == 'integrity.no_atime_unavailable', 'Wrong discovery failure')
        require(not summary['discovery_complete'], 'Unsupported discovery reported complete')
        require(report['entries'] == [] and summary['entries'] == 0, 'Unavailable discovery leaked entries')
        require(all(count == 0 for count in summary['counts'].values()), 'Unavailable discovery counted evidence')
    else:
        require(code == 0 and summary['state'] == 'completed' and summary['discovery_complete'],
                'Clean Linux corpus did not complete')
        require(summary['entries'] == len(report['entries']), 'Corpus count mismatch')
        require(summary['counts']['no_reported_findings'] == len(report['entries']), 'Clean corpus classification')
        require(all(count == 0 for key, count in summary['counts'].items()
                    if key != 'no_reported_findings'), 'Unexpected clean corpus outcome')


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
    v2_schema = json.loads((ROOT / 'schemas/report-v2.schema.json').read_bytes())
    v2_validator = jsonschema.Draft202012Validator(v2_schema)
    directory_validator = corpus_validator()
    sys.path.insert(0, str(ROOT / 'tests/contracts'))
    from support import validate_semantics
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
        corpus_root = root / 'corpus'
        corpus_root.mkdir()
        corpus_source = corpus_root / 'clean.txt'
        corpus_source.write_bytes(b'ordinary corpus words\n')
        sources = [clean, unicode_file, malformed, corpus_source]
        expected = {p: p.read_bytes() for p in sources}
        before = {p: p.stat() for p in sources}
        runtime_env = os.environ | {'PATH': ''}
        for binary in (built, install / ('aletharsis' + suffix)):
            for option, needle in [('--version', b'Aletharsis'), ('--help', b'audit')]:
                process = run([str(binary), option], env=runtime_env)
                require(process.returncode == 0 and needle.lower() in process.stdout.lower(),
                        f'{binary.name} {option} failed: {process.stderr!r}')
                require(not process.stderr, 'Unexpected help/version stderr')
            candidates = sources if system == 'Linux' else sources + [root / 'missing.txt']
            for source in candidates:
                for view in ('audit', 'unicode', 'metadata', 'structure'):
                    for version in ('1.0', '2.0'):
                        command = [str(binary), view, str(source), '--json']
                        if version == '2.0':
                            command.append('--schema-version=2.0')
                        first = run(command, env=runtime_env)
                        second = run(command, env=runtime_env)
                        require((first.returncode, first.stdout, first.stderr) ==
                                (second.returncode, second.stdout, second.stderr), 'Nondeterministic process output')
                        require(not first.stderr, 'Unexpected audit stderr')
                        report = json.loads(first.stdout)
                        (validator if version == '1.0' else v2_validator).validate(report)
                        if version == '2.0':
                            validate_semantics(report)
                        require(first.returncode == report['summary']['exit_code'], 'Exit/report mismatch')
                        if system != 'Linux':
                            require(first.returncode == 4 and report['status'] == 'failed', 'Unsupported host audited source')
                            require(all(report['file'][k] is None for k in ('sha256', 'size', 'parser')), 'Partial identity leaked')
                            require(report['evidence'] == {'texts': [], 'metadata': {}, 'structure': {}}, 'Partial evidence leaked')
                            if version == '1.0':
                                require(len(report['findings']) == 1 and report['findings'][0]['id'] == 'parser.failure',
                                        'Missing acquisition failure')
                                # A test of the current explicit diagnostic, not a new error-code protocol.
                                require('verified no-atime reader' in report['findings'][0]['evidence']['message'],
                                        'Failure was unrelated to unsupported acquisition')
                            else:
                                require(not report['findings'] and not report['results'], 'Unrun reader produced findings/results')
                                acquire = next(e for e in report['executions'] if e['capability_ref'] == 'aletharsis.acquire')
                                require(acquire['state'] == 'not_run' and acquire['reason_code'] == 'integrity.no_atime_unavailable',
                                        'Unavailable reader was invoked or misreported')
                                require(all(e['state'] == 'not_run' for e in report['executions']), 'Unsupported platform ran analysis')
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
                                                  'source': source.name, 'view': view, 'schema_version': version, 'exit_code': first.returncode})
            for version in ('1.0', '2.0'):
                for output_format in ('--json', '--jsonl'):
                    command = [str(binary), 'audit', str(corpus_root), output_format,
                               '--schema-version=' + version]
                    first, second = run(command, env=runtime_env), run(command, env=runtime_env)
                    require((first.returncode, first.stdout, first.stderr) ==
                            (second.returncode, second.stdout, second.stderr), 'Nondeterministic corpus output')
                    require(not first.stderr, 'Unexpected corpus stderr')
                    if output_format == '--jsonl':
                        records = [json.loads(line) for line in first.stdout.splitlines()]
                        report = {'header': records[0], 'entries': records[1:-1], 'summary': records[-1]}
                    else:
                        report = json.loads(first.stdout)
                    validate_directory(report, first.returncode, system, version, directory_validator)
                    if system == 'Linux':
                        require(len(report['entries']) == 1, 'Missing corpus fixture')
                        entry = report['entries'][0]
                        require(entry['relative_path'] == 'clean.txt' and entry['state'] == 'no_reported_findings',
                                'Wrong corpus entry')
                        require(entry['report']['file']['sha256'] == hashlib.sha256(expected[corpus_source]).hexdigest(),
                                'Wrong corpus source digest')
                        if version == '2.0':
                            validate_semantics(entry['report'])
                    results['checks'].append({'binary': 'built' if binary == built else 'installed',
                                              'source': 'corpus', 'view': 'audit', 'format': output_format,
                                              'schema_version': version, 'exit_code': first.returncode})
                for view in ('unicode', 'metadata', 'structure'):
                    process = run([str(binary), view, str(corpus_root), '--json',
                                   '--schema-version=' + version], env=runtime_env)
                    require(process.returncode == 4 and not process.stdout and process.stderr,
                            'Directory subview did not reject unsupported command')
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
