from ..models import DocumentEvidence, Finding


class MetadataAnalyzer:
    def analyze(self, evidence: DocumentEvidence) -> list[Finding]:
        return [Finding("metadata.present", "INFO", 1.0, "metadata", "observed_fact",
                        f"{key} metadata present", "Metadata is evidence, not proof of watermarking.",
                        {"key": key, "value": value}) for key, value in sorted(evidence.metadata.items())]
