"""Offline DA-001 conformance oracle; never invokes workers or opens blob paths."""
import json
from hashlib import sha256
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parents[2]
SCHEMA = json.loads((ROOT / 'schemas/adapter-exchange-v1.schema.json').read_bytes())
VALIDATOR = Draft202012Validator(SCHEMA, format_checker=FormatChecker())


def require(condition, message):
    if not condition:
        raise ValueError(message)


def request_digest(request):
    """JCS-equivalent request subset: fixed ASCII keys, strings and safe integers."""
    value = {k: v for k, v in request.items() if k != 'request_id'}
    raw = json.dumps({'domain': 'aletharsis.adapter-request/1', 'value': value},
                     sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()
    return sha256(raw).hexdigest()


def validate(record):
    """Validate a parsed synthetic transcript including retained exact-byte blobs."""
    VALIDATOR.validate(record)
    request, response, outcome = (record[k] for k in ('request', 'response', 'outcome'))
    require(request['request_id'] == request_digest(request), 'request identity mismatch')
    require(len(json.dumps(request, ensure_ascii=False, separators=(',', ':')).encode()) <= 65536,
            'request header budget exceeded')
    blobs = {}
    for digest, hex_bytes in record['blobs'].items():
        raw = bytes.fromhex(hex_bytes)
        require(sha256(raw).hexdigest() == digest, 'blob digest mismatch')
        blobs[digest] = raw

    def artifact(identity):
        require(identity['sha256'] in blobs, 'missing blob')
        raw = blobs[identity['sha256']]
        require(len(raw) == identity['byte_length'], 'blob length mismatch')
        return raw

    for name in ('source', 'input', 'config', 'policy'):
        artifact(request[name])
    if request['trust'] is not None:
        artifact(request['trust'])
    require(request['input']['byte_length'] <= request['limits']['input_bytes'], 'input budget exceeded')
    if request['input_kind'] == 'source':
        require(request['source'] == request['input'], 'source/input identity mismatch')
    previous_artifact = request['source']
    for transform in request['transforms']:
        require(transform['input'] == previous_artifact, 'broken derivation chain')
        parent, child = artifact(transform['input']), artifact(transform['output'])
        previous_artifact = transform['output']
        excluded = []
        end = 0
        for span in transform['exclusions']:
            require(end <= span['start'] < span['end'] <= len(parent), 'invalid exclusion')
            excluded.append((span['start'], span['end']))
            end = span['end']
        if transform['quality'] == 'unavailable':
            require(not transform['pairs'], 'unavailable mapping with pairs')
            continue
        end = 0
        for pair in transform['pairs']:
            src, dst = pair['source'], pair['target']
            require(0 <= src['start'] < src['end'] <= len(parent), 'invalid source mapping')
            require(dst['start'] == end < dst['end'] <= len(child), 'invalid target mapping')
            require(all(src['end'] <= a or b <= src['start'] for a,b in excluded), 'mapping crosses exclusion')
            end = dst['end']
            if transform['quality'] == 'exact':
                require(parent[src['start']:src['end']] == child[dst['start']:dst['end']],
                        'false exact mapping')
        require(end == len(child), 'incomplete target mapping')
    require(previous_artifact == request['input'], 'unbound input artifact')
    require((request['input_kind'] == 'source') == (not request['transforms']), 'input kind/derivation mismatch')
    state, reason = outcome['state'], outcome['reason']
    require((record['cleanup'] == 'failed') == (reason == 'cleanup_failed'), 'cleanup status mismatch')
    if state == 'not_run':
        require(record['cleanup'] == 'not_started', 'unavailable worker started')
    elif reason != 'cleanup_failed':
        require(record['cleanup'] == 'complete', 'cleanup incomplete')
    if state not in ('completed', 'partial'):
        require(response is None, 'failure cannot promote results')
        return
    require(response is not None, 'missing response')
    require(response['state'] == state, 'response state mismatch')
    require(response['request_id'] == request['request_id'], 'response request mismatch')
    require(response['input_sha256'] == request['input']['sha256'], 'response source mismatch')
    raw = artifact(response['raw_provider_result'])
    require(len(raw) <= request['limits']['output_bytes'], 'raw output budget exceeded')
    # Transport frame and stderr share this budget in the production host. The
    # fixture contains only the response JSON and raw provider bytes, no stderr.
    frame = json.dumps(response, ensure_ascii=False, separators=(',', ':')).encode()
    manifest_bytes = sum(len(artifact(result['manifest'])) for result in response['results']
                         if result.get('manifest') is not None)
    require(4 + len(frame) + len(raw) + manifest_bytes <= request['limits']['output_bytes'],
            'output budget exceeded')
    length = request['input']['byte_length']

    def ranges(items):
        previous = 0
        for span in items:
            require(previous <= span['start'] < span['end'] <= length, 'invalid span')
            previous = span['end']
        return [(s['start'], s['end']) for s in items]

    covered, excluded = ranges(response['analyzed']), ranges(response['excluded'])
    previous = 0
    for start, end in sorted(covered + excluded):
        require(start == previous, 'coverage gap or overlap')
        previous = end
    require(previous == length, 'unaccounted input')
    if state == 'completed':
        require(not excluded, 'completed with exclusions')
    else:
        require(covered and excluded and response['results'], 'partial without usable coverage')
    require(response['results'], 'completed response without outcome')
    require(len(response['results']) == 1, 'one aggregate result required')
    if request['operation'] != 'extract':
        require(state == 'completed', 'whole-input operation cannot be partial')
    for result in response['results']:
        require(result['kind'] == request['operation'], 'result operation mismatch')
        kind = result['kind']
        if kind == 'extract':
            spans = ranges(result['spans'])
            require(all(any(a <= start < end <= b for a, b in covered) for start, end in spans), 'result outside coverage')
            if result['outcome'] == 'absent':
                require(not spans and result['manifest'] is None, 'absence with evidence')
            else:
                require(spans, 'unlocalized carrier')
            if result['outcome'] != 'observed':
                require(result['manifest'] is None, 'manifest from unextracted carrier')
            if result['manifest'] is not None:
                artifact(result['manifest'])
        elif kind == 'verify':
            if result['manifest'] is not None:
                artifact(result['manifest'])
            if result['discovery'] == 'absent':
                require(result['manifest'] is None and result['signature'] == 'not_checked'
                        and result['binding'] == 'not_checked' and result['trust'] == 'not_evaluated'
                        and result['revocation'] == 'not_checked' and result['freshness'] == 'unknown',
                        'absence with verification claims')
            if result['trust'] != 'not_evaluated':
                require(request['trust'] is not None, 'trust without material')
        else:
            require(result['sample_sha256'] == request['input']['sha256'], 'statistical sample mismatch')
            require(result['configuration_sha256'] == request['config']['sha256'], 'statistical config mismatch')


def decode_frame(data, limit=65536):
    """Offline frame oracle: reject oversized headers before JSON decoding."""
    require(len(data) >= 4, 'truncated frame')
    size = int.from_bytes(data[:4], 'big')
    require(size <= limit, 'oversized frame')
    require(len(data) == 4 + size, 'truncated or trailing frame')
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, 'duplicate key')
            result[key] = value
        return result
    def constant(value):
        raise ValueError('non-finite number')
    def visit(value, depth=0):
        require(depth <= 32, 'depth exceeded')
        if isinstance(value, str):
            value.encode('utf-8', errors='strict')
        elif isinstance(value, (int, float)) and not isinstance(value, bool):
            require(-9007199254740991 <= value <= 9007199254740991, 'unsafe number')
        elif isinstance(value, dict):
            for key, item in value.items():
                visit(key, depth + 1)
                visit(item, depth + 1)
        elif isinstance(value, list):
            for item in value:
                visit(item, depth + 1)
    value = json.loads(data[4:].decode('utf-8'), object_pairs_hook=pairs,
                       parse_constant=constant)
    visit(value)
    return value
