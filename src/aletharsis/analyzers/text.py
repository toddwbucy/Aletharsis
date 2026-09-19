import re
import unicodedata as ud

from ..models import DocumentEvidence, Finding
from .unicode import escaped, location


class TextAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        findings = []
        for segment in evidence.texts:
            active = [name for name, count in segment.line_endings.items() if count]
            if len(active) > 1 or "cr" in active:
                findings.append(Finding(
                    "text.line_endings", "LOW", 1.0, "document_structure", "observed_fact",
                    "Mixed or legacy line endings detected",
                    "Line-ending differences can result from ordinary editing or platform conversion.",
                    segment.line_endings, {"source": segment.source}))
            if segment.bom:
                findings.append(Finding("text.bom", "INFO", 1.0, "document_structure", "observed_fact",
                                        "Byte order mark detected", "An initial BOM is an ordinary encoding marker.",
                                        {"hex": segment.bom, "encoding": segment.encoding},
                                        {"source": segment.source, "byte_offset": 0}))
            embedded = [i for i, c in enumerate(segment.text) if c == "\ufeff" and i != 0]
            if embedded:
                findings.append(Finding("text.embedded_bom", "LOW", 1.0, "document_structure", "observed_fact",
                                        "Embedded BOM character detected", "Possible concatenated text or embedded formatting marker.",
                                        {"count": len(embedded)}, location(segment, embedded)))
            for match in re.finditer(r"[^\W\d_]+", segment.text):
                scripts = {ud.name(c, "").split(" ")[0] for c in match[0] if c.isalpha()}
                if "LATIN" in scripts and scripts & {"CYRILLIC", "GREEK"}:
                    findings.append(Finding(
                        "text.mixed_script", "LOW", 0.6, "unicode", "suspicious_pattern",
                        "Mixed Latin and Greek/Cyrillic letters within a token",
                        "Mixed scripts can contain visually similar letters, but can also be intentional. This is not a comprehensive confusables detector.",
                        {"escaped_token": escaped(match[0]), "scripts": sorted(scripts)},
                        location(segment, [match.start()])))
        return findings
