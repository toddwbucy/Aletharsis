"""Run one already-built offline probe per case, retaining exact source identity.

Execute this driver inside the documented read-only/network-isolated cgroup.
The enclosing cgroup sets the aggregate memory/CPU limit; this driver sets each
case's wall deadline and bounds output with regular files plus RLIMIT_FSIZE.
"""
import json
import os
from pathlib import Path
import resource
import subprocess
import sys

from cases import corpus, digest


def limit_output():
    resource.setrlimit(resource.RLIMIT_FSIZE, (16 * 1024 * 1024, 16 * 1024 * 1024))
    resource.setrlimit(resource.RLIMIT_CPU, (55, 60))


def main():
    binary, destination = sys.argv[1:]
    root = Path(destination)
    root.mkdir(exist_ok=True)
    results = []
    for case in corpus():
        raw = case.pop("raw")
        source = root / (case["id"] + ".input")
        source.write_bytes(raw)
        before = digest(raw)
        output = root / (case["id"] + ".stdout")
        errors = root / (case["id"] + ".stderr")
        with output.open("wb") as out, errors.open("wb") as err:
            try:
                proc = subprocess.run([binary, case["method"], str(source)], stdout=out, stderr=err,
                                      timeout=60, preexec_fn=limit_output, check=False)
                observation = json.loads(output.read_bytes()) if proc.returncode == 0 else {"execution_failure": proc.returncode}
            except subprocess.TimeoutExpired:
                observation = {"execution_failure": "timeout"}
        assert digest(source.read_bytes()) == before, "source modified"
        results.append(case | {"source_sha256": before, "source_bytes": len(raw),
                               "observed": observation,
                               "differences": [key for key, value in case["expected"].items() if observation.get(key) != value]})
    report = {"cases": results, "case_count": len(results),
              "execution_failures": sum("execution_failure" in x["observed"] for x in results),
              "expectation_mismatches": sum(bool(x["differences"]) for x in results)}
    (root / "results.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    print(json.dumps({key: value for key, value in report.items() if key != "cases"}))


if __name__ == "__main__":
    main()
