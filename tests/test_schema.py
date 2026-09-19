"""Validate emitted evidence and reject malformed nested report contracts."""

from copy import deepcopy
from dataclasses import asdict
import json
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator

from aletharsis.analyzers.metadata import MetadataAnalyzer
from aletharsis.audit import audit
from aletharsis.models import DocumentEvidence, Finding


@pytest.fixture(scope="module")
def schema():
    """Load and validate the schema once for contract mutation tests."""
    value = json.loads((Path(__file__).parents[1] / "schemas/report.schema.json").read_text())
    Draft202012Validator.check_schema(value)
    return value


@pytest.fixture
def finding_validator(schema):
    """Validate a finding with access to the schema's shared definitions."""
    return Draft202012Validator({"$defs": schema["$defs"], "$ref": "#/$defs/finding"})


@pytest.fixture
def findings(fixtures, tmp_path):
    """Cover every current rule with emitted evidence and PR #4's contract."""
    observed = {}
    for path in sorted(fixtures.iterdir()):
        for finding in audit(path).findings:
            observed.setdefault(finding.id, asdict(finding))
    extra = tmp_path / "extra.txt"
    extra.write_text("ordinary prose\u00ad\u00a0 e\u0301\n"
                     "cHJvdmVuYW5jZS1pZC0xMjM0NTY3ODkwYWJjZGVm\n", encoding="utf-8")
    for finding in audit(extra).findings:
        observed.setdefault(finding.id, asdict(finding))
    metadata, = MetadataAnalyzer().analyze(DocumentEvidence(metadata={"creator": "Reviewer"}))
    observed[metadata.id] = asdict(metadata)
    # PR #4 is independently mergeable. Exercise its known wire contract even
    # before that analyzer is on this branch; when present, use its real output.
    observed.setdefault("unicode.emoji", asdict(Finding(
        "unicode.emoji", "LOW", 1.0, "unicode", "observed_fact", "Emoji detected", "Review context.",
        {"code_point": "U+1F600", "name": "GRINNING FACE", "count": 1,
         "emoji_data_version": "17.0", "match_basis": "unicode_emoji_property",
         "count_unit": "code_point", "requires_context_review": True,
         "contexts": [{"character_offset": 0, "escaped_text": "\\U0001f600"}], "contexts_omitted": 0},
        {"source": "file", "character_offsets": [0], "byte_offsets": [0]})))
    return observed


def object_paths(value, path=()):
    """Locate nested objects so each fixed contract gets an unknown-key test."""
    if isinstance(value, dict):
        yield path
        for key, child in value.items():
            yield from object_paths(child, (*path, key))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from object_paths(child, (*path, index))


def at_path(value, path):
    """Retrieve a nested object without changing the original sample."""
    for key in path:
        value = value[key]
    return value


def test_every_rule_has_a_valid_discriminated_contract(schema, finding_validator, findings):
    """Every declared ID must have a covered positive example."""
    ids = {schema["$defs"][item["$ref"].split("/")[-1]]["properties"]["id"]["const"]
           for item in schema["$defs"]["finding"]["oneOf"]}
    assert set(findings) == ids
    for rule, finding in findings.items():
        assert finding_validator.is_valid(finding), rule


def test_unknown_keys_rejected_in_every_finding_object(finding_validator, findings):
    """Close evidence, location, contexts, alphabets, and threshold objects."""
    for rule, finding in findings.items():
        for path in object_paths(finding):
            malformed = deepcopy(finding)
            at_path(malformed, path)["unexpected_key"] = "not evidence"
            assert not finding_validator.is_valid(malformed), (rule, path)


def test_missing_fields_rejected_in_every_finding_object(finding_validator, findings):
    """The emitted fixed shapes require each of their documented fields."""
    for rule, finding in findings.items():
        for path in object_paths(finding):
            for key in at_path(finding, path):
                malformed = deepcopy(finding)
                del at_path(malformed, path)[key]
                assert not finding_validator.is_valid(malformed), (rule, path, key)


@pytest.mark.parametrize("field,value", [
    ("count", "1"), ("count", -1), ("leading_bom", "false"),
    ("code_point", "200B"), ("contexts_omitted", -1),
    ("contexts", [{"character_offset": -1, "escaped_text": "text"}]),
])
def test_unicode_evidence_types(finding_validator, findings, field, value):
    """Reject malformed data instead of letting renderers guess types."""
    finding = deepcopy(findings["unicode.zero_width"])
    finding["evidence"][field] = value
    assert not finding_validator.is_valid(finding)


@pytest.mark.parametrize("field,value", [
    ("character_offsets", [-1]), ("byte_offsets", ["3"]),
    ("character_offsets", []), ("byte_offsets", 3), ("source", None),
])
def test_location_types(finding_validator, findings, field, value):
    """Coordinates retain their exact renderer names and nonnegative units."""
    finding = deepcopy(findings["unicode.zero_width"])
    finding["location"][field] = value
    assert not finding_validator.is_valid(finding)


def test_renamed_offsets_and_wrong_rule_shapes_rejected(finding_validator, findings):
    """A generic dictionary or another rule's valid payload is not sufficient."""
    finding = deepcopy(findings["unicode.zero_width"])
    finding["location"]["offsets"] = finding["location"].pop("character_offsets")
    assert not finding_validator.is_valid(finding)
    for replacement_id in ("identifier.uuid", "unicode.unknown_future_rule"):
        finding = deepcopy(findings["unicode.zero_width"])
        finding["id"] = replacement_id
        assert not finding_validator.is_valid(finding)
    finding = deepcopy(findings["pattern.tag_run"])
    finding["evidence"] = deepcopy(findings["pattern.variation_selector_run"]["evidence"])
    assert not finding_validator.is_valid(finding)  # Missing tag ASCII projection.


def test_binary_and_emoji_nested_types(finding_validator, findings):
    """Constrain numeric ratios and emoji-specific semantics."""
    finding = deepcopy(findings["pattern.zero_width_binary"])
    finding["evidence"]["thresholds"]["minimum_minority_fraction"] = 1.5
    assert not finding_validator.is_valid(finding)
    finding = deepcopy(findings["pattern.zero_width_binary"])
    finding["evidence"]["alphabet"]["U+200B"] = "12"
    assert not finding_validator.is_valid(finding)
    for field, value in (("requires_context_review", "true"), ("count_unit", "glyph"),
                         ("match_basis", "guessed")):
        finding = deepcopy(findings["unicode.emoji"])
        finding["evidence"][field] = value
        assert not finding_validator.is_valid(finding)


def test_metadata_and_structure_contracts(schema, fixtures):
    """Optional normalized metadata and complete text structure have fixed keys."""
    validator = Draft202012Validator(schema)
    report = audit(fixtures / "clean_ascii.txt").to_dict()
    assert validator.is_valid(report)
    report["evidence"]["metadata"] = {"creator": "Reviewer", "revision": "3"}
    assert validator.is_valid(report)
    for section, value in [
        ("metadata", {"unexpected_key": "value"}), ("metadata", {"creator": {"name": "Reviewer"}}),
        ("metadata", {"revision": 3}), ("structure", {"inspection": "literal source text"}),
        ("structure", {"inspection": "literal source text", "byte_length": "100"}),
        ("structure", {"inspection": "literal source text", "byte_length": 100, "hidden": True}),
    ]:
        malformed = deepcopy(report)
        malformed["evidence"][section] = value
        assert not validator.is_valid(malformed), (section, value)


def test_failure_variants_preserve_required_decode_details(schema, fixtures):
    """Read failures allow empty structure; decode failures require byte detail."""
    validator = Draft202012Validator(schema)
    for path in (fixtures / "missing", fixtures / "invalid_utf8.txt"):
        report = audit(path).to_dict()
        assert validator.is_valid(report)
        malformed = deepcopy(report)
        malformed["findings"][0]["location"] = {"source": "file"}
        assert not validator.is_valid(malformed)
    malformed = audit(fixtures / "invalid_utf8.txt").to_dict()
    del malformed["findings"][0]["evidence"]["byte_start"]
    assert not validator.is_valid(malformed)


def test_example_report_still_valid(schema):
    """Tightening the contract preserves the published deterministic example."""
    report = json.loads((Path(__file__).parents[1] / "examples/suspicious-report.json").read_text())
    Draft202012Validator(schema).validate(report)
