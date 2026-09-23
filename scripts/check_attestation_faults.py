"""Verify generated authority probes against isolated importer guard faults.

Development-only: run through scripts/run_bounded.py. Go overlays substitute
temporary copies; no source file or frozen fixture is edited.
"""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile


def run_probe(command, root):
    try:
        return subprocess.run(command, cwd=root, capture_output=True,
                              text=True, timeout=45)
    except subprocess.TimeoutExpired as exc:
        raise SystemExit(json.dumps({"status": "failed", "reason": "probe_timeout",
                                     "timeout_seconds": 45})) from exc


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    source = root / "internal/evidence/v4/metadata_coverage.go"
    original = source.read_text()
    changes = {
        "pooled-exclusion": [
            ("ownExcluded := normalizedSpans(o.Excluded)", "ownExcluded := excluded")],
        "coincident-sibling": [
            ("if _, exists := children[element.Span]; exists || element.Span.Start >= element.Span.End {",
             "if element.Span.Start >= element.Span.End {")],
        "zero-property": [
            ("exists || element.Span.Start >= element.Span.End", "exists"),
            (" || span.Start >= span.End", "")],
        "detached-property": [
            ("\t\t\tif (i == 0) != (element.Parent == nil) {\n\t\t\t\treturn ErrLinkage\n\t\t\t}\n", ""),
            ("for i, element := range doc.Elements {", "for _, element := range doc.Elements {")],
    }
    command = [args.go, "test", "-count=1", "-p", "1"]
    test = ["./internal/evidence/v4", "-run",
            "^(TestGeneratedProjectionAuthority|TestXMLSingleRootIndependentGuard)$"]
    baseline = run_probe(command + test, root)
    if baseline.returncode:
        raise RuntimeError("unmodified authority probes must pass\n" + baseline.stdout + baseline.stderr)
    changes["xml-root"] = [(" || (i == 0) != (e.Parent == nil)", "")]
    results = {}
    with tempfile.TemporaryDirectory(prefix="aletharsis-attestation-") as tmp:
        directory = Path(tmp)
        for name, replacements in changes.items():
            target = source if name != "xml-root" else source.with_name("xml.go")
            text = original if name != "xml-root" else target.read_text()
            for old, new in replacements:
                if text.count(old) != 1:
                    raise RuntimeError(f"{name}: fault target changed; review this probe")
                text = text.replace(old, new)
            mutant = directory / (name + ".go")
            mutant.write_text(text)
            overlay = directory / (name + ".json")
            overlay.write_text(json.dumps({"Replace": {str(target): str(mutant)}}))
            run = run_probe(command + ["-overlay", str(overlay)] + test, root)
            # Compilation failures, timeouts, or an unrelated failing test do
            # not constitute detection of the reintroduced guard defect.
            expected_test = ("--- FAIL: TestXMLSingleRootIndependentGuard" if name == "xml-root"
                             else "--- FAIL: TestGeneratedProjectionAuthority/")
            if (run.returncode != 1
                    or expected_test not in run.stdout
                    or (name != "xml-root" and "/" + name + " (" not in run.stdout)
                    or name + ": <nil>" not in run.stdout):
                raise RuntimeError(f"{name}: expected semantic survivor not detected\n"
                                   + run.stdout + run.stderr)
            results[name] = {"generated_probe_detected_fault": True}
    print(json.dumps(results, sort_keys=True))


if __name__ == "__main__":
    main()
