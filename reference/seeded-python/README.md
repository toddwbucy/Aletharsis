# Frozen seeded Python cases

These 210 cases replace the live differential dependency on the retired Python
application. Expected reports were captured from CPython 3.12.13 / Unicode 15.0.0
at the source revision in [the retirement record](../python-retirement.json).
Each input and complete report was compared with Go before the Python source was
removed. The original 37-case corpus in `reference/python-behavior` is unchanged.

Run `python3 scripts/check_seeded.py bin/aletharsis` from the repository root.
No Python auditor is installed or imported. The script verifies artifact hashes,
replays exact input bytes in temporary files, and compares report values and exit
codes using the original documented migration exceptions.

See [capture format, recovery and validation](../../docs/migration/python-retirement.md).
Do not regenerate these expectations from the candidate under test.
