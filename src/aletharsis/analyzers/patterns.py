from collections import Counter

from ..models import DocumentEvidence, Finding
from .unicode import kind, location


def shortest_period(sequence: str) -> int | None:
    for period in range(1, min(32, len(sequence) // 3) + 1):
        if all(c == sequence[i % period] for i, c in enumerate(sequence)):
            return period
    return None


class PatternAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        findings = []
        for segment in evidence.texts:
            text = segment.text
            states = [i for i, c in enumerate(text) if c in "\u200b\u200c"]
            sequence = "".join(text[i] for i in states)
            counts = Counter(sequence)
            # Deliberately exclude ZWJ (common emoji usage). Two symbols alone
            # do not prove binary data; require density/spacing and sample size.
            if len(states) >= 32 and len(counts) == 2 and min(counts.values()) / len(states) >= 0.1:
                gaps = [b - a for a, b in zip(states, states[1:])]
                contiguous = sum(gap == 1 for gap in gaps) / len(gaps)
                regular = len(set(gaps)) == 1
                if contiguous >= 0.75 or regular:
                    findings.append(Finding(
                        "pattern.zero_width_binary", "HIGH", 0.8, "possible_steganography", "suspicious_pattern",
                        "Possible zero-width binary encoding",
                        "A two-symbol ZWSP/ZWNJ alphabet forms a long contiguous or regularly spaced sequence. This is consistent with an invisible encoding scheme; no payload, author or watermark intent is confirmed.",
                        {"count": len(states), "alphabet": {f"U+{ord(c):04X}": n for c, n in sorted(counts.items())},
                         "adjacent_fraction": contiguous, "constant_spacing": regular,
                         "sequence_period": shortest_period(sequence),
                         "thresholds": {"minimum_count": 32, "minimum_minority_fraction": 0.1,
                                        "minimum_adjacent_fraction_or_constant_spacing": 0.75}},
                        location(segment, states)))
            for group in ("variation_selector", "tag"):
                positions = [i for i, c in enumerate(text) if kind(c) == group]
                runs: list[list[int]] = []
                for p in positions:
                    if runs and runs[-1][-1] + 1 == p:
                        runs[-1].append(p)
                    else:
                        runs.append([p])
                for run in runs:
                    threshold = 16 if group == "tag" else 8
                    if len(run) < threshold:
                        continue
                    details = {"count": len(run), "minimum_run": threshold,
                               "alphabet_size": len({text[p] for p in run})}
                    if group == "tag":
                        details["ascii_projection"] = "".join(chr(ord(text[p]) - 0xE0000)
                                                              for p in run if 0xE0020 <= ord(text[p]) <= 0xE007E)
                    findings.append(Finding(
                        f"pattern.{group}_run", "MEDIUM", 0.75, "possible_steganography", "suspicious_pattern",
                        f"Long contiguous {group.replace('_', ' ')} sequence",
                        "This run is consistent with encoded character data. Tags and variation selectors also have legitimate uses; interpretation requires review.",
                        details, location(segment, run)))
            positions = [i for i, c in enumerate(text) if kind(c) == "zero_width" and not (i == 0 and c == "\ufeff")]
            if len(positions) >= 12:
                gaps = [b - a for a, b in zip(positions, positions[1:])]
                if len(set(gaps)) == 1 and gaps[0] > 1:
                    findings.append(Finding(
                        "pattern.periodic_insertion", "MEDIUM", 0.7, "possible_steganography", "suspicious_pattern",
                        "Periodic invisible-character insertion",
                        "Invisible formatting characters recur at an exact character interval. Formatting conventions may also produce this pattern.",
                        {"count": len(positions), "character_interval": gaps[0], "minimum_count": 12},
                        location(segment, positions)))
        return findings
