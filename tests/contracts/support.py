"""Offline conformance oracle, not a production importer or a general JCS library."""
from hashlib import sha256
import json
from pathlib import Path

from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[2]
MAX_REPORT_BYTES = 8 * 1024 * 1024
MAX_DEPTH = 64


def require(condition, message):
    if not condition:
        raise ValueError(message)


def strict_json(raw):
    require(len(raw) <= MAX_REPORT_BYTES, 'report budget exceeded')
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, 'duplicate JSON key')
            result[key] = value
        return result
    def constant(value):
        raise ValueError('non-finite JSON number')
    try:
        value = json.loads(raw.decode('utf-8'), object_pairs_hook=pairs, parse_constant=constant)
    except RecursionError:
        raise ValueError('report depth exceeded') from None
    def visit(item, depth):
        require(depth <= MAX_DEPTH, 'report depth exceeded')
        if isinstance(item, str):
            item.encode('utf-8', errors='strict')
        elif isinstance(item, (int, float)) and not isinstance(item, bool):
            require(-9007199254740991 <= item <= 9007199254740991, 'unsafe JSON number')
        elif isinstance(item, dict):
            for k, v in item.items():
                visit(k, depth + 1)
                visit(v, depth + 1)
        elif isinstance(item, list):
            for v in item:
                visit(v, depth + 1)
    visit(value, 0)
    return value


def identity_bytes(domain, value):
    """JCS subset used by these contracts: null/bool/safe integers/Unicode only.

    Nonintegral floating-point RFC vectors are retained separately for the Go implementation
    gate. This helper refuses nonintegral floats rather than approximating JCS.
    """
    def encode(v):
        if v is None or isinstance(v, bool):
            return json.dumps(v)
        if isinstance(v, int) or isinstance(v, float) and v.is_integer():
            require(abs(v) <= 9007199254740991, 'unsafe identity integer')
            return str(int(v))
        if isinstance(v, str):
            v.encode('utf-8')
            return json.dumps(v, ensure_ascii=False)
        if isinstance(v, list):
            return '[' + ','.join(encode(x) for x in v) + ']'
        if isinstance(v, dict):
            require(all(isinstance(k, str) for k in v), 'non-string identity key')
            return '{' + ','.join(encode(k) + ':' + encode(v[k]) for k in sorted(v, key=lambda k: k.encode('utf-16-be'))) + '}'
        raise ValueError('unsupported identity value')
    return encode({'domain': domain, 'value': value}).encode('utf-8')


def identity(domain, value):
    return sha256(identity_bytes(domain, value)).hexdigest()


def schema(version):
    name = 'report.schema.json' if version == '1.0' else 'report-v2.schema.json'
    return json.loads((ROOT / 'schemas' / name).read_bytes())


def import_report(raw):
    """Hash exact imported bytes; preserve report; never open a content reference."""
    digest = sha256(raw).hexdigest()
    report = strict_json(raw)
    require(isinstance(report, dict), 'report must be an object')
    version = report.get('schema_version')
    if version not in ('1.0', '2.0'):
        return {'report_artifact_sha256': digest, 'status': 'unsupported_version',
                'coverage': 'unknown', 'report': report, 'adapter_diagnostics': []}
    Draft202012Validator(schema(version)).validate(report)
    if version == '2.0':
        validate_semantics(report)
        return {'report_artifact_sha256': digest, 'status': 'validated',
                'coverage': 'declared', 'report': report, 'adapter_diagnostics': []}
    diagnostics = []
    for index, finding in enumerate(report['findings']):
        if finding['id'] == 'parser.failure':
            diagnostics.append({'code': 'legacy.failure_code_unavailable',
                                'report_pointer': f'/findings/{index}', 'failure_code': None})
    return {'report_artifact_sha256': digest, 'status': 'legacy', 'coverage': 'unknown',
            'report': report, 'adapter_diagnostics': diagnostics}


def validate_semantics(r):
    """Executable minimum consumer checks; wire shape is validated first."""
    def index(collection, field):
        records = {x[field]: x for x in r[collection]}
        require(len(records) == len(r[collection]), 'duplicate reference')
        return records
    caps = index('capabilities', 'id')
    executions = index('executions', 'execution_ref')
    artifacts = index('artifacts', 'artifact_ref')
    anchors = index('anchors', 'anchor_ref')
    diagnostics = index('diagnostics', 'diagnostic_ref')
    index('results', 'result_ref')
    index('findings', 'finding_ref')
    def get(records, key):
        require(key in records, 'dangling reference')
        return records[key]
    def pointer(p):
        parts = p.split('/')
        require(parts[:3] == ['', 'evidence', 'texts'], 'unsupported content pointer')
        try:
            item = r['evidence']['texts'][int(parts[3])]
            for key in parts[4:]:
                item = item[key]
        except (IndexError, KeyError, ValueError):
            raise ValueError('invalid content pointer') from None
        return item
    def mapping(m):
        if m['quality'] != 'unavailable':
            get(artifacts, m['from_artifact_ref'])
            get(artifacts, m['to_artifact_ref'])
            if m['data_ref'] is not None:
                require(isinstance(pointer(m['data_ref']), list), 'invalid mapping data')
    def intervals(scope):
        a = get(artifacts, scope['artifact_ref'])
        unit = scope['unit']
        if unit == 'whole_artifact':
            return [(0, a['byte_length'])] if a['byte_length'] is not None else None
        bound = a['byte_length']
        if unit == 'scalar':
            c = a['content_ref']
            require(c is not None and c['kind'] == 'report_pointer', 'unverifiable scalar scope')
            bound = len(pointer(c['pointer']))
        require(bound is not None, 'scope without known extent')
        previous = -1
        for region in scope['regions']:
            start, end = region['start'], region['end']
            require(previous <= start < end <= bound, 'invalid or overlapping scope')
            previous = end
        return [(x['start'], x['end']) for x in scope['regions']]
    def subset(inner, outer):
        require(inner['artifact_ref'] == outer['artifact_ref'], 'scope artifact mismatch')
        require(inner['unit'] == outer['unit'] or outer['unit'] == 'whole_artifact' and inner['unit'] == 'byte', 'scope unit mismatch')
        inside, outside = intervals(inner), intervals(outer)
        require(inside is not None and outside is not None, 'unverifiable analyzed scope')
        require(all(any(a <= start <= end <= b for a,b in outside) for start,end in inside), 'scope exceeds request')
    require(sum(a['kind'] == 'source' for a in artifacts.values()) == 1, 'one source artifact required')
    require({'acquisition', 'parser'} <= {c['role'] for c in caps.values()}, 'missing operational capabilities')
    require(all(e['capability_ref'] in caps for e in executions.values()), 'dangling capability reference')
    require(set(caps) == {e['capability_ref'] for e in executions.values()}, 'capability planning incomplete')
    seen = set()
    for a in r['artifacts']:
        require(all(parent in seen for parent in a['parents']), 'cyclic, missing or out-of-order parent')
        seen.add(a['artifact_ref'])
        mapping(a['mapping'])
        if a['transform'] is not None:
            require(a['transform']['inputs'] == a['parents'], 'transform/parent mismatch')
            for x in a['transform']['exclusions']:
                if x['scope'] is not None:
                    intervals(x['scope'])
        if a['kind'] == 'source':
            require(a['sha256'] == r['file']['sha256'] and a['byte_length'] == r['file']['size'], 'source identity mismatch')
            require(not a['parents'] and a['transform'] is None, 'source cannot be transformed')
        content = a['content_ref']
        if content is not None:
            if content['kind'] == 'report_pointer':
                raw = pointer(content['pointer']).encode('utf-8')
                require(a['sha256'] == sha256(raw).hexdigest() and a['byte_length'] == len(raw), 'artifact digest/length mismatch')
                require(a['representation']['encoding'] == 'utf-8', 'pointer text must use UTF-8')
            else:
                require(content['sha256'] == a['sha256'], 'blob digest mismatch')
    for t in r['evidence']['texts']:
        offsets = t['byte_offsets']
        require(len(offsets) == len(t['text']) + 1, 'scalar boundary count mismatch')
        require(all(a < b for a,b in zip(offsets, offsets[1:])), 'nonmonotonic byte offsets')
        require(r['file']['size'] is not None and offsets[-1] <= r['file']['size'], 'byte offsets outside source')
        encoding = t['encoding']
        if encoding in ('utf-8','utf-16-le','utf-16-be','utf-32-le','utf-32-be'):
            require([b-a for a,b in zip(offsets,offsets[1:])] == [len(c.encode(encoding)) for c in t['text']], 'byte/scalar width mismatch')
    for e in executions.values():
        cap = get(caps, e['capability_ref'])
        intervals(e['requested_scope'])
        for scope in e['analyzed_scope']:
            subset(scope, e['requested_scope'])
        for x in e['exclusions']:
            if x['scope'] is not None:
                subset(x['scope'], e['requested_scope'])
        for ref in e['diagnostic_refs']:
            require(get(diagnostics, ref)['execution_ref'] == e['execution_ref'], 'diagnostic execution mismatch')
        # Regions must not overlap each other or known exclusions. A partial
        # operation needs an explicit remainder; it cannot relabel full coverage.
        covered = []
        for sc in e['analyzed_scope']:
            covered.extend(intervals(sc))
        covered.sort()
        require(all(a[1] <= b[0] for a,b in zip(covered,covered[1:])), 'overlapping analyzed scope')
        excluded = []
        for x in e['exclusions']:
            if x['scope'] is not None:
                excluded.extend(intervals(x['scope']))
        excluded.sort()
        require(all(a[1] <= b[0] for a,b in zip(excluded,excluded[1:])), 'overlapping exclusions')
        require(all(end <= a or b <= start for start,end in covered for a,b in excluded), 'analyzed/excluded overlap')
        if e['state'] == 'partial':
            require(any(result['execution_ref'] == e['execution_ref'] for result in r['results']), 'partial execution without usable results')
            if not any(x['unknown_remainder'] for x in e['exclusions']):
                def merged(regions):
                    result = []
                    for start,end in sorted(regions):
                        if result and result[-1][1] == start:
                            result[-1] = (result[-1][0],end)
                        else:
                            result.append((start,end))
                    return result
                require(merged(covered + excluded) == merged(intervals(e['requested_scope'])), 'unaccounted requested region')
        if e['state'] == 'completed':
            require(e['analyzed_scope'] == [e['requested_scope']], 'incomplete completed scope')
        if cap['participation'] == 'disabled':
            require(e['state'] == 'not_run' and e['reason_code'] == 'execution.disabled', 'disabled execution ran')
        elif cap['availability']['state'] != 'available':
            require(e['state'] == 'not_run' and e['reason_code'] == cap['availability']['reason_code'], 'unavailable execution ran')
        if e['config']['sha256'] is not None:
            c = e['config']
            require(c['sha256'] == identity('aletharsis.config/1', {k:v for k,v in c.items() if k != 'sha256'}), 'configuration digest mismatch')
    for a in anchors.values():
        if a['artifact_ref'] is not None:
            get(artifacts, a['artifact_ref'])
        if a['execution_ref'] is not None:
            get(executions, a['execution_ref'])
        mapping(a['mapping'])
        loc = a['locator']
        if a['kind'] == 'text':
            t = pointer(loc['segment_pointer'])
            art = get(artifacts, a['artifact_ref'])
            require(art['kind'] == 'text' and art['content_ref'] == {'kind':'report_pointer','pointer':loc['segment_pointer']+'/text'}, 'text anchor artifact mismatch')
            selection = []
            previous = -1
            for span in loc['spans']:
                start, end = span['scalar']['start'], span['scalar']['end']
                require(previous <= start < end <= len(t['text']), 'invalid scalar span')
                require(span['byte'] == {'start':t['byte_offsets'][start],'end':t['byte_offsets'][end]}, 'anchor coordinate mismatch')
                previous = end
                selection.append({'span':span,'text':t['text'][start:end]})
            require(loc['selected_text_sha256'] == identity(loc['digest_domain'], selection), 'selection digest mismatch')
        elif a['kind'] == 'credential':
            require(get(artifacts,loc['manifest_ref'])['kind'] == 'manifest', 'not a manifest')
            if loc['verification_execution_ref'] is not None:
                get(executions,loc['verification_execution_ref'])
        elif a['kind'] == 'statistical_sample':
            sample = get(artifacts,loc['sample_ref'])
            require(sample['kind'] == 'sample' and sample['sha256'] is not None, 'sample identity missing')
            require(a['artifact_ref'] == loc['sample_ref'] == loc['scope']['artifact_ref'], 'sample scope mismatch')
            intervals(loc['scope'])
        elif a['kind'] == 'legacy_unknown':
            get(diagnostics,loc['diagnostic_ref'])
    for result in r['results']:
        e = get(executions,result['execution_ref'])
        cap = get(caps,e['capability_ref'])
        require(e['state'] in ('completed','partial') and cap['mechanism'] == 'structural' and cap['role'] == 'analyzer', 'result from ineligible execution')
        require(result['payload']['operation'] == cap['id'], 'result operation mismatch')
        require(result['payload']['scope'] in e['analyzed_scope'], 'result outside analyzed scope')
        for ref in result['anchor_refs']:
            require(get(anchors,ref)['execution_ref'] == e['execution_ref'], 'result anchor execution mismatch')
    for e in executions.values():
        cap = get(caps,e['capability_ref'])
        if e['state'] in ('completed','partial') and cap['role'] == 'analyzer' and cap['mechanism'] == 'structural':
            result_scopes = [x['payload']['scope'] for x in r['results'] if x['execution_ref'] == e['execution_ref']]
            require(all(sc in result_scopes for sc in e['analyzed_scope']), 'analyzed scope without result')
    for finding in r['findings']:
        e = get(executions,finding['execution_ref'])
        require(finding['mechanism'] == get(caps,e['capability_ref'])['mechanism'], 'finding mechanism mismatch')
        for ref in finding['anchor_refs']:
            require(get(anchors,ref)['execution_ref'] == e['execution_ref'], 'finding anchor execution mismatch')
        location = finding['location']
        if 'character_offsets' in location and 'byte_offsets' in location:
            segments = [t for t in r['evidence']['texts'] if t['source'] == location['source']]
            require(len(segments) == 1, 'ambiguous finding source')
            t = segments[0]
            scalars, byte_offsets = location['character_offsets'], location['byte_offsets']
            require(len(scalars) == len(byte_offsets), 'finding coordinate count mismatch')
            require(all(pos < len(t['text']) and t['byte_offsets'][pos] == off for pos,off in zip(scalars,byte_offsets)), 'finding coordinate mismatch')
        if finding['id'] == 'parser.failure':
            require(e['state'] == 'failed' and any(get(diagnostics,x)['code'] == finding['evidence']['failure_code'] for x in e['diagnostic_refs']), 'failure code mismatch')
        else:
            require(e['state'] in ('completed','partial'), 'finding from unrun operation')
    for diagnostic in diagnostics.values():
        if diagnostic['execution_ref'] is not None:
            get(executions,diagnostic['execution_ref'])
        if diagnostic['scope'] is not None:
            intervals(diagnostic['scope'])
    view_categories = {
        'audit': [],
        'unicode': ['possible_steganography', 'unicode'],
        'metadata': ['identifier', 'metadata', 'provenance'],
        'structure': ['document_structure', 'embedded_content', 'hidden_content', 'visual_watermark'],
    }
    require(r['view']['finding_categories'] == view_categories[r['view']['name']], 'view category mismatch')
    require(r['view']['name'] == 'audit' or all(f['category'] == 'parser' or f['category'] in r['view']['finding_categories'] for f in r['findings']), 'finding outside view')
    counts = {key:sum(f['severity'].lower() == key for f in r['findings']) for key in ('high','medium','low','info')}
    require(all(r['summary'][key] == value for key,value in counts.items()) and r['summary']['findings'] == len(r['findings']), 'summary count mismatch')
    active = [e for e in executions.values() if get(caps,e['capability_ref'])['participation'] != 'disabled']
    usable = bool(r['results'])
    if any(e['state'] == 'failed' and get(caps,e['capability_ref'])['role'] in ('acquisition','parser') for e in active):
        expected = 'failed'
    elif any(e['state'] == 'partial' for e in active):
        expected = 'partial'
    elif any(e['state'] == 'canceled' for e in active):
        expected = 'partial' if usable else 'canceled'
    elif any(e['state'] != 'completed' for e in active):
        optional_failure = any(e['state'] != 'completed' and get(caps,e['capability_ref'])['participation'] == 'optional' for e in active)
        expected = 'partial' if usable or optional_failure else 'failed'
    else:
        expected = 'completed'
    require(r['status'] == expected, 'aggregate status mismatch')
    exit_code = 4 if expected != 'completed' else next((n for key,n in [('high',3),('medium',2),('low',1),('info',1)] if counts[key]),0)
    require(r['summary']['exit_code'] == exit_code, 'exit code mismatch')
