import json
import os
from pathlib import Path
import subprocess
import sys

import jsonschema
import pytest

from aletharsis.audit import audit
from aletharsis.cli import main
from aletharsis.reporters.json import render


def test_help_version():
    for args, expected in [(["--help"], "audit"), (["--version"], "0.1.0")]:
        result = subprocess.run([sys.executable, "-m", "aletharsis", *args], capture_output=True, text=True)
        assert result.returncode == 0
        assert expected in result.stdout


@pytest.mark.parametrize("name,code", [("clean_ascii.txt", 0), ("isolated_zwsp.txt", 1),
                                     ("identifiers.md", 2), ("binary_zero_width.txt", 3), ("invalid_utf8.txt", 4)])
def test_exit_codes_json(fixtures, capsys, name, code):
    assert main(["audit", str(fixtures / name), "--json"]) == code
    report = json.loads(capsys.readouterr().out)
    assert report["summary"]["exit_code"] == code
    assert report["file"]["sha256"]


def test_output_and_no_overwrite(fixtures, tmp_path, capsys):
    source = tmp_path / "source.txt"
    original = b"original text\n"
    source.write_bytes(original)
    hardlink = tmp_path / "hardlink.txt"
    os.link(source, hardlink)
    symlink = tmp_path / "symlink.txt"
    symlink.symlink_to(source)
    for target in [source, hardlink, symlink]:
        assert main(["audit", str(source), "--output", str(target)]) == 4
    assert source.read_bytes() == original
    output = tmp_path / "report.json"
    assert main(["audit", str(source), "--output", str(output)]) == 0
    assert json.loads(output.read_text())["status"] == "completed"
    assert "could not create report" in capsys.readouterr().err


def test_console_safe(fixtures, capsys):
    assert main(["audit", str(fixtures / "control.txt"), "--verbose"]) == 2
    output = capsys.readouterr().out
    assert "ALETHARSIS FORENSIC AUDIT" in output
    assert "U+001B" in output
    assert "\x1b" not in output and "\x00" not in output


@pytest.mark.parametrize("command", ["unicode", "metadata", "structure"])
def test_views(fixtures, command, capsys):
    code = main([command, str(fixtures / "identifiers.md"), "--json"])
    report = json.loads(capsys.readouterr().out)
    assert code == (2 if command == "metadata" else 0)
    assert report["evidence"]["texts"]
    assert command in report["limitations"][-1]


def test_schema_and_deterministic_json(fixtures):
    schema = json.loads((Path(__file__).parents[1] / "schemas/report.schema.json").read_text())
    jsonschema.Draft202012Validator.check_schema(schema)
    for path in sorted(fixtures.iterdir()):
        report = audit(path)
        first = render(report)
        assert first == render(audit(path))
        jsonschema.validate(json.loads(first), schema)
    jsonschema.validate(audit(fixtures / "missing").to_dict(), schema)


def test_usage_failure():
    result = subprocess.run([sys.executable, "-m", "aletharsis", "audit"], capture_output=True)
    assert result.returncode == 4


def test_installed_entry_point(fixtures):
    executable = Path(sys.executable).parent / "aletharsis"
    result = subprocess.run([str(executable), "audit", str(fixtures / "binary_zero_width.txt"), "--json"],
                            capture_output=True, text=True)
    assert result.returncode == 3
    assert json.loads(result.stdout)["summary"]["high"] == 1
