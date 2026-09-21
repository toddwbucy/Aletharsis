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
| Hiberius/hiberius-unicode-toolkit | `1d86b8d951fca3cd7efbe637267f57e391dd7570`; unchanged Scan event handler in Node 26.8.1 with inert DOM facade, not a browser test |

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

Review-run resource measurements are retained in `unicode/run1.stderr` and
`unicode/run2.stderr`: 2.679/2.692 cgroup CPU seconds, 3.494/3.491 seconds service
runtime and 76/66.4 MiB peak memory. Both remain within the original study budget.
Validation passed 50 focused tests and 385 full offline tests, including synthetic
handler timeout/overrun cases and retention of failed cleanup/output outcomes.
