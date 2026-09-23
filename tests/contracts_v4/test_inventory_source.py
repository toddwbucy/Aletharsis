"""Independent source-binding checks for the captured native inventory fixture.

Report reproduction belongs to the increment-4 producer; these checks need only
Python's standard ZIP/hash support and run on the wire-contract branch itself.
"""
from copy import deepcopy
import hashlib
import io
import json
from pathlib import Path
import struct
import zipfile

import pytest

FIXTURES = Path(__file__).with_name('fixtures')


def verify_source(report, source):
    digest = lambda value: hashlib.sha256(value).hexdigest()
    assert report['file']['sha256'] == digest(source)
    assert report['file']['size'] == len(source)
    office = report['evidence']['office']
    package, = office['packages']
    assert package['source_sha256'] == digest(source)
    assert package['source_byte_length'] == len(source)
    parts = {}
    with zipfile.ZipFile(io.BytesIO(source)) as archive:
        assert sorted(archive.namelist()) == sorted(p['name'] for p in package['parts'])
        for part in package['parts']:
            info = archive.getinfo(part['name'])
            name_size, extra_size = struct.unpack_from('<HH', source, info.header_offset + 26)
            start = info.header_offset + 30 + name_size + extra_size
            end = start + info.compress_size
            assert part['compressed_span'] == {'start': start, 'end': end}
            assert part['compressed_sha256'] == digest(source[start:end])
            assert part['method'] == info.compress_type
            raw = archive.read(info)
            assert part['state'] == 'completed'
            assert part['byte_length'] == len(raw)
            assert part['sha256'] == digest(raw)
            parts[part['part_ref']] = raw
    xml = {x['xml_ref']: x for x in office['xml']}
    for record in [*office['scopes'], *office['metadata']]:
        raw = parts[record['part_ref']]
        text = record.get('text', record.get('lexical_value'))
        origins = record.get('origins', record.get('value_origins'))
        projected = []
        for origin in origins:
            # This fixture deliberately contains literal stored scalars only.
            assert origin['kind'] == 'stored'
            assert origin['transformation'] == 'literal'
            span = origin['source']
            value = raw[span['start']:span['end']].decode('utf-8')
            assert len(value) == 1
            local = origin['utf8']
            assert text.encode()[local['start']:local['end']].decode() == value
            document = xml[origin['xml_ref']]
            assert document['part_ref'] == record['part_ref']
            scalar = document['segments'][origin['segment']]['scalars'][origin['scalar']]
            assert scalar['source'] == span and scalar['code_point'] == ord(value)
            projected.append(value)
        assert ''.join(projected) == text
    for relationship in office['relationships']:
        loc = relationship['location']
        span = loc['span']
        assert parts[loc['part_ref']][span['start']:span['end']].startswith(b'<Relationship ')
    for obj in office['objects']:
        assert obj['sha256'] == digest(parts[obj['part_ref']])


def test_inventory_fixture_binds_every_source_part_and_origin():
    verify_source(json.loads((FIXTURES / 'metadata-inventory.json').read_bytes()),
                  (FIXTURES / 'metadata-inventory.docx').read_bytes())


@pytest.mark.parametrize('damage', ['source', 'compressed_hash', 'part_hash', 'scope_origin', 'metadata_origin'])
def test_inventory_source_check_rejects_mutations(damage):
    report = deepcopy(json.loads((FIXTURES / 'metadata-inventory.json').read_bytes()))
    source = (FIXTURES / 'metadata-inventory.docx').read_bytes()
    office = report['evidence']['office']
    if damage == 'source':
        source = source[:-1] + bytes([source[-1] ^ 1])
    elif damage in ('compressed_hash', 'part_hash'):
        key = 'compressed_sha256' if damage == 'compressed_hash' else 'sha256'
        office['packages'][0]['parts'][-1][key] = '0' * 64
    elif damage == 'scope_origin':
        office['scopes'][0]['origins'][0]['source']['start'] += 1
    else:
        next(m for m in office['metadata'] if m['value_origins'])['value_origins'][0]['source']['start'] += 1
    with pytest.raises(AssertionError):
        verify_source(report, source)
