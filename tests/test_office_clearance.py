"""Offline integrity of retained Office clearance evidence, not runtime clearance."""
import hashlib
import json
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
INVENTORY = json.loads((ROOT / "docs/reuse/evaluations/office/static-inventory.json").read_bytes())
NOTICES = INVENTORY["licenses"]
EXPECTED_MEMBERS = {
    "LICENSE.md",
    "oletools/thirdparty/xglob/LICENSE.txt",
    "olefile/LICENSE.txt",
    "olefile-0.47.dist-info/LICENSE.txt",
}


def test_complete_retained_notice_set():
    assert len(NOTICES) == len(EXPECTED_MEMBERS)
    assert {item["archive_member"] for item in NOTICES} == EXPECTED_MEMBERS
    assert len({item["retained_path"] for item in NOTICES}) == len(NOTICES)
    expected_retained = {
        "docs/reuse/licenses/decalage2--oletools.txt",
        "docs/reuse/evaluations/office/xglob-license.txt",
        "docs/reuse/evaluations/office/olefile-package-license.txt",
        "docs/reuse/evaluations/office/olefile-distribution-license.txt",
    }
    assert {item["retained_path"] for item in NOTICES} == expected_retained


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
