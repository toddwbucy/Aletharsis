"""Compare the Go CLI with 210 frozen Python cases; no Python auditor required."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

from check_parity import AUDIT_TIMEOUT_SECONDS, canonical, differences, verify_references


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('binary', type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve()
    _, retirement = verify_references()
    cases = [json.loads(line) for line in Path('reference/seeded-python/cases.jsonl').read_text().splitlines()]
    assert len(cases) == retirement['seeded_case_count'], 'Seeded case count changed'
    failures = []
    with tempfile.TemporaryDirectory(prefix='aletharsis-seeded-') as folder:
        for i, case in enumerate(cases):
            name = f'{i}.txt'
            raw = bytes.fromhex(case['input_hex'])
            expected = case['report']
            assert expected['file']['path'] == name
            assert expected['file']['filename'] == name
            assert expected['file']['size'] == len(raw)
            assert expected['file']['sha256'] == hashlib.sha256(raw).hexdigest()
            (Path(folder) / name).write_bytes(raw)
            errors = []
            try:
                run = subprocess.run([str(binary), 'audit', name, '--json'], cwd=folder,
                                     capture_output=True, text=True, timeout=AUDIT_TIMEOUT_SECONDS)
                actual = json.loads(run.stdout)
                errors = list(differences(canonical(expected), canonical(actual)))
                if run.returncode != expected['summary']['exit_code']:
                    errors.append('exit code differs')
            except subprocess.TimeoutExpired:
                errors.append(f'audit timed out after {AUDIT_TIMEOUT_SECONDS} seconds')
            except (json.JSONDecodeError, KeyError, TypeError, OSError) as error:
                errors.append(f'invalid candidate result: {error}')
            if errors:
                failures.append(i)
                print(f'case {i}', *errors[:3], sep='\n  ')
    print(f'{len(cases)-len(failures)}/{len(cases)} frozen seeded cases match.')
    return bool(failures)


if __name__ == '__main__':
    raise SystemExit(main())
