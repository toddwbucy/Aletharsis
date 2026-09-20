"""Bounded offline byte-verifier probes; run inside the documented cgroup/namespace."""
import hashlib
import json
import os
from pathlib import Path
import pty
import resource
import select
import subprocess
import sys
import time


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def snapshot(root):
    return {str(p.relative_to(root)): digest(p.read_bytes()) for p in sorted(root.rglob("*")) if p.is_file()}


def child_limits():
    resource.setrlimit(resource.RLIMIT_FSIZE, (32 * 1024**2, 32 * 1024**2))
    resource.setrlimit(resource.RLIMIT_CPU, (115, 120))


def field(record, path):
    for part in path.split("."):
        if not isinstance(record, dict):
            return None
        record = record.get(part)
    return record


def main():
    binary, upstream, output = sys.argv[1:]
    source, root = Path(upstream), Path(output)
    root.mkdir(exist_ok=True)
    config = root / "config"
    config.mkdir(exist_ok=True)
    (config / "c2pa.json").write_text('{"telemetry_enabled":true}\n')
    config_before = snapshot(config)
    corpus_path = source / "tests/vectors/core/corpus.json"
    corpus = json.loads(corpus_path.read_bytes())
    cases = []
    for vector in corpus["vectors"]:
        asset = corpus_path.parent / vector["path"]
        assert digest(asset.read_bytes()) == vector["sha256"]
        expected = vector["normative_expected"]
        cases.append({"id": vector["id"], "asset": asset, "mime": vector["mime_type"],
                      "options": {"no_default_trust": True, "validation_time": vector["fixed_validation_time"]},
                      "expected": {"report.validation_state": expected["validation_state"],
                                   "report.trust.status": "not_evaluated"},
                      "required_success": expected["required_success_codes"],
                      "forbidden_success": expected["forbidden_success_codes"],
                      "expected_failures": expected["failure_codes"], "provenance": vector["source"]})
    signed = source / "tests/fixtures/signed_test.jpg"
    for name, no_default, interactive in [("signed_no_trust", True, False), ("signed_default_trust", False, False), ("signed_interactive", True, True)]:
        cases.append({"id": name, "asset": signed, "mime": "image/jpeg", "interactive": interactive,
                      "options": {"no_default_trust": no_default, "validation_time": "2026-09-20T00:00:00Z"},
                      "expected": {"report.present": True, "report.integrity": "valid", "report.hard_binding": "match",
                                   "report.trust.status": "not_evaluated" if no_default else "not_valid_for_supplied_material"},
                      "provenance": {"source": "upstream synthetic Apache-2.0 fixture; tests/fixtures/README.md"}})
    plain = root / "plain.txt"
    plain.write_bytes(b"Ordinary unsigned text.\n")
    cases.append({"id": "unsigned_text", "asset": plain, "mime": "text/plain",
                  "options": {"no_default_trust": True, "validation_time": "2026-09-20T00:00:00Z"},
                  "expected": {"report.present": False}, "provenance": {"source": "independently constructed ASCII"}})
    oversized = root / "oversized.txt"
    with oversized.open("wb") as f:
        f.truncate(32 * 1024**2 + 1)
    cases.append({"id": "oversized", "asset": oversized, "mime": "text/plain", "options": {},
                  "expected": {"failure": "input_limit"}, "provenance": {"source": "independent sparse boundary input; rejected before upstream"}})
    results = []
    for case in cases:
        asset = case["asset"]
        raw = asset.read_bytes()
        identity = digest(raw)
        option_file = root / (case["id"] + ".options.json")
        option_file.write_text(json.dumps(case["options"], sort_keys=True) + "\n")
        out_path = root / (case["id"] + ".stdout")
        err_path = root / (case["id"] + ".stderr")
        env = os.environ | {"ENCYPHER_C2PA_TELEMETRY": "true", "ENCYPHER_C2PA_CONFIG_DIR": str(config)}
        master = slave = None
        if case.get("interactive"):
            master, slave = pty.openpty()
        with out_path.open("wb") as out, err_path.open("wb") as err:
            try:
                proc = subprocess.run([binary, str(asset), case["mime"], str(option_file)], stdout=out,
                                      stderr=slave if slave is not None else err,
                                      stdin=slave if slave is not None else subprocess.DEVNULL,
                                      env=env, timeout=120, preexec_fn=child_limits)
                observed = json.loads(out_path.read_bytes()) if proc.returncode == 0 else {"execution_failure": proc.returncode}
            except subprocess.TimeoutExpired:
                observed = {"execution_failure": "timeout"}
            finally:
                if slave is not None:
                    os.close(slave)
                    if select.select([master], [], [], 0)[0]:
                        try:
                            err.write(os.read(master, 65536))
                        except OSError:
                            pass
                    os.close(master)
        differences = [key for key, value in case["expected"].items() if field(observed, key) != value]
        validation = observed.get("report", {}).get("validation_results", {})
        success = {x["code"] for x in validation.get("success", [])}
        failures = {x["code"] for x in validation.get("failure", [])}
        for code in case.get("required_success", []):
            if code not in success:
                differences.append("missing_success:" + code)
        for code in case.get("forbidden_success", []):
            if code in success:
                differences.append("forbidden_success:" + code)
        if "expected_failures" in case and failures != set(case["expected_failures"]):
            differences.append("failure_codes")
        assert digest(asset.read_bytes()) == identity
        assert snapshot(config) == config_before
        results.append({"id": case["id"], "source_sha256": identity, "source_bytes": len(raw),
                        "mime": case["mime"], "options": case["options"],
                        "provenance": case["provenance"], "expected": case["expected"],
                        "required_success": case.get("required_success", []),
                        "forbidden_success": case.get("forbidden_success", []),
                        "expected_failures": case.get("expected_failures"),
                        "observed": observed, "differences": differences,
                        "stderr_bytes": err_path.stat().st_size, "config_unchanged": True,
                        "interactive": case.get("interactive", False)})
    # Worker cancellation, not a claim of in-process Rust cancellation support.
    stress = root / "cancel.txt"
    stress.write_bytes(b"a" * (32 * 1024**2))
    opts = root / "cancel.options.json"
    opts.write_text('{"no_default_trust":true,"validation_time":"2026-09-20T00:00:00Z"}\n')
    with (root / "cancel.stdout").open("wb") as out, (root / "cancel.stderr").open("wb") as err:
        worker = subprocess.Popen([binary, str(stress), "text/plain", str(opts)], stdout=out, stderr=err,
                                  stdin=subprocess.DEVNULL, start_new_session=True, preexec_fn=child_limits)
        time.sleep(0.01)
        live = worker.poll() is None
        if live:
            os.killpg(worker.pid, 15)
        try:
            status = worker.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(worker.pid, 9)
            status = worker.wait(timeout=5)
    report = {"cases": results, "case_count": len(results), "upstream_corpus_sha256": digest(corpus_path.read_bytes()),
              "expectation_mismatches": sum(bool(x["differences"]) for x in results),
              "execution_failures": sum("execution_failure" in x["observed"] for x in results),
              "cancellation": {"worker_observed_live": live, "exit_code": status,
                               "reaped": worker.poll() is not None, "scope": "isolated worker, not in-process cancellation"}}
    (root / "results.json").write_text(json.dumps(report, sort_keys=True, indent=2) + "\n")
    print(json.dumps({k: v for k, v in report.items() if k != "cases"}))


if __name__ == "__main__":
    main()
