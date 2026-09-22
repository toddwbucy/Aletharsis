"""Construction invariants for the additive Office wire contract."""
import importlib.util
import json
from pathlib import Path

from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[2]
_spec = importlib.util.spec_from_file_location('office_schema_builder', Path(__file__).with_name('build_schema.py'))
_builder = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_builder)


def walk(value):
    yield value
    if isinstance(value, dict):
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def test_schema_is_reproducible_and_well_formed():
    schema = json.loads((ROOT / 'schemas/report-v4.schema.json').read_bytes())
    assert schema == _builder.build()
    Draft202012Validator.check_schema(schema)
    for node in walk(schema):
        if isinstance(node, dict) and '$ref' in node:
            assert node['$ref'].startswith('#/$defs/')
            assert node['$ref'].split('/')[-1] in schema['$defs']


def test_every_office_object_is_closed_and_required():
    schema = _builder.build()
    for name, definition in schema['$defs'].items():
        if not name.startswith('office'):
            continue
        for node in walk(definition):
            if isinstance(node, dict) and node.get('type') == 'object':
                assert node['additionalProperties'] is False
                assert set(node['properties']) == set(node['required'])


def test_adapter_envelope_is_retained_without_redefinition():
    old = json.loads((ROOT / 'schemas/report-v3.schema.json').read_bytes())
    new = _builder.build()
    assert new['properties']['adapter_runs'] == old['properties']['adapter_runs']
    assert 'adapter_runs' in new['required']
    for name, definition in old['$defs'].items():
        if name.startswith('adapter') or name in (
            'carrier_extractionResult', 'credential_verificationResult',
            'statistical_analysisResult', 'byteMap'):
            assert new['$defs'][name] == definition


def test_all_patterns_remain_compatible_with_go_regexp():
    for node in walk(_builder.build()):
        if isinstance(node, dict) and 'pattern' in node:
            assert not any(token in node['pattern'] for token in ('(?=', '(?!', '(?<=', '(?<!'))


def test_office_location_alternatives_follow_analyzer_scope():
    old = json.loads((ROOT / 'schemas/report-v3.schema.json').read_bytes())
    new = _builder.build()
    for variant in old['$defs']['finding']['oneOf']:
        key = variant['$ref'].split('/')[-1]
        previous = old['$defs'][key]['properties']['location']
        current = new['$defs'][key]['properties']['location']
        if key.startswith(('unicode_', 'pattern_')):
            target = ('officeOffsetLocation' if previous['$ref'].endswith('/offsetLocation')
                      else 'officeScopeLocation')
            assert current == {'oneOf': [previous, {'$ref': '#/$defs/' + target}]}
        else:
            assert current == previous


def test_office_coordinate_variants_reject_mixed_and_missing_coordinates():
    schema = _builder.build()
    validator = Draft202012Validator({'$defs': schema['$defs'],
        '$ref': '#/$defs/officeOffsetLocation'})
    valid = {'kind': 'office_offsets', 'scope_ref': 'office-scope/0',
             'scope_character_offsets': [0], 'scope_byte_offsets': [0]}
    assert validator.is_valid(valid)
    for key in valid:
        assert not validator.is_valid({k: v for k, v in valid.items() if k != key})
    for key in ('character_offsets', 'byte_offsets', 'part_ref', 'object_ref'):
        assert not validator.is_valid({**valid, key: []})
    for value in ('office-scope/00', 'office-scope/-1', 'office-part/0'):
        assert not validator.is_valid({**valid, 'scope_ref': value})


def test_envelope_construction_and_presentation_states():
    spec = importlib.util.spec_from_file_location('office_envelopes',
        Path(__file__).with_name('build_envelopes.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    envelopes = module.build()
    for name, schema in envelopes.items():
        assert schema == json.loads((ROOT / 'schemas' / name).read_bytes())
        Draft202012Validator.check_schema(schema)
    tree = envelopes['reveal-tree-v2.schema.json']
    validator = Draft202012Validator({'$defs': tree['$defs'], '$ref': '#/$defs/source'})
    base = {'relative_path': 'example.docx', 'detection_state': 'partial',
            'presentation_outcome': 'unsupported', 'reason': 'presentation.unsupported',
            'report_artifact_sha256': 'a' * 64,
            'artifacts': [{'name': 'report.json', 'size': 1, 'sha256': 'a' * 64}]}
    assert validator.is_valid(base)
    assert not validator.is_valid({**base, 'state': 'unsupported'})
    assert not validator.is_valid({**base, 'detection_state': 'failed'})
    assert validator.is_valid({**base, 'detection_state': 'failed',
                               'presentation_outcome': 'not_attempted'})
    assert validator.is_valid({**base, 'presentation_outcome': 'failed',
                               'reason': 'execution.resource_limit'})
    assert not validator.is_valid({**base, 'presentation_outcome': 'revealed'})


def test_complete_flat_fixtures_are_reproducible_and_wire_valid():
    spec = importlib.util.spec_from_file_location('office_fixture_builder',
        Path(__file__).with_name('build_fixtures.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    expected = module.build()
    paths = sorted(Path(__file__).with_name('fixtures').glob('flat-*.json'))
    assert len(paths) == len(expected) == 8
    validator = Draft202012Validator(_builder.build())
    for path in paths:
        fixture = json.loads(path.read_bytes())
        assert fixture == expected[path.name]
        validator.validate(fixture)
