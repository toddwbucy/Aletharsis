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
    declared = {(item["archive"], item["archive_member"]): item["retained_path"] for item in members if item["retained"]}
    actual = {(item["archive"], item["archive_member"]): item["retained_path"] for item in NOTICES}
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
        notice = next(n for n in NOTICES if (n["archive"], n["archive_member"]) == (item["archive"], item["archive_member"]))
        assert (notice["sha256"], notice["size"]) == (item["sha256"], item["size"])
        path = Path(item["retained_path"])
        assert not path.is_absolute() and ".." not in path.parts


@pytest.mark.parametrize("item", NOTICES, ids=lambda item: item["archive_member"])
def test_retained_notice_exact_bytes(item):
    raw = (ROOT / item["retained_path"]).read_bytes()
    assert all(holder.encode() in raw for holder in item["holders"])
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
    assert wheel in (ROOT / "docs/reuse/evaluations/office.md").read_text()
    expected = [{"archive": a["filename"], "archive_member": None, "sha256": a["sha256"], "size": a["size"], "retained_path": None} for a in INVENTORY["archives"]]
    expected += [{key: n[key] for key in ("archive", "archive_member", "sha256", "size", "retained_path")} for n in NOTICES]
    assert record["runtime"]["artifacts"] == expected


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
    for item in members:
        header = item["header_notice"]
        assert header["status"] in {"observed", "absent", "mismatched"}
        assert INVENTORY["header_notice_scope"] and header["qualification"]
        if header["status"] == "absent":
            assert not header["holders"] and not header["observed_lines"]
        else:
            assert header["holders"] and header["observed_lines"]
        if header["status"] != "observed":
            assert "`" + item["archive_member"] + "`" in narrative
        name = item["archive_member"]
        expected = ("olefile/LICENSE.txt" if name.startswith("olefile/") else
                    "oletools/thirdparty/xglob/LICENSE.txt" if name.startswith("oletools/thirdparty/xglob/") else
                    None if name == "oletools/thirdparty/__init__.py" else "oletools/LICENSE.txt")
        assert header["proposed_governing_notice"] == expected
        assert isinstance(item["other_credits"], list) and INVENTORY["other_credits_scope"]
        for credit in item["other_credits"]:
            assert credit["line"] > 0 and credit["text"]



def test_credits_disclaimer_and_provisioned_notices():
    modules = {item["archive_member"]: item for item in INVENTORY["candidate_modules"]}
    for name, line in (("oletools/ooxml.py", 10), ("oletools/common/log_helper/log_helper.py", 26)):
        assert any(c["line"] == line and "Intra2net AG" in c["text"] for c in modules[name]["other_credits"])
    disclaimer = modules["oletools/ppt_record_parser.py"]["header_notice"]["disclaimer_lines"]
    assert [line["line"] for line in disclaimer] == list(range(9, 31))
    assert "Redistribution" in disclaimer[2]["text"]
    assert INVENTORY["provisioning_notices"] == [{key: n[key] for key in ("archive", "archive_member", "retained_path")} for n in NOTICES]


def test_write_entry_point_inventory():
    points = INVENTORY["write_entry_points"]
    expected = {
        ("olefile/olefile.py", "OleFileIO.__init__", 1048),
        ("olefile/olefile.py", "OleFileIO.open", 1193),
        ("olefile/olefile.py", "OleFileIO.write_sect", 1719),
        ("olefile/olefile.py", "OleFileIO.write_stream", 1990),
        ("oletools/record_base.py", "OleRecordFile.open", 143),
        ("oletools/common/io_encoding.py", "uopen", 149),
        ("oletools/oleobj.py", "process_file", 836),
        ("oletools/oleobj.py", "main", 962),
        ("olefile/olefile.py", "OleFileIO._write_mini_sect", 1745),
        ("olefile/olefile.py", "OleFileIO._write_mini_stream", 1972),
    }
    assert {(p["archive_member"], p["symbol"], p["line"]) for p in points} == expected
    modules = {(m["archive"], m["archive_member"]) for m in INVENTORY["candidate_modules"]}
    assert all((p["archive"], p["archive_member"]) in modules for p in points)


def test_declared_metadata_and_credit_scope():
    metadata = INVENTORY["declared_license_metadata"]
    assert {(m["archive"], m["archive_member"], m["line"]) for m in metadata} == {
        ("oletools.tar.gz", "setup.py", 64), ("oletools.tar.gz", "setup.py", 74),
        ("olefile-0.47-py2.py3-none-any.whl", "olefile-0.47.dist-info/METADATA", 8),
        ("olefile-0.47-py2.py3-none-any.whl", "olefile-0.47.dist-info/METADATA", 16)}
    assert all("BSD" in m["text"] for m in metadata)
    assert "Hand-curated" in INVENTORY["write_entry_points_scope"]
    modules = {m["archive_member"]: m for m in INVENTORY["candidate_modules"]}
    for name, line in (("oletools/oleobj.py", 8), ("oletools/thirdparty/xglob/xglob.py", 15), ("olefile/olefile.py", 91)):
        assert any(c["line"] == line and "Philippe Lagadec" in c["text"] for c in modules[name]["other_credits"])


def test_notice_difference_matches_retained_bytes():
    import difflib
    root = (ROOT / "docs/reuse/licenses/decalage2--oletools.txt").read_text().splitlines()
    package = (ROOT / "docs/reuse/evaluations/office/oletools-package-license.txt").read_text().splitlines()
    assert INVENTORY["oletools_notice_difference"] == list(difflib.unified_diff(root, package, fromfile="LICENSE.md", tofile="oletools/LICENSE.txt", lineterm=""))
