"""Reproduce the proposed self-contained v3 schema from the frozen v2 contract."""
from copy import deepcopy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def obj(properties):
    return {'type': 'object', 'properties': properties, 'required': list(properties),
            'additionalProperties': False}


def ref(name):
    return {'$ref': '#/$defs/' + name}


def nullable(value):
    return {'anyOf': [value, {'type': 'null'}]}


def array(value, **bounds):
    return {'type': 'array', 'items': value, **bounds}


def build():
    s = json.loads((ROOT / 'schemas/report-v2.schema.json').read_bytes())
    da = json.loads((ROOT / 'schemas/adapter-exchange-v1.schema.json').read_bytes())['$defs']
    s['$id'] = 'https://aletharsis.invalid/schemas/report-v3.schema.json'
    s['title'] = 'Aletharsis report 3.0 (EC-004 conformance proposal)'
    s['properties']['schema_version'] = {'const': '3.0'}
    s['properties']['adapter_runs'] = array(ref('adapterRun'))
    s['required'].append('adapter_runs')
    d = s['$defs']
    text = {'type': 'string', 'minLength': 1}
    protocol = {'const': 'aletharsis.adapter-exchange/1'}
    d['adapterLimits'] = deepcopy(da['limits'])
    d['adapterConfig'] = obj({'schema': {'const': 'aletharsis.adapter-config/1'},
        'revision': text, 'settings': obj({'protocol': protocol,
            'operation': {'enum': ['extract', 'verify', 'statistical']},
            'provider_config_ref': ref('artifactRef'), 'policy_ref': ref('artifactRef'),
            'trust_ref': nullable(ref('artifactRef')),
            'validation_time': {'type': 'string', 'format': 'date-time'},
            'limits': ref('adapterLimits')}), 'sha256': ref('digest'), 'key_ref': {'type': 'null'}})
    d['config']['oneOf'].append(ref('adapterConfig'))
    d['artifact']['properties']['kind']['enum'] += [
        'adapter_request', 'adapter_response', 'configuration', 'policy', 'trust_material']
    # Reserve the inline-map method so the old pointer variant cannot fake it.
    d['mapping']['oneOf'][1]['properties']['method']['not'] = {'const': 'aletharsis.byte-map'}
    d['byteMap'] = obj({'quality': {'enum': ['exact', 'derived']},
        'from_artifact_ref': ref('artifactRef'), 'to_artifact_ref': ref('artifactRef'),
        'method': {'const': 'aletharsis.byte-map'}, 'version': {'const': '1'},
        'pairs': array(obj({'source': ref('region'), 'target': ref('region')})),
        'exclusions': array(ref('region'))})
    d['mapping']['oneOf'].append(ref('byteMap'))
    anchor = deepcopy(d['anchor']['oneOf'][0])
    anchor['properties']['kind'] = {'const': 'artifact_bytes'}
    anchor['properties']['locator'] = obj({'version': {'const': '1'},
        'spans': array(ref('region'), minItems=1)})
    d['anchor']['oneOf'].append(anchor)
    d['adapterRun'] = obj({'execution_ref': ref('executionRef'), 'protocol': protocol,
        'request_id': nullable(ref('digest')), 'request_ref': nullable(ref('artifactRef')),
        'response_ref': nullable(ref('artifactRef')), 'raw_result_ref': nullable(ref('artifactRef')),
        'quarantined_refs': array(ref('artifactRef'), uniqueItems=True),
        'launch_profile': nullable(obj({'id': text, 'version': text, 'policy_ref': ref('artifactRef')})),
        'started': {'type': 'boolean'}, 'cleanup': {'enum': ['not_started', 'complete', 'failed']}})
    d['adapterRun']['allOf'] = [
        {'if': {'properties': {'started': {'const': False}}},
         'then': {'properties': {'cleanup': {'const': 'not_started'}}},
         'else': {'properties': {'cleanup': {'enum': ['complete', 'failed']},
             'request_id': ref('digest'), 'request_ref': ref('artifactRef'),
             'launch_profile': obj({'id': text, 'version': text, 'policy_ref': ref('artifactRef')})}}}]
    d['structuralResult'] = d['result']
    d['result'] = {'oneOf': [ref('structuralResult')]}
    payloads = {
        'carrier_extraction': obj({'operation': text, 'scopes': array(ref('scope'), minItems=1),
            'outcome': deepcopy(da['extract']['properties']['outcome']), 'input_ref': ref('artifactRef'),
            'spans': array(ref('region')), 'manifest_ref': nullable(ref('artifactRef'))}),
        'credential_verification': obj({'operation': text, 'scope': ref('scope'),
            'input_ref': ref('artifactRef'), 'discovery': deepcopy(da['verify']['properties']['discovery']),
            'manifest_ref': nullable(ref('artifactRef')),
            **{k: deepcopy(da['verify']['properties'][k]) for k in
               ('signature', 'binding', 'trust', 'revocation', 'freshness')}}),
        'statistical_analysis': obj({'operation': text, 'scope': ref('scope'),
            'sample_ref': ref('artifactRef'), 'algorithm': text, 'algorithm_version': text,
            'configuration_sha256': ref('digest'), 'outcome': text,
            'score': nullable(deepcopy(da['score']))})}
    for kind, payload in payloads.items():
        result = deepcopy(d['structuralResult'])
        result['properties']['kind'] = {'const': kind}
        result['properties']['payload'] = payload
        d[kind + 'Result'] = result
        d['result']['oneOf'].append(ref(kind + 'Result'))
    diagnostic = deepcopy(d['diagnostic']['oneOf'][2])
    diagnostic['properties']['stage'] = {'const': 'adapter'}
    diagnostic['properties']['execution_ref'] = ref('executionRef')
    diagnostic['properties']['details'] = obj({'protocol': protocol,
        'primary_code': nullable(ref('code')), 'cleanup': {'enum': ['not_started', 'complete', 'failed']},
        'limit': nullable({'enum': list(da['limits']['properties'])}),
        'limit_value': nullable({'type': 'integer', 'minimum': 1, 'maximum': 9007199254740991})})
    d['diagnostic']['oneOf'].append(diagnostic)
    return s


if __name__ == '__main__':
    (ROOT / 'schemas/report-v3.schema.json').write_bytes((json.dumps(build(), indent=2) + '\n').encode('utf-8'))
