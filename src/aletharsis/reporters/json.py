import json

from ..models import Report


def render(report: Report) -> str:
    return json.dumps(report.to_dict(), ensure_ascii=True, sort_keys=True, indent=2, allow_nan=False) + "\n"
