"""Deterministic DOCX/ODT wire fixtures with independently constructed byte locations.

This tests the contract, not the future CLI producer. The retained ZIP allows
independent verification of container/part identities and lexical coordinates.
"""
from copy import deepcopy
import hashlib
import io
import json
from pathlib import Path
import re
import struct
import zipfile

ROOT = Path(__file__).resolve().parents[2]
OUT = Path(__file__).with_name('fixtures')
NS = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'
OFFICE_NS = 'urn:oasis:names:tc:opendocument:xmlns:office:1.0'
TEXT_NS = 'urn:oasis:names:tc:opendocument:xmlns:text:1.0'


def digest(b):
    return hashlib.sha256(b).hexdigest()


def build(format='docx'):
    if format not in ('docx', 'odt'):
        raise ValueError('unsupported fixture format')
    r = json.loads((OUT / 'flat-structural-observation.json').read_bytes())
    text = r['evidence']['texts'][0]['text']
    body = (f'<w:document xmlns:w="{NS}"><w:body><w:p><w:r><w:t>' + text +
            '</w:t></w:r></w:p></w:body></w:document>').encode()
    members = {
        '[Content_Types].xml': b'<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>',
        '_rels/.rels': b'<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>',
        'word/document.xml': body,
    }
    if format == 'odt':
        body = (f'<office:document-content xmlns:office="{OFFICE_NS}" xmlns:text="{TEXT_NS}" office:version="1.3">'
                '<office:body><office:text><text:p><text:span>' + text +
                '</text:span></text:p></office:text></office:body></office:document-content>').encode()
        members = {
            'mimetype': b'application/vnd.oasis.opendocument.text',
            'META-INF/manifest.xml': b'<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/></manifest:manifest>',
            'content.xml': body,
        }
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, 'w') as z:
        for name, value in members.items():
            z.writestr(zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0)), value)
    source = stream.getvalue()
    source_hash = digest(source)
    package = {'package_ref': 'office-package/0', 'source_artifact_ref': 'artifact/0',
        'source_sha256': source_hash, 'source_byte_length': len(source),
        'parser_version': 'zip-parts/1', 'format': format, 'state': 'completed',
        'issues': [], 'parts': [], 'outcomes': [], 'limits': {
            'source_bytes': 8 << 20, 'part_count': 4096, 'part_bytes': 32 << 20,
            'aggregate_bytes': 64 << 20, 'max_scope_text_utf8_bytes': 4 << 20,
            'max_scope_scalar_origins': 200000, 'report_input_bytes': 16 << 20,
            'report_output_bytes': 16 << 20, 'report_nodes': 4194304, 'report_depth': 64}}
    source_art = deepcopy(r['artifacts'][0]); source_art.update(sha256=source_hash, byte_length=len(source))
    artifacts = [source_art]
    with zipfile.ZipFile(io.BytesIO(source)) as z:
        for i, name in enumerate(members):
            info = z.getinfo(name)
            n, extra = struct.unpack_from('<HH', source, info.header_offset + 26)
            start = info.header_offset + 30 + n + extra
            part_ref = f'office-part/{i}'; asset_ref = f'artifact/{i+1}'
            part = {'part_ref': part_ref, 'package_ref': package['package_ref'], 'name': name,
                'artifact_ref': asset_ref, 'method': 0, 'compressed_span': {'start': start, 'end': start + info.compress_size},
                'compressed_sha256': digest(source[start:start+info.compress_size]),
                'sha256': digest(members[name]), 'byte_length': len(members[name]), 'state': 'completed', 'issues': []}
            package['parts'].append(part)
            package['outcomes'].append({'operation': 'aletharsis.parse.office_package', 'execution_ref': 'exec/1',
                'part_ref': part_ref, 'state': 'completed', 'codes': [], 'diagnostic_refs': [],
                'assessed': [{'start': 0, 'end': len(members[name])}], 'excluded': []})
            asset = deepcopy(source_art)
            asset.update(artifact_ref=asset_ref, kind='package_part', sha256=part['sha256'],
                         byte_length=part['byte_length'], parents=['artifact/0'])
            artifacts.append(asset)
    part = package['parts'][-1]
    tokens, elements, stack = [], [], []
    for match in re.finditer(rb'<[^>]+>|[^<]+', body):
        raw = match.group(); span = {'start': match.start(), 'end': match.end()}
        if raw.startswith(b'</'):
            element = stack.pop(); elements[element]['span']['end'] = match.end(); kind = 'end'
        elif raw.startswith(b'<'):
            element = len(elements)
            prefix, local = raw[1:].split(b'>')[0].split(b' ')[0].decode().split(':', 1)
            namespace = {'w': NS, 'office': OFFICE_NS, 'text': TEXT_NS}[prefix]
            elements.append({'index': element, 'parent': stack[-1] if stack else None,
                'namespace': namespace, 'local_name': local, 'span': dict(span)})
            stack.append(element); kind = 'start'
        else:
            element = stack[-1]; kind = 'text'; text_span = span; text_token = len(tokens)
        tokens.append({'index': len(tokens), 'kind': kind, 'span': span, 'element': element})
    scalars, origins, pos = [], [], 0
    for i, char in enumerate(text):
        end = pos + len(char.encode())
        scalar = {'scalar': i, 'code_point': ord(char), 'source': {'start': text_span['start']+pos, 'end': text_span['start']+end},
                  'utf8': {'start': pos, 'end': end}, 'transformation': 'literal'}
        scalars.append(scalar)
        origins.append({'kind': 'stored', 'xml_ref': 'office-xml/0', 'text_index': 0, 'segment': 0,
                        **{k: deepcopy(v) for k, v in scalar.items() if k != 'code_point'}})
        pos = end
    xml = {'xml_ref': 'office-xml/0', 'part_ref': part['part_ref'], 'parser_version': 'xml-parts/1',
        'mapper_version': 'xml-text-map/1', 'tokens': tokens, 'elements': elements, 'issues': [], 'controls': [],
        'segments': [{'token': text_token, 'element': 4, 'token_span': text_span, 'content_span': text_span,
                     'cdata': False, 'text': text, 'sha256': digest(text.encode()), 'scalars': scalars}]}
    scope = {'scope_ref': 'office-scope/0', 'part_ref': part['part_ref'], 'local_id': ('word' if format == 'docx' else 'odt') + '-scope/0',
        'extractor_version': ('word' if format == 'docx' else 'odt') + '-text/1',
        'assembler_version': ('word' if format == 'docx' else 'odt') + '-analysis/1', 'role': 'body',
        'text': text, 'sha256': digest(text.encode()), 'origins': origins,
        'hashes': r['evidence']['texts'][0]['hashes'], 'boundaries': [], 'issues': []}
    identity = [source_hash, part['name'], part['sha256'], scope['extractor_version'], scope['assembler_version'], scope['local_id']]
    scope['identity_sha256'] = digest(b'aletharsis.office-scope/1\0' + json.dumps(identity, separators=(',', ':')).encode())
    text_art = deepcopy(r['artifacts'][1]); text_art.update(artifact_ref='artifact/4', parents=[part['artifact_ref']],
        content_ref={'kind': 'office_scope', 'scope_ref': scope['scope_ref']}, transform=None,
        mapping={'quality': 'unavailable', 'reason_code': 'mapping.not_applicable'})
    artifacts.append(text_art); r['artifacts'] = artifacts
    filename = 'office-minimal.docx' if format == 'docx' else 'office-odt-minimal.odt'
    r['file'].update(path=filename, filename=filename, extension='.' + format,
        mime=('application/vnd.openxmlformats-officedocument.wordprocessingml.document' if format == 'docx'
              else 'application/vnd.oasis.opendocument.text'), format=format,
        parser='office-fixture/1', size=len(source), sha256=source_hash)
    for c in r['capabilities']:
        if c['id'] == 'aletharsis.parse.text': c['id'] = 'aletharsis.parse.office_package'
    for e in r['executions']:
        if e['capability_ref'] == 'aletharsis.parse.text': e['capability_ref'] = 'aletharsis.parse.office_package'
        for s in [e['requested_scope'], *e['analyzed_scope']]:
            if s['artifact_ref'] == 'artifact/1': s['artifact_ref'] = 'artifact/4'
    r['results'][0]['payload']['scope']['artifact_ref'] = 'artifact/4'
    r['anchors'][0].update(kind='office_scope', artifact_ref='artifact/4', mapping=text_art['mapping'],
        locator={'kind': 'office_scope', 'scope_ref': scope['scope_ref']})
    for f in r['findings']:
        old = f['location']; f['location'] = {'kind': 'office_offsets', 'scope_ref': scope['scope_ref'],
            'scope_character_offsets': old['character_offsets'], 'scope_byte_offsets': old['byte_offsets']}
    r['evidence'] = {'texts': [], 'metadata': {}, 'structure': {}, 'office': {
        'packages': [package], 'xml': [xml], 'scopes': [scope], 'metadata': [], 'relationships': [], 'objects': []}}
    return r, source


if __name__ == '__main__':
    for format in ('docx', 'odt'):
        report, source = build(format)
        name = Path(report['file']['filename'])
        (OUT / name.with_suffix('.json')).write_text(json.dumps(report, indent=2) + '\n')
        (OUT / name).write_bytes(source)
