# Offline C2PA verifier feasibility (#38)

This module is a development probe, not a production adapter. It calls the pinned
Go byte API with explicit telemetry-off and fixed verification options. The
runner checks pinned fixture hashes, observed integrity/trust semantics, an
interactive no-prompt case, unchanged inputs/preferences and worker termination.
It never calls the SDK's original-path convenience API.

Provision separately and explicitly:

- Source archive at `https://codeload.github.com/encypherai/encypher-c2pa/tar.gz/da85f07f2e0d084854be30dd3dc89dc0d2a9cde9`, SHA-256
  `9dcc42ea327ae057b7e8d3a2e338dbb540ddd005ed0042e2931cc6f8953aace8`.
- Rust 1.88.0 Linux/amd64 minimal toolchain, Go 1.27.1, a C compiler and linker.
- After inspecting root/fixture and dependency license declarations, use
  `cargo fetch --locked --target x86_64-unknown-linux-gnu --manifest-path <pinned-source>/Cargo.toml`
  with an isolated `CARGO_HOME`. This downloads the pinned lockfile's packages;
  it does not run build scripts. Do not change global user toolchain configuration.
- A Linux host with bubblewrap and working systemd user services. Python 3.12+
  drives the study; no Python runtime is added to Aletharsis.

Then run without network:

```bash
python experiments/c2pa-verifier/reproduce.py \
  --archive /tmp/encypher-c2pa.tar.gz \
  --rust /absolute/path/to/rust-1.88.0-distribution \
  --cargo-home /tmp/verifier-cargo-cache \
  --go /absolute/path/to/go-distribution \
  --work /tmp/verifier-study-fresh
```

The work directory must not exist. Archive identity/type/expansion checks precede
builds. The script copies registry archive/index caches, re-extracts packages,
builds with `--frozen`, and records the sandbox argv and resource/output logs.
The namespace exposes only system/toolchain/source/work files, not user home or
network. Rust builds have a 30-minute deadline, the Go build five minutes and the
case group ten minutes, all at one CPU/2 GiB/no swap. Per-case deadlines, CPU and
output limits are in the driver. Storage is measured after stages, not hard-quota
enforced; broader hostile experiments need filesystem quotas.

Read `<work>/cases/results.json`; a zero runner exit means completed measurement,
not automatically matching expectations. Inspect execution failures, mismatches
and whether a cancellation worker was actually observed alive. Retain the original
result when correcting a harness expectation; do not rewrite a failed observation
into a pass. The fixture corpus is provided by the pinned SDK archive and checked
by hash. Its original rights and source revision must accompany any later
redistribution; this PR does not distribute fixture bytes or built SDK binaries.

No third-party verification service is contacted. Upstream reference expectations
are not a second implementation; independent-verifier comparisons remain a release
gate. The report distinguishes those limits from the native Linux observations.
