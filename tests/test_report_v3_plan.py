"""Executable migration-design expectations, not a report-3.0 implementation."""
import hashlib
import json
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
PLAN = json.loads((ROOT / 'docs/specs/examples/report-v3-migration-cases.json').read_bytes())
DA_SCHEMA = json.loads((ROOT / 'schemas/adapter-exchange-v1.schema.json').read_bytes())


def aggregate(case):
    """Planning oracle for the retained v2 aggregate policy (not a wire decoder)."""
    state = case['adapter_state']
    if case['operational_failure']:
        status = 'failed'
    elif case['adapter_participation'] == 'disabled':
        status = 'completed'
    elif state == 'partial':
        status = 'partial'
    elif state == 'canceled':
        status = 'partial' if case['usable_results'] else 'canceled'
    elif state != 'completed':
        status = 'partial' if case['usable_results'] or case['adapter_participation'] == 'optional' else 'failed'
    else:
        status = 'completed'
    return status, case['severity_exit'] if status == 'completed' else 4


def test_plan_scope_and_complete_da_fixture_inventory():
    assert PLAN['document_kind'] == 'migration_design_vectors'
    assert PLAN['not_wire_reports'] is True
    assert len(PLAN['cases']) == 13
    assert len({c['id'] for c in PLAN['cases']}) == 13
    assert {c['da_fixture'] for c in PLAN['cases']} == {
        str(p.relative_to(ROOT)) for p in (ROOT / 'tests/adapters/fixtures').glob('*.json')}
    assert PLAN['scenario_assumptions'] == {
        'native_operation': 'completed with usable result', 'native_severity_findings': 0,
        'adapter_participation': 'optional', 'adapter_findings': 0}


def test_reason_mapping_covers_every_da_terminal_variant():
    variants = DA_SCHEMA['$defs']['outcome']['oneOf']
    assert {v['properties']['reason']['const'] for v in variants} == {
        row['host_reason'] for row in PLAN['reason_map']}
    assert len(PLAN['reason_map']) == len(variants)
    reasons = {r['host_reason']: r for r in PLAN['reason_map']}
    assert reasons['unavailable']['report_code'] is None
    assert reasons['unavailable']['reason_source'] == 'capability'
    assert reasons['none']['report_code'] is None
    assert reasons['timeout']['report_code'] == 'execution.timeout'
    assert reasons['canceled']['report_code'] == 'execution.canceled'
    for name in ('malformed_output', 'oversized_output', 'source_mismatch',
                 'provider_failure', 'cleanup_failed', 'partial'):
        assert reasons[name]['report_code'].startswith('adapter.')


@pytest.mark.parametrize('case', PLAN['cases'], ids=lambda c: c['id'])
def test_migration_outcome_expectations(case):
    """Accepted transcripts constrain the proposed mapping, including negatives."""
    raw = (ROOT / case['da_fixture']).read_bytes()
    assert hashlib.sha256(raw).hexdigest() == case['fixture_sha256']
    fixture = json.loads(raw)
    assert (case['host_state'], case['host_reason']) == (
        fixture['outcome']['state'], fixture['outcome']['reason'])
    assert case['operation'] == fixture['request']['operation']
    expected = case['expected']
    assert expected['execution_state'] == case['host_state']
    reason = next(r for r in PLAN['reason_map'] if r['host_reason'] == case['host_reason'])
    assert expected['report_reason'] == reason['report_code']
    assert expected['reason_source'] == reason['reason_source']
    if fixture['response'] is None:
        assert expected['result_count'] == 0
        assert expected['result_kind'] is None
    else:
        assert expected['result_count'] == len(fixture['response']['results']) == 1
        result = next(r for r in PLAN['result_map'] if r['operation'] == case['operation'])
        assert expected['result_kind'] == result['kind']
    status, exit_code = aggregate({'operational_failure': False,
        'adapter_participation': 'optional', 'adapter_state': case['host_state'],
        'usable_results': True, 'severity_exit': 0})
    assert (expected['mixed_native_report_status'], expected['mixed_native_exit_code']) == (status, exit_code)


@pytest.mark.parametrize('case', PLAN['aggregate_cases'], ids=lambda c: c['id'])
def test_aggregate_planning_cases(case):
    assert aggregate(case) == (case['expected_status'], case['expected_exit'])


def test_mechanisms_remain_separate_and_new_contracts_do_not_relabel_native():
    assert PLAN['result_map'] == [
        {'operation': 'extract', 'kind': 'carrier_extraction', 'contract_version': '1',
         'role': 'analyzer', 'mechanism': 'structural'},
        {'operation': 'verify', 'kind': 'credential_verification', 'contract_version': '1',
         'role': 'verifier', 'mechanism': 'cryptographic'},
        {'operation': 'statistical', 'kind': 'statistical_analysis', 'contract_version': '1',
         'role': 'analyzer', 'mechanism': 'statistical'}]


def test_existing_schema_semantics_are_unchanged():
    """Canonical structural hashes permit checkout line endings, not schema edits."""
    compatibility = PLAN['compatibility']
    assert compatibility['production_schema_default'] == '1.0'
    assert compatibility['existing_opt_in'] == '2.0'
    assert compatibility['proposed_opt_in'] == '3.0'
    assert compatibility['old_importer_status'] == 'unsupported_version'
    assert compatibility['old_importer_coverage'] == 'unknown'
    assert set(compatibility['frozen_schema_semantic_sha256']) == {
        'schemas/report.schema.json', 'schemas/report-v2.schema.json',
        'schemas/adapter-exchange-v1.schema.json'}
    for path, digest in compatibility['frozen_schema_semantic_sha256'].items():
        raw = json.dumps(json.loads((ROOT / path).read_bytes()), sort_keys=True,
                         separators=(',', ':'), ensure_ascii=True).encode()
        assert hashlib.sha256(raw).hexdigest() == digest
