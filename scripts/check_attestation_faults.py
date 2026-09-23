"""Verify generated authority probes against isolated importer guard faults.

Development-only: run through scripts/run_bounded.py. Go overlays substitute
temporary copies; no source file or frozen fixture is edited.
"""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile


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
    test = ["./internal/evidence/v4", "-run", "^TestGeneratedProjectionAuthority$"]
    baseline = subprocess.run(command + test, cwd=root, capture_output=True,
                              text=True, timeout=15)
    if baseline.returncode:
        raise RuntimeError("unmodified authority probes must pass\n" + baseline.stdout + baseline.stderr)
    results = {}
    with tempfile.TemporaryDirectory(prefix="aletharsis-attestation-") as tmp:
        directory = Path(tmp)
        for name, replacements in changes.items():
            text = original
            for old, new in replacements:
                if text.count(old) != 1:
                    raise RuntimeError(f"{name}: fault target changed; review this probe")
                text = text.replace(old, new)
            mutant = directory / (name + ".go")
            mutant.write_text(text)
            overlay = directory / (name + ".json")
            overlay.write_text(json.dumps({"Replace": {str(source): str(mutant)}}))
            run = subprocess.run(command + ["-overlay", str(overlay)] + test,
                                 cwd=root, capture_output=True, text=True, timeout=15)
            # Compilation failures, timeouts, or an unrelated failing test do
            # not constitute detection of the reintroduced guard defect.
            if (run.returncode != 1
                    or "--- FAIL: TestGeneratedProjectionAuthority/" not in run.stdout
                    or "/" + name + " (" not in run.stdout
                    or name + ": <nil>" not in run.stdout):
                raise RuntimeError(f"{name}: expected semantic survivor not detected\n"
                                   + run.stdout + run.stderr)
            results[name] = {"generated_probe_detected_fault": True}
    print(json.dumps(results, sort_keys=True))


if __name__ == "__main__":
    main()
