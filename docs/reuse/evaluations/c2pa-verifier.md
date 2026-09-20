# G2: encypher-c2pa verifier feasibility

Recommendation: **revise into a bounded worker design before adoption**. The
byte-verification API is promising, and the tested integrity/trust distinctions
fit Aletharsis. This is a Linux feasibility result, not cryptographic certification,
format-wide conformance or approval of a production dependency. Tracking: #38;
required adapter contracts: #36; release gate: #45; parent #21.

## Pin, build and distribution

The source is `encypherai/encypher-c2pa` at
`da85f07f2e0d084854be30dd3dc89dc0d2a9cde9`, with archive SHA-256
`9dcc42ea327ae057b7e8d3a2e338dbb540ddd005ed0042e2931cc6f8953aace8`.
Its workspace version is **1.0.6**, not the v1.0.5 latest-release observation in G0.
The pinned `Cargo.lock` was fetched explicitly, then all compilation ran offline
with Rust **1.88.0**, one build job, release mode and Go **1.27.1**.

The Linux-filtered normal/build dependency graph reachable from the C ABI and CLI
contains 153 packages, with exact versions, registry checksums and declared license
expressions in [dependencies.json](c2pa-verifier/dependencies.json). Root Apache-2.0
and fixture rights were inspected; declarations cover permissive alternatives,
but this is not a per-file redistribution clearance. No upstream implementation,
binaries, certificate payloads or image fixtures are redistributed by this PR.
Full security/advisory and notice review remains G6.

| Built artifact | Bytes |
| --- | ---: |
| Rust static C ABI library | 23,697,780 |
| Rust shared C ABI library | 5,063,456 |
| Upstream CLI | 5,698,664 |
| Independent Go byte probe linked to static library | 11,730,128 |

The Go binding links an archive at a repository-relative target path and uses CGO.
The probe ELF still requires system `libm`, `libgcc_s`, `libc` and the Linux loader.
One executable therefore does not mean a hermetic or pure-Go deployment. Two clean
Rust target directories produced matching hashes for all three Rust artifacts;
[manifest.json](c2pa-verifier/manifest.json) retains both identities. This establishes
same-environment reproducibility only, not cross-host/toolchain reproducibility.

| Platform | Evidence |
| --- | --- |
| Linux amd64 | Native Rust/CLI/Go build and byte-verification cases executed |
| Linux arm64 | Not built or executed in this study |
| macOS | Binding has Darwin linker directives; native validation not performed |
| Windows | No Windows-specific linker directive in the pinned Go binding; packaging/linking requires a separate investigation |

Do not infer native SDK support from Aletharsis core CI cross-builds. Those jobs do
not build this optional experiment module.

## Verification and authority

The final 11-case run has no execution failures or expectation mismatches.
Six pinned `contentauth/c2pa-rs` reference assets are loaded through upstream's
core corpus, checked against their declared hashes, and compared with its explicit
success/failure expectations. They supply an independent fixture origin, **not a
second verifier execution**. Synthetic upstream signed fixtures have explicit
Apache-2.0 rights in their README and contain test credentials, not customer data.
Independent inputs exercise unsigned text and the probe's size boundary.

Observed dimensions remain separate:

- A valid signature and matching content can coexist with `trust.status =
  not_evaluated` when default trust is disabled.
- A content-hash mismatch can coexist with a valid signature.
- An invalid signature is distinct from unknown binding, and both differ from
  no credential. The unsigned-text response uses `present = false`; its other
  fields must not be translated into a suspicious-document finding.
- Enabling the bundled trust material for the synthetic signer yields
  `not_valid_for_supplied_material` while cryptographic integrity remains valid.
  Offline revocation remains `not_checked` and freshness `unknown`; these are not
  passed checks. Fixed validation times, effective option hashes and the bundled
  trust-file identities are retained.

The initial exploratory run expected the informal label `untrusted`. Inspection
of the pinned `docs/REPORT_SCHEMA.md` confirmed the actual contract uses
`not_valid_for_supplied_material`. [exploratory-results.json](c2pa-verifier/exploratory-results.json)
retains that single harness-vocabulary mismatch; it is not presented as an upstream
bug. The final expectation was corrected against that contract and rerun.

These cases do not independently prove every signature algorithm, assertion or
format. No second verifier was built/run, and no positive custom trust-chain
acceptance claim is made. That differential work and a wider licensed adversarial
corpus remain prerequisites to a supported production verification profile.

## Side effects and execution limits

Every verification explicitly passes telemetry `enabled: false`. The ambient
`ENCYPHER_C2PA_TELEMETRY` value and saved preference are both enabled, deliberately
conflicting with the call. The interactive case attaches a pseudo-terminal to
stdin/stderr. All cases produced zero stderr bytes, no prompt or persisted-config
change, and unchanged source file/buffer hashes. Source fixtures were mounted
read-only. Private network namespaces prevented external transmission. Socket
attempts were not separately traced; do not interpret namespace isolation as
proof of syscall absence.

The binding has no context/cancellation parameter for `Verify`. Canceling a Go
caller does not establish that the in-process Rust invocation has stopped. The
worker demonstration instead terminates and reaps an observed-live process on a
bounded 32 MiB text input. That is process-boundary evidence, not cooperative
cancellation inside Rust or a completed production sidecar protocol.

Rust build: 137.932 seconds wall time, 113.642 CPU seconds and 648.3 MiB memory
peak. Resource transcripts retain the Go build, clean rebuild and case runs. A
2 GiB cgroup memory ceiling, zero swap and one-core CPU quota applied. Cases have
120-second wall deadlines, 115/120-second CPU soft/hard limits, and 32 MiB output
file limits. An enclosing runtime limit kills the process group on overrun. The
probe rejects inputs above 32 MiB before calling the SDK. Storage was below the
8 GiB study budget; reproduction checks retained storage after stages, **not a
filesystem quota**. No performance guarantee or worst-case bound follows.

## Adoption conditions and next design work

Prefer a small optional worker receiving immutable acquired bytes, with the
explicit telemetry-off option and a caller-selected, hash-identified trust policy.
Do not integrate the upstream path-reading convenience API into acquisition or
export its raw result objects as Aletharsis's wire contract. Rust/C ABI packing,
raw-response limits, time/memory/output enforcement, child cleanup, absent-runtime
behavior and native target validation require #36/#45 before implementation.

Keep signature, binding, static-trust evaluation, unavailable revocation and human
interpretation distinct. Preserve raw vendor semantics and exact source, options,
trust material and extracted-manifest identities. Do not allow ambient preference,
validation time or trust updates to silently change reports. Source code indicates
an explicit false override bypasses preference resolution; retain execution tests
for this behavior on every accepted upstream update.

This study does not approve the sidecar protocol, configure a trust service,
verify DOCX/PDF support, add a core dependency, or change schema 2.0. The registry
remains `evaluating`. Next step is the adapter-contract work informed by this and
the separate carrier study, followed by a narrowly scoped adoption proposal.

## Reproduction and evidence

[Experiment instructions](../../../experiments/c2pa-verifier/README.md) separate
explicit pinned provisioning from offline execution. Raw observations, the initial
expectation correction, corpus/source identities, dependency inventory, trust
snapshot hashes, build hashes and resource logs are retained in the adjacent
[c2pa-verifier directory](c2pa-verifier/manifest.json). Core CI verifies retained
artifact identities and report assertions; it does not fetch or run the SDK.
