# G6 checklist validation evidence

Scope: checklist/schema, two current C2PA gap records, offline validation only.
No component adoption, download, live detector or cross-repository change.

Recorded 2026-09-21, Linux/amd64, Python 3.12.13 in the existing test environment.
The initial implementation/test interval was 04:25:40–04:29:18 UTC (218 seconds);
subsequent PR preparation/review polling is outside that measured interval.
No claim of total engineering-time precision is made. Work remains within #45's
8-hour design ceiling; stop and revise the budget before exceeding it.

Commands from repository root:

```bash
python -m pytest -q tests/test_adoption_records.py tests/test_reuse_registry.py tests/test_carrier_feasibility.py tests/test_verifier_feasibility.py
/usr/bin/time -f 'wall=%e cpu_user=%U cpu_system=%S max_rss_kib=%M' python -m pytest -q
git diff --check
```

Observed: 74 focused tests pass; 283 complete offline tests pass. Full-suite time:
2.28 seconds wall, 2.23 seconds user CPU, 0.06 seconds system CPU, maximum resident
set 91,868 KiB. These are observed test costs, not enforced process ceilings or
worst-case guarantees. Retained adoption directory/schema/test data occupied
48 KiB before this note; total new retained evidence remains below 1 MiB, within
the 1 GiB design ceiling. No live-detector compute was incurred.

The full suite includes owning feasibility-manifest validators; this complements
G6's exact digest checks on the referenced study documents and manifests. No Go
runtime code changed. CI independently runs the normal backend/platform checks.
Reviewer acceptance remains pending and is not inferred from these results.
