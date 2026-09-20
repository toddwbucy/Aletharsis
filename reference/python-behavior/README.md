# Frozen Python behavior

This directory is the migration oracle, not newly generated expected output from
the Go implementation. `manifest.json` identifies the merged Python source commit,
CPython/Unicode versions, source hashes, input hashes, report hashes, Unicode
oracle hash and reviewed schema snapshot hash.

The 37 reports retain complete Python evidence. The Unicode oracle includes
all-scalar property/normalization digests, inventoried names and 155 normalization
vectors. Existing fixtures remain in `tests/fixtures`; extra inputs live here.
The schema snapshot preserves the PR #3 contract; the live schema remains separate.

Run from the repository root after building a candidate executable:

```bash
python3 scripts/check_parity.py /path/to/aletharsis
# Additional frozen seeded corpus; no Python auditor required:
python3 scripts/check_seeded.py /path/to/aletharsis
```

The frozen comparison verifies hashes, exact report values and exit codes, and
runs each input twice to check deterministic serialization. Its only semantic
exceptions are the implementation version, order among findings and native error
message prose. Error types, decode reasons, evidence and coordinates must match.
The seeded comparison adds 210 captured Unicode/boundary cases. Historical source
hashes are retained as provenance; [the retirement record](../python-retirement.json)
pins the original manifest and its exact retired-source inventory. Retained
fixtures, reports, schema and Unicode data remain mandatory and hash-checked.

Do not regenerate these files to make a failing port pass. The Python application
and capture scripts have been retired; they remain recoverable from the source
revision recorded in the retirement manifest. See the [retirement guide](../../docs/migration/python-retirement.md)
for capture/runtime identities, historical reproduction and the current test
commands. Review any changed oracle as a behavior-contract change.
