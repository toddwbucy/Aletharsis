"""Emoji presence is reviewable evidence even in legitimate-looking code."""

import codecs

import pytest

from aletharsis.audit import audit


def emoji_findings(report):
    """Select the independent emoji inventory, excluding formatting findings."""
    return [finding for finding in report.findings if finding.id == "unicode.emoji"]


@pytest.mark.parametrize("suffix", [".py", ".js", ".ts", ".rs", ".go", ".c", ".sh", ".md", ".txt", ".emojic"])
def test_emoji_detected_in_text_and_source_files(tmp_path, suffix):
    """Detection does not depend on a small source-code extension allowlist."""
    path = tmp_path / ("source" + suffix)
    path.write_text("# Note 😀\n", encoding="utf-8")
    report = audit(path)
    finding, = emoji_findings(report)
    assert finding.severity == "LOW"
    assert finding.classification == "observed_fact"
    assert finding.evidence["code_point"] == "U+1F600"
    assert finding.evidence["emoji_data_version"] == "17.0"
    assert finding.location["character_offsets"] == [7]
    assert finding.location["byte_offsets"] == [7]
    assert report.exit_code == 1


def test_comments_docstrings_and_application_strings_all_reported(tmp_path):
    """Application justification belongs to review, not automatic exemptions."""
    path = tmp_path / "app.py"
    content = '# 😀 comment\n"""😀 docstring"""\nstatus = "😀"\n'
    path.write_text(content, encoding="utf-8")
    finding, = emoji_findings(audit(path))
    assert finding.evidence["count"] == 3
    assert finding.evidence["requires_context_review"] is True
    assert finding.location["character_offsets"] == [i for i, c in enumerate(content) if c == "😀"]
    assert finding.location["byte_offsets"] == [len(content[:i].encode()) for i, c in enumerate(content) if c == "😀"]


@pytest.mark.parametrize("content,expected", [
    ("👩\u200d💻" * 12, {"U+1F469": 12, "U+1F4BB": 12}),
    ("🇬🇧", {"U+1F1EC": 1, "U+1F1E7": 1}),
    ("👍🏽", {"U+1F44D": 1, "U+1F3FD": 1}),
    ("1\ufe0f\u20e3 #\u20e3 *\ufe0f\u20e3", {"U+0031": 1, "U+0023": 1, "U+002A": 1}),
    ("© ©\ufe0e ©\ufe0f", {"U+00A9": 3}),
    ("\U0001faea", {"U+1FAEA": 1}),  # Present in Emoji 17.0, regardless of Python's name database.
])
def test_components_and_text_presentation_still_visible(tmp_path, content, expected):
    """Inventory component code points without inventing a glyph count."""
    path = tmp_path / "content.py"
    path.write_text(content, encoding="utf-8")
    report = audit(path)
    findings = emoji_findings(report)
    assert {f.evidence["code_point"]: f.evidence["count"] for f in findings} == expected
    assert all(f.evidence["count_unit"] == "code_point" for f in findings)
    assert not any(f.category == "possible_steganography" for f in report.findings)


def test_ascii_and_literal_escapes_are_not_emoji(tmp_path):
    """Ordinary syntax and undecoded ASCII escapes must not become emoji."""
    path = tmp_path / "arithmetic.py"
    path.write_text('value = 123 * 456 # plain comment\nescaped = "\\U0001F600"\n', encoding="utf-8")
    assert emoji_findings(audit(path)) == []


def test_utf16_offsets_and_all_occurrences(tmp_path):
    """Keep original byte offsets and complete positions beyond context samples."""
    path = tmp_path / "utf16.py"
    path.write_bytes(codecs.BOM_UTF16_LE + ("é😀" * 10).encode("utf-16-le"))
    finding, = emoji_findings(audit(path))
    assert finding.evidence["count"] == 10
    assert finding.location["character_offsets"] == list(range(2, 21, 2))
    assert finding.location["byte_offsets"] == list(range(4, 59, 6))
    assert len(finding.evidence["contexts"]) == 8
    assert finding.evidence["contexts_omitted"] == 2
