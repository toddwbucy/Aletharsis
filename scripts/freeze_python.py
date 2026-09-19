"""Freeze reviewed Python behavior. Run with Python 3.12 / Unicode 15.0.0.

Existing reference artifacts are never overwritten; regeneration is a deliberate
reviewed operation in a new destination/revision, not part of normal tests.
"""
from pathlib import Path
import hashlib
import json
import unicodedata as ud
from aletharsis.audit import audit, LIMITATIONS

root = Path('reference/python-behavior')
assert ud.unidata_version == '15.0.0'
assert (root/'unicode.json').is_file(), 'Run freeze_unicode.py first in a fresh capture'
assert (root/'report.schema.json').is_file(), 'Supply the reviewed schema snapshot first'
for directory in ['inputs', 'reports']:
    (root/directory).mkdir(parents=True, exist_ok=True)
extras = {
 'emoji.py': '# 😀 note\n"""👩\u200d💻 docstring"""\nstatus = "👍🏽 🇬🇧 1\ufe0f\u20e3 ©"\n',
 'unicode.txt': 'é\u200b\u200c\u200d\u2060\u2061\u2062\u2063\u2064\ufeff\u00ad\u00a0\u202f\u2009\U000e0100e\u0301\n',
 'encoded.txt': 'cHJvdmVuYW5jZS1pZC0xMjM0NTY3ODkwYWJjZGVm\n',
 'boundary.txt': 'é12345678-1234-1234-1234-123456789abc\n x12345678-1234-1234-1234-123456789abc\n author: yes\n',
 'long_marks.txt': 'A' + '\u0305' * 40 + '\u0301\u0323' + '\u034f' + '\u0300' * 40,
 'normalization.txt': '\u1100\u1161\u11a8 Å ﬃ Ａ \u0344 \u2126\u1e9b\u0323\n',
 'empty.txt': '',
 'unsupported.pdf': '%PDF-1.7\n',
 'disguised.txt': '%PDF-1.7\n',
 'invalid_controls.bin': '\x00' * 30,
}
for name, content in extras.items():
    with (root/'inputs'/name).open('xb') as f: f.write(content.encode())
for enc in ['utf-16-le','utf-16-be','utf-32-le','utf-32-be']:
    with (root/'inputs'/f'{enc}.txt').open('xb') as f: f.write(('\ufeffé😀\u200b\r\n').encode(enc))
for name, content in [('bad16.txt', b'\xff\xfe\x00\xd8A\x00'), ('truncated32.txt',b'\xff\xfe\x00\x00A')]:
    with (root/'inputs'/name).open('xb') as f: f.write(content)
manifest = {'python_commit':'a0401f93e948b334b51f7332a5729df7647eab24', 'python_version':'3.12.13',
            'unicode_version':ud.unidata_version, 'emoji_version':'17.0', 'files':{}, 'cases':[],
            'schema_commit':'36c88ee18e65154e8694e117f6db06d40c2b36ec',
            'schema_sha256':hashlib.sha256((root/'report.schema.json').read_bytes()).hexdigest(),
            'unicode_oracle_sha256':hashlib.sha256((root/'unicode.json').read_bytes()).hexdigest()}
for path in sorted([*Path('src').rglob('*.py'), *Path('src/aletharsis/data').glob('*.txt'), Path('pyproject.toml')]):
    manifest['files'][str(path)] = hashlib.sha256(path.read_bytes()).hexdigest()
templates = {}
for i,path in enumerate(sorted([*Path('tests/fixtures').iterdir(), *root.joinpath('inputs').iterdir()])):
    report = audit(path).to_dict()
    output = f'reports/{i:03d}.json'
    with (root/output).open('x') as f: json.dump(report,f,ensure_ascii=True,sort_keys=True,indent=2); f.write('\n')
    manifest['cases'].append({'input':str(path), 'report':output, 'sha256':report['file']['sha256'], 'exit_code':report['summary']['exit_code'], 'report_sha256':hashlib.sha256((root/output).read_bytes()).hexdigest()})
    for finding in report['findings']:
        templates.setdefault(finding['id'], {k:finding[k] for k in ['id','severity','confidence','category','classification','title','description']})
with (root/'manifest.json').open('x') as f: json.dump(manifest,f,indent=2,sort_keys=True); f.write('\n')
def write_new(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('x') as f:
        f.write(value)

write_new('internal/analyzers/data/messages.json', json.dumps(templates,indent=2,ensure_ascii=True,sort_keys=True)+'\n')
write_new('internal/analyzers/data/limitations.json', json.dumps(LIMITATIONS,indent=2)+'\n')
# Pin classification to the reference UCD, independent of the Go toolchain UCD.
rows=[]
start=0
last=ud.category(chr(0))
for cp in range(1,0x110001):
    category=ud.category(chr(cp)) if cp<0x110000 else None
    if category != last:
        rows.append(f'{start:X};{cp-1:X};{last}')
        start,last=cp,category
write_new('internal/unicoderef/categories15.txt', '# Generated from Python Unicode 15.0.0; see Unicode license in ../analyzers/data/\n'+'\n'.join(rows)+'\n')
print(f'Frozen {len(manifest["cases"])} report cases and {len(templates)} finding templates.')
