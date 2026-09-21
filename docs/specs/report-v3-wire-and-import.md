# EC-004 — Report 3.0 wire and offline conformance

Status: proposed wire gate under #36/#21, following EC-003 accepted in PR #50.
[Schema](../../schemas/report-v3.schema.json) and
[conformance corpus](../../tests/contracts_v3) implement review unit 2 of the
[migration design](report-v3-migration.md). This is not Go import/emission support,
production detector adoption, a worker supervisor or a schema-default switch.

## Wire contract

The self-contained Draft 2020-12 schema retains the native report-2.0 envelope,
findings, structural results and native configuration variants. It requires version
3.0 and `adapter_runs`, and adds the closed EC-003 result/configuration, byte-map,
artifact-byte anchor and adapter diagnostic variants. New records cannot carry
arbitrary provider maps, execution instructions or secret keys. Schema identifiers
are names, not remote download locations. Every `$ref` is local.

`build_schema.py` reproducibly derives this separate schema from the frozen v2 and
DA-001 schemas. The old schemas are read-only inputs, never rewritten. Generated
schema bytes are committed for Go bundling in the next increment. Shape checks
alone do not establish graph correctness, source integrity or provider correctness.

The offline importer checks strict UTF-8 JSON, duplicate keys, finite numeric
values, depth and size, then schema and graph semantics. Report size is at most
8 MiB and JSON depth at most 64. Integer coordinates/counters use the inherited
safe-integer schema bounds; statistical scores retain finite binary64 semantics
and are not constrained to [0,1] or the safe-integer range. Existing v1/v2 import
behavior is delegated unchanged to the existing oracle. Unknown versions retain
exact-byte identity and unknown coverage. Imports do not resolve paths or URLs.

The test oracle supports only the explicitly supplied `conformance-v3/1` catalog.
Its local catalog maps native operations and synthetic adapters independently of
report declarations. Unknown catalogs/capabilities are rejected for inert inspection.
The two real-world declarations remain unavailable: no fixture advertises access
to an actual credential SDK or Anthropic detector. Production catalog validation,
version dispatch and public support are future Go implementation gates.

## Graph and byte verification

All local references must resolve without cycles or duplicate identities. Artifacts
are topologically ordered; report-local references can identify different stages
with equal byte hashes. Run order matches adapter execution planning, including
not-run declarations. A native execution cannot carry an adapter config/run/result,
and an adapter cannot produce an uncontracted finding or native structural result.

Exactly one aggregate result is required for a completed/partial adapter. Failed,
canceled and not-run adapters have no accepted response or substantive result.
Started/cleanup state, launch policy, request identity, cause diagnostics, partial
exclusions and common/adapter limit agreement are validated separately. Cleanup
failure preserves a distinct primary cause. Quarantined output never becomes
accepted evidence; its presence does not make a result usable.

Partial extraction covers disjoint checked byte regions with explicit remaining
exclusions. Verification/statistical operations require completed whole-input
coverage. Result scope/anchors must reference the selected input artifact, never
the request header. Source identity, config/policy/trust identity, sample identity,
manifest identity and raw response identity stay distinct. Evaluated trust requires
retained trust material. Credential absence cannot carry evaluated signature/binding
or trust claims. An invalid credential can coexist with completed execution.

The blob-check API takes an explicit digest-to-**bytes** mapping from the caller.
It cannot open report-controlled paths, invoke a provider or fetch a missing blob.
The supplied store is bounded to 64 MiB total and 32 MiB per value. The corpus loader
reads only repository-owned fixture files. Each resolved artifact is checked against
its length and digest. Supplying an empty/partial store is permitted: unresolved
references are returned explicitly. `status: validated` means wire/graph consistency,
not that every referenced byte was available. `source_authority` remains
`not_granted_by_import`, even after resolving fixture bytes.

When exact header blobs are supplied, additionally check:

- Request shape, exact-header digest versus DA-001 composite request ID, selected
  source/input, transformation chain, effective config/policy/trust/time/limits,
  and pinned wrapper/upstream/executable identity.
- Response shape, input/request identity, checked/excluded regions, one aggregate
  result, manifest/raw identities, promoted payload fields and preserved limitations.
- Declared retained response/manifest sizes fit the output cap, even without blob
  resolution. Actual stderr and total process I/O enforcement require the future
  supervisor's runtime tests; report metadata cannot prove those controls executed.

For inline byte maps, `from_artifact_ref` is the parent/source side of each pair;
`to_artifact_ref` is the derived/target side. A map attached to an artifact must
name that artifact as its target and a parent as its source. Target spans completely
cover target bytes in order, source spans stay within source bounds and exclude
removed regions. Derived maps can reorder/repeat source spans. Exact maps require
equal widths, and identical bytes when both sides are available. An unresolved
map is not exact-navigation authority merely because its record claims exactness.
Native scalar/source-byte maps keep their prior opposite navigation convention;
method/version selects the interpretation, never a generic field-name guess.

## Fixtures and independent expectations

The 31 positive files are complete reports, not planning vectors:

- Eight native compatibility scenarios preserve every original v2 field except
  version/catalog and the added run ledger. They include acquisition/decode failure,
  cancellation, unavailable capabilities, native partial timeout and filtered views.
- A native-only report has an empty adapter ledger and no adapter declarations.
- Thirteen mixed native/adapter reports cover every DA-001 outcome. Raw request,
  response, configuration, source and manifest bytes are retained by exact digest.
- Additional cases cover NFC mapping, absent/malformed/ambiguous carriers, extracted
  manifest bytes, empty input, filtered results, supplied trust and a large statistic.

The 16 negative files are also complete reports, with separately declared expected
schema/semantic failures. They cover missing runs, unknown members, stale hashes,
wrong input references, bad scope and offsets, false exact maps, invented trust,
changed verdicts, missing primary causes and lost provider limitations. Additional
mutation tests exercise duplicated results/runs, quarantine promotion, byte corruption,
resource limits and untrusted JSON. No upstream code, signed credentials or customer
files are copied. Synthetic signature/statistical labels are **not actual detections**.

`coordinate-vectors.json` provides manually specified UTF-8 and UTF-16 boundaries
for decomposed accent, embedded BOM, zero-width space, supplementary emoji and CRLF.
The NFC carrier moves from source bytes [6,9) to selected-input bytes [5,8).
That derived mapping does not authorize deleting source bytes. Request-header and
composite-request digests are tested as separate identities. The retained manifest
binds exact report, blob, vector and catalog bytes for the later Go parity suite.

## Reproduction and remaining gates

With the existing test-only environment:

```bash
python tests/contracts_v3/build_schema.py
python tests/contracts_v3/build_fixtures.py
python tests/contracts_v3/build_invalid.py
python -m pytest -q tests/contracts_v3
python -m pytest -q
git diff --check
```

Generators are explicit development commands, not imports or application features.
They construct independently specified synthetic outcomes; tests never treat a
provider vote as ground truth. Regeneration must preserve committed identities.
Existing backend CI discovers this suite; no Go production source or dependency
changes are needed. Python remains test infrastructure only.

Review this wire/schema gate before Go work. Next is bounded typed Go decode/encode
and graph validation against all positive and negative reports, including retained
byte resolution as an explicitly authorized step. Then opt-in production assembly,
consumer/exit-code/view tests and migration acceptance. #36 remains open through
those gates; #7/#21 and per-adoption #45 remain open. The current CLI still defaults
to schema 1.0 and only implements opt-in 2.0.

## Validation record

Local Python 3.12.13 / Linux amd64: **335 tests passed**, including 70 v3 checks;
`git diff --check` passed. The 334-case resource run before adding the pure-native
fixture measured 2.85 seconds wall, 2.79 user CPU, 0.04 system CPU and 91,284 KiB
peak RSS. The final 335-case suite also passed. Corpus storage is about 1.3 MiB;
the isolated checkout is below 9 MiB, within #36's 1-GiB budget. No SDK, external
provider, network audit, GPU or model execution occurred. The exact-byte corpus
manifest passes after explicit regeneration. Production schemas 1.0/2.0 and DA-001
match their pre-migration structural hashes; frozen native fixtures are unchanged.

This evidence covers the offline wire contract only. No claim of Go parity, native
worker isolation, cryptographic correctness or production availability follows.
