# G1 adapter contract review evidence

Disposition: **proposed for review**, #36 / parent #21. DA-001 defines the local
exchange boundary informed by accepted carrier/verifier studies #47/#48. No
production adapter, SDK dependency, schema-2.0 change or candidate approval.

## Deliverables and reproduction

- [Normative contract](../../specs/detector-adapter-v1.md): identities, byte maps,
  raw retention, separated provider claims, frame limits, isolation, cancellation,
  cleanup and a separate report-3.0 migration gate.
- [Closed exchange schema](../../../schemas/adapter-exchange-v1.schema.json).
- [Offline conformance cases](../../../tests/adapters/test_adapter_contract.py):
  thirteen complete positive transcripts plus negative mutations and frame cases.
  All data is independently constructed synthetic contract evidence under this
  repository's license. No upstream fixture/code or signed credential is copied.
- Both C2PA inventory entries link the proposal and remain `evaluating`.

Using the existing test-only Python environment with `scripts/requirements-ci.txt`:

```bash
python -m pytest -q tests/adapters
python -m pytest -q
git diff --check
```

CI's existing schema/reference stage runs these checks without downloads beyond
its already pinned test dependencies. No upstream process or network is exercised.
Test-only Python does not become a dependency of the Go binary.

## Measured validation and budget

Local environment: Python 3.12.13, Linux amd64. Initial full conformance/reference
run: **238 passed** (52 adapter checks plus 186 existing checks). Wall time 2.24 s;
user CPU 2.21 s, system CPU 0.05 s; peak RSS 92,244 KiB, measured with GNU time.
Work began 2026-09-20 19:08 UTC; this first validation completed at 19:15 UTC.
After adding the exact-byte manifest check, the final full suite passed **239
tests** (53 adapter checks) in 2.07 s. The entire isolated checkout occupied under 7 MiB. No GPU, model/API execution,
new toolchain, provider build or cross-repository work occurred. This is far below
the #36 initial 12 engineering hour / 1 CPU hour / 1 GiB scope; CI uses the existing
bounded validation jobs. Exact changed contract/fixture identities are recorded
in `adapter-contract-manifest.json`; its offline test detects drift.

## Limits and next gates

These are design conformance tests, not a production frame parser or supervisor.
They do not establish OS enforcement, platform support, resource exhaustion safety,
cryptographic validity, real provider compatibility or secret-handling support.
Frame cases are in-memory; timeout/cancellation fixtures describe required host
outcomes, not actual child termination. Runtime adversarial and native tests belong
to the future implementation/adoption PRs and #45.

Review DA-001 before implementation. The separate report migration must supply
complete report fixtures, Go decode/emit parity, importer/consumer changes and
aggregate/exit-code behavior. No result is squeezed into `structural_scan`, no
ambient trust policy is inferred, and unavailable detection is not a negative.
The upstream HTML mismatches, independent-verifier gap and non-Linux SDK validation
remain open. #7 and #21 remain open; merge accepts this design scope only.
