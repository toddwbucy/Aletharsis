"""Build the versioned corpus and presentation envelopes; leave v1 frozen."""
from copy import deepcopy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def load(name):
    return json.loads((ROOT / 'schemas' / name).read_bytes())


def build():
    corpus = load('corpus-v1.schema.json')
    corpus['$id'] = corpus['$id'].replace('-v1.', '-v2.')
    d = corpus['$defs']
    d['header']['properties']['contract']['const'] = 'aletharsis.corpus/2'
    d['header']['properties']['report_schema']['enum'] = ['2.0', '4.0']
    entry = d['entry']
    entry['properties']['state']['enum'].append('partial')
    entry['properties']['report']['oneOf'] = [
        {'$ref': 'report-v2.schema.json'}, {'$ref': 'report-v4.schema.json'}]
    entry['properties']['report_schema'] = {'enum': ['2.0', '4.0']}
    entry['properties']['highest_finding_severity'] = {'enum': [None, 'INFO', 'LOW', 'MEDIUM', 'HIGH']}
    entry['dependentRequired']['report'] += ['report_schema', 'highest_finding_severity']
    for key in ('report_schema', 'highest_finding_severity'):
        entry['dependentRequired'][key] = ['report']
    entry['allOf'][0]['if']['properties']['state']['enum'].append('partial')
    for version in ('2.0', '4.0'):
        entry['allOf'].append({'if': {'required': ['report_schema'],
            'properties': {'report_schema': {'const': version}}},
            'then': {'properties': {'report': {'properties': {'schema_version': {'const': version}}}}}})
    # Report-bearing observer failures may retain a completed/partial report.
    for state, report_states in {'partial': ['partial'],
            'no_reported_findings': ['completed'], 'requires_review': ['completed'],
            'unsupported': ['failed'], 'canceled': ['canceled']}.items():
        entry['allOf'].append({'if': {'required': ['report'], 'properties': {'state': {'const': state}}},
            'then': {'properties': {'report': {'properties': {'status': {'enum': report_states}}}}}})
    counts = d['summary']['properties']['counts']
    counts['properties']['partial'] = {'type': 'integer', 'minimum': 0}
    counts['required'].append('partial')
    d['summary']['properties']['finding_severity_counts'] = {
        'type': 'object', 'additionalProperties': False,
        'required': ['INFO', 'LOW', 'MEDIUM', 'HIGH'],
        'properties': {k: {'type': 'integer', 'minimum': 0} for k in ('INFO', 'LOW', 'MEDIUM', 'HIGH')}}
    d['summary']['required'].append('finding_severity_counts')
    document = load('corpus-document-v1.schema.json')
    document = json.loads(json.dumps(document).replace('corpus-document-v1.', 'corpus-document-v2.')
                          .replace('corpus-v1.', 'corpus-v2.'))
    tree = load('reveal-tree-v1.schema.json')
    tree['$id'] = tree['$id'].replace('-v1.', '-v2.')
    tree['properties']['contract']['const'] = 'aletharsis.reveal-tree/2'
    source = tree['$defs']['source']
    del source['properties']['state']
    source['required'].remove('state')
    source['required'] += ['detection_state', 'presentation_outcome']
    source['properties']['detection_state'] = deepcopy(entry['properties']['state'])
    source['properties']['presentation_outcome'] = {
        'enum': ['revealed', 'failed', 'unsupported', 'not_attempted']}
    source['allOf'][0]['if']['properties'] = {'presentation_outcome': {'const': 'revealed'}}
    source['allOf'][0]['then']['properties']['detection_state'] = {
        'enum': ['no_reported_findings', 'requires_review', 'partial']}
    source['allOf'][1]['if']['properties'] = {'detection_state': {'const': 'skipped'}}
    source['allOf'].append({'if': {'properties': {'detection_state': {
        'enum': ['failed', 'unsupported', 'skipped', 'canceled']}}},
        'then': {'properties': {'presentation_outcome': {'const': 'not_attempted'}}}})
    source['allOf'].append({'if': {'properties': {'presentation_outcome': {
        'enum': ['revealed', 'unsupported']}}},
        'then': {'required': ['report_artifact_sha256']}})
    return {'corpus-v2.schema.json': corpus, 'corpus-document-v2.schema.json': document,
            'reveal-tree-v2.schema.json': tree}


if __name__ == '__main__':
    for name, schema in build().items():
        (ROOT / 'schemas' / name).write_text(json.dumps(schema, indent=2) + '\n')
