"""The distribution verifier must reject fabricated unsupported-host coverage."""
from copy import deepcopy

import pytest

from scripts.check_distribution import corpus_validator, validate_directory


@pytest.mark.parametrize('system', ['Darwin', 'Windows'])
@pytest.mark.parametrize('version', ['1.0', '2.0'])
def test_unavailable_directory_contract(system, version):
    report = {
        'header': {'type': 'header', 'contract': 'aletharsis.corpus/1',
                   'workspace': 'corpus', 'report_schema': version,
                   'limits': {'file_input_bytes': 1024, 'acquisition_bytes': 4096,
                              'output_bytes': 4096, 'concurrency': 1, 'entries': 100,
                              'depth': 10, 'path_bytes': 1024}},
        'entries': [],
        'summary': {'type': 'summary', 'state': 'failed',
                    'reason': 'integrity.no_atime_unavailable', 'discovery_complete': False,
                    'entries': 0, 'exit_code': 4,
                    'counts': dict.fromkeys(('no_reported_findings', 'requires_review',
                                            'unsupported', 'failed', 'skipped', 'canceled'), 0)},
    }
    validator = corpus_validator()
    validate_directory(report, 4, system, version, validator)
    for field, value in [('discovery_complete', True), ('reason', 'input.open_failed'),
                         ('entries', 1)]:
        changed = deepcopy(report)
        changed['summary'][field] = value
        with pytest.raises(AssertionError):
            validate_directory(changed, 4, system, version, validator)
    changed = deepcopy(report)
    changed['summary']['counts']['no_reported_findings'] = 1
    with pytest.raises(AssertionError):
        validate_directory(changed, 4, system, version, validator)
    with pytest.raises(AssertionError):
        validate_directory(report, 0, system, version, validator)
