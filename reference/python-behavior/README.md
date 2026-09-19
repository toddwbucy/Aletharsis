# Frozen Python behavior

This directory is the migration oracle, not newly generated expected output from
the Go implementation. `manifest.json` identifies the merged Python source commit,
CPython/Unicode versions, source hashes, input hashes, report hashes, Unicode
oracle hash and reviewed schema snapshot hash.

The 37 reports retain complete Python evidence. The Unicode oracle includes
all-scalar property/normalization digests, inventoried names and 155 normalization
vectors. Existing fixtures remain in `tests/fixtures`; extra inputs live here.
The schema snapshot comes from PR #3 and does not merge that pending PR.

Run from the repository root after building a candidate executable:

```bash
python3 scripts/check_parity.py /path/to/aletharsis
# Optional live comparison, using CPython 3.12 / Unicode 15.0.0:
PYTHONPATH=src python scripts/check_differential.py /path/to/aletharsis
```

The frozen comparison verifies hashes, exact report values and exit codes, and
runs each input twice to check deterministic serialization. Its only semantic
exceptions are the implementation version, order among findings and native error
message prose. Error types, decode reasons, evidence and coordinates must match.
The live comparison adds 210 seeded Unicode/boundary cases.

Do not regenerate these files to make a failing port pass. Deliberate recapture
requires a fresh checkout/destination, the recorded Python revision/runtime and
the reviewed schema snapshot. Run `freeze_unicode.py` before `freeze_python.py`,
with `PYTHONPATH=src`. These scripts create files exclusively and intentionally
fail if outputs already exist. The latter also produces Unicode classification
and finding-description tables for the port; those tables are implementation
inputs and belong in the subsequent implementation review. Review any changed
oracle as a behavior-contract change.
