"""Compare observations, not votes; expected positions come from authored cases."""
import json
from pathlib import Path
import sys

VISIBLE_EMOJI = {'U+1F469','U+1F4BB','U+2764','U+1F600'}
TYPOGRAPHY = {'U+201C','U+201D','U+2014','U+2019','U+2026'}


# Explicit omissions verified for these pinned inventories; new misses require review.
INVENTORY = {
 'hiberius': {'U+0301','U+180B','U+180C','U+180D','U+FE00','U+FE01','U+FE0F','U+E0100'},
 'juriku': {'U+0000','U+001B','U+007F','U+0085','U+061C','U+115F','U+1160','U+17B4','U+17B5','U+2026','U+3164','U+FFA0'},
}
for inventory in INVENTORY.values():
    inventory.update(f'U+{cp:04X}' for cp in [*range(0xE0061,0xE0070),0xE007F])


def reason(tool, case, cp, scalar, native_status, hiberius_status='completed'):
    if tool == 'hiberius' and hiberius_status != 'completed':
        return 'coverage', 'No completed HIBERIUS verdict; do not infer inventory absence.'
    if tool == 'aletharsis' and native_status != 'completed':
        return 'parser', 'Binary identification rejected this source; no text audit completed. Do not infer absence.'
    if tool == 'juriku_word' and (cp in TYPOGRAPHY or cp == 'U+00A0') and case == 'word_typography':
        return 'policy', 'Explicit Word exclusion mode suppresses common typography/NBSP; original bytes remain observable.'
    if tool in ('aletharsis', 'hiberius') and cp in TYPOGRAPHY and case == 'word_typography':
        return 'policy', 'Visible punctuation is outside this scan inventory (or explicitly not marked); not evidence of a hidden watermark.'
    if tool.startswith('juriku') and cp == 'U+FEFF' and scalar == 0:
        return 'policy', 'Read-only upstream detector intentionally omits the leading BOM, but not an embedded BOM.'
    if tool.startswith('juriku') and case == 'emoji_selector' and cp == 'U+FE0F':
        return 'policy', 'Pinned emoji 2.15.0 recognizes the pair; upstream deliberately omits its selector.'
    if tool != 'aletharsis' and cp in VISIBLE_EMOJI:
        return 'coverage', 'Comparator does not inventory visible emoji; Aletharsis deliberately does. No attribution claim.'
    if tool == 'aletharsis' and case == 'hangul_fillers' and cp in {'U+115F','U+1160','U+3164','U+FFA0'}:
        return 'inventory', 'Native Kind covers controls/marks but misses these letter-category fillers. Candidate #14 inventory extension; not a watermark verdict.'
    if cp in INVENTORY.get('juriku' if tool.startswith('juriku') else tool, set()):
        return 'inventory', 'Code point is present at the independently specified position but absent from this configured comparator inventory.'
    if tool == 'aletharsis':
        return 'defect', 'Unexpected native omission against authored expectations; requires investigation, not an inventory exemption.'
    return 'unadjudicated', 'Unexplained comparator omission; requires source/fixture review.'


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
            missing, extra = expected-actual, actual-expected
            shifted = {cp for _,cp in missing} & {cp for _,cp in extra}
            for scalar,cp in sorted(missing):
                category,detail=reason(tool,case['id'],cp,scalar,native['status'],hiberius['scan_status'])
                if cp in shifted:
                    category,detail='offset','Code point appears at an unexpected scalar; inspect detector/probe coordinates.'
                differences.append({'tool':tool,'scalar':scalar,'code_point':cp,'direction':'not_reported','category':category,'reason':detail})
            for scalar,cp in sorted(actual-expected):
                differences.append({'tool':tool,'scalar':scalar,'code_point':cp,'direction':'additional','category':'offset' if cp in shifted else 'unadjudicated','reason':'Requires independent fixture/source review; do not automatically bless an extra observation.'})
        output.append({'case':case['id'],'native_status':native['status'],
            'hiberius_status':hiberius['scan_status'], 'observed':observed,'differences':differences,
            'native_pattern_ids':sorted({f['id'] for f in native['findings'] if f['id'].startswith('pattern.')}),
            'interpretation':case['interpretation']})
    return output


if __name__=='__main__':
    print(json.dumps(summarize(Path(sys.argv[1])),ensure_ascii=True,indent=2))
