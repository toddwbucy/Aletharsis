# Offline C2PA carrier experiment (#37)

This is a separate development-only Go module. It is not reachable from the
production module or CLI. No upstream code or fixture payloads are vendored.
`cases.py` independently constructs synthetic inputs and states expectations before
execution; `main.go` invokes the upstream extractors without writing inputs.
`run_cases.py` retains observations and discrepancies instead of treating upstream
outputs as ground truth. The eight-byte placeholder is not a signed credential.

Provision explicitly before running (network permitted only for this step):

```bash
curl --fail --location --max-time 90 --max-filesize 104857600 \
  https://codeload.github.com/encypherai/c2pa-text/tar.gz/ad4eaee3705ea5edb610ab37041be496d012e583 \
  -o /tmp/c2pa-text.tar.gz
GOTOOLCHAIN=local GOMODCACHE=/tmp/c2pa-study-modules go mod download golang.org/x/text@v0.14.0
```

Use an explicitly provisioned Go 1.27.1 toolchain (the run recorded in this PR),
Linux/amd64, Python 3.12+ and working `bwrap` / systemd user service support. Read the
root MIT and dependency BSD-3-Clause licenses before executing the study; the
source archive and module sums are recorded in the feasibility report. This
command does not install a global package or change the production `go.mod`.

```bash
python experiments/c2pa-text/reproduce.py \
  --archive /tmp/c2pa-text.tar.gz \
  --go /absolute/path/to/go-distribution \
  --modules /tmp/c2pa-study-modules \
  --work /tmp/c2pa-study-fresh
```

`--work` must not exist. The archive digest, type/expanded-size checks and safe tar
extraction precede builds. Network is unavailable during compilation and cases;
only the supplied module cache is mounted. cgroup memory/CPU limits, per-case
limits and raw logs are described in the report. Provisioning failure, timeout or
budget exhaustion is failed/partial evidence, never a passed detector test.
The size check is retrospective; use a filesystem quota for hostile broader
experiments. The exact pinned source, small corpus and measured storage are the
scope of this run.

Results: `<work>/cases/results.json`; upstream test log and per-run resource/argv
records: `<work>/results/`. A zero driver exit means the experiment completed,
not that the upstream detector met every expectation. Inspect `execution_failures`
and `expectation_mismatches`. `results.json` must match the retained run for this
corpus/toolchain/pin; timing, process IDs and service-unit names need not match.

The selected upstream tests include their own in-memory embedding helpers and
golden vectors; that is development conformance testing, not adoption of a writer.
Original user documents are never used. External reference examples use
`example.invalid`; they must remain unfetched.
