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
