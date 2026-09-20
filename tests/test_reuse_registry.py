"""Offline planning-registry contract and exact upstream license evidence."""

from copy import deepcopy
import hashlib
import json
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker
import pytest

ROOT = Path(__file__).resolve().parents[1]
DIRECTORY = ROOT / "docs/reuse"
REGISTRY = json.loads((DIRECTORY / "candidates.json").read_bytes())
SCHEMA = json.loads((ROOT / "schemas/reuse-registry.schema.json").read_bytes())
VALIDATOR = Draft202012Validator(SCHEMA, format_checker=FormatChecker())
EXPECTED_REPOSITORIES = {
    "encypherai/c2pa-text", "encypherai/encypher-c2pa",
    "juriku/hidden-characters-detector", "Hiberius/hiberius-unicode-toolkit",
    "yeshan-jun/invisible-character-detector", "decalage2/oletools",
    "ridpath/pdfscapel", "THU-BPM/MarkLLM", "pasquini-dario/LLMmap",
    "dreamor/llm-fingerprint", "Clevis22/StegZero",
}


def sample():
    """Isolate a record for schema mutations without changing inventory."""
    value = deepcopy(REGISTRY)
    value["components"] = value["components"][:1]
    return value, value["components"][0]


def test_inventory_and_schema():
    """Every originally scoped candidate is pinned and represented once."""
    Draft202012Validator.check_schema(SCHEMA)
    VALIDATOR.validate(REGISTRY)
    records = REGISTRY["components"]
    assert len(records) == len(EXPECTED_REPOSITORIES)
    assert len({r["id"] for r in records}) == len(records)
    assert {r["repository"].removeprefix("https://github.com/") for r in records} == EXPECTED_REPOSITORIES


@pytest.mark.parametrize("record", REGISTRY["components"], ids=lambda r: r["id"])
def test_pinned_license_evidence(record):
    """Original license bytes and source link agree with the recorded pin."""
    license_info = record["license"]
    assert hashlib.sha256((DIRECTORY / license_info["retained_path"]).read_bytes()).hexdigest() == license_info["sha256"]
    assert license_info["source_url"] == f'{record["repository"]}/blob/{record["pin"]["commit"]}/{license_info["upstream_path"]}'
    assert record["assessment"]["readme_url"].startswith(f'{record["repository"]}/blob/{record["pin"]["commit"]}/')
    assert record["assessment"]["tree_listing_truncated"] is False


@pytest.mark.parametrize("key,value", [
    ("integration_class", "plugin"), ("adoption_status", "ready"),
    ("adoption_status", "approved"), ("adoption_status", "rejected"),
    ("adoption_status", "retired"), ("adoption_status", "deferred"),
])
def test_unknown_roles_and_unsupported_decisions_rejected(key, value):
    """A role cannot smuggle approval; every terminal disposition needs evidence."""
    document, record = sample()
    record[key] = value
    assert not VALIDATOR.is_valid(document)


def test_proposed_does_not_claim_production_or_redistribution():
    """Planning records cannot assert shipped capabilities or copied code."""
    for path in ("production", "upstream_code", "fixtures"):
        document, record = sample()
        if path == "production":
            record["supported_scope"][path] = ["all formats"]
        else:
            record["redistribution"][path] = True
        assert not VALIDATOR.is_valid(document)


def approved_example():
    """Synthetic reviewed record for testing the schema, not an adoption decision."""
    document, record = sample()
    record["adoption_status"] = "approved"
    record["license"]["clearance"] = "reviewed"
    record["runtime"]["assessment"] = "verified"
    record["assessment"].update(vulnerability_review="reviewed", build_reproducibility="verified")
    record["decision"] = {
        "disposition": "approve", "record": "https://github.com/toddwbucy/Aletharsis/pull/46",
        "reviewer": "test-reviewer", "scope": "synthetic extraction example",
        "gates": ["https://example.invalid/gate"], "reason": "schema test only",
    }
    return document, record


def test_reviewed_shape_and_missing_acceptance_requirements():
    """Approval must include clearance and verified runtime/build/security review."""
    document, record = approved_example()
    VALIDATOR.validate(document)
    for parent, key, value in [
        ("license", "clearance", "pending"),
        ("runtime", "assessment", "unverified"),
        ("assessment", "vulnerability_review", "pending"),
        ("assessment", "build_reproducibility", "not_tested"),
        ("decision", "gates", []),
        ("decision", "disposition", "reject"),
    ]:
        changed = deepcopy(document)
        changed["components"][0][parent][key] = value
        assert not VALIDATOR.is_valid(changed), (parent, key)


def test_invalid_pin_license_path_and_vector_state():
    """Floating revisions and unreviewed vector assertions fail closed."""
    for parent, key, value in [
        ("pin", "commit", "main"), ("pin", "source_tree_git_sha1", "abc"),
        ("pin", "artifact_sha256", "not-a-digest"),
        ("license", "retained_path", "../../LICENSE"),
        ("license", "sha256", "0" * 63),
        ("vectors", "status", "reviewed"),
    ]:
        document, record = sample()
        record[parent][key] = value
        assert not VALIDATOR.is_valid(document), (parent, key)


def objects(value, path=()):
    """Enumerate fixed objects for unknown-key and required-field mutations."""
    if isinstance(value, dict):
        yield path, value
        for key, child in value.items():
            yield from objects(child, (*path, key))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from objects(child, (*path, index))


def test_fixed_objects_are_closed_and_fields_required():
    """Unknown keys and missing fields cannot silently alter registry semantics."""
    document, _ = approved_example()
    for path, original in objects(document):
        for key in (None, *original):
            changed = deepcopy(document)
            target = changed
            for part in path:
                target = target[part]
            if key is None:
                target["unrecognized"] = "ignored?"
            else:
                del target[key]
            assert not VALIDATOR.is_valid(changed), (path, key)


def test_reviewed_vector_has_complete_provenance():
    """Accepted fixtures bind kind and serialization to exact bytes and lineage."""
    document, record = approved_example()
    record["vectors"].update(status="reviewed", provenance=[{
        "origin": "https://example.invalid/fixture", "license": "MIT",
        "artifact_kind": "raw_text", "serialization": "UTF-8, no BOM, LF",
        "sha256": "a" * 64, "generator_revision": None,
        "transformations": [], "expected_evidence": "independently counted U+200B",
    }])
    VALIDATOR.validate(document)
    for key in record["vectors"]["provenance"][0]:
        changed = deepcopy(document)
        del changed["components"][0]["vectors"]["provenance"][0][key]
        assert not VALIDATOR.is_valid(changed), key
    record["vectors"]["provenance"][0]["executable"] = "run me"
    assert not VALIDATOR.is_valid(document)


@pytest.mark.parametrize("status,disposition", [
    ("rejected", "reject"), ("retired", "retire"), ("deferred", "defer"),
])
def test_nonadoption_disposition_requires_matching_decision(status, disposition):
    """Reject/retire/defer records are explicit decisions, not implicit approval."""
    document, record = approved_example()
    record["adoption_status"] = status
    record["decision"]["disposition"] = disposition
    VALIDATOR.validate(document)
    record["decision"]["disposition"] = "approve"
    assert not VALIDATOR.is_valid(document)


@pytest.mark.parametrize("status,disposition", [
    ("approved", "approve"), ("rejected", "reject"),
    ("retired", "retire"), ("deferred", "defer"),
])
@pytest.mark.parametrize("field", ["upstream_code", "fixtures"])
def test_redistribution_requires_reviewed_rights_and_vectors(status, disposition, field):
    """Historical/rejected status does not bypass rights or fixture provenance."""
    document, record = approved_example()
    record["adoption_status"] = status
    record["decision"]["disposition"] = disposition
    record["redistribution"][field] = True
    if field == "fixtures":
        assert not VALIDATOR.is_valid(document)
        record["vectors"].update(status="reviewed", provenance=[{
            "origin": "https://example.invalid/fixture", "license": "MIT",
            "artifact_kind": "raw_text", "serialization": "UTF-8, no BOM, LF",
            "sha256": "b" * 64, "generator_revision": None,
            "transformations": [], "expected_evidence": "clean ASCII",
        }])
    VALIDATOR.validate(document)
    record["license"]["clearance"] = "pending"
    assert not VALIDATOR.is_valid(document)


@pytest.mark.parametrize("key,value", [
    ("observed_on", "2026-99-99"), ("observed_on", "2026-02-29"),
    ("observed_on", "2026-09-20T00:00:00Z"),
    ("last_push", "yesterday"), ("last_push", "2026-02-30T10:50:27Z"),
    ("last_push", "2026-09-20T25:00:00Z"),
    ("last_push", "2026-09-20T10:00:00"),
])
def test_invalid_calendar_dates_and_timestamps(key, value):
    """Formats are enforced, not silently treated as annotations."""
    document, record = sample()
    record["assessment"][key] = value
    assert not VALIDATOR.is_valid(document)


def test_valid_leap_date_and_offset_timestamp():
    """Valid calendar and timezone representations remain accepted."""
    document, record = sample()
    record["assessment"].update(observed_on="2024-02-29", last_push="2024-02-29T12:00:00+05:30")
    VALIDATOR.validate(document)


@pytest.mark.parametrize("url", [
    "https://example.invalid/decision", "https://github.com/other/repo/pull/46",
    "https://github.com/toddwbucy/Aletharsis/issues/35",
    "https://githubXcom/toddwbucy/Aletharsis/pull/46",
    "https://github.com/toddwbucy/Aletharsis/pull/0",
])
def test_decisions_require_aletharsis_pull_request_url(url):
    """Decision syntax identifies this repository's PRs, not arbitrary HTTPS."""
    document, record = approved_example()
    record["decision"]["record"] = url
    assert not VALIDATOR.is_valid(document)
