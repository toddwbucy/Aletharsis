"""Unpack only the already-downloaded, hash-pinned study inputs. No network."""
import argparse
import hashlib
import json
from pathlib import Path
import tarfile
import zipfile

PINS = {
 'juriku':('juriku.tar.gz','e00598a843224d60f6c2a0d458812265ce8963f8ff22105557c578e411ff2c6f'),
 'hiberius':('hiberius.tar.gz','14c18b6ffb2cebafb78d0e93dc4de3cf8e49a369e7981584c421af04d501a437'),
 'emoji':('emoji-2.15.0-py3-none-any.whl','205296793d66a89d88af4688fa57fd6496732eb48917a87175a023c8138995eb'),
}


def verify_tree(dest, key):
 inventory=json.loads((Path(__file__).parent/'input-trees.json').read_bytes())[key]
 roots=list(dest.iterdir()) if key!='emoji' else [dest]
 if len(roots)!=1 or not roots[0].is_dir(): raise ValueError('unexpected source root')
 root=roots[0]
 if any(p.is_symlink() for p in root.rglob('*')): raise ValueError('source tree contains symlink')
 actual={p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
         for p in root.rglob('*') if p.is_file()}
 if actual!=inventory: raise ValueError('source tree identity mismatch')


def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--downloads',type=Path,required=True)
 p.add_argument('--output',type=Path,required=True)
 p.add_argument('--allow-repacked-source',action='store_true',help='Allow different source tar bytes only after exact pinned file-tree verification; wheel remains hash-pinned')
 args=p.parse_args()
 args.output.mkdir(exist_ok=False)
 receipts={}
 for key,(name,expected) in PINS.items():
  archive=args.downloads/name
  if archive.stat().st_size>32*1024**2: raise ValueError('source archive size mismatch')
  actual=hashlib.sha256(archive.read_bytes()).hexdigest()
  if actual!=expected and (not args.allow_repacked_source or key=='emoji'):
   raise ValueError('source archive identity mismatch')
  dest=args.output/key;dest.mkdir()
  if key=='emoji':
   with zipfile.ZipFile(archive) as z:
    if sum(f.file_size for f in z.infolist())>16*1024**2: raise ValueError('oversized wheel')
    if any(Path(n).is_absolute() or '..' in Path(n).parts for n in z.namelist()): raise ValueError('unsafe wheel path')
    z.extractall(dest)
  else:
   with tarfile.open(archive) as a:
    members=a.getmembers()
    if sum(m.size for m in members)>32*1024**2 or not all(m.isfile() or m.isdir() for m in members):
     raise ValueError('oversized or non-regular source archive')
    a.extractall(dest,filter='data')
  verify_tree(dest,key)
  receipts[name]={'observed_sha256':actual,'historical_sha256':expected,'tree_verified':True}
 (args.output/'provisioning.json').write_text(json.dumps(receipts,indent=2)+'\n')

if __name__=='__main__':main()
