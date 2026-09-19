"""Content signatures take precedence over extension hints."""

import codecs
import io
from pathlib import Path
import zipfile

TEXT_TYPES = {
    ".txt": "text/plain", ".md": "text/markdown", ".rst": "text/x-rst",
    ".csv": "text/csv", ".json": "application/json", ".xml": "application/xml",
    ".html": "text/html", ".htm": "text/html",
}


def encoding_for(data: bytes) -> tuple[str, str | None]:
    # Check UTF-32 first: its LE BOM begins with the UTF-16 LE BOM.
    for marker, encoding in ((codecs.BOM_UTF32_LE, "utf-32-le"),
                             (codecs.BOM_UTF32_BE, "utf-32-be"),
                             (codecs.BOM_UTF8, "utf-8"),
                             (codecs.BOM_UTF16_LE, "utf-16-le"),
                             (codecs.BOM_UTF16_BE, "utf-16-be")):
        if data.startswith(marker):
            return encoding, marker.hex()
    return "utf-8", None


def identify(data: bytes, path: Path) -> tuple[str, str, str]:
    if data[:1024].lstrip().startswith(b"%PDF-"):
        return "pdf", "application/pdf", "PDF signature"
    if data.startswith((b"PK\x03\x04", b"PK\x05\x06", b"PK\x07\x08")):
        try:
            with zipfile.ZipFile(io.BytesIO(data)) as archive:
                names = set(archive.namelist())
            if {"[Content_Types].xml", "word/document.xml"} <= names:
                return "docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "OOXML archive members"
        except (zipfile.BadZipFile, ValueError):
            pass
        return "zip", "application/zip", "ZIP signature"
    for signature in (b"\x89PNG", b"\xff\xd8\xff", b"GIF87a", b"GIF89a", b"\x7fELF", b"\x1f\x8b", b"\xd0\xcf\x11\xe0"):
        if data.startswith(signature):
            return "binary", "application/octet-stream", "binary signature"
    encoding, bom = encoding_for(data)
    try:
        source = data.decode(encoding)
    except UnicodeDecodeError:
        if path.suffix.lower() in TEXT_TYPES:
            return "text", TEXT_TYPES[path.suffix.lower()], "text extension; decoding requires validation"
        return "binary", "application/octet-stream", "not valid supported Unicode text"
    if not bom and source and sum(ord(c) < 32 and c not in "\t\n\r" for c in source) / len(source) > 0.1:
        return "binary", "application/octet-stream", "high binary-control density"
    head = source.lstrip("\ufeff \t\r\n").lower()
    if head.startswith(("<!doctype html", "<html")):
        return "text", "text/html", "HTML signature; source text inspection"
    if head.startswith("<?xml"):
        return "text", "application/xml", "XML declaration; source text inspection"
    return "text", TEXT_TYPES.get(path.suffix.lower(), "text/plain"), "valid Unicode text; extension is a MIME hint"
