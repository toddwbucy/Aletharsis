"""Foundation tests also run before audit-engine integration."""

import codecs
import json
from pathlib import Path

from aletharsis.identify import identify
from aletharsis.models import FileIdentity, Finding, Report
from aletharsis.parsers.text import TextParser
from aletharsis.reporters.json import render


def test_content_identification_overrides_extension():
    assert identify(b"%PDF-1.7\n", Path("notes.txt"))[:2] == ("pdf", "application/pdf")
    assert identify(b"ordinary text", Path("notes"))[:2] == ("text", "text/plain")


def test_parser_preserves_bom_and_original_byte_offsets():
    source = codecs.BOM_UTF16_LE + "é😀\r\n".encode("utf-16-le")
    segment, = TextParser().parse(source).texts
    assert segment.text == "\ufeffé😀\r\n"
    assert segment.byte_offsets == [0, 2, 4, 8, 10, 12]
    assert segment.line_endings == {"crlf": 1, "lf": 0, "cr": 0}


def test_report_summary_and_json_are_deterministic():
    report = Report("0.1.0", "1.0", FileIdentity("file.txt", "file.txt", ".txt"))
    assert report.exit_code == 0
    report.findings.append(Finding(
        "test.observation", "MEDIUM", 1.0, "identifier", "observed_fact",
        "Example identifier", "Test evidence", {"value": "é"}))
    result = render(report)
    assert result == render(report)
    assert "\\u00e9" in result
    assert json.loads(result)["summary"] == {
        "findings": 1, "high": 0, "medium": 1, "low": 0, "info": 0, "exit_code": 2}
    report.status = "failed"
    assert report.exit_code == 4
