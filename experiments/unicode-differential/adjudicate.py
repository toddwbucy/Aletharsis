"""Compare observations, not votes; expected positions come from authored cases."""
import json
from pathlib import Path
import sys

VISIBLE_EMOJI = {'U+1F469','U+1F4BB','U+2764','U+1F600'}
TYPOGRAPHY = {'U+201C','U+201D','U+2014','U+2019','U+2026'}


def reason(tool, case, cp, scalar, native_status):
    if tool == 'aletharsis' and native_status != 'completed':
        return 'parser', 'Binary identification rejected this source; no text audit completed. Do not infer absence.'
    if tool == 'juriku_word' and (cp in TYPOGRAPHY or cp == 'U+00A0') and case == 'word_typography':
        return 'policy', 'Explicit Word exclusion mode suppresses common typography/NBSP; original bytes remain observable.'
    if cp in TYPOGRAPHY:
        return 'policy', 'Visible punctuation is outside this scan inventory (or explicitly not marked); not evidence of a hidden watermark.'
    if tool.startswith('juriku') and cp == 'U+FEFF' and scalar == 0:
        return 'policy', 'Read-only upstream detector intentionally omits the leading BOM, but not an embedded BOM.'
    if tool.startswith('juriku') and case == 'emoji_selector' and cp == 'U+FE0F':
        return 'policy', 'Pinned emoji 2.15.0 recognizes the pair; upstream deliberately omits its selector.'
    if cp in VISIBLE_EMOJI:
        return 'coverage', 'Comparator does not inventory visible emoji; Aletharsis deliberately does. No attribution claim.'
    if tool == 'aletharsis' and case == 'hangul_fillers':
        return 'inventory', 'Native Kind covers controls/marks but misses these letter-category fillers. Candidate #14 inventory extension; not a watermark verdict.'
    return 'inventory', 'Code point is present at the independently specified position but absent from this configured comparator inventory.'


def summarize(root):
    cases=json.loads((root/'corpus.json').read_bytes())
    output=[]
    for case in cases:
        directory=root/case['id']
        native=json.loads((directory/'aletharsis.stdout.json').read_bytes())
        juriku=json.loads((directory/'juriku.stdout.json').read_bytes())
        hiberius=json.loads((directory/'hiberius.stdout.json').read_bytes())
        observed={'aletharsis':[], 'juriku_inventory':[], 'juriku_word':[], 'hiberius':[]}
        for f in native['findings']:
            if f['id'].startswith('unicode.') and 'code_point' in f['evidence']:
                observed['aletharsis'] += [{'scalar':p,'code_point':f['evidence']['code_point']} for p in f['location']['character_offsets']]
        for target,mode in [('juriku_inventory','inventory'),('juriku_word','word_exclusions')]:
            observed[target]=[{'scalar':m['char_idx'],'code_point':m['code_point']} for m in juriku['results'][mode]]
        observed['hiberius']=[{'scalar':m['scalar'],'code_point':m['code_point']} for m in hiberius['observations']]
        expected={(o['scalar'],o['code_point']) for o in case['expected_observations']}
        differences=[]
        for tool,rows in observed.items():
            rows.sort(key=lambda r:(r['scalar'],r['code_point']))
            actual={(r['scalar'],r['code_point']) for r in rows}
            for scalar,cp in sorted(expected-actual):
                category,detail=reason(tool,case['id'],cp,scalar,native['status'])
                differences.append({'tool':tool,'scalar':scalar,'code_point':cp,'direction':'not_reported','category':category,'reason':detail})
            for scalar,cp in sorted(actual-expected):
                differences.append({'tool':tool,'scalar':scalar,'code_point':cp,'direction':'additional','category':'unadjudicated','reason':'Requires independent fixture/source review; do not automatically bless an extra observation.'})
        output.append({'case':case['id'],'native_status':native['status'],
            'hiberius_status':hiberius['scan_status'], 'observed':observed,'differences':differences,
            'native_pattern_ids':sorted({f['id'] for f in native['findings'] if f['id'].startswith('pattern.')}),
            'interpretation':case['interpretation']})
    return output


if __name__=='__main__':
    print(json.dumps(summarize(Path(sys.argv[1])),ensure_ascii=True,indent=2))
