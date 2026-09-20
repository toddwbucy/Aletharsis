# Python application retirement

The Go application is the sole production implementation. This change removes the
legacy Python CLI after the Go port and the testing work tracked in Issue #7.
It changes neither Go detector behavior nor schema 1.0.

## Removed and retained

| Removed from the working tree | Replacement or retained evidence |
| --- | --- |
| `src/aletharsis/`, Python entry point and `pyproject.toml` packaging | Go CLI; historical source remains in Git |
| Duplicate Python application tests | Go unit/property/integrity tests and compiled-CLI integration contracts |
| Python fixture/oracle generators | Existing fixtures and original captured reports/Unicode data remain byte-for-byte; historical generation code remains in Git |
| Live `check_differential.py` | `scripts/check_seeded.py`, comparing all 210 captured cases against the compiled Go binary |
| Editable package install and live-oracle CI steps | Test-only dependencies, independent schema tests and frozen comparisons |

Python remains useful for executable integration, JSON Schema mutation tests,
reference comparisons, distribution checks and performance measurements. These
utilities do not import or install the retired application. `pytest.ini` replaces
only test discovery configuration; no Python application packaging remains.

The original 37 reports, manifest, schema snapshot, Unicode oracle, fixture bytes
and Go embedded data are unchanged. Schema mutation tests retain positive samples
in `tests/schema-cases.json`, including metadata and read failures. Current Go
report emission continues to be validated independently by `integration/`.

## Capture and identity

[reference/python-retirement.json](../../reference/python-retirement.json) records
source commit `90fb16c` in full, CPython 3.12.13 / Unicode 15.0.0, the original
manifest digest, exact historical source-file hashes, original seeded-generator
hash, seed 1729, case count and new artifact digests.

The 210 inputs use the exact generator from that source revision: 150 mixed
Unicode samples, 36 provenance-label/whitespace cases and 24 UUID-boundary cases.
Expected reports were captured from the Python auditor **before removal**, then
compared with the built Go 0.2.0 executable. They were not generated from Go.

`reference/seeded-python/cases.jsonl` stores one JSON object per case:

- `input_hex`: exact UTF-8 input bytes encoded as hexadecimal.
- `report`: complete Python report for relative filename `<index>.txt`.

Each case is replayed in a temporary directory using that same relative filename;
no source-path fields need to be discarded during comparison. Source size and
SHA-256 are checked against the decoded input bytes. Existing parity exceptions
remain limited to implementation version, finding order and native parser-failure
message prose. Original findings, coordinates, hashes and exit codes still match.

Historical source hashes are provenance, not missing current fixtures. The parity
checker verifies the original manifest digest and exact retired inventory against
the retirement record, then continues to require every retained input, report,
schema, Unicode oracle and newly captured artifact. It does not grant a general
exception for missing files. If a historical source path exists, its digest is
still checked. These checks detect accidental drift; trust in a deliberately
changed manifest still depends on repository review.

The corpus and metadata are protected from Git line-ending conversion. Canonical
JSONL capture uses ASCII-escaped, sorted-key JSON with compact separators and LF
record terminators; input hashes cover decoded input bytes, not JSON spellings.
Schema examples are test data, not a new runtime evidence contract.

## Recovery and independent reproduction

Use a separate checkout of the full `source_commit` recorded in the retirement
record to recover the Python application, original tests and generators. That
revision contains the complete source and `scripts/check_differential.py` before
removal. Verify its source-file and generator hashes against the retirement record.
Use CPython 3.12.13 / Unicode 15.0.0 for historical execution.

To reproduce the seeded capture, use the historical generator's input construction,
write each input as `<index>.txt` in a temporary working directory, and call the
historical `audit(Path(name)).to_dict()` on that relative name. Serialize the input
hex and report as described above. Compare the resulting JSONL digest with the
retirement record; never overwrite committed references as a routine test step.
The schema examples retain historical mutation-test samples, including a read
failure; they are separately hashed and validated, not part of the seeded corpus.

## Validation commands

From the repository root on supported Linux, after building `bin/aletharsis`:

```bash
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r scripts/requirements-ci.txt
python -m pytest -q
python -m pytest -q integration --aletharsis-binary=bin/aletharsis
python scripts/check_parity.py bin/aletharsis
python scripts/check_seeded.py bin/aletharsis
python -m unittest discover -s benchmarks -p 'test_*.py' -v
go test ./...
go test -race ./...
go vet ./...
```

A clean test environment needs no `aletharsis` Python module or executable.
Future behavior changes require a deliberate version/compatibility decision;
these fixed samples are regression evidence, not an extensible live Python oracle
or proof of correctness for every possible input. New behavior belongs in Go tests
and reviewed executable/schema fixtures.
