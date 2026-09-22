"""Offline integrity of retained Office clearance evidence, not runtime clearance."""
import hashlib
import json
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
INVENTORY = json.loads((ROOT / "docs/reuse/evaluations/office/static-inventory.json").read_bytes())
NOTICES = INVENTORY["licenses"]
def test_complete_retained_notice_set():
    members = INVENTORY["notice_members"]
    assert members
    identities = {(item["archive"], item["archive_member"]) for item in members}
    assert len(identities) == len(members)
    declared = {item["archive_member"]: item["retained_path"] for item in members if item["retained"]}
    actual = {item["archive_member"]: item["retained_path"] for item in NOTICES}
    assert actual and actual == declared
    assert len(actual) == len(NOTICES)
    on_disk = {p.relative_to(ROOT).as_posix() for p in (ROOT / "docs/reuse/evaluations/office").glob("*-license.txt")}
    on_disk.add("docs/reuse/licenses/decalage2--oletools.txt")
    assert set(actual.values()) == on_disk
    for item in members:
        assert type(item["retained"]) is bool
        assert item["reason"]
        if not item["retained"]:
            assert item["retained_path"] is None
            continue
        notice = next(n for n in NOTICES if n["archive_member"] == item["archive_member"])
        assert (notice["sha256"], notice["size"]) == (item["sha256"], item["size"])
        path = Path(item["retained_path"])
        assert not path.is_absolute() and ".." not in path.parts


@pytest.mark.parametrize("item", NOTICES, ids=lambda item: item["archive_member"])
def test_retained_notice_exact_bytes(item):
    raw = (ROOT / item["retained_path"]).read_bytes()
    assert len(raw) == item["size"]
    assert hashlib.sha256(raw).hexdigest() == item["sha256"]


def test_registry_and_inventory_identities_agree():
    registry = json.loads((ROOT / "docs/reuse/candidates.json").read_bytes())
    record = next(item for item in registry["components"] if item["id"] == "decalage2--oletools")
    archives = {item["filename"]: item for item in INVENTORY["archives"]}
    assert record["pin"]["artifact_sha256"] == archives["oletools.tar.gz"]["sha256"]
    root = next(item for item in NOTICES if item["archive_member"] == "LICENSE.md")
    assert record["license"]["sha256"] == root["sha256"]
    assert root["retained_path"] == "docs/reuse/" + record["license"]["retained_path"]
    assert record["license"]["clearance"] == "pending"
    assert record["runtime"]["assessment"] == "unverified"
    assert INVENTORY["upstream_executed"] is False
    assert INVENTORY["assessment"] == "static_only_not_execution_clearance"
    wheel = archives["olefile-0.47-py2.py3-none-any.whl"]["sha256"]
    assert wheel in record["runtime"]["dependencies"]
    assert wheel in (ROOT / "docs/reuse/evaluations/office.md").read_text()
    for notice in NOTICES:
        if notice["component"] == "olefile":
            assert notice["sha256"] in record["runtime"]["dependencies"]


@pytest.mark.parametrize("item", NOTICES, ids=lambda item: item["archive_member"])
def test_notice_rights_identity(item):
    assert item["holders"] and all(isinstance(s, str) and s for s in item["holders"])
    assert item["license_description"]
    if item["component"] == "olefile":
        assert item["spdx"] == "NOASSERTION"
        assert "PIL" in item["license_description"]
    elif item["component"] == "oletools.thirdparty.xglob":
        assert item["spdx"] == "BSD-2-Clause"
    else:
        assert item["component"] == "oletools"
        assert item["spdx"] == "BSD-2-Clause AND MIT"


def test_every_candidate_has_observed_or_explicitly_missing_header():
    narrative = (ROOT / "docs/reuse/evaluations/office.md").read_text()
    members = INVENTORY["candidate_modules"]
    assert members
    notices = {item["archive_member"] for item in NOTICES}
    for item in members:
        header = item["header_notice"]
        assert header["status"] in {"observed", "absent", "mismatched"}
        assert header["scope"] and header["qualification"]
        if header["status"] == "absent":
            assert not header["holders"] and not header["observed_lines"]
        else:
            assert header["holders"] and header["observed_lines"]
        if header["status"] != "observed":
            assert "`" + item["archive_member"] + "`" in narrative
        assert header["proposed_governing_notice"] in notices or header["proposed_governing_notice"] is None
