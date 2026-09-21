"""Generate synthetic full wire reports; no detector/provider is invoked."""
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path
import unicodedata

from v3_support import identity, import_report

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
BLOBS = {}
PROTOCOL = 'aletharsis.adapter-exchange/1'
CATALOG = json.loads((HERE / 'catalog.json').read_bytes())


def encoded(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode()


def digest(raw):
    BLOBS[sha256(raw).hexdigest()] = raw
    return {'sha256': sha256(raw).hexdigest(), 'byte_length': len(raw)}


def unavailable():
    return {'quality':'unavailable', 'reason_code':'mapping.not_applicable'}


def run_record(e):
    return {'execution_ref':e['execution_ref'], 'protocol':PROTOCOL,
            'request_id':None, 'request_ref':None, 'response_ref':None, 'raw_result_ref':None,
            'quarantined_refs':[], 'launch_profile':None, 'started':False, 'cleanup':'not_started'}


def native(name):
    r = json.loads((ROOT / 'tests/contracts/fixtures' / (name + '.json')).read_bytes())
    r['schema_version'] = '3.0'
    r['catalog_version'] = CATALOG['catalog_version']
    r['adapter_runs'] = [run_record(e) for e in r['executions'] if e['capability_ref'] in CATALOG['adapters']]
    return r


def add_artifact(r, kind, raw, parents=(), operation='fixture.copy', config=None, mapping=None):
    ref = 'artifact/' + str(len(r['artifacts']))
    ident = digest(raw)
    a = {'artifact_ref':ref, 'kind':kind,
         'representation':{'serialization':'exact-bytes/1', 'encoding':None,
                           'normalization':'none', 'line_endings':'preserved'},
         **ident, 'content_ref':{'kind':'retained_blob', 'sha256':ident['sha256']},
         'unavailable_reason':None, 'parents':list(parents),
         'transform':{'operation':operation, 'version':'1', 'config_sha256':config,
                      'inputs':list(parents), 'exclusions':[]} if parents else None,
         'mapping':mapping or unavailable()}
    r['artifacts'].append(a)
    return a


def recalculate(r):
    caps = {c['id']:c for c in r['capabilities']}
    active = [e for e in r['executions'] if caps[e['capability_ref']]['participation'] != 'disabled']
    if any(e['state'] == 'failed' and caps[e['capability_ref']]['role'] in ('acquisition','parser') for e in active):
        status = 'failed'
    elif any(e['state'] == 'partial' for e in active):
        status = 'partial'
    elif any(e['state'] == 'canceled' for e in active):
        status = 'partial' if r['results'] else 'canceled'
    elif any(e['state'] != 'completed' for e in active):
        status = 'partial' if r['results'] or any(caps[e['capability_ref']]['participation'] == 'optional' and e['state'] != 'completed' for e in active) else 'failed'
    else:
        status = 'completed'
    r['status'] = status
    r['summary']['exit_code'] = 4 if status != 'completed' else next((n for k,n in [('high',3),('medium',2),('low',1),('info',1)] if r['summary'][k]),0)


def synthetic(name, normalized=False, variant=None):
    da = json.loads((ROOT / 'tests/adapters/fixtures' / (name + '.json')).read_bytes())
    r = native('clean-unavailable')
    for e in r['executions']:
        if e['capability_ref'] in CATALOG['adapters']:
            e['reason_code'] = 'execution.disabled'
    for c in r['capabilities']:
        if c['id'] in CATALOG['adapters']:
            c['participation'] = 'disabled'
    source_bytes = bytes.fromhex(da['blobs'][da['request']['source']['sha256']])
    if variant == 'empty':
        source_bytes = b''
    text = source_bytes.decode()
    r['file'].update(sha256=sha256(source_bytes).hexdigest(), size=len(source_bytes),
                     path='synthetic/adapter.txt', filename='adapter.txt')
    t = r['evidence']['texts'][0]
    offsets = [0]
    for char in text:
        offsets.append(offsets[-1] + len(char.encode()))
    t.update(text=text, byte_offsets=offsets, bom=None, encoding='utf-8')
    t['hashes'] = {k:sha256(source_bytes).hexdigest() for k in t['hashes']}
    t['hashes']['nfc_text_sha256'] = sha256(unicodedata.normalize('NFC',text).encode()).hexdigest()
    t['hashes']['nfkc_text_sha256'] = sha256(unicodedata.normalize('NFKC',text).encode()).hexdigest()
    t['hashes']['formatting_removed_sha256'] = sha256(text.replace('\ufeff','').replace('\u200b','').encode()).hexdigest()
    t['normalization'].update(nfc_changed=unicodedata.normalize('NFC',text)!=text,
        nfkc_changed=unicodedata.normalize('NFKC',text)!=text,formatting_removed_count=text.count('\ufeff')+text.count('\u200b'))
    t['line_endings'] = {'cr':0,'crlf':text.count('\r\n'),'lf':0}
    r['evidence']['structure']['byte_length'] = len(source_bytes)
    for a in r['artifacts']:
        a.update(digest(source_bytes))
    r['artifacts'][0].update(content_ref={'kind':'retained_blob','sha256':sha256(source_bytes).hexdigest()}, unavailable_reason=None)
    r['results'][0]['payload']['outcome'] = 'observations_present'
    r['limitations'].append('Synthetic contract evidence; no actual adapter, signature or statistical detector executed.')
    op = da['request']['operation']
    cap_id = 'fixture.adapter.' + op
    _, role, mechanism, _ = CATALOG['adapters'][cap_id]
    cap = deepcopy(r['capabilities'][0])
    request = deepcopy(da['request'])
    request['source'] = digest(source_bytes)
    cap.update(id=cap_id, revision='1', role=role, mechanism=mechanism,
               purpose='watermark_detection' if op == 'statistical' else None, participation='optional',
               availability={'state':'available','reason_code':None},
               implementation={'id':request['adapter']['id'], 'version':request['adapter']['version'],
                    'upstream':{'repository':'https://example.invalid/synthetic', 'revision':request['adapter']['upstream_commit'], 'sha256':None},
                    'data':[{'id':'executable','version':'fixture-1','sha256':request['adapter']['executable_sha256']}]})
    limits = request['limits']
    cap['limits'].update(input_bytes=limits['input_bytes'], output_bytes=limits['output_bytes'], execution_ms=limits['wall_ms'])
    e = {'execution_ref':'exec/'+str(len(r['executions'])), 'capability_ref':cap_id,
         'config':None, 'requested_scope':{'artifact_ref':'artifact/0','unit':'whole_artifact'},
         'analyzed_scope':[], 'exclusions':[], 'state':da['outcome']['state'],
         'reason_code':None, 'diagnostic_refs':[]}
    run = run_record(e)
    r['capabilities'].append(cap)
    r['capabilities'].sort(key=lambda c:c['id'])
    r['executions'].append(e)
    r['adapter_runs'].append(run)
    input_art = r['artifacts'][0]
    request['transforms'] = []
    if op == 'statistical' or normalized:
        input_bytes = unicodedata.normalize('NFC',text).encode() if normalized else source_bytes
        mapping = {'quality':'derived' if normalized else 'exact', 'from_artifact_ref':'artifact/0',
                   'to_artifact_ref':'artifact/'+str(len(r['artifacts'])), 'method':'aletharsis.byte-map',
                   'version':'1', 'pairs':[{'source':{'start':0,'end':len(source_bytes)},
                                           'target':{'start':0,'end':len(input_bytes)}}], 'exclusions':[]}
        input_art = add_artifact(r, 'text' if normalized else 'sample', input_bytes, ['artifact/0'],
                                 'unicode.nfc' if normalized else 'sample.identity', mapping=mapping)
        if normalized:
            input_art['representation'].update(serialization='unicode-scalars-utf8/1',encoding='utf-8',normalization='NFC')
        request['input_kind'] = 'normalized' if normalized else 'sample'
        request['transforms'] = [{'input':request['source'], 'output':digest(input_bytes),
                                 'operation':input_art['transform']['operation'], 'version':'1',
                                 'quality':mapping['quality'], 'pairs':mapping['pairs'], 'exclusions':[]}]
    input_ref = input_art['artifact_ref']
    request['input'] = {'sha256':input_art['sha256'], 'byte_length':input_art['byte_length']}
    e['requested_scope']['artifact_ref'] = input_ref
    cfg_art = add_artifact(r,'configuration',bytes.fromhex(da['blobs'][request['config']['sha256']]))
    policy = add_artifact(r,'policy',bytes.fromhex(da['blobs'][request['policy']['sha256']]))
    settings = {'protocol':PROTOCOL,'operation':op,'provider_config_ref':cfg_art['artifact_ref'],
                'policy_ref':policy['artifact_ref'],'trust_ref':None,
                'validation_time':request['validation_time'],'limits':limits}
    if variant == 'trusted':
        trust = add_artifact(r,'trust_material',b'synthetic trust snapshot')
        settings['trust_ref'] = trust['artifact_ref']
        request['trust'] = {'sha256':trust['sha256'], 'byte_length':trust['byte_length']}
    config = {'schema':'aletharsis.adapter-config/1','revision':'1','settings':settings,'key_ref':None}
    config['sha256'] = identity('aletharsis.config/1',config)
    e['config'] = config
    reason = da['outcome']['reason']
    reasons = {'partial':'adapter.partial_scope','timeout':'execution.timeout','malformed_output':'adapter.malformed_output',
               'oversized_output':'adapter.output_limit','source_mismatch':'adapter.source_mismatch',
               'provider_failure':'adapter.provider_failure','cleanup_failed':'adapter.cleanup_failed',
               'canceled':'execution.canceled','unavailable':'capability.runtime_missing','none':None}
    e['reason_code'] = reasons[reason]
    if reason == 'unavailable':
        cap['availability'] = {'state':'unavailable','reason_code':e['reason_code']}
        cap['implementation'] = None
    else:
        request['request_id'] = identity('aletharsis.adapter-request/1',{k:v for k,v in request.items() if k!='request_id'})
        req_art = add_artifact(r,'adapter_request',encoded(request))
        run.update(request_id=request['request_id'],request_ref=req_art['artifact_ref'])
        run['started'] = reason != 'source_mismatch'
        run['cleanup'] = 'failed' if reason == 'cleanup_failed' else 'complete' if run['started'] else 'not_started'
        if run['started']:
            run['launch_profile'] = {'id':'fixture.posix-worker','version':'1','policy_ref':policy['artifact_ref']}
    if e['state'] in ('failed','partial','canceled'):
        diag_ref = 'diagnostic/'+str(len(r['diagnostics']))
        r['diagnostics'].append({'diagnostic_ref':diag_ref,'execution_ref':e['execution_ref'],
            'stage':'adapter','code':e['reason_code'],'message':'Synthetic host outcome.',
            'scope':e['requested_scope'],'details':{'protocol':PROTOCOL,
                'primary_code':'execution.timeout' if reason=='cleanup_failed' else None,
                'cleanup':run['cleanup'],'limit':None,'limit_value':None}})
        e['diagnostic_refs'].append(diag_ref)
    if da['response'] is not None:
        response = deepcopy(da['response'])
        response.update(request_id=request['request_id'],input_sha256=input_art['sha256'])
        if normalized:
            response['analyzed'] = [{'start':0,'end':input_art['byte_length']}]
            response['results'][0]['spans'] = [{'start':5,'end':8}]
        if op == 'statistical':
            response['results'][0]['sample_sha256'] = input_art['sha256']
        e['analyzed_scope'] = [deepcopy(e['requested_scope'])] if e['state']=='completed' else [
            {'artifact_ref':input_ref,'unit':'byte','regions':response['analyzed']}]
        e['exclusions'] = [{'scope':{'artifact_ref':input_ref,'unit':'byte','regions':response['excluded']},
                            'unknown_remainder':False,'reason_code':'adapter.partial_scope'}] if response['excluded'] else []
        provider = response['results'][0]
        if variant in ('absent','malformed','ambiguous','empty'):
            provider['outcome'] = 'absent' if variant=='empty' else variant
            if variant in ('absent','empty'):
                provider['spans'] = []
            if variant == 'empty':
                response['analyzed'] = []
        if variant == 'manifest':
            manifest_bytes = b'synthetic extracted manifest'
            provider['manifest'] = digest(manifest_bytes)
            da['blobs'][provider['manifest']['sha256']] = manifest_bytes.hex()
        if variant == 'trusted':
            provider['trust'] = 'accepted'
        if variant == 'large-score':
            provider['score']['value'] = 1e100
        manifest = None
        if provider.get('manifest'):
            manifest = add_artifact(r,'manifest',bytes.fromhex(da['blobs'][provider['manifest']['sha256']]),
                                    [input_ref],cap_id,config['sha256'])
        raw_result = add_artifact(r,'detector_response',bytes.fromhex(da['blobs'][response['raw_provider_result']['sha256']]),
                                 [input_ref],cap_id,config['sha256'])
        resp_art = add_artifact(r,'adapter_response',encoded(response),[input_ref],cap_id,config['sha256'])
        run.update(response_ref=resp_art['artifact_ref'],raw_result_ref=raw_result['artifact_ref'])
        payload = {'operation':cap_id}
        kind = {'extract':'carrier_extraction','verify':'credential_verification','statistical':'statistical_analysis'}[op]
        if op == 'extract':
            payload.update(scopes=e['analyzed_scope'],outcome=provider['outcome'],input_ref=input_ref,
                           spans=provider['spans'],manifest_ref=manifest['artifact_ref'] if manifest else None)
        elif op == 'verify':
            payload.update(scope=e['requested_scope'],input_ref=input_ref,
                           manifest_ref=manifest['artifact_ref'] if manifest else None,
                           **{k:provider[k] for k in ('discovery','signature','binding','trust','revocation','freshness')})
        else:
            payload.update(scope=e['requested_scope'],sample_ref=input_ref,
                           **{k:provider[k] for k in ('algorithm','algorithm_version','configuration_sha256','outcome','score')})
        anchor_refs = []
        if op=='extract' and payload['spans'] or op=='verify' and manifest or op=='statistical':
            anchor_ref = 'anchor/'+str(len(r['anchors']))
            if op == 'extract':
                anchor_kind, locator, artifact_ref = 'artifact_bytes', {'version':'1','spans':payload['spans']}, input_ref
            elif op == 'verify':
                anchor_kind, locator, artifact_ref = 'credential', {'version':'1','manifest_ref':manifest['artifact_ref'],
                    'assertion_id':None,'verification_execution_ref':e['execution_ref']}, manifest['artifact_ref']
            else:
                anchor_kind, locator, artifact_ref = 'statistical_sample', {'version':'1','sample_ref':input_ref,
                    'scope':e['requested_scope']}, input_ref
            r['anchors'].append({'anchor_ref':anchor_ref,'kind':anchor_kind,'artifact_ref':artifact_ref,
                                 'execution_ref':e['execution_ref'],'mapping':unavailable(),'locator':locator})
            anchor_refs.append(anchor_ref)
        r['results'].append({'result_ref':'result/'+str(len(r['results'])),'execution_ref':e['execution_ref'],
            'kind':kind,'contract_version':'1','anchor_refs':anchor_refs,'payload':payload,'limitations':response['limitations']})
    if reason == 'malformed_output':
        quarantined = add_artifact(r,'detector_response',b'{malformed', [input_ref],cap_id,config['sha256'])
        run['quarantined_refs'].append(quarantined['artifact_ref'])
    if variant == 'filtered':
        r['view'] = {'name':'metadata','finding_categories':['identifier','metadata','provenance']}
    recalculate(r)
    return r


def write(name, report):
    raw = json.dumps(report,indent=2,ensure_ascii=False).encode()+b'\n'
    import_report(raw, BLOBS)
    (HERE / 'fixtures' / (name+'.json')).write_bytes(raw)


def main():
    for path in sorted((ROOT / 'tests/contracts/fixtures').glob('*.json')):
        write('native-'+path.stem, native(path.stem))
    pure = native('structural-observation')
    pure['capabilities'] = [c for c in pure['capabilities'] if c['id'] in CATALOG['native']]
    pure['executions'] = [e for e in pure['executions'] if e['capability_ref'] in CATALOG['native']]
    pure['adapter_runs'] = []
    write('native-only', pure)
    for path in sorted((ROOT / 'tests/adapters/fixtures').glob('*.json')):
        write('adapter-'+path.stem,synthetic(path.stem))
    write('adapter-normalized',synthetic('extracted',normalized=True))
    for variant in ('absent','malformed','ambiguous','manifest','empty','filtered'):
        write('carrier-'+variant,synthetic('extracted',variant=variant))
    write('verification-trusted',synthetic('verification',variant='trusted'))
    write('statistical-large-score',synthetic('statistical',variant='large-score'))
    for digest_name, raw in BLOBS.items():
        (HERE / 'blobs' / digest_name).write_bytes(raw)


if __name__ == '__main__':
    main()
