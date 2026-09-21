"""Unpack only the already-downloaded, hash-pinned study inputs. No network."""
import argparse
import hashlib
from pathlib import Path
import tarfile
import zipfile

PINS = {
 'juriku':('juriku.tar.gz','e00598a843224d60f6c2a0d458812265ce8963f8ff22105557c578e411ff2c6f'),
 'hiberius':('hiberius.tar.gz','14c18b6ffb2cebafb78d0e93dc4de3cf8e49a369e7981584c421af04d501a437'),
 'emoji':('emoji-2.15.0-py3-none-any.whl','205296793d66a89d88af4688fa57fd6496732eb48917a87175a023c8138995eb'),
}


def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--downloads',type=Path,required=True)
 p.add_argument('--output',type=Path,required=True)
 args=p.parse_args()
 args.output.mkdir(exist_ok=False)
 for key,(name,expected) in PINS.items():
  archive=args.downloads/name
  if archive.stat().st_size>32*1024**2 or hashlib.sha256(archive.read_bytes()).hexdigest()!=expected:
   raise ValueError('source archive identity or size mismatch')
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

if __name__=='__main__':main()
