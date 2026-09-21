"""Offline G6 record shape, identity, evidence retention and approval boundaries."""
from copy import deepcopy
import hashlib
import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator, ValidationError

ROOT = Path(__file__).resolve().parents[1]
SCHEMA = json.loads((ROOT / 'schemas/adoption-record-v1.schema.json').read_bytes())
VALIDATOR = Draft202012Validator(SCHEMA)
REGISTRY = {r['id']: r for r in json.loads((ROOT / 'docs/reuse/candidates.json').read_bytes())['components']}
PATHS = sorted((ROOT / 'docs/reuse/adoptions').glob('*.json'))


def validate(record):
    VALIDATOR.validate(record)
    component = REGISTRY[record['component']]
    assert record['commit'] == component['pin']['commit']
    assert record['archive_sha256'] == component['pin']['artifact_sha256']
    assert record['license_sha256'] == component['license']['sha256']
    assert record['owner'] == component['acceptance_owner']
    assert record['implementer'] == component['implementer']
    assert record['technical_reviewer'] == component['technical_reviewer']
    assert component['evaluation_issue'] in record['tracking']
    for number in (7, 21, 45):
        assert f'https://github.com/toddwbucy/Aletharsis/issues/{number}' in record['tracking']
    paths = [e['path'] for e in record['evidence']]
    assert len(paths) == len(set(paths))
    for e in record['evidence']:
        path = ROOT / e['path']
        assert '..' not in Path(e['path']).parts
        assert path.resolve().is_relative_to(ROOT)
        assert not path.is_symlink()
        assert hashlib.sha256(path.read_bytes()).hexdigest() == e['sha256']
    for check in record['checks'].values():
        assert all(i < len(record['evidence']) for i in check['evidence'])
    acceptance = record['acceptance']
    if acceptance is not None:
        assert acceptance['owner'] == record['owner']
        assert acceptance['technical_reviewer'] == record['technical_reviewer']
        assert acceptance['disposition'] == record['recommendation']
    if record['recommendation'] == 'approve' and acceptance is not None:
        assert component['adoption_status'] == 'approved'
        assert component['decision']['disposition'] == 'approve'
        assert component['decision']['record'] == acceptance['record']
    if component['adoption_status'] == 'approved':
        assert record['recommendation'] == 'approve'
        assert acceptance is not None


def test_inventory():
    Draft202012Validator.check_schema(SCHEMA)
    assert {p.stem for p in PATHS} == {'encypherai--c2pa-text', 'encypherai--encypher-c2pa'}
    records = [json.loads(p.read_bytes()) for p in PATHS]
    assert len({r['component'] for r in records}) == len(PATHS)
    for p, record in zip(PATHS, records):
        assert record['component'] == p.stem


@pytest.mark.parametrize('path', PATHS, ids=lambda p: p.stem)
def test_retained_adoption_gap_records(path):
    record = json.loads(path.read_bytes())
    validate(record)
    assert record['recommendation'] == 'revise'
    assert record['acceptance'] is None
    assert REGISTRY[record['component']]['adoption_status'] == 'evaluating'


@pytest.mark.parametrize('mutation', [
    'unknown_key', 'missing_check', 'unknown_status', 'unsupported_approval',
    'stale_pin', 'stale_archive', 'stale_license', 'wrong_owner',
    'missing_evidence', 'stale_evidence', 'invalid_reference', 'traversal',
    'mismatched_acceptance', 'registry_approval_without_record',
])
def test_invalid_records_rejected(mutation):
    record = deepcopy(json.loads(PATHS[0].read_bytes()))
    original_status = REGISTRY[record['component']]['adoption_status']
    if mutation == 'unknown_key': record['approved_for_all'] = True
    elif mutation == 'missing_check': del record['checks']['native_platforms']
    elif mutation == 'unknown_status': record['checks']['live_adapter']['status'] = 'looks_good'
    elif mutation == 'unsupported_approval': record['recommendation'] = 'approve'
    elif mutation == 'stale_pin': record['commit'] = '0' * 40
    elif mutation == 'stale_archive': record['archive_sha256'] = '0' * 64
    elif mutation == 'stale_license': record['license_sha256'] = '0' * 64
    elif mutation == 'wrong_owner': record['owner'] = 'someone-else'
    elif mutation == 'missing_evidence': record['checks']['resources']['evidence'] = []
    elif mutation == 'stale_evidence': record['evidence'][0]['sha256'] = '0' * 64
    elif mutation == 'invalid_reference': record['checks']['resources']['evidence'] = [len(record['evidence'])]
    elif mutation == 'traversal': record['evidence'][0]['path'] = 'docs/../README.md'
    elif mutation == 'mismatched_acceptance':
        record['acceptance'] = {'owner':record['owner'], 'technical_reviewer':record['technical_reviewer'], 'record':record['evaluation_pr'], 'disposition':'approve'}
    elif mutation == 'registry_approval_without_record': REGISTRY[record['component']]['adoption_status'] = 'approved'
    try:
        with pytest.raises((AssertionError, ValidationError)):
            validate(record)
    finally:
        REGISTRY[record['component']]['adoption_status'] = original_status


def test_recommendation_is_not_acceptance():
    record = deepcopy(json.loads(PATHS[0].read_bytes()))
    record['recommendation'] = 'approve'
    for check in record['checks'].values():
        check.update(status='passed', evidence=[0])
    # Shape-only example: schema checks completeness, not factual sufficiency.
    # Human reviewers must reject these unsupported synthetic pass assertions.
    VALIDATOR.validate(record)
    assert record['acceptance'] is None
