"""Inventory literal emoji-capable content without inferring its purpose."""

from collections import defaultdict
from functools import lru_cache
from importlib.resources import files
import unicodedata

from ..models import DocumentEvidence, Finding
from .unicode import escaped, location

EMOJI_VERSION = "17.0"
KEYCAP_BASES = frozenset("#*0123456789")


@lru_cache(maxsize=1)
def emoji_codepoints() -> frozenset[int]:
    """Load the bundled, versioned Unicode Emoji property without networking."""
    data = files("aletharsis").joinpath("data", f"emoji-{EMOJI_VERSION}.txt").read_text(encoding="utf-8")
    points: set[int] = set()
    for line in data.splitlines():
        value = line.split("#", 1)[0].strip()
        if not value:
            continue
        bounds = value.split("..")
        start, end = int(bounds[0], 16), int(bounds[-1], 16)
        points.update(range(start, end + 1))
    return frozenset(points)


class EmojiAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        """Flag emoji-capable code points, including those in comments/strings.

        Counts describe code points, not rendered emoji or grapheme clusters.
        ASCII digits, '#' and '*' only qualify as bases of keycap sequences.
        Text-presentation symbols remain explicit review evidence, not proof
        that a font rendered an emoji or that content is unnecessary.
        """
        findings = []
        points = emoji_codepoints()
        for segment in evidence.texts:
            inventory: dict[str, list[int]] = defaultdict(list)
            for index, character in enumerate(segment.text):
                if ord(character) not in points:
                    continue
                if character in KEYCAP_BASES and not (
                    segment.text.startswith("\u20e3", index + 1)
                    or segment.text.startswith("\ufe0f\u20e3", index + 1)
                ):
                    continue
                inventory[character].append(index)
            for character, positions in sorted(inventory.items(), key=lambda item: ord(item[0])):
                cp = f"U+{ord(character):04X}"
                name = unicodedata.name(character, "NAME UNAVAILABLE IN RUNTIME UNICODE DATABASE")
                findings.append(Finding(
                    "unicode.emoji", "LOW", 1.0, "unicode", "observed_fact",
                    f"Emoji-capable {cp} {name} detected",
                    "Review whether this content is required for the application or document. "
                    "Comments, docstrings, string literals and test data are not automatically exempt. "
                    "Presence does not establish hidden encoding, malicious intent or permission to remove it. "
                    "Some symbols also have ordinary text presentation; counts refer to code points, not rendered emoji.",
                    {"code_point": cp, "name": name, "count": len(positions),
                     "emoji_data_version": EMOJI_VERSION,
                     "match_basis": "keycap_base" if character in KEYCAP_BASES else "unicode_emoji_property",
                     "count_unit": "code_point", "requires_context_review": True,
                     "contexts": [{"character_offset": p,
                                   "escaped_text": escaped(segment.text[max(0, p - 16):p + 17])}
                                  for p in positions[:8]],
                     "contexts_omitted": max(0, len(positions) - 8)},
                    location(segment, positions),
                ))
        return findings
