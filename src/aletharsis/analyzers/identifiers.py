import base64
from collections import defaultdict
import re

from ..models import DocumentEvidence, Finding
from .unicode import escaped, location

UUID = re.compile(r"(?i)(?<![\w-])[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}(?![\w-])")
MARKER = re.compile(r"(?i)\b(?:generated\s+by|watermark|provenance|tracking[-_ ]id|document[-_ ]id|author)\s*[:=]\s*[^\r\n<>]{1,160}")
ENCODED = re.compile(r"(?<![\w+/])(?:[A-Za-z0-9+/]{32,}={0,2})(?![\w+/=])")


class IdentifierAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        findings = []
        for segment in evidence.texts:
            identifiers: dict[str, list[int]] = defaultdict(list)
            for match in UUID.finditer(segment.text):
                identifiers[match[0]].append(match.start())
            for value, positions in sorted(identifiers.items()):
                findings.append(Finding(
                    "identifier.uuid", "MEDIUM", 0.99, "identifier", "observed_fact",
                    "UUID-like identifier present",
                    "A UUID-shaped value may serve as a persistent identifier. Its persistence, uniqueness, purpose and origin cannot be inferred from syntax alone.",
                    {"value": value, "count": len(positions), "repeated": len(positions) > 1}, location(segment, positions)))
            for match in MARKER.finditer(segment.text):
                findings.append(Finding(
                    "provenance.text_marker", "LOW", 0.9, "provenance", "observed_fact",
                    "Explicit provenance or identifier label detected",
                    "The source contains a provenance-related label, possibly in a comment. The statement is unverified and could be an example or ordinary prose.",
                    {"escaped_marker": escaped(match[0])}, location(segment, [match.start()])))
            for match in ENCODED.finditer(segment.text):
                value = match[0]
                if len(value) % 4 or not (any(c.isdigit() for c in value) and any(c.isalpha() for c in value)):
                    continue
                try:
                    decoded = base64.b64decode(value, validate=True)
                except ValueError:
                    continue
                findings.append(Finding(
                    "text.encoded_candidate", "INFO", 0.5, "embedded_content", "suspicious_pattern",
                    "Long encoding-compatible token detected",
                    "This alphanumeric token is syntactically valid Base64; hashes, identifiers and ordinary application data can match. Decoded content is not executed or recursively analyzed.",
                    {"value": value, "character_count": len(value), "base64_decoded_bytes": len(decoded)},
                    location(segment, [match.start()])))
        return findings
