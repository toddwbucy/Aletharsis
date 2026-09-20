# DA-001 — Bounded detector adapter contract, version 1

Status: proposed for G1 review under #36; acceptance requires the maintainer's
reviewed merge. Parent: [ADR-0002](../adr/0002-detector-reuse-policy.md), #21.
Inputs: accepted native #17 contracts and feasibility PRs #47/#48.
This document and its offline conformance cases specify a boundary; they do not
implement an adapter, authorize a dependency, or advertise a new capability.

## 1. Authority and scope

The host owns acquisition, byte identity, invocation policy, result validation,
coverage, locations and reporting. The worker receives one bounded immutable
asset and explicit configuration, never the original path or filesystem authority.
Worker responses are untrusted claims. Only the host may mint report artifacts,
anchors, execution states and findings after validation. Results cannot authorize
remediation, fetches, commands, trust changes or another detector invocation.

Version 1 covers local single-asset extraction, credential verification and an
optional statistical operation. Batch orchestration creates separate invocations.
Remote services, multiple concurrent requests per worker, unsolicited callbacks,
key delivery and streaming partial results are outside this version. A detector
requiring any of these remains unavailable until a reviewed extension exists.
Direct library integrations obey the same identity/result rules. A foreign library
without enforceable cancellation/resource bounds uses an isolated worker; Go
context cancellation alone does not stop a Rust/C call.

## 2. Identity and representations

All digests are lowercase SHA-256 of the exact indicated bytes. Never hash a
rendering, JSON reserialization or normalized text and call it the source hash.

| Artifact | Bytes covered |
| --- | --- |
| source | Exact acquired file bytes, including BOM and original line endings |
| decoded | UTF-8 encoding of decoded Unicode scalars, no implicit NFC or newline changes |
| normalized | UTF-8 of the explicitly named/versioned normalization operation |
| excluded/binding | Exact output of the named/versioned carrier-exclusion or asset-binding procedure |
| manifest | Extracted binary manifest bytes before provider decoding or JSON conversion |
| raw provider result | Exact bounded provider response bytes before Aletharsis interpretation |
| policy/trust/config | Exact retained inert configuration or bundle bytes with documented serialization |

Each derived artifact retains parents, operation/version, configuration digest,
exclusions and mapping quality under the native artifact model. Object/stream or
package-part digests supplement, never replace, the outer source identity. Never
synthesize a manifest digest from a provider's decoded JSON; if the provider cannot
expose manifest bytes, report extraction/mapping unavailable rather than guessing.

Byte ranges are zero-based half-open intervals in the named artifact. Scalar,
UTF-16 viewer coordinates, PDF object locations and OOXML locators are distinct
units. Exact source mapping requires independently validated boundaries/content.
NFC can compose or reorder scalars: raw-input offsets returned by c2pa-text cannot
index its NFC clean output. Retain both artifacts and a derived/unavailable map.
A one-to-many map is a list of source/target spans, not a fabricated constant delta.
Carrier exclusions retain every removed interval and the binding specification
revision. Overlapping or ambiguous carriers remain observations; do not choose one
silently. HTML requires host parsing and conforming placement; regex discovery in
comments, body, data attributes or similarly named tags is not a head association.
Structured carriers also require host syntax/context validation. A malformed
wrapper is neither credential absence nor a verified credential.

The executable exchange uses acquired `source` and selected `input` identities.
The host evidence graph validates their parent chain before dispatch. The request
serializes that chain in `transforms`: ordered input/output identities, operation
and version, map quality, source/target byte-span pairs and exclusions. Each output
is the next input; the chain starts at source and ends at the selected input.
Source input has an empty chain. Every pair is in bounds, excludes removed parent
regions and collectively covers output bytes in order. Source spans may reorder
or repeat for a derived map. An exact pair must preserve identical bytes; decoding
or normalization that changes bytes is derived, even if scalar navigation is
otherwise known. Unavailable maps have no pairs. The transformation metadata does
not prove the algorithm ran: adoption tests must independently check it. Input may
be a decoded/binding/sample artifact, but never acquires source-byte coordinates
merely by carrying the same filename. Worker carrier spans refer only to input
bytes; the host must separately prove any source mapping.

## 3. Request and transport

[adapter-exchange-v1.schema.json](../../schemas/adapter-exchange-v1.schema.json)
defines the closed request, response and host-outcome transcript. Unknown fields,
unknown versions, duplicate JSON keys, invalid UTF-8, non-finite numbers, unsafe
integers, excessive depth and trailing data fail closed. No dynamic schema loading.
The fixtures use inline hex blobs solely as an offline byte-store substitute;
production transport does not hex/base64 expand inputs into JSON.

One process handles one request. Stdin is an unsigned 32-bit big-endian JSON-header
length, exactly that many UTF-8 request bytes, then exactly `input.byte_length`
asset bytes and EOF. Header maximum is 64 KiB; JSON depth maximum 32. No original
paths, URLs, environment inheritance or shell command strings are protocol fields.
The request carries a deterministic request digest, capability, wrapper build
identity, upstream pin, input/source hashes, policy/config/trust identities,
validation time and positive limits. Config and optional trust bundles are
preprovisioned read-only by digest in the sandbox; their exact bytes are retained.
They are inert data interpreted by a pinned adapter, never executable code.
Version 1 carries no secret material. A statistical algorithm needing keys stays
unavailable until a separate secret-delivery/security contract is reviewed.

Stdout is one length-prefixed response JSON frame followed by EOF. The response
references raw provider bytes by digest; bounded raw bytes use a host-provided
private spool, not an arbitrary worker path. For the initial POSIX launch profile,
inherited FD 3 is the raw-provider spool and FD 4 is the optional extracted-manifest
spool; stdin/stdout/stderr are FDs 0/1/2. Close every other inherited descriptor.
Both spools are host-created, quota-bounded files held by handle. A null manifest
requires an empty manifest spool. No worker-selected paths or filenames are read.
Windows handle transport requires its own reviewed launch profile before support. The host accepts only its assigned
regular file handle, refuses links/special files, hashes the actual bytes and
checks the raw-result cap. Stderr is separately bounded, escaped and quarantined;
it is not report prose or a command channel. No terminal/interactive stdin.
The host rejects a declared excessive frame length before allocating it and counts
actual bytes while reading. Total output, including raw/manifest spools and stderr, must
stay within the request output limit. The protocol version is not negotiated down.

The worker verifies input length/hash before calling its provider and echoes the
request identity and input hash in its response. The host verifies them again.
Neither an echoed hash nor a successful subprocess exit is sufficient acceptance.
Record executable digest, wrapper version, upstream commit, effective options,
fixed validation instant, trust bundle identity (or explicit no-trust policy), and
policy snapshot. Ambient preferences, clock defaults and auto-updated trust lists
must not silently affect a run. Explicit telemetry-off overrides ambient settings.

## 4. Lifecycle and failures

Host state is authoritative. Before launch, validate provisioning, pin, native
platform support, policy, identities, graph and budgets. Missing runtime, disabled
capability or unsupported platform yields `not_run/unavailable`, not a finding or
negative detector result. Source/input mismatch yields `failed/source_mismatch`
before dispatch or acceptance. Do not reopen the original path to recover.

Apply wall, CPU, memory, input, output and scratch limits before execution. The
schema carries per-run budgets; adoption selects tighter measured profiles, never
silently raises them. The global v1 ceilings are 120 seconds wall, 120 CPU seconds,
2 GiB memory, 32 MiB input, 32 MiB total output and 256 MiB scratch. A limit that
cannot be enforced on a platform makes that adapter unavailable there. Provisioning
is explicit and separate from audits; no audit-time network, prompts or downloads.
Use an empty environment plus an allowlist, denied network, read-only inputs and
bundles, private writable quota-limited scratch, and no home-directory access.
Native enforcement tests are required for each advertised platform.

On deadline or cancellation, stop accepting responses, terminate the whole process
tree, allow at most one second grace, force termination and reap all descendants.
Release handles, remove private scratch and record cleanup success. On cleanup
failure stop further launches and report failure; do not claim cancellation has
finished. Diagnostics must preserve the primary cause as well as cleanup failure.
The transcript's single reason describes the terminal host disposition; detailed
host diagnostics retain both causes in the future report model.

| Host state / reason | Accepted results |
| --- | --- |
| not_run / unavailable | None; no response |
| failed / timeout, malformed_output, oversized_output, source_mismatch, provider_failure, cleanup_failed | None |
| canceled / canceled | None |
| partial / partial | Valid response with usable results and explicit nonoverlapping exclusions |
| completed / none | Valid response covering the entire requested input |

The request digest is RFC 8785 JCS SHA-256 of `{"domain":
"aletharsis.adapter-request/1", "value": <request without request_id>}`.
Identical requests may share this identity; host execution references distinguish
invocations. A response is accepted only on its own invocation channel.

Version 1 commits only a complete validated response after successful process exit
and cleanup. Timeout after a prefix or even a complete frame discards it as usable
results (bounded bytes may remain quarantined). No salvage of partial JSON or late
frames. A provider may return a deliberate `partial` response with disjoint checked
ranges and exclusions; the union must exactly account for the requested input.
Partial results are supported only for extraction in version 1. Verification and
statistical results describe the entire input; partial sample work requires a new
explicit input artifact and invocation. No implicit whole-document negative from a checked subset. Empty input can complete
with empty ranges; meaningful provider limitations must still be reported.

## 5. Typed result boundaries

`operation` selects a fixed result shape. An adapter emits exactly one aggregate result in its result list
with raw provider evidence retained for each accepted response. All result spans
must lie in completed ranges. Raw/provider fields are never pasted into the public
wire model; the adapter's versioned mapping defines every promoted claim.

- **Extraction:** outcome `absent`, `observed`, `malformed` or `ambiguous`;
  carrier input-byte spans and nullable extracted-manifest identity. Absence has
  no spans/manifest. Other outcomes require localized observations in this v1
  profile; unsupported mapping yields no extract result and a failed/partial
  operation, never a guessed offset. Only `observed` may carry extracted bytes.
  Extraction establishes no signature, binding or trust claim. Discovery of an
  external reference does not authorize fetching it.
- **Verification:** discovery separately from signature (`valid`, `invalid`,
  `not_checked`), asset binding (`match`, `mismatch`, `not_checked`), trust
  (`accepted`, `rejected`, `not_evaluated`), revocation and freshness. Credential
  absence requires all other checks unevaluated and no manifest. Discovery may be
  present while exact manifest identity is unavailable; retain that limitation.
  Signature-valid plus binding-mismatch is legal. Trust acceptance requires an
  explicit trust bundle; no-trust is not rejection. Offline `not_checked` revocation
  and `unknown` freshness are not passed checks. Assertions remain claims even if
  signature and trust succeed. Version 1 does not claim assertion truth.
- **Statistical:** algorithm/version/configuration, exact input sample digest,
  provider-defined outcome, nullable score with named units and semantics reference.
  A score is a finite JSON number, not universal confidence/probability. Do not
  invent a conversion. Tokenizer/model identities and sample transformations belong
  in the retained configuration/parent graph. No passive authorship claim, unknown
  model rejection or calibrated probability without its own validated profile.

Raw response retention has a strict cap and access mode 0600 (directory 0700), is
opt-in under the local evidence-retention policy, and may expose sensitive content.
If policy prevents raw retention, this v1 reproducible profile is unavailable;
a future redacted profile must label reduced reproducibility, not substitute a
redacted digest for original bytes. The fixtures contain only synthetic data.

## 6. Report/schema migration gate

Do not add these payloads to schema 2.0, its Go tagged unions or capability catalog.
Its strict importers correctly reject unknown result variants. This exchange is
an internal contract proposal, not a schema-2.0 report or a public service API.
Before production implementation, propose report **3.0** in a separate reviewed
migration PR: closed extraction/verification/statistical result variants, raw blob
references, configuration/trust/time identity, new diagnostics and catalog entries,
and semantic cross-reference validators. Reuse native artifacts, anchors and
execution concepts where valid; extend location-map payloads explicitly rather
than abusing the current text-offset pointer shape. Keep schema 1.0/2.0 import
behavior, old reports and identities unchanged. New consumers must preserve exact
import bytes/digest and indicate unsupported versions without negative inference.

That PR must provide full report fixtures and Go marshal/import parity, consumer
alignment, aggregate/exit-code cases and mixed native/optional failure tests.
The adapter transcript's short reason codes are local to this protocol; it may
not inject them into the current global failure registry. The migration defines
namespaced codes and exact coverage/summary consequences before implementation.
No statistical/cryptographic result is a `structural_scan` compatibility shim.
A reviewed exchange contract does not by itself accept this future report version.

## 7. Conformance and remaining gates

`tests/adapters` exercises the closed schema and cross-record checks offline. The
synthetic byte store verifies input, source, config/trust, manifest and raw digests.
Thirteen positive fixtures include unavailable, timeout, partial, malformed/oversized output,
source mismatch, credential dimensions and provider-specific score semantics.
Negative mutations reject invalid claims, identities and bounds. These are contract
checks, not tests of OS sandboxing, C2PA cryptography or a working sidecar.

Run `python -m pytest -q tests/adapters`, then the complete schema/reference suite.
CI already discovers these tests. No network, SDK, upstream fixtures or runtime
dependency is added. [Validation record](../reuse/evaluations/adapter-contract.md)
records the actual run and budgets. Both C2PA registry entries remain evaluating;
G6 native enforcement, independent verifier/vector comparisons, licensing/notices,
disable/rollback and adoption review remain open. #7 and #21 stay open.
