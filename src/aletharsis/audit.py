"""Read a bounded snapshot, parse it, then run independent analyzers."""

import hashlib
import os
from pathlib import Path
import stat

from . import __version__
from .analyzers.base import Analyzer
from .analyzers.identifiers import IdentifierAnalyzer
from .analyzers.metadata import MetadataAnalyzer
from .analyzers.patterns import PatternAnalyzer
from .analyzers.text import TextAnalyzer
from .analyzers.unicode import UnicodeAnalyzer
from .identify import identify
from .models import FileIdentity, Finding, Report
from .parsers.base import Parser
from .parsers.text import TextParser

MAX_BYTES = 8 * 1024 * 1024
PARSERS: dict[str, Parser] = {"text": TextParser()}
ANALYZERS: tuple[Analyzer, ...] = (UnicodeAnalyzer(), TextAnalyzer(), PatternAnalyzer(),
                                  IdentifierAnalyzer(), MetadataAnalyzer())
LIMITATIONS = [
    "A suspicious artifact is not necessarily a watermark. Observable evidence and structural patterns do not establish intent or provenance.",
    "Statistical model-output watermarks cannot be determined by these analyzers. No AI authorship or vendor attribution is performed.",
    "M0/M1 inspect literal text source only: HTML/XML markup, JSON escapes, entities and rendered content are not decoded or interpreted. Structured metadata is not extracted.",
    "DOCX/PDF and batch auditing are not implemented in this milestone. Absence of findings is not proof of absence of a watermark.",
    "Pattern thresholds are heuristics, not calibrated probabilities. Confidence describes support for the stated observation or pattern, not watermark intent.",
]


def read_snapshot(path: Path, max_bytes: int) -> bytes:
    if not hasattr(os, "O_NOATIME"):
        raise OSError("This platform lacks O_NOATIME; strict timestamp-preserving reads are unavailable")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOATIME | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        before = os.fstat(stream.fileno())
        if not stat.S_ISREG(before.st_mode):
            raise ValueError("Only regular files are supported; directory auditing is planned for M4")
        if before.st_size > max_bytes:
            raise ValueError(f"File exceeds the {max_bytes}-byte analysis limit")
        data = stream.read(max_bytes + 1)
        after = os.fstat(stream.fileno())
    if len(data) > max_bytes:
        raise ValueError(f"File exceeds the {max_bytes}-byte analysis limit")
    if (before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (after.st_size, after.st_mtime_ns, after.st_ctime_ns):
        raise ValueError("Source changed during snapshot acquisition; evidence may be inconsistent")
    return data


def audit(path: Path, *, max_bytes: int = MAX_BYTES) -> Report:
    path = Path(path)
    identity = FileIdentity(str(path), path.name, path.suffix.lower())
    report = Report(__version__, "1.0", identity, limitations=list(LIMITATIONS))
    try:
        data = read_snapshot(path, max_bytes)
        identity.size = len(data)
        identity.sha256 = hashlib.sha256(data).hexdigest()
        identity.format, identity.mime, identity.identification_basis = identify(data, path)
        parser = PARSERS.get(identity.format)
        if parser is None:
            raise ValueError(f"Unsupported format: {identity.format}; M0/M1 support Unicode text source files")
        identity.parser = parser.name
        report.evidence = parser.parse(data)
        for analyzer in ANALYZERS:
            report.findings.extend(analyzer.analyze(report.evidence))
        report.findings.sort(key=lambda f: (-{"INFO": 0, "LOW": 1, "MEDIUM": 2, "HIGH": 3}[f.severity], f.id,
                                            str(f.location), f.title))
    except (OSError, ValueError) as exc:
        report.status = "failed"
        details: dict = {"error_type": type(exc).__name__, "message": str(exc)}
        if isinstance(exc, UnicodeDecodeError):
            details.update(byte_start=exc.start, byte_end=exc.end, reason=exc.reason)
        report.findings.append(Finding(
            "parser.failure", "INFO", 1.0, "parser", "observed_fact", "Audit could not complete",
            "The file was not fully analyzed. Review the error before interpreting findings.", details))
    return report
