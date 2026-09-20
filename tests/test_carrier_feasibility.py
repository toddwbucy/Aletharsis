"""Verify retained experiment evidence offline; this is not a live detector test."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS = ROOT / "docs/reuse/evaluations/c2pa-text"


def test_retained_artifacts_match_recorded_hashes():
    """Logs/results and the fresh-run comparisons retain their recorded identity."""
    manifest = json.loads((ARTIFACTS / "manifest.json").read_bytes())
    assert set(manifest["measured_files"]) == {p.name for p in ARTIFACTS.iterdir() if p.name != "manifest.json"}
    for name, digest in manifest["measured_files"].items():
        assert hashlib.sha256((ARTIFACTS / name).read_bytes()).hexdigest() == digest
    assert manifest["reproduction_identical_results"] is True
    assert manifest["reproduction_identical_binary"] is True


def test_independent_inputs_and_expectations_match_recorded_run():
    """Changing a case cannot silently preserve an old, purportedly valid result."""
    spec = importlib.util.spec_from_file_location("carrier_cases", ROOT / "experiments/c2pa-text/cases.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    cases = module.corpus()
    report = json.loads((ARTIFACTS / "results.json").read_bytes())
    assert len(cases) == report["case_count"] == 39
    assert len({c["id"] for c in cases}) == len(cases)
    assert len(report["cases"]) == len(cases)
    for generated, retained in zip(cases, report["cases"], strict=True):
        assert generated["id"] == retained["id"]
        assert hashlib.sha256(generated["raw"]).hexdigest() == retained["source_sha256"]
        assert len(generated["raw"]) == retained["source_bytes"]
        assert generated["expected"] == retained["expected"]
        assert generated["rationale"] == retained["rationale"]
        assert generated["method"] == retained["method"]
        assert retained["differences"] == [k for k, v in retained["expected"].items() if retained["observed"].get(k) != v]
    assert report["execution_failures"] == 0
    assert report["expectation_mismatches"] == 8
    assert {c["id"] for c in report["cases"] if c["differences"]} == {
        "html_single", "html_upper", "html_attribute_space", "html_comment",
        "html_data_type", "html_prefix_tag", "html_body", "html_reference_single",
    }


def test_upstream_test_transcript_completed():
    """The retained transcript actually contains the claimed 28 passing tests."""
    events = [json.loads(line) for line in (ARTIFACTS / "upstream-tests.jsonl").read_bytes().splitlines()]
    assert not any(e["Action"] == "fail" for e in events)
    passed = [e["Test"] for e in events if e["Action"] == "pass" and "Test" in e]
    assert len(passed) == len(set(passed)) == 28
    assert "TestGoldenVectors" in passed
    assert "TestHTMLGoldenVectors" in passed
    assert events[-1]["Action"] == "pass"
