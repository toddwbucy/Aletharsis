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
    assert record['component'] in REGISTRY, 'unknown registry component'
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
        assert path.is_file(), 'evidence must be an existing regular file'
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
        assert isinstance(component['decision'], dict), 'registry decision must record approval'
        assert component['decision']['disposition'] == 'approve'
        assert component['decision']['record'] == acceptance['record']
    if component['adoption_status'] == 'approved':
        assert record['recommendation'] == 'approve'
        assert acceptance is not None


def test_inventory():
    Draft202012Validator.check_schema(SCHEMA)
    assert PATHS, 'adoption inventory must not be empty'
    assert {p.stem for p in PATHS} <= REGISTRY.keys()
    for component in REGISTRY.values():
        if component['adoption_status'] == 'approved':
            assert component['id'] in {p.stem for p in PATHS}
    records = [json.loads(p.read_bytes()) for p in PATHS]
    assert len({r['component'] for r in records}) == len(PATHS)
    for p, record in zip(PATHS, records):
        assert record['component'] == p.stem


@pytest.mark.parametrize('path', PATHS, ids=lambda p: p.stem)
def test_retained_adoption_gap_records(path):
    record = json.loads(path.read_bytes())
    validate(record)


@pytest.fixture
def pending_record(monkeypatch):
    # Synthetic gate tests must not depend on a retained record staying unapproved.
    record = deepcopy(json.loads(PATHS[0].read_bytes()))
    record.update(recommendation='revise', acceptance=None)
    for check in record['checks'].values():
        check.clear()
        check.update(status='partial', detail='Synthetic gate test.', evidence=[0])
    component = deepcopy(REGISTRY[record['component']])
    component.update(adoption_status='evaluating', decision=None)
    monkeypatch.setitem(REGISTRY, record['component'], component)
    return record


@pytest.mark.parametrize('mutation', [
    'unknown_key', 'missing_check', 'unknown_status', 'unsupported_approval',
    'stale_pin', 'stale_archive', 'stale_license', 'wrong_owner',
    'missing_evidence', 'stale_evidence', 'invalid_reference', 'traversal',
    'mismatched_acceptance', 'registry_approval_without_record',
    'directory_evidence', 'missing_file', 'unknown_component',
])
def test_invalid_records_rejected(mutation, pending_record):
    record = deepcopy(pending_record)
    original_component = record['component']
    original_status = REGISTRY[original_component]['adoption_status']
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
    elif mutation == 'directory_evidence': record['evidence'][0]['path'] = 'docs/reuse/evaluations'
    elif mutation == 'missing_file': record['evidence'][0]['path'] = 'docs/reuse/no-such-evidence.txt'
    elif mutation == 'unknown_component': record['component'] = 'unknown-component'
    elif mutation == 'traversal': record['evidence'][0]['path'] = 'docs/../README.md'
    elif mutation == 'mismatched_acceptance':
        record['acceptance'] = {'owner':record['owner'], 'technical_reviewer':record['technical_reviewer'], 'record':record['evaluation_pr'], 'disposition':'approve'}
    elif mutation == 'registry_approval_without_record': REGISTRY[record['component']]['adoption_status'] = 'approved'
    try:
        with pytest.raises((AssertionError, ValidationError)):
            validate(record)
    finally:
        REGISTRY[original_component]['adoption_status'] = original_status


def test_recommendation_is_not_acceptance(pending_record):
    record = deepcopy(pending_record)
    record['recommendation'] = 'approve'
    for check in record['checks'].values():
        check.update(status='passed', evidence=[0])
    # Shape-only example: schema checks completeness, not factual sufficiency.
    # Human reviewers must reject these unsupported synthetic pass assertions.
    validate(record)
    assert record['acceptance'] is None
    assert REGISTRY[record['component']]['adoption_status'] != 'approved'


@pytest.mark.parametrize('disposition', ['approve', 'revise', 'reject'])
def test_acceptance_lifecycle(disposition, monkeypatch, pending_record):
    record = deepcopy(pending_record)
    record['recommendation'] = disposition
    if disposition == 'approve':
        for check in record['checks'].values():
            check.update(status='passed', evidence=[0])
    record['acceptance'] = dict(owner=record['owner'],
        technical_reviewer=record['technical_reviewer'],
        record=record['evaluation_pr'], disposition=disposition)
    component = deepcopy(REGISTRY[record['component']])
    if disposition == 'approve':
        component['adoption_status'] = 'approved'
        component['decision'] = dict(disposition='approve', record=record['evaluation_pr'])
    monkeypatch.setitem(REGISTRY, record['component'], component)
    validate(record)
    if disposition == 'approve':
        component['decision'] = None
        with pytest.raises(AssertionError, match='registry decision'):
            validate(record)
        record['acceptance'] = None
        with pytest.raises(AssertionError):
            validate(record)


@pytest.mark.parametrize('fault', ['all_na', 'no_evidence', 'no_justification', 'blank_justification', 'none'])
def test_not_applicable_requires_supported_scope(fault, pending_record):
    record = deepcopy(pending_record)
    record['recommendation'] = 'approve'
    for check in record['checks'].values():
        check.update(status='passed', evidence=[0])
    check = record['checks']['live_adapter']
    check.update(status='not_applicable', evidence=[0],
                 scope_justification='Synthetic shape test: developer-only fixture oracle; no adapter shipped.')
    if fault == 'all_na':
        for check in record['checks'].values():
            check.update(status='not_applicable', evidence=[0], scope_justification='Synthetic exclusion.')
    elif fault == 'no_evidence': check['evidence'] = []
    elif fault == 'no_justification': del check['scope_justification']
    elif fault == 'blank_justification': check['scope_justification'] = '   '
    if fault == 'none':
        VALIDATOR.validate(record)
    else:
        with pytest.raises(ValidationError):
            VALIDATOR.validate(record)


def test_inventory_accepts_new_registered_component(tmp_path, monkeypatch, pending_record):
    record = deepcopy(pending_record)
    component = deepcopy(REGISTRY[record['component']])
    record['component'] = component['id'] = 'synthetic--new-component'
    monkeypatch.setitem(REGISTRY, component['id'], component)
    path = tmp_path / (component['id'] + '.json')
    path.write_text(json.dumps(record))
    monkeypatch.setitem(globals(), 'PATHS', [*PATHS, path])
    test_inventory()
    validate(record)


def test_inventory_rejects_non_record_sidecar(tmp_path, monkeypatch):
    path = tmp_path / 'index.json'
    path.write_text('{}')
    monkeypatch.setitem(globals(), 'PATHS', [*PATHS, path])
    with pytest.raises(AssertionError):
        test_inventory()
