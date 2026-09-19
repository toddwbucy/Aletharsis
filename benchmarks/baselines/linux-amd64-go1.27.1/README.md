# Initial Linux/amd64 resource baseline

Measured 2026-09-19. These are observations on one development machine, not service guarantees or worst-case bounds. All 54 serial measurements and 36 separate concurrency measurements completed with valid source identities and expected exit statuses. No timeouts occurred.

- Production behavior: merged PR #12 (`7b47808163ccf05667fc87ade13e9d5ca1e35a99`).
- Measurement code/build revision: `9e71a894bcf81aa9888b2af879388612d03fe2aa`; clean worktree at build/process-run start. Later documentation and harness regression tests do not change the measured code.
- Intel Core Ultra 7 265HX, 20 logical CPUs, Linux 7.2.2-1-cachyos x86_64, Go 1.27.1, Python 3.12.13, GNU time 1.10. Each process/stage uses GOMAXPROCS=2. Host load and power state were not controlled; no other Aletharsis benchmarks ran concurrently with these retained measurements.
- Executable SHA-256: `72c1011f24b757bbdb639e01c0ca295b1250baadeb8cf994baf644c1b19f861b`.
- [Raw process measurements](results.json), [corpus identities](corpus-manifest.json), and [raw stage measurements](stages.txt) are retained; large inputs and reports are reproducibly generated, not committed.

Reproduction commands and metric definitions are in the [performance guide](../../../docs/testing/performance.md). From the repository root, the measured commands were equivalent to:

```bash
GOTOOLCHAIN=local go build -trimpath -o /tmp/aletharsis-perf-final ./cmd/aletharsis
python scripts/measure_backend.py --binary /tmp/aletharsis-perf-final \
  --go go --output /tmp/aletharsis-measure-final --repeats 3 --timeout 120
GOTOOLCHAIN=local GOMAXPROCS=2 ALETHARSIS_BENCH_SIZES=65536,1048576,8388608 \
  go test ./internal/benchmarks -run='^$' -bench=BenchmarkStages \
  -benchmem -benchtime=1x -count=3 -timeout=19m
```

The local commands selected absolute paths to the pinned Go/Python executables and reused `/tmp` Go build/module caches. Run the CLI experiment to completion before the stage experiment. No warm-up samples were discarded. One-operation stage samples avoid turning a large-input benchmark into an unbounded calibration exercise, but are noisier than extended microbenchmarks.

## Whole executable

Wall times are median and observed min–max across three fresh processes. RSS is the largest observed per-process peak, not a median. MiB means 1,048,576 bytes.

| Corpus | Input MiB | Median seconds | Min–max seconds | Max peak RSS MiB | JSON MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| ascii | 0.0625 | 0.041 | 0.040–0.043 | 20.7 | 1.12 |
| multilingual | 0.0625 | 0.027 | 0.027–0.027 | 14.8 | 0.73 |
| source | 0.0625 | 0.038 | 0.038–0.039 | 23.1 | 1.12 |
| zero_width | 0.0625 | 0.026 | 0.026–0.027 | 26.9 | 1.87 |
| periodic | 0.0625 | 0.032 | 0.031–0.033 | 24.7 | 1.50 |
| combining | 0.0625 | 0.041 | 0.037–0.041 | 24.7 | 1.77 |
| ascii | 1 | 0.562 | 0.560–0.575 | 212.6 | 18.96 |
| multilingual | 1 | 0.321 | 0.313–0.344 | 130.3 | 12.13 |
| source | 1 | 0.516 | 0.504–0.517 | 209.5 | 18.97 |
| zero_width | 1 | 0.353 | 0.333–0.360 | 335.2 | 31.74 |
| periodic | 1 | 0.458 | 0.437–0.478 | 275.2 | 25.39 |
| combining | 1 | 0.505 | 0.504–0.535 | 283.2 | 29.84 |
| ascii | 8 | 5.573 | 5.521–5.576 | 1787.9 | 159.08 |
| multilingual | 8 | 2.923 | 2.901–2.937 | 1097.4 | 101.03 |
| source | 8 | 5.149 | 5.144–5.168 | 1794.4 | 159.06 |
| zero_width | 8 | 2.822 | 2.794–2.931 | 2455.7 | 266.16 |
| periodic | 8 | 4.042 | 4.007–4.076 | 2046.6 | 213.08 |
| combining | 8 | 4.722 | 4.720–4.763 | 2757.0 | 249.89 |

The 8 MiB ASCII input reports **zero findings**, yet retains 8,388,609 scalar-to-byte coordinates and produces about 159 MiB of JSON. The zero-width report reaches about 266 MiB. Findings alone are therefore a poor predictor of report size. The largest measured child RSS is about 2.69 GiB (combining), and the 8 MiB cap is not a small process-memory cap.

## Concurrency (64 KiB only)

Each batch has twelve jobs; batch time includes parent decoding/validation and scheduling. Three worker-count experiments are individual observations, not repeated throughput estimates.

| Workers | Batch seconds | Jobs/second | Max child RSS MiB | Upper envelope MiB |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 0.447 | 26.8 | 26.2 | 26.2 |
| 2 | 0.230 | 52.2 | 26.3 | 48.9 |
| 4 | 0.133 | 90.0 | 25.5 | 98.8 |

The envelope sums the largest N per-child peaks; it is **not** measured simultaneous RSS and excludes the Python parent. These small-file results do not validate multi-worker large-file operation.

## Proposed starting budgets and regression review

- Plan for **one large-file audit at a time** initially. Reserve roughly **5 GiB per large-file worker**, plus separate frontend/service/OS headroom, when sizing an environment for this corpus. This exceeds 1.5× the observed 2.69 GiB peak, but is a planning allowance, not an enforced bound or a guarantee for all inputs.
- Exercise frontend imports through at least **300 MiB of JSON** on the target hardware, covering the measured 266 MiB report with some headroom. Measure retained raw bytes, hashing, decoded JSON, coordinate indexes and UI objects together. The backend's memory figure excludes all those frontend costs. Do not interpret this test target as a report-size limit.
- For regression review, rerun on matching hardware/toolchains with matching corpus/output identities. Investigate median wall-time increases above **20%**, peak RSS above **15%**, or stage allocated bytes above **10%**, confirmed in three fresh measurement batches. Treat these as proposed review triggers, not CI failure thresholds; establish normal variance and any statistical comparison policy first.
- Record deliberate report/schema changes separately: output size increases can reflect additional evidence, while reductions may hide lost evidence. Existing correctness/parity/schema/coordinate gates remain prerequisites for accepting an optimization.

Serialization allocates multiple report-sized buffers and coordinate arrays remain large. These are inspection leads from code/measurements, not a profiler attribution of the complete RSS peak. Any optimization, compact coordinate representation, bounded output proposal, or change in scan scheduling belongs in a separate reviewed change.

## Stage measurements at 8 MiB

Each cell is **median seconds / median allocated MiB per operation**, across three one-operation samples. Allocations are cumulative Go `B/op`, **not peak RSS**; untimed preparation is excluded. Full 64 KiB/1 MiB results and allocation counts are in `stages.txt`.

| Corpus | Parse s / MiB | Analyze s / MiB | Serialize s / MiB | Audit s / MiB |
| --- | ---: | ---: | ---: | ---: |
| ascii | 0.088 / 455.0 | 4.384 / 1811.6 | 0.863 / 3076.4 | 4.556 / 2745.8 |
| multilingual | 0.055 / 271.5 | 2.140 / 961.2 | 0.614 / 1789.8 | 2.279 / 1545.7 |
| source | 0.089 / 455.0 | 4.123 / 1706.2 | 0.809 / 2983.8 | 4.301 / 2640.1 |
| zero_width | 0.048 / 202.3 | 1.064 / 859.6 | 1.454 / 4765.9 | 1.229 / 1485.4 |
| periodic | 0.065 / 319.7 | 2.655 / 1163.6 | 1.179 / 3893.3 | 2.811 / 1819.2 |
| combining | 0.053 / 271.5 | 2.909 / 1476.9 | 1.421 / 4388.3 | 3.184 / 2544.4 |
