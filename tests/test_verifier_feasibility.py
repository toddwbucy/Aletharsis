"""Offline evidence checks; live SDK execution remains a separate bounded study."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS = ROOT / "docs/reuse/evaluations/c2pa-verifier"


def read(name):
    return json.loads((ARTIFACTS / name).read_bytes())


def test_exact_artifact_identity_and_reproduction_pairs():
    """Both build/result identities substantiate the recorded comparisons."""
    manifest = read("manifest.json")
    assert set(manifest["measured_files"]) == {p.name for p in ARTIFACTS.iterdir() if p.name != "manifest.json"}
    for name, expected in manifest["measured_files"].items():
        assert hashlib.sha256((ARTIFACTS / name).read_bytes()).hexdigest() == expected
    for artifact in [*manifest["artifacts"].values(), manifest["results"]]:
        assert artifact["identical"] == (artifact["primary_sha256"] == artifact["reproduction_sha256"])
        assert artifact["identical"] is True
        if "fresh_driver_sha256" in artifact:
            assert artifact["fresh_driver_sha256"] == artifact["primary_sha256"]
    assert manifest["results"]["primary_sha256"] == manifest["measured_files"]["results.json"]


def test_final_case_expectations_and_source_integrity():
    """Retained summaries must agree with actual nested observations."""
    report = read("results.json")
    assert report["case_count"] == len(report["cases"]) == 11
    assert len({c["id"] for c in report["cases"]}) == 11
    assert report["expectation_mismatches"] == report["execution_failures"] == 0
    for case in report["cases"]:
        assert case["differences"] == []
        assert case["config_unchanged"] is True
        assert case["stderr_bytes"] == 0
        assert "execution_failure" not in case["observed"]
        for path, expected in case["expected"].items():
            value = case["observed"]
            for key in path.split("."):
                value = value[key]
            assert value == expected, (case["id"], path)
        if case["id"] != "oversized":
            assert case["observed"]["source_sha256"] == case["source_sha256"]
            assert case["observed"]["source_buffer_unchanged"] is True
            assert len(case["observed"]["effective_options_sha256"]) == 64
        validation = case["observed"].get("report", {}).get("validation_results", {})
        success = {s["code"] for s in validation.get("success", [])}
        assert set(case["required_success"]) <= success
        assert not set(case["forbidden_success"]) & success
        if case["expected_failures"] is not None:
            assert {s["code"] for s in validation["failure"]} == set(case["expected_failures"])


def test_integrity_binding_trust_and_absence_remain_distinct():
    """The evidence supports separate result dimensions, not a generic verdict."""
    cases = {c["id"]: c for c in read("results.json")["cases"]}
    def report(name):
        return cases[name]["observed"]["report"]
    assert report("reference-tampered-data-hash")["signature"] == "valid"
    assert report("reference-tampered-data-hash")["hard_binding"] == "mismatch"
    assert report("reference-tampered-claim-signature")["signature"] == "invalid"
    assert report("signed_no_trust")["trust"]["status"] == "not_evaluated"
    assert report("signed_default_trust")["integrity"] == "valid"
    assert report("signed_default_trust")["trust"]["status"] == "not_valid_for_supplied_material"
    assert report("signed_default_trust")["trust"]["revocation"]["status"] == "not_checked"
    assert report("unsigned_text")["present"] is False
    assert cases["signed_interactive"]["interactive"] is True


def test_exploratory_vocabulary_mismatch_is_preserved():
    """Correcting an expectation does not erase the initial failed observation."""
    initial = read("exploratory-results.json")
    assert initial["expectation_mismatches"] == 1
    failed = [c for c in initial["cases"] if c["differences"]]
    assert len(failed) == 1
    assert failed[0]["id"] == "signed_default_trust"
    assert failed[0]["expected"]["report.trust.status"] == "untrusted"
    assert failed[0]["observed"]["report"]["trust"]["status"] == "not_valid_for_supplied_material"


def test_cancellation_observed_a_live_worker_and_reaped_it():
    """A pre-completed worker would not prove process cancellation."""
    record = read("results.json")["cancellation"]
    assert record["worker_observed_live"] is True
    assert record["exit_code"] == -15
    assert record["reaped"] is True
    assert record["scope"] == "isolated worker, not in-process cancellation"


def test_locked_dependency_inventory():
    """The dependency record identifies sources/checksums without claiming clearance."""
    dependencies = read("dependencies.json")
    assert len(dependencies) == read("manifest.json")["dependency_packages"] == 153
    assert len({(d["name"], d["version"]) for d in dependencies}) == len(dependencies)
    for dependency in dependencies:
        assert dependency["declared_license"]
        if dependency["source"] is not None:
            assert len(dependency["lockfile_checksum"]) == 64
