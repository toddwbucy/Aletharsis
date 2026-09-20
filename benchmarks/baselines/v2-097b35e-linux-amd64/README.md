# Initial v2 resource probe — acceptance remains open

This probe measures the clean CLI build at `097b35ebad5ad79035a55055cc71e345937f95f4` (PR #30) on Linux/amd64 with Go 1.27.1. Exact binary, harness and corpus digests, build information and machine context are retained in `results.json`. The harness checkout is marked dirty because it contains the new schema-selection/validation support; the candidate executable itself records `vcs.modified=false`.

One serial sample per corpus/size and a single-worker batch of two samples per 64 KiB case were measured. This is a diagnostic probe, not a statistically established performance baseline or a hard latency/memory guarantee. Only synthetic corpus data was used. Python validation happens outside the measured child and its memory is excluded.

```sh
go build -trimpath -o /tmp/aletharsis-v2-release-probe ./cmd/aletharsis
python scripts/measure_backend.py --binary /tmp/aletharsis-v2-release-probe \
  --output /tmp/aletharsis-v2-resource-probe --schema-version 2.0 \
  --sizes 65536,1048576,8388608 --repeats 1 --workers 1 \
  --concurrency-size 65536 --timeout 60
```

| Input case / size | Outcome | JSON bytes | Wall seconds | Peak RSS KiB |
| --- | --- | ---: | ---: | ---: |
| ASCII / 64 KiB | Valid report | 469,437 | 0.125 | 22,996 |
| Multilingual / 64 KiB | Valid report | 437,141 | 0.156 | 25,340 |
| Source / 64 KiB | Valid report | 535,555 | 0.154 | 27,268 |
| Zero-width / 64 KiB | Budget rejection | 0 | 0.022 | 15,884 |
| Periodic / 64 KiB | Budget rejection | 0 | 0.044 | 20,532 |
| Combining / 64 KiB | Budget rejection | 0 | 0.036 | 17,976 |
| ASCII / 1 MiB | Budget rejection | 0 | 0.494 | 185,060 |
| ASCII / 8 MiB | Budget rejection | 0 | 5.068 | 1,312,100 |

All six 1 MiB and all six 8 MiB cases were rejected with exit 4 and `execution.resource_limit`, without stdout report bytes. None timed out. The other case measurements and concurrency rows remain in the raw result. A successful *measurement* of a rejection is not a successful audit: `resource_rejected=true` and `valid_report=false` remain distinct.

## Consequences for #17

The current defaults and record checks reject useful inputs well below the 8 MiB acquisition limit. The 8 MiB ASCII case also consumes approximately 1.25 GiB RSS before rejection. These observations do **not** establish acceptable production resource behavior, full 8 MiB v2 support, or readiness to switch the default schema.

Before acceptance, investigate the interaction of finding ordering-key limits, selection/anchor record budgets and whole-report node counts; avoid re-decoding or duplicating evidence unnecessarily and reject impossible budgets earlier where safe. Any budget changes must preserve independent import validation and coordinate fidelity, then repeat this corpus. Do not silently truncate findings or raise all limits merely to make the test pass. Retain schema 1.0 as the default during review.

The separate browser/viewer spike remains required. These backend measurements do not select a component, measure a browser heap or complete F0.
