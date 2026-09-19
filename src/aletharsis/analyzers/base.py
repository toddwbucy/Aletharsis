from typing import Protocol

from ..models import DocumentEvidence, Finding


class Analyzer(Protocol):
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]: ...
