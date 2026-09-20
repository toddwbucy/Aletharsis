"""Exercise harness failure accounting with disposable synthetic processes."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(sys.platform == 'linux', 'GNU time measurement requires Linux')
class MeasurementTests(unittest.TestCase):
    def run_case(self, broken):
        cases = json.loads((ROOT / 'benchmarks/corpus.json').read_text())['cases']
        repeats = 1
        # One worker-count batch runs each case twice, after the serial repeats.
        runs_per_case = repeats + 2
        expected_total = len(cases) * runs_per_case
        expected_timeouts = sum(c['name'] == 'ascii' for c in cases) * runs_per_case
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary = root / 'candidate'
            binary.write_text(f'''#!{sys.executable}
import hashlib, json, pathlib, sys, time
p = pathlib.Path(sys.argv[2])
if {broken!r} and p.name.startswith('ascii-'):
    time.sleep(20)
if {broken!r}:
    print('invalid report')
    raise SystemExit(0)
data = p.read_bytes()
print(json.dumps({{'status': 'completed',
 'file': {{'sha256': hashlib.sha256(data).hexdigest(), 'size': len(data)}},
 'summary': {{'exit_code': 1, 'findings': 0}},
 'evidence': {{'texts': []}}, 'findings': []}}))
raise SystemExit(1)
''')
            binary.chmod(0o700)
            output = root / 'measurement'
            command = [sys.executable, str(ROOT / 'scripts/measure_backend.py'),
                       '--binary', str(binary), '--output', str(output),
                       '--go', '/bin/echo', '--sizes', '16', '--concurrency-size', '16',
                       '--repeats', str(repeats), '--workers', '1', '--timeout', '1']
            run = subprocess.run(command, capture_output=True, text=True, timeout=20)
            self.assertEqual(run.returncode, int(broken), run.stderr)
            result = json.loads((output / 'results.json').read_text())
            rows = result['runs'] + result['concurrency'][0]['runs']
            self.assertEqual(len(rows), expected_total)
            self.assertTrue(all(r['measurement_valid'] == (not broken) for r in rows))
            self.assertEqual(sum(r['timed_out'] for r in rows), expected_timeouts if broken else 0)
            self.assertEqual(len(list((output / 'reports').glob('*.json'))), expected_total if broken else 0)
            manifest = (output / 'corpus-manifest.json').read_bytes()
            again = subprocess.run(command, capture_output=True, text=True, timeout=10)
            self.assertNotEqual(again.returncode, 0)
            self.assertEqual((output / 'corpus-manifest.json').read_bytes(), manifest)

    def test_success_and_exclusive_destination(self):
        self.run_case(False)

    def test_timeout_and_invalid_report_fail_measurement(self):
        self.run_case(True)


class V2ValidationTests(unittest.TestCase):
    def test_v2_measurement_requires_wire_and_semantic_contract(self):
        import importlib.util
        spec = importlib.util.spec_from_file_location('measurement', ROOT / 'scripts/measure_backend.py')
        harness = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(harness)
        report = json.loads((ROOT / 'tests/contracts/fixtures/clean-unavailable.json').read_text())
        harness.validate_v2_report(report)
        report['executions'][0]['capability_ref'] = 'missing-capability'
        with self.assertRaises(ValueError):
            harness.validate_v2_report(report)
        with self.assertRaises(ValueError):
            harness.validate_v2_report({'status': 'completed'})


@unittest.skipUnless(sys.platform == 'linux', 'GNU time measurement requires Linux')
class V2RejectionTests(unittest.TestCase):
    def test_rejection_is_measured_but_never_a_valid_report(self):
        cases = json.loads((ROOT / 'benchmarks/corpus.json').read_text())['cases']
        for partial in (False, True):
            with self.subTest(partial=partial), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                binary = root / 'candidate'
                binary.write_text(f"""#!{sys.executable}
import sys
assert '--schema-version=2.0' in sys.argv
if {partial!r}: print('{{')
print('aletharsis: execution.resource_limit: could not assemble a complete report', file=sys.stderr)
raise SystemExit(4)
""")
                binary.chmod(0o700)
                output = root / 'measurement'
                run = subprocess.run([sys.executable, str(ROOT / 'scripts/measure_backend.py'),
                    '--binary', str(binary), '--output', str(output), '--go', '/bin/echo',
                    '--schema-version', '2.0', '--sizes', '16', '--concurrency-size', '16',
                    '--repeats', '1', '--workers', '1', '--timeout', '1'],
                    capture_output=True, text=True, timeout=20)
                self.assertEqual(run.returncode, int(partial), run.stderr)
                result = json.loads((output / 'results.json').read_text())
                self.assertEqual(result['environment']['report_schema_version'], '2.0')
                rows = result['runs'] + result['concurrency'][0]['runs']
                self.assertEqual(len(rows), len(cases) * 3)
                for row in rows:
                    self.assertFalse(row['valid_report'])
                    self.assertEqual(row['resource_rejected'], not partial)
                    self.assertEqual(row['measurement_valid'], not partial)


if __name__ == '__main__':
    unittest.main()
