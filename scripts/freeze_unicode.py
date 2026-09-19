"""Generate exhaustive Unicode behavior digests from the pinned Python oracle."""
from pathlib import Path
import hashlib
import json
import random
import unicodedata as ud
from aletharsis.analyzers.unicode import kind
from aletharsis.analyzers.emoji import emoji_codepoints
assert ud.unidata_version == '15.0.0'
Path('reference/python-behavior').mkdir(parents=True, exist_ok=True)
h = hashlib.sha256(); names=hashlib.sha256();all_text=[]
for cp in range(0x110000):
    if 0xd800 <= cp <= 0xdfff: continue
    c=chr(cp);all_text.append(c)
    prefix=ud.name(c,'').split(' ')[0] if c.isalpha() else ''
    h.update(f'{cp:X};{ud.category(c)};{int(c.isspace())};{int(c.isalpha())};{int(c.isalnum() or c=="_")};{prefix}\n'.encode())
    if kind(c) or cp in emoji_codepoints():
        names.update(f'{cp:X};{ud.name(c,"UNNAMED CONTROL")}\n'.encode())
rng=random.Random(1729)
alphabet='AéÅÅ\u1100\u1161\u11a8\u034f\u0300\u0301\u0305\u0315\u0323\u0344\ufb03\uff21\u1e9b\u2126'
vectors=[]
for _ in range(150):
    text=''.join(rng.choice(alphabet) for _ in range(rng.randint(1,120)))
    vectors.append({'text':text,'nfc':ud.normalize('NFC',text),'nfkc':ud.normalize('NFKC',text)})
for text in ['A'+'\u0305'*n+'\u0301\u0323' for n in (29,30,31,60,1000)]:
    vectors.append({'text':text,'nfc':ud.normalize('NFC',text),'nfkc':ud.normalize('NFKC',text)})
all_text=''.join(all_text)
value={'unicode_version':ud.unidata_version,'properties_sha256':h.hexdigest(),'names_sha256':names.hexdigest(),
 'all_scalars_nfc_sha256':hashlib.sha256(ud.normalize('NFC',all_text).encode()).hexdigest(),
 'all_scalars_nfkc_sha256':hashlib.sha256(ud.normalize('NFKC',all_text).encode()).hexdigest(),'normalization_vectors':vectors}
with Path('reference/python-behavior/unicode.json').open('x') as f: json.dump(value,f,ensure_ascii=True,indent=2); f.write('\n')
print('Frozen all-scalar Unicode digests and 155 normalization vectors.')
