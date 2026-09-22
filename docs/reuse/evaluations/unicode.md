# G3: independent Unicode comparison study

Recommendation: **revise the native inventory/profile plan and retain these tools
as scoped development comparators; no production adoption**. Tracking: [#39](https://github.com/toddwbucy/Aletharsis/issues/39),
parent #21; follow-up requirements #14/#15, adoption #45. Owner acceptance pending.

Thirty independently authored synthetic cases were executed against Aletharsis and
two pinned upstream read-only detection probes. All 90 processes exited within the
configured limits; this does **not** mean every source was analyzed. Native
Aletharsis completed 29 audits and rejected one control-heavy source as binary.
HIBERIUS emits no verdict or visualization for empty input. This is recorded as
`no_verdict_emitted`, not inferred from input length; it does not establish a
completed scan.

Repeating the final corpus produced identical bytes for all 90 stdout and all 90
stderr artifacts. Twenty cases have inventory, policy, coverage or parser
differences. They remain in the report, not adjusted away to obtain agreement.
No attribution or intent is inferred from a character being present.

## Pinned surfaces and independent expectations

| Tool | Pin and executed surface |
| --- | --- |
| Aletharsis | Main `abc513e354e864a38d3d57649667c22ba6080cc7`, Go 1.27.1, compiled `audit --json`; native original-byte acquisition/identification/decoding |
| juriku/hidden-characters-detector | `c75a379a181750eab788cf2eb668109caa8b88e6`; unchanged `_process_line` with cleaning off, typography/IVS on, both inventory and Word-exclusion modes; Python 3.12.13 plus hash-pinned emoji 2.15.0 |
| Hiberius/hiberius-unicode-toolkit | `1d86b8d951fca3cd7efbe637267f57e391dd7570`; unchanged Scan event handler in Node 26.8.2 with inert DOM facade, not a browser test |

[Corpus](unicode/corpus.json) records Apache-2.0 ownership, exact raw hex, encoding,
source and decoded-text SHA-256, literal decoding transformation, independently
selected code points, scalar/original-byte/UTF-16 coordinates and interpretation.
The [generator](../../../experiments/unicode-differential/corpus.py) is independent
of detector output. Visible typography and emoji are deliberate comparison targets,
not a requirement that every hidden-character scanner flag visible characters.

Root licenses match G0's retained MIT files. Emoji's wheel license is BSD-3-Clause;
all source/data/license identities are in the [input inventory](../../../experiments/unicode-differential/input-trees.json).
No upstream source, wheel, binary or upstream-authored fixture is redistributed.
Independent fixtures and probe code are Aletharsis-authored. No StegZero or untrace
source is used, and no unverified successor entry is added to the registry.

## Findings and adjudication

| Observation | Interpretation / required follow-up |
| --- | --- |
| Four Hangul fillers (U+115F, U+1160, U+3164, U+FFA0) appear in HIBERIUS but not native Aletharsis or Juriku | **Native inventory gap.** These are letter-category characters, outside the current control/mark selection. #14 should add a reviewed invisible-letter inventory/contract extension with legitimate-language context, preserving conservative claims; do not silently recategorize them as control characters |
| Persian ZWNJ and Arabic ZWJ are reported by all configured probes | Presence is real, but legitimate joining context is not encoded as expectedness here. HIBERIUS's pinned Scan implementation does not perform the proposed general language/emoji contextual exclusion; no inference from its dirty verdict to malicious intent |
| Juriku omits FE0F in the recognized heart emoji, but reports orphan FE0F | Explicit emoji-library policy. Aletharsis retains selector and visible emoji observations; #14 expectedness must be separate from observation and pattern analysis |
| Juriku Word mode suppresses several quotes/dashes and NBSP | Explicit policy, not source absence. Narrow NBSP remains observed in this sample. Neither a global whitelist nor matching this tool's filtered output is an acceptable truth oracle |
| Juriku omits the leading BOM but retains the embedded BOM | Explicit read-only BOM policy. Aletharsis preserves both; profile/reveal must distinguish them without losing bytes |
| HIBERIUS omits basic/supplementary variation selectors, Unicode tags, ordinary combining acute and Mongolian selectors | Pinned inventory coverage, not proof of clean text. Its short/long tag and selector cases can show a clean verdict while Aletharsis retains evidence |
| Juriku inventory mode omits U+2026 despite typography being enabled, along with tags, Arabic Letter Mark, Khmer invisible marks and the tested controls/fillers | Pinned inventory gaps in this scoped detector configuration; no claim about other tools/successor versions |
| Aletharsis rejects the NUL/ESC/DEL/NEL sample during binary identification | Parser/identification coverage difference. Its report is failed with `parser.failure`, not no findings; upstream probes receive explicitly decoded text and do not test their file parsers |
| Native binary and selector runs produce pattern findings; a 16-character tag run exposes `abcdefghijklmno` | Deterministic structural evidence only. A four-character tag example remains inventory-only under the native 16-character run threshold. No vendor watermark assertion or channel-generation capability |
| Original UTF-16/32 bytes, scalar indices and UTF-16 viewer units differ after non-BMP and combining characters | Native original-byte coordinates agree with independent expectations. Upstream probes operate on literal decoded text; HIBERIUS positions are reconstructed from scan nodes. #15 must retain these representation distinctions |

[Comparison](unicode/comparison.json) retains every per-occurrence difference and
adjudication. Its 167 rows include repeated policy-mode observations; they are not
167 independently confirmed defects. No offset mismatch or unexpected extra
observation appeared in this corpus. That does not establish universal correctness.

## Evidence, limitations and resources

[Manifest](unicode/manifest.json) binds retained raw responses, source fixtures,
corpus, comparison, environments and run logs. [Tests](../../../tests/test_unicode_differential.py)
verify the independent generator, original and viewer coordinates, raw identities,
repeat-run equality and specific differences. The initial implementation passed
298 offline tests (33 added for the study); that is a historical count before
the main merge and review fixes. No live upstream execution runs in normal CI.

All original synthetic bytes remained unchanged. Upstream probes have read-only
source/input mounts, no home/preferences and no network namespace access. No
cleaning/injection/clipboard/download action was invoked. Network attempts were
not separately traced: isolation is not proof that no syscall was attempted.
These probes are not complete browser, file-parser, directory traversal, renderer,
security sandbox or native-platform validation. Pathspec is absent and its expected
warning retained. Pinned Node has system-library dependencies. Production CLI
runtime independence is unchanged.

Each case: 25 seconds execution plus 5 seconds cleanup (30 total), 25 CPU
seconds, 8 MiB input/combined output;
4 MiB stdout and 4 MiB stderr caps. The enclosing job has 512 MiB memory, no swap,
one-core quota, 64-task ceiling and 900-second deadline. Pre-review runs consumed
2.635 and 2.666 cgroup CPU seconds, 3.454 and 3.498 wall seconds, and 67.7/67.1 MiB
peak memory. Native build consumed 10.292 CPU seconds, 10.768 wall seconds and
306.7 MiB peak memory. Two earlier 29-case exploratory/reproduction runs consumed
2.567 and 2.570 CPU seconds; the long tag boundary was subsequently added without
changing the existing expectations. Two initial 30-case runs used another 5.280
CPU seconds; final runs tightened the execution/cleanup split within the 30-second
case ceiling, with all raw responses unchanged. An initial build invocation failed before
compilation because its service working directory was not set; the corrected
command and successful build log are retained. No upstream code ran in that failure.

Before review remediation, total cgroup-measured build/probe compute was about 26.0 seconds, far below #39's
2 CPU-hour ceiling. Python/GNU-time child accounting undercounts some namespaced
children; prefer retained cgroup totals. At 04:48:26 UTC on 2026-09-21, the study
artifact interval since first archive creation at 04:32:57 UTC was 15m29s; subsequent
document/PR preparation is additional, still within the 16 engineering-hour budget.
The study directory occupied about 22 MiB. Even including the entire existing
shared Go toolchain, module/build caches and worktree, measured space was under
900 MiB, below 2 GiB. Disk size is checked/observed, not filesystem-quota enforced.
No GPU/API spend, customer/private samples or remote detector submission occurred.

## Disposition and next gates

1. Use this corpus to specify #14's context/expectedness handling and a narrowly
   reviewed native Hangul-filler inventory extension. Preserve raw observations
   and pattern-level overrides; do not copy upstream suppression policy wholesale.
2. Use the encoded/non-BMP/combining/BOM cases for #15 reveal maps and source hashes.
   Preserve literal marker-looking text and distinguish source from displayed units.
3. Any broader comparator use, upstream update or redistribution needs a new pin,
   explicit scope and G6 review. Full browser/file-parser and other-platform claims
   require their own bounded execution evidence.

The two registry candidates move from proposed to evaluating, with empty production
scope and no adoption decision. The study resolves a bounded comparison increment;
#39 remains open until owner review accepts its disposition. #14/#15/#21 and other
integration gates are not completed by it.

Reproduce using the [explicit provisioning and offline commands](../../../experiments/unicode-differential/README.md).

## PR #54 review remediation

The retained results/environment/run logs were regenerated twice after correcting
the probe and adjudication; the prior evidence remains in commit `a5eb8cd`. The
new runs use the same pinned trees, corpus, runtime and native executable, with
no upstream downloads. All 90 stdout and 90 stderr artifacts match between runs.
Native and Juriku outputs and every source byte also match the prior evidence.
HIBERIUS now derives status from emitted verdict/visualization and reports unknown
secret visibility until the handler assigns it; the facade does not parse HTML
attributes. Script initialization and Scan execution each have a 1-second VM
deadline within the existing outer process/cgroup limits.

Native execution no longer mounts a comparator source tree. Per-stream rlimits
enforce the combined output ceiling; a defensive overrun check and a second
cleanup timeout produce retainable failure records before the runner stops. An
unreaped process is never reported as successfully cleaned up. These changes
do not claim the VM or facade is a production security boundary.

First-review resource measurements are retained in commit `ab163c3`:
2.679/2.692 cgroup CPU seconds, 3.494/3.491 seconds service
runtime and 76/66.4 MiB peak memory. Both remain within the original study budget.
Validation passed 50 focused tests and 385 full offline tests, including synthetic
handler timeout/overrun cases and retention of failed cleanup/output outcomes.

## Runtime identity and mount follow-up

Second-review runs replace the current environment/results/resource logs; previous
observations remain in `ab163c3`. The harness now queries the supplied Juriku
interpreter, enforces Python 3.12.13 and records its executable hash separately
from the launcher version. Node version and hash both refer to `/usr/bin/node`.
These are executable identities, not a hash inventory of the Python standard
library or the host system libraries. Comparator/data trees retain their separate
file-by-file pin checks. Native execution receives none of `/upstream`, `/probe`,
`/python` or `/emoji`; HIBERIUS receives neither `/python` nor `/emoji`.

Both 30-case runs succeeded. Every original source and raw stdout/stderr artifact
is byte-identical to the prior evidence and between runs. Current logs record
3.516/3.513 seconds service runtime, 2.732/2.705 cgroup CPU seconds and
108.1/62.8 MiB peak memory, within the original study budget. Validation passed
52 focused tests and 387 full offline tests. The full branch comparison
`git diff --check origin/main` passes with evidence-specific whitespace attributes;
intentional CRLF bytes were preserved. No new upstream provisioning occurred.

## Third review remediation

The final revised probes were executed twice on 2026-09-21 using the same pinned
inputs, native binary and bounded offline cgroup configuration. All 30 sources,
90 stdout files and 90 stderr files match between runs and the preceding retained
study. Comparison results remain unchanged. Metadata now records explicit reaping
and output finality, and hashes only the executable/configuration probe list.
Documentation is excluded from that list. Retained runtime evidence identifies
Node 26.8.2; the registry and reproduction prerequisites now agree.

Cgroup measurements: run 1 used 3.456 seconds wall, 2.639 seconds CPU and 63 MiB
peak memory; run 2 used 3.478 seconds wall, 2.619 seconds CPU and 63.1 MiB peak.
All 393 offline tests passed, including 58 study/harness checks. Synthetic tests
now exercise incomplete HIBERIUS coverage, unexplained native omissions, shifted
coordinates, DOM output/event drift and unreaped non-final output. Unknown misses
are not automatically assigned an inventory exemption. The existing inventory
exceptions describe the reviewed pinned tool surfaces, not future detector versions.

## Fourth review remediation

Repeated both bounded offline runs on 2026-09-21 after narrowing native filler and
typography exemptions, mounting only hashed probe files, isolating runtime version
queries, requiring exact comparator-tree coverage, rejecting non-boolean hidden
assignments and removing an unused probe import. All 30 sources, 90 stdout files
and 90 stderr files match between runs and the preceding retained evidence;
comparison rows are unchanged. Execution metadata and hashes were freshly captured.

Cgroup measurements: run 1 used 3.529 seconds wall, 2.665 seconds CPU and 108.8 MiB
peak memory; run 2 used 3.550 seconds wall, 2.737 seconds CPU and 63.5 MiB peak.
The full offline suite passed 402 tests, including 67 study/harness checks.
No production detector or component-adoption status changed.

## Fifth review remediation

Re-ran the bounded offline study twice on 2026-09-21 after restricting comparator
inventory/visible-emoji exceptions to reviewed case/code-point pairs, rejecting
blank written verdicts and missing chip categories, and checking every stable
execution-identity field across both runs. New contexts now require adjudication;
no verdict write remains distinct from a written empty string. Both environment
files' complete probe maps are checked against the retained probe source.

Re-provisioned only the three already-approved, hash-pinned archives and verified
the existing tree inventories and root licenses. Rebuilt the same native commit
`abc513e354e864a38d3d57649667c22ba6080cc7` offline with Go 1.27.1, `-p=1 -trimpath`,
and the documented 512 MiB cgroup. The new executable hash is
`f36771b5de8651aa2aab2e0fe3eeb82421dd3e0e1b7677a7ac63ad5b51f8b642`;
it is not byte-identical to the historical build. Current environments retain this
new identity. `review-rebuild.stderr` records 10.395 seconds service runtime,
9.956 seconds cgroup CPU and 288.9 MiB peak memory. An initial build launcher
omitted the working directory and failed before compilation; its diagnostic is
retained in `review-build-attempt.stderr`, not presented as a successful build.

All 30 source fixtures, 90 stdout files and 90 stderr files are byte-identical
between both runs and the preceding evidence; the 167 comparison rows are
unchanged. The current manifest identifies 231 artifacts, including fresh run
metadata and seven additional build/run logs. Run 1 used 3.431 seconds service
runtime, 2.622 seconds cgroup CPU and 98.4 MiB peak memory; run 2 used 3.346 seconds,
2.586 seconds CPU and 63.7 MiB peak memory. These are execution observations, not
estimates of total engineering effort. Historical measurements above retain their
original scope.

Validation: 421 full offline tests passed, including 86 study/harness checks,
using Python 3.12.13 and pinned CI requirements (including the date-time format
validator). Full-suite measurements: 4.27 seconds wall, 4.13 seconds user CPU,
0.12 seconds system CPU and 94,072 KiB maximum RSS. `git diff --check` passed.
No production Go detector or component-adoption status changed.

## Sixth review remediation

HIBERIUS probe JSON now escapes non-ASCII, including recovered carrier content.
Both fresh bounded offline runs agree on all 30 source files, 90 stdout files and
90 stderr files. Compared with the preceding study, 29 HIBERIUS stdout files
change only in JSON serialization; their decoded objects are identical. All other
raw artifacts and all 167 comparison rows remain unchanged. No detector conclusion
or adoption status changed.

Possible offset mismatches now supplement rather than overwrite an omission's
reviewed classification. Unexplained extra observations remain unadjudicated.
Juriku derives and explicitly enforces input integrity, including under optimized
Python. Synthetic regressions exercise bidi/astral output escaping, mutation under
both Python modes and a reviewed inventory gap alongside a stray observation.

Provisioning still defaults to historical archive hashes and now also verifies
complete extracted file inventories. An explicit `--allow-repacked-source` option
accepts changed source-tar compression only if every extracted file matches the
pinned inventory; the wheel remains byte-pinned. Synthetic tests reject changed
contents and absent opt-in. The current study used the original hash-matching
archives; its new provisioning receipt preserves that fact. Historical registry
archive identities are unchanged.

The manifest now covers 236 artifacts, including provisioning and fresh resource
logs. Run 1 used 3.415 seconds service runtime, 2.598 seconds cgroup CPU and 63.1 MiB
peak memory; run 2 used 3.452 seconds, 2.599 seconds CPU and 63.4 MiB peak memory.
Both used the same previously rebuilt native executable and trusted runtimes.
Validation: all 432 offline tests passed, including 97 study/harness checks;
`git diff --check` passed. No production Go code changed.

## Seventh review remediation

Made Juriku library availability and runner payload/source bounds explicit runtime
checks, preserved observed pathspec availability, and made HIBERIUS fail on input
mutation or duplicate element-event registration. Added optimized-Python library
precondition tests, synthetic mutation/listener cases, and exact coverage-set
checks with mutations demonstrating rejection of new coverage loss.

Two fresh bounded offline runs reproduce all 30 sources, 90 stdout files and
90 stderr files byte-for-byte against each other and the sixth-pass evidence.
All 167 comparison rows are unchanged. Run 1 used 3.374 seconds service runtime,
2.565 seconds cgroup CPU and 63.8 MiB peak memory; run 2 used 3.429 seconds,
2.613 seconds CPU and 64.1 MiB peak memory. The same pinned inputs, native binary
and runtimes were reused, with fresh probe identities. The manifest now covers
240 artifacts, including four new run logs. All 442 offline tests passed,
including 107 study/harness checks; diff checks passed. No production logic,
adoption status or detector conclusions changed.

## Eighth review remediation

Pinned this narrative's checkout bytes with `-text`, added explicit native
occurrence-location diagnostics shared by adjudication and coordinate checks,
and reset HIBERIUS initial visualization/verdict state. Unsupported innerHTML
writes now fail explicitly. Current Unicode code-point finding variants require
offsetLocation; null normalization locations remain valid and are not occurrences.
The new error path prevents future malformed input from being silently skipped.

Both fresh bounded offline runs reproduced all 30 sources, 90 stdout files and
90 stderr files byte-for-byte against each other and the prior evidence. All 167
comparison rows remain unchanged. Run 1 used 3.474 seconds service runtime,
2.643 seconds cgroup CPU and 63.8 MiB peak memory; run 2 used 3.504 seconds,
2.642 seconds CPU and 63.2 MiB peak memory. The manifest covers 244 artifacts.
All 612 offline tests passed, including 116 study/harness checks; Git attribute
and diff checks passed. No production detector or adoption status changed.
