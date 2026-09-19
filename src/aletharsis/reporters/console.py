import json

from ..safety import escaped
from ..models import Report


def render(report: Report, *, verbose: bool = False) -> str:
    identity = report.file
    summary = report.to_dict()["summary"]
    lines = ["ALETHARSIS FORENSIC AUDIT", "", "File",
             f"  Path: {escaped(identity.path)}", f"  Type: {identity.format} ({identity.mime})",
             f"  Size: {identity.size} bytes", f"  SHA256: {identity.sha256 or 'unavailable'}",
             f"  Parser: {identity.parser or 'unavailable'}", f"  Status: {report.status}",
             "", f"Summary: {summary['findings']} findings; exit code {report.exit_code}",
             "  " + "  ".join(f"{s.upper()} {summary[s]}" for s in ("high", "medium", "low", "info")), ""]
    for finding in report.findings:
        lines.extend([f"[{finding.severity}] {escaped(finding.title)}",
                      f"  ID: {finding.id} | Category: {finding.category} | {finding.classification}",
                      f"  Confidence: {finding.confidence:.2f}", f"  {escaped(finding.description)}"])
        if verbose:
            lines.append("  Evidence: " + json.dumps(finding.evidence, ensure_ascii=True, sort_keys=True))
            lines.append("  Location: " + json.dumps(finding.location, ensure_ascii=True, sort_keys=True))
        else:
            if "count" in finding.evidence:
                lines.append(f"  Occurrences: {finding.evidence['count']}")
            offsets = finding.location.get("byte_offsets", [])
            if offsets:
                lines.append(f"  Byte offsets (first 8): {offsets[:8]}")
            if finding.id == "parser.failure":
                lines.append("  Error: " + escaped(finding.evidence["message"]))
        lines.append("")
    if verbose:
        for segment in report.evidence.texts:
            lines.append("Text hashes: " + json.dumps(segment.hashes, sort_keys=True))
    lines.append("Limitations")
    lines.extend(f"  - {item}" for item in report.limitations)
    return "\n".join(lines) + "\n"
