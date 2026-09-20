"""Proposed v2 wire and import conformance; no production emitter is implied."""
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path

from jsonschema import Draft202012Validator, ValidationError
import pytest

from support import ROOT, identity, identity_bytes, import_report, schema, strict_json, validate_semantics

FIXTURES = Path(__file__).parent / 'fixtures'


def report(name='structural-observation'):
    return json.loads((FIXTURES / (name + '.json')).read_bytes())


def validate(r):
    Draft202012Validator(schema('2.0')).validate(r)
    validate_semantics(r)


FIXTURE_PATHS = sorted(FIXTURES.glob('*.json'))
assert len(FIXTURE_PATHS) == 8, 'Expected exactly eight complete wire fixtures'


@pytest.mark.parametrize('path', FIXTURE_PATHS, ids=lambda p:p.stem)
def test_complete_wire_fixtures(path):
    raw = path.read_bytes()
    imported = import_report(raw)
    assert imported['status'] == 'validated'
    assert imported['report_artifact_sha256'] == sha256(raw).hexdigest()
    assert imported['report'] == json.loads(raw)


@pytest.mark.parametrize('index', ['00', '01', '-1', '+0', ' 0', '0 ', '0\n', '\u0660', ''])
def test_noncanonical_content_pointer_index_rejected(index):
    r = report('clean-unavailable')
    r['artifacts'][1]['content_ref']['pointer'] = f'/evidence/texts/{index}/text'
    # Exercise the semantic guard even for forms already rejected by the schema.
    with pytest.raises(ValueError, match='^invalid content pointer$'):
        validate_semantics(r)


def test_canonical_nonzero_content_pointer_preserves_nested_lookup():
    r = report('clean-unavailable')
    r['evidence']['texts'].append(deepcopy(r['evidence']['texts'][0]))
    r['artifacts'][1]['content_ref']['pointer'] = '/evidence/texts/1/text'
    r['artifacts'][1]['mapping']['data_ref'] = '/evidence/texts/1/byte_offsets'
    validate(r)


def test_schema_itself_and_v1_still_rejects_v2():
    Draft202012Validator.check_schema(schema('2.0'))
    assert not Draft202012Validator(schema('1.0')).is_valid(report())


def objects(value, path=()):
    if isinstance(value, dict):
        yield path
        for k,v in value.items():
            yield from objects(v, (*path,k))
    elif isinstance(value,list):
        for i,v in enumerate(value):
            yield from objects(v, (*path,i))


def at(value, path):
    for key in path:
        value = value[key]
    return value


@pytest.mark.parametrize('name', ['structural-observation','decode-failed','partial-timeout'])
def test_fixed_objects_reject_unknown_fields(name):
    original = report(name)
    validator = Draft202012Validator(schema('2.0'))
    for path in objects(original):
        r = deepcopy(original)
        at(r,path)['surprise'] = 'untrusted data'
        assert not validator.is_valid(r), path


@pytest.mark.parametrize('reason,digest,content,valid', [
    (None, 'a'*64, {'kind':'retained_blob','sha256':'a'*64}, True),
    ('not_acquired', None, None, True),
    ('extraction_unavailable', None, None, True),
    ('not_retained', 'a'*64, None, True),
    ('redacted', 'a'*64, None, True),
    (None, 'a'*64, None, False),
    ('redacted', None, None, False),
    ('not_acquired', 'a'*64, None, False),
    ('not_retained', 'a'*64, {'kind':'retained_blob','sha256':'a'*64}, False),
    ('unknown', None, None, False),
])
def test_artifact_absence_contract(reason,digest,content,valid):
    a = report()['artifacts'][0]
    a.update(unavailable_reason=reason,sha256=digest,content_ref=content,byte_length=0 if digest else None)
    s=schema('2.0')
    validator=Draft202012Validator({'$defs':s['$defs'],'$ref':'#/$defs/artifact'})
    assert validator.is_valid(a) is valid


@pytest.mark.parametrize('path,value,error', [
    (('executions',2,'capability_ref'),'missing','dangling'),
    (('artifacts',1,'parents'),['artifact/1'],'cyclic'),
    (('artifacts',1,'sha256'),'0'*64,'digest'),
    (('artifacts',1,'content_ref','pointer'),'/evidence/texts/9/text','pointer'),
    (('anchors',0,'locator','spans',0,'byte','start'),12,'coordinate'),
    (('anchors',0,'locator','spans',0,'scalar','end'),999,'scalar'),
    (('anchors',0,'locator','selected_text_sha256'),'0'*64,'selection'),
    (('evidence','texts',0,'byte_offsets',12),13,'width'),
    (('results',0,'execution_ref'),'exec/3','ineligible'),
    (('results',0,'payload','operation'),'wrong.operation','operation'),
    (('findings',0,'anchor_refs'),['anchor/99'],'dangling'),
    (('findings',0,'execution_ref'),'exec/3','mechanism'),
    (('summary','exit_code'),0,'exit code'),
    (('summary','low'),0,'summary'),
    (('executions',2,'config','sha256'),'0'*64,'configuration'),
])
def test_semantic_mutations(path,value,error):
    r=report();at(r,path[:-1])[path[-1]]=value
    # These are intentionally valid JSON Schema shapes, invalid evidence.
    Draft202012Validator(schema('2.0')).validate(r)
    with pytest.raises(ValueError,match=error): validate_semantics(r)


@pytest.mark.parametrize('name', ['required-unavailable','partial-timeout','decode-failed'])
def test_failure_cannot_be_presented_as_clean(name):
    r=report(name);r['status']='completed';r['summary']['exit_code']=0
    with pytest.raises(ValueError,match='aggregate'): validate(r)


def test_partial_result_cannot_claim_whole_request():
    r=report('partial-timeout');r['results'][0]['payload']['scope']=r['executions'][2]['requested_scope']
    with pytest.raises(ValueError,match='outside analyzed'): validate(r)


def test_analyzed_region_cannot_exceed_request():
    r=report('partial-timeout');r['executions'][2]['analyzed_scope'][0]['regions'][0]['end']=999
    with pytest.raises(ValueError,match='scope'): validate(r)


def test_failure_code_must_match_diagnostic():
    r=report('decode-failed');r['findings'][0]['evidence']['failure_code']='file.not_found'
    with pytest.raises(ValueError,match='failure code'): validate(r)


def test_unknown_diagnostic_code_remains_failure():
    r=report('decode-failed');r['diagnostics'][0]['code']='future.new_failure';r['findings'][0]['evidence']['failure_code']='future.new_failure'
    validate(r)
    assert r['summary']['exit_code']==4


def test_filters_preserve_full_run_and_original_references():
    full=report();filtered=report('filtered-observation')
    for key in ('evidence','capabilities','executions','artifacts','anchors','results','diagnostics'):
        assert full[key]==filtered[key]
    assert filtered['findings']==[]
    assert filtered['results'][0]['payload']['outcome']=='observations_present'


@pytest.mark.parametrize('kind', ['credential_verification','statistical_watermark','raw_vendor_result'])
def test_unreviewed_result_contracts_rejected(kind):
    r=report();r['results'][0]['kind']=kind
    with pytest.raises(ValidationError): validate(r)


def test_unsupported_version_is_inert_not_guessed():
    r={'schema_version':'99.0','command':'do not execute','url':'https://example.invalid/private'}
    imported=import_report(json.dumps(r).encode())
    assert imported['status']=='unsupported_version'
    assert imported['coverage']=='unknown'
    assert imported['report']==r


@pytest.mark.parametrize('raw', [b'{"schema_version":"1.0","schema_version":"2.0"}', b'{"x":NaN}', b'{"x":1e999}', b'{"x":"\\ud800"}', b'{"x":9007199254740992}', b'\xff', b'['*100+b'0'+b']'*100])
def test_invalid_json_or_unicode_is_rejected(raw):
    with pytest.raises((ValueError,UnicodeError)): strict_json(raw)


def test_import_budget():
    with pytest.raises(ValueError,match='budget'): import_report(b' '*(8*1024*1024+1))


def test_v1_import_preserves_original_and_does_not_invent_coverage():
    manifest=json.loads((ROOT/'reference/python-behavior/manifest.json').read_bytes())
    for case in manifest['cases']:
        raw=(ROOT/'reference/python-behavior'/case['report']).read_bytes()
        imported=import_report(raw)
        assert imported['coverage']=='unknown' and imported['status']=='legacy'
        assert imported['report']==json.loads(raw)
        assert imported['report_artifact_sha256']==sha256(raw).hexdigest()
        for d in imported['adapter_diagnostics']:
            assert d['failure_code'] is None and d['code']=='legacy.failure_code_unavailable'
        assert 'executions' not in imported['report']


def test_report_identity_is_exact_bytes_not_reserialized_json():
    raw=(FIXTURES/'clean-unavailable.json').read_bytes()
    compact=json.dumps(json.loads(raw),separators=(',',':')).encode()
    assert import_report(raw)['report']==import_report(compact)['report']
    assert import_report(raw)['report_artifact_sha256']!=import_report(compact)['report_artifact_sha256']


def test_native_payload_projection_is_unchanged():
    manifest=json.loads((ROOT/'reference/python-behavior/manifest.json').read_bytes())
    for name,source in [('clean-unavailable','clean_ascii.txt'),('structural-observation','isolated_zwsp.txt'),('decode-failed','bad16.txt')]:
        r=report(name)
        old_case=next(c for c in manifest['cases'] if c['input'].endswith('/'+source))
        old=json.loads((ROOT/'reference/python-behavior'/old_case['report']).read_bytes())
        projected={k:r[k] for k in old};projected['schema_version']='1.0'
        for f in projected['findings']:
            for k in ('finding_ref','execution_ref','mechanism','anchor_refs'): del f[k]
            f['evidence'].pop('failure_code',None)
        assert projected==old


def test_identity_preserves_unicode_and_span_boundaries():
    assert identity('test/1','é') != identity('test/1','e\u0301')
    assert identity('test/1',['ab','c']) != identity('test/1',['a','bc'])
    assert identity_bytes('test/1',{'\ue000':1,'😀':2}) == '{"domain":"test/1","value":{"😀":2,"\ue000":1}}'.encode()
    assert identity('test/1',1.0) == identity('test/1',1)
    assert identity('test/1',-0.0) == identity('test/1',0)
    with pytest.raises(ValueError): identity('test/1',1.5)


@pytest.mark.parametrize('kind', ['structural_object','credential','statistical_sample','legacy_unknown'])
def test_reserved_anchor_shapes_are_closed_and_typed(kind):
    locators={
        'structural_object':{'version':'1','format':'ooxml','part':'word/document.xml','object_id':'paragraph/0','object_sha256':None},
        'credential':{'version':'1','manifest_ref':'artifact/2','assertion_id':None,'verification_execution_ref':None},
        'statistical_sample':{'version':'1','sample_ref':'artifact/2','scope':{'artifact_ref':'artifact/2','unit':'whole_artifact'}},
        'legacy_unknown':{'version':'1','report_pointer':'/findings/0','diagnostic_ref':'diagnostic/0'},
    }
    anchor={'anchor_ref':'anchor/0','kind':kind,'artifact_ref':None if kind=='legacy_unknown' else 'artifact/2','execution_ref':None if kind=='legacy_unknown' else 'exec/2','mapping':{'quality':'unavailable','reason_code':'mapping.not_available'},'locator':locators[kind]}
    s=schema('2.0');validator=Draft202012Validator({'$defs':s['$defs'],'$ref':'#/$defs/anchor'})
    assert validator.is_valid(anchor)
    anchor['locator']['byte_offsets']=[0,100]
    assert not validator.is_valid(anchor)


def test_partial_coverage_must_account_for_remainder():
    r=report('partial-timeout');r['executions'][2]['exclusions'][0]['scope']['regions'][0]['start']=19
    with pytest.raises(ValueError,match='unaccounted'): validate(r)
    r=report('partial-timeout');r['executions'][2]['exclusions'][0]['scope']['regions'][0]['start']=17
    with pytest.raises(ValueError,match='overlap'): validate(r)


def test_duplicate_ids_and_missing_plan_are_rejected():
    r=report();r['executions'].append(deepcopy(r['executions'][2]))
    with pytest.raises(ValueError,match='duplicate'): validate(r)
    r=report();r['executions'].pop()
    with pytest.raises(ValueError,match='planning'): validate(r)


def test_contract_reference_import_never_opens_report_paths(monkeypatch):
    r=report('clean-unavailable');r['file']['path']='/private/do-not-open'
    s=schema('2.0')
    monkeypatch.setattr('support.schema', lambda version:s)
    def forbidden(*args,**kwargs):
        raise AssertionError('attempt to open imported content')
    monkeypatch.setattr('builtins.open',forbidden)
    monkeypatch.setattr(Path,'open',forbidden)
    assert import_report(json.dumps(r).encode())['status']=='validated'


def test_portable_identity_vectors():
    vectors=json.loads((Path(__file__).parent/'identity-vectors.json').read_bytes())
    for v in vectors['identity_vectors']:
        expected=bytes.fromhex(v['canonical_utf8_hex'])
        assert identity_bytes(v['domain'],v['value'])==expected, v['id']
        assert sha256(expected).hexdigest()==v['sha256']
    # Full floating-point canonicalization is a separate Go implementation gate;
    # this checks preservation of the fixtures, not canonicalizer conformance.
    for v in vectors['future_general_jcs_numbers']:
        assert json.loads(v['input'])==json.loads(v['canonical'])


def test_all_native_finding_payloads_remain_valid():
    findings=json.loads((ROOT/'tests/schema-cases.json').read_bytes())['findings']
    s=schema('2.0');validator=Draft202012Validator({'$defs':s['$defs'],'$ref':'#/$defs/finding'})
    for f in findings.values():
        failure=f['id']=='parser.failure'
        f.update(finding_ref='finding/0',execution_ref='exec/0',mechanism=None if failure else 'structural',anchor_refs=[])
        if failure: f['evidence']['failure_code']='audit.failed'
        validator.validate(f)


def test_unsafe_offsets_and_external_content_refs_rejected():
    r=report();r['anchors'][0]['locator']['spans'][0]['scalar']['end']=9007199254740992
    with pytest.raises(ValidationError): validate(r)
    r=report();r['artifacts'][0]['content_ref']={'kind':'url','url':'https://example.invalid/manifest'}
    with pytest.raises(ValidationError): validate(r)


def test_out_of_range_exit_code_rejected():
    r=report();r['summary']['exit_code']=5
    with pytest.raises(ValidationError): validate(r)


def test_finding_coordinates_must_agree_with_text_map():
    r=report();r['findings'][0]['location']['byte_offsets']=[12]
    with pytest.raises(ValueError,match='finding coordinate'): validate(r)


def test_completed_analyzer_requires_result():
    r=report();r['results']=[]
    with pytest.raises(ValueError,match='without result'): validate(r)


def test_view_cannot_misdescribe_selected_categories():
    r=report('filtered-observation');r['view']['finding_categories']=[]
    with pytest.raises(ValueError,match='view category'): validate(r)
