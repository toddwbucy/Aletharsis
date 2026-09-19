"""Stable, serializable evidence types. Offsets are zero-based."""

from dataclasses import asdict, dataclass, field
from typing import Any, Literal

Severity = Literal["INFO", "LOW", "MEDIUM", "HIGH"]
Classification = Literal["observed_fact", "suspicious_pattern", "likely_mechanism", "undetermined"]
Category = Literal["metadata", "unicode", "hidden_content", "document_structure",
                   "embedded_content", "identifier", "visual_watermark",
                   "possible_steganography", "provenance", "parser"]
RANK = {"INFO": 1, "LOW": 1, "MEDIUM": 2, "HIGH": 3}


@dataclass
class Finding:
    id: str
    severity: Severity
    confidence: float
    category: Category
    classification: Classification
    title: str
    description: str
    evidence: dict[str, Any] = field(default_factory=dict)
    location: dict[str, Any] = field(default_factory=dict)


@dataclass
class FileIdentity:
    path: str
    filename: str
    extension: str
    mime: str = "application/octet-stream"
    format: str = "unknown"
    identification_basis: str = "unavailable"
    size: int | None = None
    sha256: str | None = None
    parser: str | None = None


@dataclass
class TextEvidence:
    source: str
    text: str
    encoding: str
    byte_offsets: list[int]
    bom: str | None = None
    line_endings: dict[str, int] = field(default_factory=dict)
    hashes: dict[str, str] = field(default_factory=dict)
    normalization: dict[str, Any] = field(default_factory=dict)


@dataclass
class DocumentEvidence:
    texts: list[TextEvidence] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)
    structure: dict[str, Any] = field(default_factory=dict)


@dataclass
class Report:
    aletharsis_version: str
    schema_version: str
    file: FileIdentity
    status: Literal["completed", "failed"] = "completed"
    evidence: DocumentEvidence = field(default_factory=DocumentEvidence)
    findings: list[Finding] = field(default_factory=list)
    limitations: list[str] = field(default_factory=list)

    @property
    def exit_code(self) -> int:
        return 4 if self.status == "failed" else max(
            (RANK[f.severity] for f in self.findings), default=0)

    def to_dict(self) -> dict[str, Any]:
        result = asdict(self)
        result["summary"] = {
            "findings": len(self.findings),
            **{s.lower(): sum(f.severity == s for f in self.findings)
               for s in ("HIGH", "MEDIUM", "LOW", "INFO")},
            "exit_code": self.exit_code,
        }
        return result
