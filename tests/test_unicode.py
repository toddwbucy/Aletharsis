import hashlib
import unicodedata

import pytest

from aletharsis.audit import audit
from aletharsis.analyzers.unicode import removable


@pytest.mark.parametrize("name", ["clean_ascii.txt", "normal_utf8.txt", "source.rst"])
def test_clean(fixtures, name):
    report = audit(fixtures / name)
    assert report.status == "completed"
    assert report.findings == []
    assert report.exit_code == 0


def test_isolated_is_not_encoding(fixtures):
    report = audit(fixtures / "isolated_zwsp.txt")
    finding, = report.findings
    assert finding.id == "unicode.zero_width"
    assert finding.severity == "LOW"
    assert finding.evidence["code_point"] == "U+200B"
    assert finding.location["byte_offsets"] == [11]
    assert finding.location["character_offsets"] == [11]
    assert report.exit_code == 1


def test_binary_pattern(fixtures):
    report = audit(fixtures / "binary_zero_width.txt")
    finding = next(f for f in report.findings if f.id == "pattern.zero_width_binary")
    assert finding.severity == "HIGH"
    assert finding.classification == "suspicious_pattern"
    assert finding.evidence["count"] == 64
    assert finding.evidence["adjacent_fraction"] == 1
    assert set(finding.evidence["alphabet"]) == {"U+200B", "U+200C"}
    assert report.exit_code == 3


@pytest.mark.parametrize("name,expected", [
    ("bidi.txt", "unicode.bidi_control"), ("variation_selectors.txt", "pattern.variation_selector_run"),
    ("tags.txt", "pattern.tag_run"), ("periodic.txt", "pattern.periodic_insertion"),
    ("mixed_script.txt", "text.mixed_script"), ("normalization.txt", "unicode.normalization"),
])
def test_specialized_findings(fixtures, name, expected):
    assert expected in {f.id for f in audit(fixtures / name).findings}


@pytest.mark.parametrize("name", ["normal_emoji.txt", "normal_persian.txt"])
def test_legitimate_formatting_not_encoding(fixtures, name):
    report = audit(fixtures / name)
    assert report.status == "completed"
    assert not any(f.category == "possible_steganography" for f in report.findings)
    assert all(f.severity in {"LOW", "INFO"} for f in report.findings)


@pytest.mark.parametrize("count", [12, 24])
def test_repeated_joined_emoji_not_periodic_encoding(tmp_path, count):
    """Regular emoji joiners remain inventory evidence, not encoding evidence."""
    path = tmp_path / "repeated_emoji.txt"
    path.write_text("\U0001f469\u200d\U0001f4bb" * count, encoding="utf-8")
    report = audit(path)
    assert report.status == "completed"
    assert not any(f.category == "possible_steganography" for f in report.findings)
    joiner, = [f for f in report.findings if f.evidence.get("code_point") == "U+200D"]
    assert joiner.id == "unicode.zero_width"
    assert joiner.evidence["count"] == count
    assert joiner.location["character_offsets"] == list(range(1, count * 3, 3))
    assert report.exit_code == 1


def test_required_inventory_and_hashes(tmp_path):
    text = "é" + "".join(chr(cp) for cp in [0x200B, 0x200C, 0x200D, 0x2060, 0x2061, 0x2062,
                                          0x2063, 0x2064, 0xFEFF, 0xAD, 0xA0, 0x202F, 0x2009, 0xE0100]) + "e\u0301"
    path = tmp_path / "unicode.txt"
    path.write_bytes(text.encode())
    report = audit(path)
    segment = report.evidence.texts[0]
    inventory = [f for f in report.findings if "code_point" in f.evidence]
    assert len(inventory) == 15
    assert next(f for f in inventory if f.evidence["code_point"] == "U+200B").location["byte_offsets"] == [2]
    digest = lambda value: hashlib.sha256(value.encode()).hexdigest()
    assert segment.hashes["raw_text_sha256"] == digest(text)
    assert segment.hashes["nfc_text_sha256"] == digest(unicodedata.normalize("NFC", text))
    assert segment.hashes["nfkc_text_sha256"] == digest(unicodedata.normalize("NFKC", text))
    assert segment.hashes["formatting_removed_sha256"] == digest("".join(c for c in text if not removable(c)))
    assert segment.normalization["nfc_changed"]


def test_contexts_bounded_but_offsets_complete(tmp_path):
    path = tmp_path / "many.txt"
    path.write_text("a\u200b" * 20, encoding="utf-8")
    finding = next(f for f in audit(path).findings if f.id == "unicode.zero_width")
    assert len(finding.location["byte_offsets"]) == 20
    assert len(finding.evidence["contexts"]) == 8
    assert finding.evidence["contexts_omitted"] == 12
