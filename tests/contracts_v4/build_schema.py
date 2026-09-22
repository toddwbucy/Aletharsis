"""Reproduce OC-001's additive Office contract from the published v3 schema.

This generator never rewrites earlier contracts. Cross-record identity and
coordinate checks belong to the supported semantic validator, not JSON Schema.
"""
from copy import deepcopy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def obj(properties):
    return {'type': 'object', 'properties': properties, 'required': list(properties),
            'additionalProperties': False}


def ref(name):
    return {'$ref': '#/$defs/' + name}


def array(item):
    return {'type': 'array', 'items': item}


def nullable(value):
    return {'anyOf': [value, {'type': 'null'}]}


def build():
    schema = json.loads((ROOT / 'schemas/report-v3.schema.json').read_bytes())
    schema['$id'] = 'https://aletharsis.invalid/schemas/report-v4.schema.json'
    schema['title'] = 'Aletharsis Office report 4.0 (OC-001)'
    schema['properties']['schema_version'] = {'const': '4.0'}
    d = schema['$defs']
    integer = {'type': 'integer', 'minimum': 0, 'maximum': 9007199254740991}
    text = {'type': 'string'}
    name = {'type': 'string', 'minLength': 1}
    for kind in ('package', 'part', 'scope', 'object', 'xml', 'relationship'):
        d['office' + kind.title() + 'Ref'] = {
            'type': 'string', 'pattern': '^office-' + kind + '/(0|[1-9][0-9]*)$'}
    state = {'enum': ['completed', 'partial', 'failed', 'canceled', 'not_run']}
    d['officeIssue'] = obj({'code': ref('code'), 'diagnostic_ref': nullable(ref('diagnosticRef')),
                           'part_ref': nullable(ref('officePartRef'))})
    d['officeSpan'] = obj({'start': integer, 'end': integer})
    d['officeOutcome'] = obj({'operation': name, 'part_ref': nullable(ref('officePartRef')),
        'state': state, 'codes': array(ref('code')), 'diagnostic_refs': array(ref('diagnosticRef')),
        'assessed': array(ref('officeSpan')), 'excluded': array(ref('officeSpan'))})
    d['officePart'] = obj({'part_ref': ref('officePartRef'), 'package_ref': ref('officePackageRef'),
        'name': name, 'method': {'enum': [0, 8]}, 'compressed_span': ref('officeSpan'),
        'compressed_sha256': ref('digest'), 'sha256': nullable(ref('digest')),
        'byte_length': nullable(integer), 'state': state, 'issues': array(ref('officeIssue'))})
    d['officeToken'] = obj({'index': integer, 'kind': name, 'span': ref('officeSpan'),
        'element': nullable(integer)})
    d['officeElement'] = obj({'index': integer, 'parent': nullable(integer),
        'namespace': text, 'local_name': name, 'span': ref('officeSpan')})
    d['officeXML'] = obj({'xml_ref': ref('officeXmlRef'), 'part_ref': ref('officePartRef'),
        'parser_version': name, 'tokens': array(ref('officeToken')),
        'elements': array(ref('officeElement')), 'issues': array(ref('officeIssue'))})
    d['officeOrigin'] = {'oneOf': [obj({
        'kind': {'const': 'stored'}, 'xml_ref': ref('officeXmlRef'),
        'text_index': integer, 'segment': integer, 'scalar': integer,
        'source': ref('officeSpan'), 'utf8': ref('officeSpan'), 'transformation': name}),
        obj({'kind': {'const': 'control_expansion'}, 'xml_ref': ref('officeXmlRef'),
             'control': integer, 'element': integer, 'repetition': integer,
             'source': ref('officeSpan'), 'utf8': ref('officeSpan'),
             'transformation': {'enum': ['space', 'tab', 'line_break']}})]}
    d['officeScalar'] = obj({'scalar': integer, 'code_point': integer,
        'source': ref('officeSpan'), 'utf8': ref('officeSpan'), 'transformation': name})
    d['officeSegment'] = obj({'token': integer, 'element': nullable(integer),
        'token_span': ref('officeSpan'), 'content_span': ref('officeSpan'),
        'cdata': {'type': 'boolean'}, 'text': text, 'sha256': ref('digest'),
        'scalars': array(ref('officeScalar'))})
    d['officeXML']['properties']['segments'] = array(ref('officeSegment'))
    d['officeXML']['required'].append('segments')
    d['officeXML']['properties']['mapper_version'] = name
    d['officeXML']['required'].append('mapper_version')
    d['officeControl'] = obj({'index': integer, 'element': integer,
        'kind': {'enum': ['space', 'tab', 'line_break']}, 'count': integer,
        'source': ref('officeSpan')})
    d['officeXML']['properties']['controls'] = array(ref('officeControl'))
    d['officeXML']['required'].append('controls')
    d['officeBoundary'] = obj({'xml_ref': ref('officeXmlRef'), 'token': integer, 'reason': name})
    d['officeScope'] = obj({'scope_ref': ref('officeScopeRef'), 'part_ref': ref('officePartRef'),
        'local_id': name, 'extractor_version': name, 'assembler_version': name,
        'identity_sha256': ref('digest'), 'role': name, 'text': text, 'sha256': ref('digest'),
        'origins': array(ref('officeOrigin')), 'hashes': ref('textHashes'),
        'boundaries': array(ref('officeBoundary')), 'issues': array(ref('officeIssue'))})
    d['officeStructuralLocation'] = obj({'kind': {'const': 'office_structure'},
        'part_ref': ref('officePartRef'), 'part_sha256': ref('digest'),
        'xml_ref': nullable(ref('officeXmlRef')), 'element': nullable(integer),
        'token': nullable(integer), 'span': nullable(ref('officeSpan')),
        'object_ref': nullable(ref('officeObjectRef'))})
    d['officeScopeLocation'] = obj({'kind': {'const': 'office_scope'}, 'scope_ref': ref('officeScopeRef')})
    d['officeOffsetLocation'] = obj({'kind': {'const': 'office_offsets'},
        'scope_ref': ref('officeScopeRef'), 'scope_character_offsets': array(integer),
        'scope_byte_offsets': array(integer)})
    d['officeMetadata'] = obj({'namespace': text, 'local_name': name, 'lexical_value': text,
        'normalized_key': nullable(name), 'part_ref': ref('officePartRef'),
        'element': integer, 'value_origins': array(ref('officeOrigin')), 'duplicate_ordinal': integer})
    d['officeRelationship'] = obj({'relationship_ref': ref('officeRelationshipRef'),
        'id': text, 'type': text, 'target': text, 'mode': text,
        'location': ref('officeStructuralLocation'), 'source_owner': text,
        'resolution_state': name, 'resolution_code': nullable(ref('code')),
        'target_part_ref': nullable(ref('officePartRef'))})
    d['officeObject'] = obj({'object_ref': ref('officeObjectRef'), 'part_ref': ref('officePartRef'),
        'sha256': nullable(ref('digest')), 'inclusion_evidence': array(name),
        'relationship_refs': array(ref('officeRelationshipRef')), 'declared_type': nullable(text),
        'observed_signature': nullable(text), 'inspection': ref('officeOutcome')})
    d['officePackage'] = obj({'package_ref': ref('officePackageRef'),
        'source_sha256': ref('digest'), 'source_byte_length': integer,
        'parser_version': name, 'format': {'enum': ['docx', 'odt', 'unknown']},
        'state': state, 'issues': array(ref('officeIssue')), 'parts': array(ref('officePart')),
        'outcomes': array(ref('officeOutcome')), 'limits': obj({k: integer for k in (
            'source_bytes', 'part_count', 'part_bytes', 'aggregate_bytes',
            'max_scope_text_utf8_bytes', 'max_scope_scalar_origins',
            'report_input_bytes', 'report_output_bytes', 'report_nodes', 'report_depth')})})
    d['officeEvidence'] = obj({k: array(ref(v)) for k, v in {
        'packages': 'officePackage', 'xml': 'officeXML', 'scopes': 'officeScope',
        'metadata': 'officeMetadata', 'relationships': 'officeRelationship',
        'objects': 'officeObject'}.items()})
    evidence = schema['properties']['evidence']
    evidence['properties']['office'] = ref('officeEvidence')
    evidence['required'].append('office')
    # Keep rule-specific evidence unchanged; only the coordinate alternatives grow.
    for variant in d['finding']['oneOf']:
        rule = d[variant['$ref'].split('/')[-1]]
        location = rule['properties']['location']
        original = deepcopy(location)
        # Only analyzers admitted by WA/OA receive Office scope coordinates.
        # Inventory/structure records are not fabricated native findings.
        if variant['$ref'].split('/')[-1].startswith(('unicode_', 'pattern_')):
            office_location = ('officeOffsetLocation' if original == ref('offsetLocation')
                               else 'officeScopeLocation')
            rule['properties']['location'] = {'oneOf': [original, ref(office_location)]}
    return schema


if __name__ == '__main__':
    (ROOT / 'schemas/report-v4.schema.json').write_text(json.dumps(build(), indent=2) + '\n')
