"""Optional seeded differential check against the retained Python implementation."""
import argparse
import json
from pathlib import Path
import random
import subprocess
import tempfile

from aletharsis.audit import audit
from check_parity import canonical, differences

parser=argparse.ArgumentParser()
parser.add_argument('binary',type=Path)
args=parser.parse_args()
binary=args.binary.resolve()
rng=random.Random(1729)
alphabet='aéаα漢가123_ \t\n\r\u001c\u200b\u200c\u200d\u2063\u2067\u2069\u00ad\u0301\u0323\u034f\ufe0f😀👍🏽\U000e0067'
texts=[''.join(rng.choice(alphabet) for _ in range(rng.randrange(1,150))) for _ in range(150)]
for label in ['author','AUTHOR','generated\u3000by','document-id','tracking_ID','TRACKİNG-ID']:
    for suffix in [' yes\n',' \n','\t\r\n','\n',' value <tag>',' \r\n next line']:
        texts.append(f'{label}:{suffix}')
texts += [prefix+'12345678-1234-1234-1234-123456789abc'+suffix
          for prefix in ['', 'é','α','1','-','!'] for suffix in ['', 'é','_','!']]
failures=[]
with tempfile.TemporaryDirectory(prefix='aletharsis-parity-') as folder:
    for i,text in enumerate(texts):
        path=Path(folder)/f'{i}.txt'
        path.write_bytes(text.encode())
        expected=audit(path).to_dict()
        run=subprocess.run([str(binary),'audit',str(path),'--json'],capture_output=True,text=True)
        actual=json.loads(run.stdout)
        errors=list(differences(canonical(expected),canonical(actual)))
        if run.returncode!=expected['summary']['exit_code']:errors.append('exit code differs')
        if errors:
            failures.append(i)
            print(repr(text),*errors[:3],sep='\n  ')
print(f'{len(texts)-len(failures)}/{len(texts)} seeded differential cases match.')
raise SystemExit(bool(failures))
