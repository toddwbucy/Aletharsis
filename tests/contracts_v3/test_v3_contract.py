"""Full report-3.0 schema, semantic graph and exact-byte conformance tests."""
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator, ValidationError, FormatChecker

from build_schema import build
from v3_support import ROOT, identity, import_report, schema, strict_json, validate_semantics

HERE = Path(__file__).resolve().parent
FIXTURES = sorted((HERE / 'fixtures').glob('*.json'))
INVALID = json.loads((HERE / 'invalid-cases.json').read_bytes())
assert len(FIXTURES) == 31, 'Expected all 31 full report fixtures'
assert len(INVALID) == 16, 'Expected all 16 invalid report fixtures'
BLOBS = {p.name:p.read_bytes() for p in (HERE / 'blobs').iterdir()}
VALIDATOR = Draft202012Validator(schema('3.0'), format_checker=FormatChecker())


def report(name='adapter-extracted'):
    return json.loads((HERE / 'fixtures' / (name+'.json')).read_bytes())


def validate(value, blobs=BLOBS):
    VALIDATOR.validate(value)
    validate_semantics(value,blobs)


@pytest.mark.parametrize('path', FIXTURES, ids=lambda p:p.stem)
def test_full_wire_reports(path):
    raw = path.read_bytes()
    imported = import_report(raw, BLOBS)
    assert imported['status'] == 'validated'
    assert imported['coverage'] == 'declared'
    assert imported['bytes'] == raw
    assert imported['report_artifact_sha256'] == sha256(raw).hexdigest()
    assert imported['report'] == json.loads(raw)
    # Import without any blob resolution still validates graph claims, not bytes.
    assert import_report(raw)['blob_checks'] == 'not_requested'


@pytest.mark.parametrize('case', INVALID, ids=lambda c:c['file'])
def test_invalid_full_reports(case):
    value = json.loads((HERE / 'invalid' / case['file']).read_bytes())
    if case['stage'] == 'schema':
        with pytest.raises(ValidationError):
            VALIDATOR.validate(value)
    else:
        VALIDATOR.validate(value)
        with pytest.raises(ValueError, match=case['reason']):
            validate_semantics(value,BLOBS)


def test_schema_reproducible_closed_and_local_only():
    assert schema('3.0') == build()
    Draft202012Validator.check_schema(schema('3.0'))
    def walk(value):
        if isinstance(value, dict):
            if '$ref' in value:
                assert value['$ref'].startswith('#/$defs/')
            if value.get('type') == 'object' and 'properties' in value:
                assert value['additionalProperties'] is False
                assert set(value.get('required', [])) <= set(value['properties'])
            for item in value.values():
                walk(item)
        elif isinstance(value,list):
            for item in value:
                walk(item)
    walk(schema('3.0'))


def test_all_outcomes_and_all_native_fixtures_are_represented():
    source = ROOT / 'tests/adapters/fixtures'
    assert {p.stem.removeprefix('adapter-') for p in FIXTURES if p.stem.startswith('adapter-') and p.stem!='adapter-normalized'} == {p.stem for p in source.glob('*.json')}
    for path in (ROOT / 'tests/contracts/fixtures').glob('*.json'):
        old = json.loads(path.read_bytes())
        new = report('native-'+path.stem)
        for key in old:
            if key not in ('schema_version','catalog_version'):
                assert new[key] == old[key]
    assert {x['file'] for x in INVALID} == {p.name for p in (HERE / 'invalid').glob('*.json')}


def test_existing_import_behavior_and_unknown_version():
    for version, name in [('2.0','tests/contracts/fixtures/structural-observation.json')]:
        raw = (ROOT / name).read_bytes()
        imported = import_report(raw)
        assert imported['status'] == 'validated'
        assert imported['report']['schema_version'] == version
        assert imported['report_artifact_sha256'] == sha256(raw).hexdigest()
    raw = b'{"schema_version":"99.0"}'
    imported = import_report(raw)
    assert imported['status'] == 'unsupported_version' and imported['coverage'] == 'unknown'
    assert imported['report_artifact_sha256'] == sha256(raw).hexdigest()
    with pytest.raises(ValidationError):
        Draft202012Validator(schema('2.0')).validate(report())


def test_duplicate_result_and_run_are_rejected():
    value = report()
    extra = deepcopy(value['results'][-1])
    extra['result_ref'] = 'result/2'
    value['results'].append(extra)
    with pytest.raises(ValueError, match='adapter aggregate result count'):
        validate(value)
    value = report()
    value['adapter_runs'].append(deepcopy(value['adapter_runs'][-1]))
    with pytest.raises(ValueError, match='adapter run planning/order'):
        validate(value)


def test_quarantined_bytes_cannot_be_promoted():
    value = report()
    value['adapter_runs'][-1]['quarantined_refs'] = [value['adapter_runs'][-1]['raw_result_ref']]
    with pytest.raises(ValueError, match='quarantined evidence promoted'):
        validate(value)


def test_corrupt_retained_bytes_are_not_trusted():
    value = report()
    blobs = dict(BLOBS)
    digest = value['artifacts'][0]['sha256']
    blobs[digest] = b'corrupt'
    with pytest.raises(ValueError, match='blob identity mismatch'):
        validate(value,blobs)


def test_changed_limits_require_matching_config_and_request():
    value = report()
    config = value['executions'][-1]['config']
    config['settings']['limits']['wall_ms'] += 1
    with pytest.raises(ValueError, match='configuration digest mismatch'):
        validate(value)
    config['sha256'] = identity('aletharsis.config/1',{k:v for k,v in config.items() if k!='sha256'})
    with pytest.raises(ValueError, match='conflicting adapter limit'):
        validate(value)


@pytest.mark.parametrize('raw', [b'{"schema_version":"3.0","schema_version":"3.0"}',
    b'{"schema_version":"3.0","x":NaN}', b'{"schema_version":"3.0","x":1e999}',
    b'{"schema_version":"3.0","x":"\\ud800"}', b'\xff'])
def test_unsafe_json(raw):
    with pytest.raises((ValueError,UnicodeError)):
        import_report(raw)


def test_report_and_blob_budgets():
    with pytest.raises(ValueError, match='report budget'):
        import_report(b' '*(8*1024*1024+1))
    with pytest.raises(ValueError, match='blob item budget'):
        validate(report(), {'unreferenced': b'x'*(32*1024*1024+1)})


def test_unicode_coordinate_vectors_do_not_conflate_input_with_source():
    vector = json.loads((HERE / 'coordinate-vectors.json').read_bytes())
    value = report(Path(vector['fixture']).stem)
    validate(value)
    source = BLOBS[value['artifacts'][0]['sha256']]
    normalized = BLOBS[value['artifacts'][2]['sha256']]
    assert source.hex() == vector['source_hex']
    assert normalized.hex() == vector['normalized_hex']
    for prefix, raw in [('source',source), ('normalized',normalized)]:
        scalars = raw.decode('utf-8')
        byte_boundaries, utf16_boundaries = [0], [0]
        for char in scalars:
            byte_boundaries.append(byte_boundaries[-1] + len(char.encode('utf-8')))
            utf16_boundaries.append(utf16_boundaries[-1] + len(char.encode('utf-16-le'))//2)
        assert byte_boundaries == vector[prefix+'_scalar_byte_boundaries']
        assert utf16_boundaries == vector[prefix+'_utf16_boundaries']
    assert value['results'][-1]['payload']['spans'] == [vector['carrier_input_byte_span']]
    assert value['artifacts'][2]['mapping']['quality'] == vector['mapping_quality'] == 'derived'
    assert vector['edit_authority'] is False
    for raw, key in [(source,'carrier_source_byte_span'),(normalized,'carrier_input_byte_span')]:
        span = vector[key]
        assert raw[span['start']:span['end']] == '\u200b'.encode()


def test_exact_request_artifact_hash_differs_from_composite_request_id():
    value = report()
    run = value['adapter_runs'][-1]
    artifact = next(a for a in value['artifacts'] if a['artifact_ref'] == run['request_ref'])
    assert artifact['sha256'] == sha256(BLOBS[artifact['sha256']]).hexdigest()
    assert artifact['sha256'] != run['request_id']
    validate(value)


def test_missing_blobs_preserve_unknown_verification_state():
    raw = (HERE/'fixtures/adapter-verification.json').read_bytes()
    imported = import_report(raw,{})
    assert imported['status'] == 'validated'
    assert imported['unresolved_blob_refs']
    assert imported['source_authority'] == 'not_granted_by_import'
    assert import_report(raw,BLOBS)['unresolved_blob_refs'] == []


def test_statistical_score_preserves_non_probability_numeric_semantics():
    value = report('statistical-large-score')
    imported = import_report(json.dumps(value).encode(), BLOBS)
    score = imported['report']['results'][-1]['payload']['score']
    assert score == {'value':1e100,'units':'test-statistic','semantics_ref':'fixture:test-statistic/1'}


def test_evaluated_trust_is_independent_from_signature_and_binding():
    value = report('verification-trusted')
    validate(value)
    p = value['results'][-1]['payload']
    assert (p['signature'],p['binding'],p['trust']) == ('valid','mismatch','accepted')
    assert value['status'] == 'completed' and value['summary']['exit_code'] == 0


def test_filtering_findings_does_not_erase_results():
    value = report('carrier-filtered')
    validate(value)
    assert value['findings'] == []
    assert len(value['results']) == 2
    assert value['adapter_runs'][-1]['response_ref'] is not None


def test_retention_size_limits_apply_without_resolving_blobs():
    value = report()
    ref = value['adapter_runs'][-1]['raw_result_ref']
    artifact = next(a for a in value['artifacts'] if a['artifact_ref'] == ref)
    artifact['byte_length'] = 5000
    with pytest.raises(ValueError, match='retained response exceeds output budget'):
        validate(value,None)


def test_new_result_cannot_be_attached_to_native_execution():
    value = report()
    extra = deepcopy(value['results'][-1])
    extra.update(result_ref='result/2', execution_ref='exec/0',anchor_refs=[])
    value['results'].append(extra)
    with pytest.raises(ValueError, match='adapter result on native execution'):
        validate(value)


def test_retained_manifest_and_complete_blob_inventory():
    manifest = json.loads((HERE / 'manifest.json').read_bytes())
    paths = [*FIXTURES, *(HERE/'invalid').glob('*.json'), *(HERE/'blobs').iterdir(),
             HERE/'coordinate-vectors.json', HERE/'catalog.json', HERE/'invalid-cases.json']
    assert set(manifest['files']) == {p.relative_to(HERE).as_posix() for p in paths}
    for path in paths:
        assert sha256(path.read_bytes()).hexdigest() == manifest['files'][path.relative_to(HERE).as_posix()]
    for digest, raw in BLOBS.items():
        assert sha256(raw).hexdigest() == digest


def test_integer_spelling_cannot_bypass_finite_score_limit():
    with pytest.raises(ValueError, match='non-finite JSON number'):
        strict_json(b'{"score":' + b'9'*400 + b'}')
    assert strict_json(b'{"score":9007199254740993}')['score'] == 9007199254740992.0
