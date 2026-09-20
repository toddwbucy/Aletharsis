# G2: c2pa-text extraction feasibility

Disposition: **revise before adoption**. This closes a bounded investigation, not
G1 adapter design, G6 release acceptance or any production capability. Tracking:
[#37](https://github.com/toddwbucy/Aletharsis/issues/37), parent #21.

The pinned Go implementation is useful for further carrier work, but its HTML
extractor is not suitable as our document-context authority. All 28 upstream tests
pass. Our independent 39-case corpus completes without crashes/timeouts and exposes
**eight HTML expectation mismatches**. Repeating the entire offline build and
corpus in a fresh directory produces byte-identical probe binaries and results.

## Identity and scope

| Item | Observed identity |
| --- | --- |
| Upstream | `encypherai/c2pa-text` commit `ad4eaee3705ea5edb610ab37041be496d012e583` |
| Source archive SHA-256 | `e66fb4ff79e9807104620ae4797b6ad2ed17d097b85f9001eb086a81e487ce26` |
| Actual Go module | `github.com/encypherai/c2pa-text/go/v3` (the pinned README still demonstrates v2) |
| Dependency | `golang.org/x/text v0.14.0`; `go.sum` module hash `h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ=` |
| Licensing | Root MIT text retained in G0; x/text BSD-3-Clause text inspected. No upstream implementation or vectors redistributed here |
| Environment | Go 1.27.1, Linux/amd64; exact Python/kernel identity in [manifest](c2pa-text/manifest.json) |
| Supported study surface | ExtractManifest, ValidateText, ExtractStructured and ExtractHTML; no signatures, trust decisions or source writers |

The [C2PA 2.4 embedding specification](https://spec.c2pa.org/specifications/specifications/2.4/specs/ContentCredentials.html)
A.7–A.9 is the normative reference. The variation-selector wrapper carries a magic,
version and declared byte length. HTML associations use the appropriate head
elements; structured carriers additionally depend on host comment/front-matter
context. Extraction is not validation of a signed credential. The corpus uses an
eight-byte JUMBF-shaped placeholder so byte recovery can be tested independently
of cryptography. None of these synthetic placeholders is a valid signed manifest.

## Executed findings

| Observation | Evidence and consequence |
| --- | --- |
| Valid HTML syntax missed | `html_single`, `html_upper`, `html_attribute_space`, `html_reference_single`: single quotes, uppercase tag names and whitespace around `=` are not recognized |
| Non-associations accepted | `html_comment`, `html_data_type`, `html_prefix_tag`, `html_body`: literal substring matching treats comments, unrelated attributes, another tag name and body content as associations. Preserve such bytes as possible misplaced/carrier evidence, not a conforming HTML association |
| Raw and normalized coordinates differ | `vs_decomposed` returns offset 3 for raw `e` + combining acute, while its clean-text derivative is two UTF-8 bytes. BOM, non-BMP, combining reorder and mixed line endings are covered. Never apply returned source coordinates to the normalized derivative |
| Malformed VS carriers can look absent through extraction alone | Truncated, unsupported-version and huge-length cases return no manifest with no extraction error. `ValidateText` separately reports corrupted-wrapper evidence. A missing prefix is not reported by that validator; native Unicode inventory must remain independent |
| Multiple VS wrappers cannot be individually selected by this API | The extraction error exposes ambiguity but loses per-wrapper payload/coordinates. G1 needs enumeration and explicit exclusions/binding representations |
| Structured API is deliberately lexical | It returns reference/bytes without source spans or host validation. Bare delimiters are extracted. Invalid base64 leaves a nonempty reference but no manifest, without an error; our evidence model must distinguish that from no carrier |
| Bounded large inputs | An 8 MiB prefix plus small wrapper is extracted at the expected byte offset. The probe rejects an input above 16 MiB before invoking upstream. This is a probe limit, not a library guarantee |

Raw expectations, observations, input hashes, rationales and mismatches are in
[results.json](c2pa-text/results.json). Mismatches intentionally remain visible;
experiment completion is not a zero-discrepancy conformance claim. Source hashes
are checked after every case. Originals are synthetic temporary inputs, not user
files. The upstream suite exercises its own golden vectors in the downloaded tree;
the golden file hash is retained, but those vectors are not copied into Aletharsis.

## Isolation and resources

Runs used bubblewrap with a private network namespace, minimal filesystem,
no user home, read-only toolchain/upstream mounts, and a systemd user cgroup.
The memory ceiling is 1 GiB with no swap, CPU quota one core. Each case has a
60-second wall deadline, 55/60-second CPU soft/hard limits and 16 MiB output-file
limits; the enclosing run has a 300-second deadline and cgroup cleanup. These
fixtures completed without reaching those ceilings. The probe exposes no child
process API. This is a Linux study harness, not a portable production sandbox.

Initial build plus upstream tests: 23.945 CPU seconds, 416.7 MiB peak memory.
Case run: 0.265 CPU seconds, 153.3 MiB peak memory. Fresh reproduction: resource
records are retained in [reproduction-build.stderr](c2pa-text/reproduction-build.stderr)
and [reproduction-cases.stderr](c2pa-text/reproduction-cases.stderr). Both binary
hash and result bytes matched. All study storage remained below the 2 GiB budget;
the harness checks retained size after completion, **not a filesystem quota**.
Wall/CPU/output and memory limits do not imply a proven worst-case complexity bound.

## Required follow-up

1. Finish #36 with distinct original, decoded, NFC binding, exclusion and manifest
   identities. Structured/HTML carrier locators must come from a format-aware
   source map. Do not substitute `clean` for authoritative source evidence.
2. Separate lexical discovery from conforming host association. Require actual
   HTML parsing/context and malformed-carrier results before accepting HTML scope;
   decide whether to seek an upstream correction or place the parser in Aletharsis.
3. Scope an adoption proposal to precisely tested APIs; re-evaluate additional
   malformed/host-format cases, adversarial resource growth, transitives/security
   advisories and native target support under G6. No blanket acceptance from these
   Linux examples. Upstream fixes need a new pinned revision and semantic diff.

No upstream issue or code change was submitted. The registry remains `evaluating`,
with license clearance and full runtime/security acceptance still pending. This
recommendation authorizes no production dependency, remote manifest retrieval,
cryptographic claim, cleanup or schema change.

## Reproduce

See [experiment instructions](../../../experiments/c2pa-text/README.md). The driver
uses supplied pinned artifacts and runs offline; normal Aletharsis CI only checks
retained study evidence and corpus identity. It does not fetch/build this detector.
