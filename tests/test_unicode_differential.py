"""Offline verification of independent Unicode corpus and retained probe evidence."""
import hashlib
import importlib.util
import json
from pathlib import Path
import sys

import pytest

ROOT=Path(__file__).resolve().parents[1]
DATA=ROOT/'docs/reuse/evaluations/unicode'
PROBE=ROOT/'experiments/unicode-differential'
MANIFEST=json.loads((DATA/'manifest.json').read_bytes())
CORPUS=json.loads((DATA/'corpus.json').read_bytes())
RESULTS=json.loads((DATA/'results.json').read_bytes())
REPLAY=json.loads((DATA/'reproduction-results.json').read_bytes())


def load(name):
    spec=importlib.util.spec_from_file_location(name,PROBE/(name+'.py'))
    module=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_complete_manifest_and_pins():
    assert len(CORPUS)==30
    assert len({c['id'] for c in CORPUS})==len(CORPUS)
    assert len(RESULTS)==len(REPLAY)==len(CORPUS)==MANIFEST['case_count']
    assert {c['case'] for c in RESULTS}=={c['id'] for c in CORPUS}
    assert [r['case'] for r in REPLAY]==[r['case'] for r in RESULTS]
    actual={p.relative_to(DATA).as_posix() for p in DATA.rglob('*') if p.is_file() and p.name!='manifest.json'}
    assert actual==set(MANIFEST['files'])
    for path,digest in MANIFEST['files'].items():
        assert hashlib.sha256((DATA/path).read_bytes()).hexdigest()==digest
    registry={r['id']:r for r in json.loads((ROOT/'docs/reuse/candidates.json').read_bytes())['components']}
    for component,name in [('juriku--hidden-characters-detector','juriku'),('hiberius--hiberius-unicode-toolkit','hiberius')]:
        assert registry[component]['pin']['artifact_sha256']==MANIFEST['archives'][name+'.tar.gz']
        assert registry[component]['adoption_status']=='evaluating'
        assert registry[component]['supported_scope']['production']==[]
    assert {name:sha for name,sha in load('provision').PINS.values()}==MANIFEST['archives']
    trees=json.loads((PROBE/'input-trees.json').read_bytes())
    for component,name in [('juriku--hidden-characters-detector','juriku'),('hiberius--hiberius-unicode-toolkit','hiberius')]:
        assert registry[component]['license']['sha256']==trees[name]['LICENSE']
    assert MANIFEST['recommendation']=='revise' and MANIFEST['owner_acceptance'] is None


def test_independent_generator_and_adjudication():
    assert load('corpus').cases()==CORPUS
    expected=load('adjudicate').summarize(DATA)
    assert expected==json.loads((DATA/'comparison.json').read_bytes())
    assert not any(d['category']=='unadjudicated' for row in expected for d in row['differences'])
    assert {d['category'] for row in expected for d in row['differences']}=={'inventory','coverage','parser','policy'}
    environment=json.loads((DATA/'environment.json').read_bytes())
    for name,digest in environment['probe_sha256'].items():
        assert hashlib.sha256((PROBE/name).read_bytes()).hexdigest()==digest


@pytest.mark.parametrize('case',CORPUS,ids=lambda c:c['id'])
def test_fixture_coordinates_and_raw_results(case):
    raw=bytes.fromhex(case['raw_hex'])
    assert raw==(DATA/case['id']/'source.bin').read_bytes()
    assert hashlib.sha256(raw).hexdigest()==case['source_sha256']
    text=raw.decode(case['serialization'])
    assert text==case['decoded_text']
    assert hashlib.sha256(text.encode()).hexdigest()==case['decoded_utf8_sha256']
    assert case['license']=='Apache-2.0'
    for observation in case['expected_observations']:
        pos=observation['scalar']
        assert observation['code_point']==f'U+{ord(text[pos]):04X}'
        assert observation['byte_start']==len(text[:pos].encode(case['serialization']))
        assert observation['byte_end']==len(text[:pos+1].encode(case['serialization']))
        assert observation['utf16_start']==len(text[:pos].encode('utf-16-le'))//2
    i=next(i for i,r in enumerate(RESULTS) if r['case']==case['id'])
    for tool,run in RESULTS[i]['tools'].items():
        assert run['source_unchanged'] and not run['timed_out']
        assert run['returncode'] in (range(5) if tool=='aletharsis' else [0])
        for key,suffix in [('stdout_sha256','stdout.json'),('stderr_sha256','stderr')]:
            assert run[key]==REPLAY[i]['tools'][tool][key]
            assert hashlib.sha256((DATA/case['id']/f'{tool}.{suffix}').read_bytes()).hexdigest()==run[key]
        assert REPLAY[i]['tools'][tool]['source_unchanged'] and not REPLAY[i]['tools'][tool]['timed_out']
    native=json.loads((DATA/case['id']/'aletharsis.stdout.json').read_bytes())
    assert native['file']['sha256']==case['source_sha256']
    for f in native['findings']:
        if f['id'].startswith('unicode.') and 'code_point' in f['evidence']:
            positions=f['location']['character_offsets']; offsets=f['location']['byte_offsets']
            assert len(positions)==len(offsets)
            for p,b in zip(positions,offsets):
                assert f['evidence']['code_point']==f'U+{ord(text[p]):04X}'
                assert b==len(text[:p].encode(case['serialization']))
    for tool in ['juriku','hiberius']:
        result=json.loads((DATA/case['id']/f'{tool}.stdout.json').read_bytes())
        assert result['input_unchanged']
        rows=result['observations'] if tool=='hiberius' else result['results']['inventory']+result['results']['word_exclusions']
        for row in rows:
            position=row['scalar'] if tool=='hiberius' else row['char_idx']
            assert row['code_point']==f'U+{ord(text[position]):04X}'


def test_actionable_gaps_and_limits_are_not_hidden():
    comparison={r['case']:r for r in json.loads((DATA/'comparison.json').read_bytes())}
    assert comparison['controls']['native_status']=='failed'
    assert comparison['empty']['hiberius_status']=='empty_input_not_scanned'
    assert comparison['hangul_fillers']['observed']['aletharsis']==[]
    assert len(comparison['hangul_fillers']['observed']['hiberius'])==4
    assert comparison['emoji_selector']['observed']['juriku_inventory']==[]
    assert len(comparison['emoji_selector']['observed']['aletharsis'])==2
    assert len(comparison['leading_bom']['observed']['aletharsis'])==2
    assert len(comparison['leading_bom']['observed']['juriku_inventory'])==1
    assert comparison['binary_sequence']['native_pattern_ids']==['pattern.zero_width_binary']
    assert comparison['selector_sequence']['native_pattern_ids']==['pattern.variation_selector_run']
    assert comparison['tag_sequence']['native_pattern_ids']==[]
    assert comparison['tag_run']['native_pattern_ids']==['pattern.tag_run']
    tags=json.loads((DATA/'tag_run/aletharsis.stdout.json').read_bytes())
    pattern=next(f for f in tags['findings'] if f['id']=='pattern.tag_run')
    assert pattern['evidence']['ascii_projection']=='abcdefghijklmno'
