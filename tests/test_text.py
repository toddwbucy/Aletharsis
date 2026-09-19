import codecs
import hashlib
import io
import os
import zipfile

import pytest

from aletharsis.audit import audit


@pytest.mark.parametrize("name,mime", [("source.html", "text/html"), ("source.xml", "application/xml"),
                                     ("source.json", "application/json"), ("mixed_endings.csv", "text/csv"),
                                     ("identifiers.md", "text/markdown")])
def test_source_formats(fixtures, name, mime):
    report = audit(fixtures / name)
    assert report.status == "completed"
    assert report.file.mime == mime
    assert report.file.parser == "text-source-v1"


def test_raw_source_no_entity_or_json_decoding(fixtures):
    report = audit(fixtures / "source.json")
    finding, = [f for f in report.findings if f.id == "unicode.zero_width"]
    assert finding.evidence["count"] == 1
    assert "\\u200b" in report.evidence.texts[0].text


def test_provenance_and_identifiers(fixtures):
    report = audit(fixtures / "identifiers.md")
    finding = next(f for f in report.findings if f.id == "identifier.uuid")
    assert finding.evidence["count"] == 2
    assert finding.evidence["repeated"]
    assert report.exit_code == 2
    assert "provenance.text_marker" in {f.id for f in audit(fixtures / "source.html").findings}


def test_line_endings_boms_controls(fixtures):
    report = audit(fixtures / "mixed_endings.csv")
    assert report.evidence.texts[0].line_endings == {"lf": 1, "crlf": 1, "cr": 1}
    assert "text.line_endings" in {f.id for f in report.findings}
    report = audit(fixtures / "bom.txt")
    assert {"text.bom", "text.embedded_bom", "unicode.zero_width"} <= {f.id for f in report.findings}
    assert report.evidence.texts[0].byte_offsets[:2] == [0, 3]
    assert any(f.id == "unicode.control" for f in audit(fixtures / "control.txt").findings)


@pytest.mark.parametrize("encoding,bom", [("utf-16-le", codecs.BOM_UTF16_LE), ("utf-16-be", codecs.BOM_UTF16_BE),
                                          ("utf-32-le", codecs.BOM_UTF32_LE), ("utf-32-be", codecs.BOM_UTF32_BE)])
def test_unicode_encodings(tmp_path, encoding, bom):
    path = tmp_path / "encoded.txt"
    data = bom + "é😀\u200b".encode(encoding)
    path.write_bytes(data)
    report = audit(path)
    assert report.status == "completed"
    segment = report.evidence.texts[0]
    assert segment.text == "\ufeffé😀\u200b"
    assert segment.byte_offsets[-1] == len(data)
    finding = next(f for f in report.findings if f.evidence.get("code_point") == "U+200B")
    assert finding.location["byte_offsets"] == [len(bom) + len("é😀".encode(encoding))]


def test_strict_decode(fixtures):
    report = audit(fixtures / "invalid_utf8.txt")
    assert report.status == "failed"
    assert report.exit_code == 4
    assert report.findings[-1].evidence["byte_start"] == 12
    assert report.file.sha256


@pytest.mark.parametrize("data,format", [(b"%PDF-1.7\n", "pdf"), (b"\x89PNG\r\n", "binary"),
                                       (b"PK\x03\x04broken", "zip"), (b"\x00" * 10, "binary")])
def test_signatures_override_text_extension(tmp_path, data, format):
    path = tmp_path / "misleading.txt"
    path.write_bytes(data)
    report = audit(path)
    assert report.file.format == format
    assert report.exit_code == 4


def test_docx_identification(tmp_path):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        archive.writestr("[Content_Types].xml", "<Types/>")
        archive.writestr("word/document.xml", "<document/>")
    path = tmp_path / "misleading.txt"
    path.write_bytes(buffer.getvalue())
    report = audit(path)
    assert report.file.format == "docx"
    assert report.exit_code == 4


def test_extension_not_required(tmp_path):
    path = tmp_path / "unknown.data"
    path.write_text("Unicode prose é\n", encoding="utf-8")
    assert audit(path).file.mime == "text/plain"
    assert audit(path).exit_code == 0


def test_read_integrity_and_determinism(tmp_path):
    path = tmp_path / "original.txt"
    data = "é hidden\u200b text\n".encode()
    path.write_bytes(data)
    os.utime(path, ns=(1_000_000_000, 2_000_000_000))
    before = path.stat()
    first = audit(path)
    second = audit(path)
    after = path.stat()
    assert first.status == "completed"
    assert first.to_dict() == second.to_dict()
    assert first.file.sha256 == hashlib.sha256(data).hexdigest()
    assert (before.st_atime_ns, before.st_mtime_ns, before.st_ctime_ns, before.st_size) == (
        after.st_atime_ns, after.st_mtime_ns, after.st_ctime_ns, after.st_size)
    assert path.read_bytes() == data


def test_limits_missing_directory_symlink(tmp_path):
    assert audit(tmp_path / "missing").exit_code == 4
    assert audit(tmp_path).exit_code == 4
    path = tmp_path / "large.txt"
    path.write_bytes(b"12345")
    assert audit(path, max_bytes=4).exit_code == 4
    link = tmp_path / "link.txt"
    link.symlink_to(path)
    assert audit(link).exit_code == 4


def test_empty(tmp_path):
    path = tmp_path / "empty.txt"
    path.write_bytes(b"")
    report = audit(path)
    assert report.exit_code == 0
    assert report.evidence.texts[0].byte_offsets == [0]


def test_base64_candidate(tmp_path):
    path = tmp_path / "encoded.txt"
    path.write_text("cHJvdmVuYW5jZS1pZC0xMjM0NTY3ODkwYWJjZGVm", encoding="utf-8")
    assert any(f.id == "text.encoded_candidate" for f in audit(path).findings)
