"""Unicode inventory and explicitly defined, in-memory comparison views."""

from collections import defaultdict
import hashlib
import unicodedata as ud

from ..models import DocumentEvidence, Finding, TextEvidence
from ..safety import escaped

ZERO_WIDTH = set(range(0x200B, 0x200E)) | set(range(0x2060, 0x2065)) | {0xFEFF}
BIDI = {0x061C, 0x200E, 0x200F} | set(range(0x202A, 0x202F)) | set(range(0x2066, 0x206A))


def kind(character: str) -> str | None:
    cp = ord(character)
    if cp in ZERO_WIDTH:
        return "zero_width"
    if cp in BIDI:
        return "bidi_control"
    if 0xFE00 <= cp <= 0xFE0F or 0xE0100 <= cp <= 0xE01EF:
        return "variation_selector"
    if 0xE0000 <= cp <= 0xE007F:
        return "tag"
    if cp == 0xAD:
        return "soft_hyphen"
    if character.isspace() and character not in " \t\r\n":
        return "unusual_whitespace"
    if ud.category(character) in ("Cc", "Cf") and character not in "\t\r\n":
        return "control"
    if ud.category(character).startswith("M"):
        return "combining"
    return None


def removable(character: str) -> bool:
    return kind(character) in {"zero_width", "bidi_control", "variation_selector", "tag", "soft_hyphen"}


def location(text: TextEvidence, positions: list[int]) -> dict:
    return {"source": text.source, "character_offsets": positions,
            "byte_offsets": [text.byte_offsets[p] for p in positions]}


def sha(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


class UnicodeAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        findings = []
        for segment in evidence.texts:
            source = segment.text
            nfc, nfkc = ud.normalize("NFC", source), ud.normalize("NFKC", source)
            stripped = "".join(c for c in source if not removable(c))
            segment.hashes = {"raw_text_sha256": sha(source), "nfc_text_sha256": sha(nfc),
                              "nfkc_text_sha256": sha(nfkc), "formatting_removed_sha256": sha(stripped)}
            segment.normalization = {"nfc_changed": source != nfc, "nfkc_changed": source != nfkc,
                                     "formatting_removed_count": len(source) - len(stripped),
                                     "unicode_version": ud.unidata_version,
                                     "hash_encoding": "utf-8",
                                     "removal_policy": "zero_width,bidi_control,variation_selector,tag,soft_hyphen"}
            inventory: dict[str, list[int]] = defaultdict(list)
            for offset, character in enumerate(source):
                if kind(character):
                    inventory[character].append(offset)
            for character, positions in sorted(inventory.items(), key=lambda pair: ord(pair[0])):
                group = kind(character)
                ordinary = group in {"combining", "unusual_whitespace", "soft_hyphen", "variation_selector"}
                leading_bom = character == "\ufeff" and positions == [0] and segment.bom is not None
                severity = "INFO" if ordinary or leading_bom else "LOW"
                if group == "control":
                    severity = "MEDIUM"
                findings.append(Finding(
                    id=f"unicode.{group}", severity=severity, confidence=1.0,
                    category="unicode", classification="observed_fact",
                    title=f"U+{ord(character):04X} {ud.name(character, 'UNNAMED CONTROL')} detected",
                    description="Character occurrences are observable facts. Language, typography, emoji and formatting can explain legitimate uses; presence alone does not establish hidden data.",
                    evidence={"code_point": f"U+{ord(character):04X}", "name": ud.name(character, "UNNAMED CONTROL"),
                              "count": len(positions), "leading_bom": leading_bom,
                              "contexts": [{"character_offset": p, "escaped_text": escaped(source[max(0, p-16):p+17])}
                                           for p in positions[:8]],
                              "contexts_omitted": max(0, len(positions) - 8)},
                    location=location(segment, positions),
                ))
            if source != nfc or source != nfkc:
                findings.append(Finding(
                    "unicode.normalization", "INFO", 1.0, "unicode", "observed_fact",
                    "Unicode normalization changes extracted text",
                    "Canonical or compatibility equivalents differ. This is common in normal text and does not imply a watermark.",
                    {"nfc_changed": source != nfc, "nfkc_changed": source != nfkc}, {"source": segment.source}))
        return findings
