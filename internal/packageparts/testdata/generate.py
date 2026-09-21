"""Explicit development-only generation of original, synthetic Office packages."""
import hashlib
import json
from pathlib import Path
from zipfile import ZIP_STORED, ZipFile, ZipInfo

ROOT = Path(__file__).parent
WORD = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'
REL = 'http://schemas.openxmlformats.org/package/2006/relationships'
ODF = 'urn:oasis:names:tc:opendocument:xmlns:'


def write(name, parts):
    with ZipFile(ROOT / name, 'w') as archive:
        for path, text in parts:
            info = ZipInfo(path, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = ZIP_STORED
            info.create_system = 3
            info.external_attr = 0o100600 << 16
            archive.writestr(info, text.encode('utf-8'))
    return {
        'name': name,
        'sha256': hashlib.sha256((ROOT / name).read_bytes()).hexdigest(),
        'parts': [{'name': p, 'size': len(t.encode('utf-8')),
                   'sha256': hashlib.sha256(t.encode('utf-8')).hexdigest()}
                  for p, t in sorted(parts)],
    }


records = []
for hidden in (False, True):
    label = 'hidden' if hidden else 'minimal'
    payload = '\u200b\u200c' * 32
    concealed = f'<w:r><w:rPr><w:vanish/></w:rPr><w:t>{payload}</w:t></w:r>' if hidden else ''
    records.append(write(label + '.docx', [
        ('[Content_Types].xml', '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>'),
        ('_rels/.rels', f'<Relationships xmlns="{REL}"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>'),
        ('word/document.xml', f'<w:document xmlns:w="{WORD}"><w:body><w:p><w:r><w:t>Hello Café 日本語 👩\u200d💻</w:t></w:r>{concealed}</w:p><w:sectPr/></w:body></w:document>'),
    ]))
    concealed = f'<text:hidden-text text:is-hidden="true">{payload}</text:hidden-text>' if hidden else ''
    records.append(write(label + '.odt', [
        ('mimetype', 'application/vnd.oasis.opendocument.text'),
        ('META-INF/manifest.xml', f'<manifest:manifest xmlns:manifest="{ODF}manifest:1.0" manifest:version="1.3"><manifest:file-entry manifest:full-path="/" manifest:version="1.3" manifest:media-type="application/vnd.oasis.opendocument.text"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/></manifest:manifest>'),
        ('content.xml', f'<office:document-content xmlns:office="{ODF}office:1.0" xmlns:text="{ODF}text:1.0" office:version="1.3"><office:body><office:text><text:p>Hello Café 日本語 👩\u200d💻{concealed}</text:p></office:text></office:body></office:document-content>'),
    ]))
(ROOT / 'manifest.json').write_text(json.dumps(records, indent=2) + '\n', encoding='utf-8')
