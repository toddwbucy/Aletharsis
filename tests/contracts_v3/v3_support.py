"""Report-3.0 offline conformance oracle; no production importer or provider execution."""
from hashlib import sha256
import json
from pathlib import Path
import re
import math
import importlib.util

from jsonschema import Draft202012Validator, FormatChecker

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
    def number(token):
        value = int(token)
        # Generic JSON/JCS numbers use binary64; bounded integer fields are
        # independently constrained by the wire schema.
        return value if abs(value) <= 9007199254740991 else float(token)
    try:
        value = json.loads(raw.decode('utf-8'), object_pairs_hook=pairs,
                           parse_int=number, parse_constant=constant)
    except RecursionError:
        raise ValueError('report depth exceeded') from None
    def visit(item, depth):
        require(depth <= MAX_DEPTH, 'report depth exceeded')
        if isinstance(item, str):
            item.encode('utf-8', errors='strict')
        elif isinstance(item, (int, float)) and not isinstance(item, bool):
            require(not isinstance(item, float) or math.isfinite(item), 'non-finite JSON number')
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
    name = {'1.0': 'report.schema.json', '2.0': 'report-v2.schema.json', '3.0': 'report-v3.schema.json'}[version]
    return json.loads((ROOT / 'schemas' / name).read_bytes())


def import_report(raw, blobs=None):
    """Retain exact bytes; optional blobs are caller-supplied bytes, never paths."""
    report = strict_json(raw)
    require(isinstance(report, dict), 'report must be an object')
    if report.get('schema_version') != '3.0':
        spec = importlib.util.spec_from_file_location('v2_oracle', ROOT / 'tests/contracts/support.py')
        legacy = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(legacy)
        return legacy.import_report(raw)
    Draft202012Validator(schema('3.0'), format_checker=FormatChecker()).validate(report)
    validate_semantics(report, blobs)
    retained = [a for a in report['artifacts'] if a['content_ref'] is not None
                and a['content_ref']['kind'] == 'retained_blob']
    unresolved = [a['artifact_ref'] for a in retained if blobs is None or a['sha256'] not in blobs]
    return {'report_artifact_sha256': sha256(raw).hexdigest(), 'status': 'validated',
            'coverage': 'declared', 'bytes': bytes(raw), 'report': report,
            'blob_checks': 'supplied_bytes_checked' if blobs is not None else 'not_requested',
            'unresolved_blob_refs': unresolved, 'source_authority': 'not_granted_by_import'}


def validate_semantics(r, blobs=None):
    """Executable minimum consumer checks; wire shape is validated first."""
    def index(collection, field):
        records = {x[field]: x for x in r[collection]}
        require(len(records) == len(r[collection]), 'duplicate reference')
        return records
    catalog = json.loads((ROOT / 'tests/contracts_v3/catalog.json').read_bytes())
    require([x['execution_ref'] for x in r['adapter_runs']] ==
            [e['execution_ref'] for e in r['executions'] if e['capability_ref'] in catalog['adapters']],
            'adapter run planning/order mismatch')
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
            require(re.fullmatch(r'0|[1-9][0-9]*', parts[3]) is not None,
                    'invalid content pointer')
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
            if m.get('data_ref') is not None:
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
        if result['kind'] != 'structural_scan':
            continue  # Closed adapter variants are checked below.
        e = get(executions,result['execution_ref'])
        cap = get(caps,e['capability_ref'])
        require(e['state'] in ('completed','partial') and cap['mechanism'] == 'structural' and cap['role'] == 'analyzer', 'result from ineligible execution')
        require(result['payload']['operation'] == cap['id'], 'result operation mismatch')
        require(result['payload']['scope'] in e['analyzed_scope'], 'result outside analyzed scope')
        for ref in result['anchor_refs']:
            require(get(anchors,ref)['execution_ref'] == e['execution_ref'], 'result anchor execution mismatch')
    for e in executions.values():
        cap = get(caps,e['capability_ref'])
        if e['execution_ref'] not in {run['execution_ref'] for run in r['adapter_runs']} and e['state'] in ('completed','partial') and cap['role'] == 'analyzer' and cap['mechanism'] == 'structural':
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

    validate_adapters(r, blobs, artifacts, executions, caps, anchors, diagnostics, intervals)


def validate_adapters(r, blobs, artifacts, executions, caps, anchors, diagnostics, intervals):
    """Validate host-owned joins before optionally checking retained byte evidence."""
    if blobs is not None:
        require(isinstance(blobs, dict) and all(isinstance(v, bytes) for v in blobs.values()), 'invalid blob store')
        require(sum(len(v) for v in blobs.values()) <= 64 * 1024 * 1024, 'blob budget exceeded')
        require(all(len(v) <= 32 * 1024 * 1024 for v in blobs.values()), 'blob item budget exceeded')
    catalog = json.loads((ROOT / 'tests/contracts_v3/catalog.json').read_bytes())
    require(r['catalog_version'] == catalog['catalog_version'], 'unsupported catalog')
    operations = catalog['adapters']
    require(set(caps) <= set(catalog['native']) | set(operations), 'unregistered capability')
    for name, cap in caps.items():
        expected = catalog['native'].get(name) or operations[name][1:3]
        require([cap['role'], cap['mechanism']] == expected, 'catalog role mismatch')
        if name in operations and operations[name][3] == 'declaration_only':
            require(cap['availability']['state'] != 'available' and cap['implementation'] is None,
                    'declaration cannot advertise implementation')
    run_ids = [run['execution_ref'] for run in r['adapter_runs']]
    planned = [e['execution_ref'] for e in r['executions'] if e['capability_ref'] in operations]
    require(run_ids == planned, 'adapter run planning/order mismatch')
    results_by_exec = {key: [x for x in r['results'] if x['execution_ref'] == key] for key in executions}
    require(all(x['execution_ref'] in executions for x in r['results']), 'dangling result execution')
    require(all(x['execution_ref'] not in run_ids for x in r['findings']), 'adapter findings not contracted')

    def get(ref, kinds=None, retained=False):
        require(ref in artifacts, 'dangling artifact reference')
        a = artifacts[ref]
        require(kinds is None or a['kind'] in kinds, 'wrong artifact kind')
        if retained:
            require(a['sha256'] is not None and a['content_ref'] is not None
                    and a['content_ref']['kind'] == 'retained_blob', 'retained identity required')
        return a

    def raw(ref):
        a = get(ref)
        if a['content_ref'] is not None and a['content_ref']['kind'] == 'report_pointer':
            index = int(a['content_ref']['pointer'].split('/')[3])
            return r['evidence']['texts'][index]['text'].encode('utf-8')
        if blobs is None or a['sha256'] not in blobs:
            return None
        value = blobs[a['sha256']]
        require(isinstance(value, bytes), 'blob must be bytes')
        require(len(value) == a['byte_length'] and sha256(value).hexdigest() == a['sha256'], 'blob identity mismatch')
        return value

    def identity_of(ref):
        a = get(ref)
        return {'sha256': a['sha256'], 'byte_length': a['byte_length']}

    def ranges(spans, length):
        previous = 0
        for span in spans:
            require(previous <= span['start'] < span['end'] <= length, 'invalid byte spans')
            previous = span['end']
        return [(s['start'], s['end']) for s in spans]

    def mapping(m):
        if m.get('method') != 'aletharsis.byte-map':
            return
        src, dst = get(m['from_artifact_ref']), get(m['to_artifact_ref'])
        require(src['byte_length'] is not None and dst['byte_length'] is not None, 'mapping extent unavailable')
        exclusions = ranges(m['exclusions'], src['byte_length'])
        parent, child = raw(src['artifact_ref']), raw(dst['artifact_ref'])
        end = 0
        for pair in m['pairs']:
            a, b = pair['source'], pair['target']
            require(0 <= a['start'] < a['end'] <= src['byte_length'], 'source map outside artifact')
            require(end == b['start'] < b['end'] <= dst['byte_length'], 'target map gap/overlap')
            require(all(a['end'] <= lo or hi <= a['start'] for lo,hi in exclusions), 'map crosses exclusion')
            end = b['end']
            if m['quality'] == 'exact':
                require(a['end'] - a['start'] == b['end'] - b['start'], 'exact map width mismatch')
                if parent is not None and child is not None:
                    require(parent[a['start']:a['end']] == child[b['start']:b['end']], 'false exact byte map')
        require(end == dst['byte_length'], 'incomplete byte map')

    for a in artifacts.values():
        raw(a['artifact_ref'])
        require((not a['parents']) == (a['transform'] is None), 'artifact parent/transform mismatch')
        mapping(a['mapping'])
        if a['mapping'].get('method') == 'aletharsis.byte-map':
            require(a['mapping']['to_artifact_ref'] == a['artifact_ref']
                    and a['mapping']['from_artifact_ref'] in a['parents'], 'detached artifact byte map')
    for a in anchors.values():
        mapping(a['mapping'])
        if a['kind'] == 'artifact_bytes':
            artifact = get(a['artifact_ref'])
            require(artifact['byte_length'] is not None, 'anchor extent unavailable')
            ranges(a['locator']['spans'], artifact['byte_length'])
            require(a['execution_ref'] in run_ids, 'byte anchor on native execution')

    source_ref = next(a['artifact_ref'] for a in artifacts.values() if a['kind'] == 'source')
    def descends(ref, ancestor):
        pending, seen = [ref], set()
        while pending:
            current = pending.pop()
            if current == ancestor:
                return True
            if current not in seen:
                seen.add(current)
                pending.extend(get(current)['parents'])
        return False

    quarantined = {ref for run in r['adapter_runs'] for ref in run['quarantined_refs']}
    for run in r['adapter_runs']:
        e = executions[run['execution_ref']]
        cap = caps[e['capability_ref']]
        operation = operations[e['capability_ref']][0]
        results = results_by_exec[e['execution_ref']]
        usable = e['state'] in ('completed', 'partial')
        require(len(results) == (1 if usable else 0), 'adapter aggregate result count')
        require((run['request_ref'] is None) == (run['request_id'] is None), 'incomplete request identity')
        require(run['started'] or e['state'] in ('not_run', 'failed'), 'unstarted usable/canceled execution')
        require(not run['started'] or e['state'] != 'not_run', 'not-run worker started')
        if run['cleanup'] == 'failed':
            require(e['state'] == 'failed' and e['reason_code'] == 'adapter.cleanup_failed', 'cleanup failure mismatch')
        require((e['reason_code'] == 'adapter.cleanup_failed') == (run['cleanup'] == 'failed'), 'cleanup reason mismatch')
        for ref in run['quarantined_refs']:
            get(ref, ['detector_response'], True)
        require(run['response_ref'] not in quarantined and run['raw_result_ref'] not in quarantined,
                'quarantined evidence promoted')
        config = e['config']
        settings = config['settings']
        if run['request_ref'] is not None:
            request_art = get(run['request_ref'], ['adapter_request'], True)
            require(request_art['byte_length'] <= 65536, 'request header budget exceeded')
            require(config['schema'] == 'aletharsis.adapter-config/1', 'adapter configuration required')
        if config['schema'] == 'aletharsis.adapter-config/1':
            require(settings['operation'] == operation, 'configuration operation mismatch')
            for field, kind in [('provider_config_ref','configuration'), ('policy_ref','policy')]:
                get(settings[field], [kind], True)
            if settings['trust_ref'] is not None:
                get(settings['trust_ref'], ['trust_material'], True)
            for common, adapter in [('input_bytes','input_bytes'), ('output_bytes','output_bytes'), ('execution_ms','wall_ms')]:
                require(cap['limits'][common] == settings['limits'][adapter], 'conflicting adapter limit')
        else:
            require(not usable and not run['started'] and config['schema'] is None, 'invalid adapter configuration')
        if run['launch_profile'] is not None:
            require(settings is not None and run['launch_profile']['policy_ref'] == settings['policy_ref'], 'launch policy mismatch')
        input_ref = e['requested_scope']['artifact_ref']
        input_art = get(input_ref, ['source','text','binding_text','sample'])
        require(descends(input_ref, source_ref), 'input not derived from source')
        require(e['requested_scope']['unit'] == 'whole_artifact', 'adapter requires whole input request')
        if usable:
            require(run['started'] and run['cleanup'] == 'complete', 'usable run cleanup incomplete')
            get(run['response_ref'], ['adapter_response'], True)
            get(run['raw_result_ref'], ['detector_response'], True)
            require(input_art['sha256'] is not None, 'unidentified adapter input')
            require(input_art['byte_length'] <= settings['limits']['input_bytes'], 'input limit exceeded')
            for ref in (run['raw_result_ref'], run['response_ref']):
                a = get(ref)
                require(input_ref in a['parents'] and a['transform']['operation'] == cap['id']
                        and a['transform']['config_sha256'] == config['sha256'], 'response provenance mismatch')
            result = results[0]
            expected = {'extract':'carrier_extraction','verify':'credential_verification','statistical':'statistical_analysis'}[operation]
            require(result['kind'] == expected, 'adapter result kind mismatch')
            p = result['payload']
            retained_size = 4 + get(run['response_ref'])['byte_length'] + get(run['raw_result_ref'])['byte_length']
            if p.get('manifest_ref') is not None:
                manifest_art = get(p['manifest_ref'], ['manifest'], True)
                retained_size += manifest_art['byte_length']
            require(retained_size <= settings['limits']['output_bytes'], 'retained response exceeds output budget')
            require(p['operation'] == cap['id'], 'result operation mismatch')
            require((p.get('input_ref') or p.get('sample_ref')) == input_ref, 'result input mismatch')
            for ref in result['anchor_refs']:
                require(ref in anchors and anchors[ref]['execution_ref'] == e['execution_ref'], 'adapter anchor execution mismatch')
            if operation == 'extract':
                require(p['scopes'] == e['analyzed_scope'], 'extraction coverage mismatch')
                spans = ranges(p['spans'], input_art['byte_length'])
                checked = [span for scope in e['analyzed_scope'] for span in intervals(scope)]
                require(all(any(a <= start < end <= b for a,b in checked) for start,end in spans), 'carrier outside checked scope')
                require((p['outcome'] == 'absent') == (not spans), 'carrier outcome/spans mismatch')
                require(p['outcome'] == 'observed' or p['manifest_ref'] is None, 'manifest from unextracted carrier')
                if e['state'] == 'partial':
                    require(all(not x['unknown_remainder'] and x['reason_code'] == 'adapter.partial_scope' for x in e['exclusions']), 'adapter exclusions must be exact')
                for ref in result['anchor_refs']:
                    a = anchors[ref]
                    require(a['kind'] == 'artifact_bytes' and a['artifact_ref'] == input_ref
                            and a['locator']['spans'] == p['spans'], 'carrier anchor mismatch')
            else:
                require(e['state'] == 'completed' and p['scope'] == e['requested_scope'], 'whole-input result coverage mismatch')
            if operation in ('extract','verify') and p['manifest_ref'] is not None:
                get(p['manifest_ref'], ['manifest'], True)
                require(descends(p['manifest_ref'], input_ref), 'manifest provenance mismatch')
            if operation == 'verify':
                if p['discovery'] == 'absent':
                    require(p['manifest_ref'] is None and p['signature'] == 'not_checked'
                            and p['binding'] == 'not_checked' and p['trust'] == 'not_evaluated'
                            and p['revocation'] == 'not_checked' and p['freshness'] == 'unknown', 'absence with verification claims')
                elif p['manifest_ref'] is None:
                    require(result['limitations'], 'missing manifest limitation')
                require(p['trust'] == 'not_evaluated' or settings['trust_ref'] is not None, 'trust without material')
                for ref in result['anchor_refs']:
                    a = anchors[ref]
                    require(a['kind'] == 'credential' and a['locator']['manifest_ref'] == p['manifest_ref']
                            and a['locator']['verification_execution_ref'] == e['execution_ref'], 'credential anchor mismatch')
            if operation == 'statistical':
                require(input_art['kind'] == 'sample', 'statistical input is not sample')
                require(p['configuration_sha256'] == get(settings['provider_config_ref'])['sha256'], 'provider configuration identity mismatch')
                for ref in result['anchor_refs']:
                    a = anchors[ref]
                    require(a['kind'] == 'statistical_sample' and a['locator']['sample_ref'] == input_ref
                            and a['locator']['scope'] == p['scope'], 'sample anchor mismatch')
        else:
            require(run['response_ref'] is None and run['raw_result_ref'] is None, 'unusable response promoted')
        for d in diagnostics.values():
            if d['execution_ref'] != e['execution_ref'] or d['stage'] != 'adapter':
                continue
            details = d['details']
            require(details['cleanup'] == run['cleanup'], 'diagnostic cleanup mismatch')
            require((details['limit'] is None) == (details['limit_value'] is None), 'incomplete limit diagnostic')
            if details['limit'] is not None:
                require(settings is not None and details['limit_value'] == settings['limits'][details['limit']], 'diagnostic limit mismatch')
            if run['cleanup'] == 'failed':
                require(details['primary_code'] is not None and details['primary_code'] != 'adapter.cleanup_failed', 'missing primary cleanup cause')
        if e['state'] in ('failed','partial','canceled'):
            require(any(diagnostics[ref]['stage'] == 'adapter' and diagnostics[ref]['code'] == e['reason_code']
                        for ref in e['diagnostic_refs']), 'missing adapter cause diagnostic')
        if run['request_ref'] is not None:
            validate_exchange_bytes(run, e, cap, input_ref, settings, results, raw, get, identity_of, source_ref)
    for d in diagnostics.values():
        require(d['stage'] != 'adapter' or d['execution_ref'] in run_ids, 'adapter diagnostic on native execution')
    for e in executions.values():
        if e['execution_ref'] not in run_ids:
            require(all(result['kind'] == 'structural_scan' for result in results_by_exec[e['execution_ref']]),
                    'adapter result on native execution')
            require(e['config']['schema'] != 'aletharsis.adapter-config/1', 'adapter configuration on native execution')


def validate_exchange_bytes(run, execution, capability, input_ref, settings, results,
                            raw, get, identity_of, source_ref):
    """Cross-check retained DA-001 headers when caller supplies those exact bytes."""
    defs = json.loads((ROOT / 'schemas/adapter-exchange-v1.schema.json').read_bytes())['$defs']
    def decode(ref, shape, cap):
        data = raw(ref)
        if data is None:
            return None
        require(len(data) <= cap, 'exchange header budget exceeded')
        value = strict_json(data)
        Draft202012Validator({'$defs': defs, '$ref': '#/$defs/' + shape},
                             format_checker=FormatChecker()).validate(value)
        return value

    request = decode(run['request_ref'], 'request', 65536)
    if request is not None:
        require(request['request_id'] == run['request_id'] == identity('aletharsis.adapter-request/1',
                {k:v for k,v in request.items() if k != 'request_id'}), 'request digest mismatch')
        require(request['source'] == identity_of(source_ref) and request['input'] == identity_of(input_ref), 'request asset identity mismatch')
        kinds = {'source':'source','decoded':'text','normalized':'text','binding':'binding_text','sample':'sample'}
        require(get(input_ref)['kind'] == kinds[request['input_kind']], 'request representation mismatch')
        require(request['config'] == identity_of(settings['provider_config_ref'])
                and request['policy'] == identity_of(settings['policy_ref'])
                and request['trust'] == (identity_of(settings['trust_ref']) if settings['trust_ref'] else None),
                'request configuration identity mismatch')
        require(request['limits'] == settings['limits'] and request['validation_time'] == settings['validation_time']
                and request['operation'] == settings['operation'], 'request execution settings mismatch')
        impl = capability['implementation']
        require(impl is not None and impl['id'] == request['adapter']['id']
                and impl['version'] == request['adapter']['version'] and impl['upstream'] is not None
                and impl['upstream']['revision'] == request['adapter']['upstream_commit'], 'request implementation mismatch')
        require(any(d['id'] == 'executable' and d['sha256'] == request['adapter']['executable_sha256']
                    for d in impl['data']), 'executable identity mismatch')
        # Walk the actual chosen artifact chain backwards, not a digest-only lookup:
        # two graph nodes may intentionally have the same byte identity.
        current = input_ref
        for transform in reversed(request['transforms']):
            art = get(current)
            require(len(art['parents']) == 1 and art['transform'] is not None, 'ambiguous request derivation')
            parent = art['parents'][0]
            require(identity_of(current) == transform['output'] and identity_of(parent) == transform['input'], 'request transform identity mismatch')
            require(art['transform']['operation'] == transform['operation'] and art['transform']['version'] == transform['version'], 'request transform method mismatch')
            m = art['mapping']
            require(m['quality'] == transform['quality'], 'request mapping quality mismatch')
            if m['quality'] == 'unavailable':
                require(not transform['pairs'], 'unavailable request mapping has pairs')
            else:
                require(m.get('method') == 'aletharsis.byte-map' and m['pairs'] == transform['pairs']
                        and m['exclusions'] == transform['exclusions'], 'request map mismatch')
            declared_exclusions = [span for x in art['transform']['exclusions']
                                   for span in (x['scope'].get('regions', []) if x['scope'] else [])]
            require(declared_exclusions == transform['exclusions'], 'request transform exclusions mismatch')
            current = parent
        require(current == source_ref, 'request derivation not rooted in source')
        require((request['input_kind'] == 'source') == (not request['transforms']), 'request input kind/chain mismatch')
    if run['response_ref'] is None:
        return
    response = decode(run['response_ref'], 'response', settings['limits']['output_bytes'])
    if response is None:
        return
    require(response['request_id'] == run['request_id'] and response['input_sha256'] == get(input_ref)['sha256'], 'response input identity mismatch')
    require(response['state'] == execution['state'], 'response execution mismatch')
    length = get(input_ref)['byte_length']
    def spans(scopes):
        result = []
        for scope in scopes:
            if scope['unit'] == 'whole_artifact':
                result.extend([{'start':0, 'end':length}] if length else [])
            else:
                require(scope['unit'] == 'byte', 'adapter scope must use bytes')
                result.extend(scope['regions'])
        return result
    require(response['analyzed'] == spans(execution['analyzed_scope'])
            and response['excluded'] == spans([x['scope'] for x in execution['exclusions']]), 'response coverage mismatch')
    require(response['raw_provider_result'] == identity_of(run['raw_result_ref']), 'raw provider identity mismatch')
    require(len(response['results']) == len(results) == 1, 'response result count mismatch')
    p = results[0]['payload']
    operation = settings['operation']
    expected = {'kind': operation}
    if operation == 'extract':
        expected.update(outcome=p['outcome'], spans=p['spans'],
                        manifest=identity_of(p['manifest_ref']) if p['manifest_ref'] else None)
    elif operation == 'verify':
        expected.update({key:p[key] for key in ('discovery','signature','binding','trust','revocation','freshness')})
        expected['manifest'] = identity_of(p['manifest_ref']) if p['manifest_ref'] else None
    else:
        expected.update({key:p[key] for key in ('algorithm','algorithm_version','configuration_sha256','outcome','score')})
        expected['sample_sha256'] = get(input_ref)['sha256']
    require(response['results'][0] == expected, 'provider result promotion mismatch')
    require(all(x in results[0]['limitations'] for x in response['limitations']), 'provider limitation lost')
    size = 4 + get(run['response_ref'])['byte_length'] + get(run['raw_result_ref'])['byte_length']
    if p.get('manifest_ref'):
        size += get(p['manifest_ref'])['byte_length']
    require(size <= settings['limits']['output_bytes'], 'retained response exceeds output budget')
