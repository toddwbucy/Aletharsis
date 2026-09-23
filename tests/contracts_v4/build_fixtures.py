"""Additive flat-report 4.0 fixtures derived without rewriting frozen v2 bytes."""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OFFICE_ARRAYS = ('packages', 'xml', 'scopes', 'metadata', 'relationships', 'objects')


def build():
    inputs = sorted((ROOT / 'tests/contracts/fixtures').glob('*.json'))
    if len(inputs) != 8:
        raise ValueError('exactly eight legacy fixtures required')
    result = {}
    for path in inputs:
        report = json.loads(path.read_bytes())
        report['schema_version'] = '4.0'
        report['adapter_runs'] = []
        report['evidence']['office'] = {key: [] for key in OFFICE_ARRAYS}
        result['flat-' + path.name] = report
    return result


if __name__ == '__main__':
    directory = Path(__file__).with_name('fixtures')
    directory.mkdir(exist_ok=True)
    for name, report in build().items():
        (directory / name).write_text(json.dumps(report, indent=2, ensure_ascii=True) + '\n')
