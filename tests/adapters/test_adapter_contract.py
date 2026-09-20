"""DA-001 positive, negative and transport conformance; no provider execution."""
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator, ValidationError

from adapter_support import SCHEMA, decode_frame, request_digest, validate

FIXTURES = Path(__file__).parent / 'fixtures'
FIXTURE_PATHS = sorted(FIXTURES.glob('*.json'))
assert len(FIXTURE_PATHS) == 13, 'Expected all thirteen DA-001 outcome fixtures'


def load(name='extracted'):
    return json.loads((FIXTURES / (name + '.json')).read_bytes())


def bind(record):
    record['request']['request_id'] = request_digest(record['request'])
    if record['response'] is not None:
        record['response']['request_id'] = record['request']['request_id']


@pytest.mark.parametrize('path', FIXTURE_PATHS, ids=lambda p: p.stem)
def test_complete_transcripts(path):
    """Each retained fixture obeys schema, identity and outcome invariants."""
    validate(json.loads(path.read_bytes()))


def test_schema_is_valid_and_fixed_objects_are_closed():
    Draft202012Validator.check_schema(SCHEMA)
    def visit(node):
        if isinstance(node, dict):
            if node.get('type') == 'object' and 'properties' in node:
                assert node['additionalProperties'] is False
                assert set(node['required']) == set(node['properties'])
            for value in node.values():
                visit(value)
        elif isinstance(node, list):
            for value in node:
                visit(value)
    visit(SCHEMA)


@pytest.mark.parametrize(('name', 'path', 'value', 'message'), [
    ('extracted', ['extra'], True, 'Additional properties'),
    ('extracted', ['request', 'request_id'], '0'*64, 'request identity'),
    ('extracted', ['response', 'input_sha256'], '0'*64, 'response source'),
    ('extracted', ['response', 'request_id'], '0'*64, 'response request'),
    ('extracted', ['response', 'analyzed', 0, 'end'], 5, 'unaccounted input'),
    ('partial', ['response', 'excluded', 0, 'start'], 8, 'coverage gap or overlap'),
    ('partial', ['response', 'results', 0, 'spans', 0, 'end'], 10, 'outside coverage'),
    ('extracted', ['response', 'results', 0, 'outcome'], 'absent', 'absence with evidence'),
    ('verification', ['response', 'results', 0, 'discovery'], 'absent', 'absence with verification'),
    ('verification', ['response', 'results', 0, 'trust'], 'accepted', 'trust without material'),
    ('statistical', ['response', 'results', 0, 'sample_sha256'], '0'*64, 'sample mismatch'),
    ('statistical', ['response', 'results', 0, 'configuration_sha256'], '0'*64, 'config mismatch'),
    ('extracted', ['cleanup'], 'failed', 'cleanup status'),
    ('unavailable', ['cleanup'], 'complete', 'unavailable worker started'),
    ('timeout', ['response'], load()['response'], 'failure cannot promote'),
    ('extracted', ['response', 'raw_provider_result', 'byte_length'], 0, 'blob length'),
    ('extracted', ['response', 'results'], [], 'not valid under any'),
])
def test_reject_false_claims(name, path, value, message):
    """Invalid evidence is rejected rather than downgraded into a negative result."""
    record = load(name)
    target = record
    for key in path[:-1]:
        target = target[key]
    target[path[-1]] = value
    with pytest.raises((ValueError, ValidationError), match=message):
        validate(record)


@pytest.mark.parametrize(('limit', 'value', 'message'), [
    ('input_bytes', 1, 'input budget'), ('output_bytes', 1, 'output budget'),
    ('wall_ms', 0, 'minimum'), ('memory_bytes', 2147483649, 'maximum'),
    ('cpu_ms', True, 'integer'),
])
def test_budgets(limit, value, message):
    record = load()
    record['request']['limits'][limit] = value
    bind(record)
    with pytest.raises((ValueError, ValidationError), match=message):
        validate(record)


def test_nfc_mapping_preserves_source_and_rejects_false_exactness():
    """Composition changes UTF-8 width; its offsets cannot become source offsets."""
    record = load()
    source = record['request']['source']
    source_bytes = bytes.fromhex(record['blobs'][source['sha256']])
    assert source_bytes.startswith(b'e\xcc\x81')
    raw = b'\xc3\xa9' + source_bytes[3:]
    normalized = {'sha256': sha256(raw).hexdigest(), 'byte_length': len(raw)}
    record['blobs'][normalized['sha256']] = raw.hex()
    record['request'].update(input=normalized, input_kind='normalized', transforms=[{
        'input': source, 'output': normalized, 'operation': 'unicode.nfc', 'version': '15.0',
        'quality': 'derived', 'pairs': [{'source': {'start': 0, 'end': len(source_bytes)},
                                        'target': {'start': 0, 'end': len(raw)}}], 'exclusions': []}])
    response = record['response']
    response['input_sha256'] = normalized['sha256']
    response['analyzed'][0]['end'] = len(raw)
    response['results'][0]['spans'] = [{'start': 5, 'end': 8}]
    bind(record)
    validate(record)
    record['request']['transforms'][0]['quality'] = 'exact'
    bind(record)
    with pytest.raises(ValueError, match='false exact mapping'):
        validate(record)


def test_digest_binds_options_and_raw_provider_bytes():
    record = load()
    record['request']['validation_time'] = '2026-09-21T00:00:00Z'
    with pytest.raises(ValueError, match='request identity'):
        validate(record)
    record = load()
    digest = record['response']['raw_provider_result']['sha256']
    record['blobs'][digest] = b'changed'.hex()
    with pytest.raises(ValueError, match='blob digest'):
        validate(record)


@pytest.mark.parametrize('raw', [b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":Infinity}',
    b'{"x":9007199254740992}', b'{"x":"\\ud800"}', b'\xff', b'{} trailing',
    b'['*34 + b'0' + b']'*34])
def test_reject_unsafe_json(raw):
    with pytest.raises((ValueError, UnicodeError)):
        decode_frame(len(raw).to_bytes(4, 'big') + raw)


def test_frame_boundaries():
    assert decode_frame(b'\0\0\0\2{}') == {}
    for frame in (b'', b'\0\0\0\3{}', b'\0\0\0\2{}x', b'\xff'*4):
        with pytest.raises(ValueError):
            decode_frame(frame)


def test_valid_signature_does_not_imply_matching_asset_or_trust():
    record = load('verification')
    validate(record)
    result = record['response']['results'][0]
    assert (result['signature'], result['binding'], result['trust']) == (
        'valid', 'mismatch', 'not_evaluated')
    material = b'synthetic trust material'
    identity = {'sha256': sha256(material).hexdigest(), 'byte_length': len(material)}
    record['blobs'][identity['sha256']] = material.hex()
    record['request']['trust'] = identity
    result['trust'] = 'accepted'
    bind(record)
    validate(record)


def test_wrong_operation_cannot_smuggle_provider_result():
    record = load()
    record['response']['results'] = load('statistical')['response']['results']
    with pytest.raises(ValueError, match='operation mismatch'):
        validate(record)


def test_unbound_derived_input_is_rejected():
    record = load()
    record['request']['input_kind'] = 'normalized'
    bind(record)
    with pytest.raises(ValueError, match='input kind/derivation'):
        validate(record)


def test_unavailable_mapping_cannot_claim_pairs():
    record = load()
    request = record['request']
    request['input_kind'] = 'decoded'
    request['transforms'] = [{'input': request['source'], 'output': request['input'],
        'operation': 'decode.utf8', 'version': '1', 'quality': 'unavailable',
        'pairs': [], 'exclusions': []}]
    bind(record)
    validate(record)
    request['transforms'][0]['pairs'] = [{'source': {'start': 0, 'end': 3},
                                         'target': {'start': 0, 'end': 3}}]
    bind(record)
    with pytest.raises(ValueError, match='unavailable mapping with pairs'):
        validate(record)


def test_verification_cannot_promote_partial_asset_check():
    record = load('verification')
    record['outcome'] = {'state': 'partial', 'reason': 'partial'}
    response = record['response']
    response.update(state='partial', analyzed=[{'start': 0, 'end': 3}],
                    excluded=[{'start': 3, 'end': record['request']['input']['byte_length']}])
    with pytest.raises(ValueError, match='whole-input operation'):
        validate(record)


def test_retained_contract_and_fixture_digests():
    """The review manifest binds every schema, oracle, spec and fixture byte."""
    root = Path(__file__).resolve().parents[2]
    manifest = json.loads((root / 'docs/reuse/evaluations/adapter-contract-manifest.json').read_bytes())
    expected_paths = {'docs/specs/detector-adapter-v1.md', 'schemas/adapter-exchange-v1.schema.json',
                      'tests/adapters/adapter_support.py', 'tests/adapters/test_adapter_contract.py'}
    expected_paths.update(str(path.relative_to(root)) for path in FIXTURE_PATHS)
    assert set(manifest['files']) == expected_paths
    for name, expected in manifest['files'].items():
        assert sha256((root / name).read_bytes()).hexdigest() == expected


@pytest.mark.parametrize('count', [0, 2])
def test_schema_requires_one_aggregate_result(count):
    """Schema-only consumers reject missing or multiple aggregate results."""
    record = load()
    record['response']['results'] *= count
    with pytest.raises(ValidationError) as error:
        Draft202012Validator(SCHEMA).validate(record)
    assert any(child.validator == ('minItems' if count == 0 else 'maxItems')
               and list(child.path) == ['results'] for child in error.value.context)
