"""Materialize independently specified invalid full reports for cross-language parity."""
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent


def mutate(report, path, value):
    target = report
    for key in path[:-1]:
        target = target[key]
    target[path[-1]] = value


def build():
    # Fixtures are explicit negative claims, not majority/provider-vote expectations.
    cases = [
        ('unknown-member','adapter-extracted',['adapter_runs',2,'command'],'run me','schema',None),
        ('missing-run','adapter-extracted',['adapter_runs'],[], 'semantic','adapter run planning'),
        ('source-hash','adapter-extracted',['file','sha256'],'0'*64,'semantic','source identity mismatch'),
        ('request-digest','adapter-extracted',['adapter_runs',2,'request_id'],'0'*64,'semantic','request digest mismatch'),
        ('response-as-input','adapter-extracted',['results',1,'payload','input_ref'],'artifact/4','semantic','result input mismatch'),
        ('partial-overlap','adapter-partial',['executions',5,'exclusions',0,'scope','regions',0,'start'],8,'semantic','analyzed/excluded overlap'),
        ('unchecked-span','adapter-partial',['results',1,'payload','spans',0,'end'],10,'semantic','carrier outside checked scope'),
        ('fabricated-trust','adapter-verification',['results',1,'payload','trust'],'accepted','semantic','trust without material'),
        ('altered-verdict','adapter-verification',['results',1,'payload','signature'],'invalid','semantic','provider result promotion mismatch'),
        ('false-nfc-map','adapter-normalized',['artifacts',2,'mapping','quality'],'exact','semantic','exact map width mismatch'),
        ('dangling-raw','adapter-extracted',['adapter_runs',2,'raw_result_ref'],'artifact/999','semantic','dangling artifact'),
        ('unstarted-completed','adapter-extracted',['adapter_runs',2,'started'],False,'schema',None),
        ('statistical-config','adapter-statistical',['results',1,'payload','configuration_sha256'],'0'*64,'semantic','provider configuration identity mismatch'),
        ('lost-limitations','adapter-verification',['results',1,'limitations'],[],'semantic','provider limitation lost'),
        ('missing-primary-cause','adapter-cleanup_failed',['diagnostics',0,'details','primary_code'],None,'semantic','missing primary cleanup cause'),
        ('bad-byte-offset','native-structural-observation',['evidence','texts',0,'byte_offsets',1],2,'semantic','nonmonotonic byte offsets'),
    ]
    manifest = []
    for name, source, path, value, stage, reason in cases:
        report = json.loads((HERE / 'fixtures' / (source+'.json')).read_bytes())
        mutate(report,path,value)
        (HERE / 'invalid' / (name+'.json')).write_bytes((json.dumps(report,indent=2,ensure_ascii=False)+'\n').encode('utf-8'))
        manifest.append({'file':name+'.json','stage':stage,'reason':reason})
    (HERE / 'invalid-cases.json').write_bytes((json.dumps(manifest,indent=2)+'\n').encode('utf-8'))


if __name__ == '__main__':
    build()
