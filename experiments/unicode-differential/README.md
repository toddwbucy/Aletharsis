# Unicode differential study reproduction

Read [the bounded plan](PLAN.md) before provisioning. Development only: no Python,
Node or external detector is added to Aletharsis's production dependencies. The
study's [retained findings](../../docs/reuse/evaluations/unicode.md) distinguish
inventory, policy, parser and coverage differences from defects.

## Pinned inputs

Obtain these exact archives explicitly, outside audit execution. Use curl's
`--fail --location --max-time 120 --max-filesize 33554432` with a named output file.
No script below downloads anything.

| Filename | URL |
| --- | --- |
| `juriku.tar.gz` | `https://codeload.github.com/juriku/hidden-characters-detector/tar.gz/c75a379a181750eab788cf2eb668109caa8b88e6` |
| `hiberius.tar.gz` | `https://codeload.github.com/Hiberius/hiberius-unicode-toolkit/tar.gz/1d86b8d951fca3cd7efbe637267f57e391dd7570` |
| `emoji-2.15.0-py3-none-any.whl` | `https://files.pythonhosted.org/packages/e1/5e/4b5aaaabddfacfe36ba7768817bd1f71a7a810a43705e531f3ae4c690767/emoji-2.15.0-py3-none-any.whl` |

`provision.py` contains the exact SHA-256 values, validates before extraction, and
rejects oversized or non-regular archive content, then verifies every extracted
file against `input-trees.json`. Supply a new output directory:

```bash
python3.12 experiments/unicode-differential/provision.py \
  --downloads /absolute/downloads --output /absolute/fresh-inputs
```

Runtime prerequisites: existing Python 3.12.13, Node 26.8.2, Linux bubblewrap,
util-linux prlimit, GNU time and a working systemd user manager. Native binary was
built with Go 1.27.1 from commit `abc513e354e864a38d3d57649667c22ba6080cc7` using
`go build -p=1 -trimpath -o /absolute/aletharsis ./cmd/aletharsis`, in a 512 MiB
cgroup with offline pre-provisioned module/toolchain caches. Pin the host/runtime
for identical reproduction; Node is distro-linked, not a hermetic runtime bundle.
Exact binary/environment identities and initial build resource log are retained.
The harness queries `/usr/bin/node` for both its version and executable hash. It
queries the supplied `--python/bin/python3.12`, requires version 3.12.13 and records
its executable SHA-256 separately from the launcher Python version. This identifies
the interpreter binary, not every standard-library/system-library byte; the supplied
Python tree remains a deliberately provisioned trusted runtime, not an archive
covered by `input-trees.json`.

Root comparator MIT license digests match the existing registry. Emoji's wheel
contains a BSD-3-Clause license and no non-dev runtime dependencies; its license
and every source/data file are identified by `input-trees.json`. No upstream code,
wheel, binary or upstream-authored fixture is redistributed. Independent fixture
construction and manifests are Apache-2.0 under Aletharsis's root license.

## Offline run

Use absolute paths. Replace the illustrative input/runtime/output paths; the
Python directory must contain `bin/python3.12` and its matching standard library.
The supplied comparator trees must match `input-trees.json` exactly before use.

```bash
systemd-run --user --wait --pipe --collect \
  -p MemoryMax=512M -p MemorySwapMax=0 -p CPUQuota=100% \
  -p TasksMax=64 -p RuntimeMaxSec=900 \
  /usr/bin/time -v /absolute/python/bin/python3.12 \
  /absolute/Aletharsis/experiments/unicode-differential/run_cases.py \
  --juriku /absolute/fresh-inputs/juriku/hidden-characters-detector-c75a379a181750eab788cf2eb668109caa8b88e6 \
  --hiberius /absolute/fresh-inputs/hiberius/hiberius-unicode-toolkit-1d86b8d951fca3cd7efbe637267f57e391dd7570 \
  --emoji /absolute/fresh-inputs/emoji \
  --python /absolute/python --binary /absolute/aletharsis \
  --output /absolute/new-results
python3.12 experiments/unicode-differential/adjudicate.py /absolute/new-results
```

Each child uses a new offline PID/network/filesystem namespace, read-only source,
input, runtime and probe mounts, no home/preferences, a private temporary area,
25 CPU seconds, 25 seconds execution plus 5 seconds cleanup (30 seconds total),
and 4 MiB per output stream (8 MiB combined), enforced by per-stream file-size
rlimits. A defensive combined-size check records an overrun if these limits change;
exactly 8 MiB is within budget.
The enclosing cgroup bounds aggregate memory to 512 MiB, has no swap and a
900-second deadline. The runner retains partial results before a failing case
stops execution, including a second cleanup timeout or output overrun. Cleanup
timeout is recorded separately with `reaped: false`, `output_final: false`, null
return code, null final output digests and null output-limit determination. Output
files may still change; the partial run must not be promoted as retained final evidence.
A fresh run is required after cgroup termination; the enclosing
cgroup remains responsible for termination. Storage is measured after execution, not filesystem-quota enforced;
the synthetic corpus is far below the 2 GiB study ceiling. This is a Linux study
harness, not proof of a portable production sandbox.

The cgroup's CPU accounting includes namespaced descendants; Python's child rusage
and GNU time undercount some namespace descendants, so do not use their lower CPU
values as complete study costs. Record both logs and prefer the cgroup aggregate.

## Comparison boundaries

Juriku: the unchanged `_process_line` read-only detector is invoked on supplied
literal decoded text, with typography and ideographic selectors enabled, in both
inventory and explicit Word-exclusion modes. Emoji 2.15.0 is supplied. Pathspec is
absent and its warning retained; directory/ignore-file traversal is not tested.
No source cleaning, normalization or upstream file decoding is invoked.

HIBERIUS: the unchanged executable script from pinned `index.html` runs with an
inert DOM facade. Only the registered Scan click handler is invoked. The probe
reads chips/text nodes to reconstruct scalar positions, checking each chip against
the input. These are **probe-derived** positions, not an upstream location API.
This is not a browser rendering, clipboard, download or HTML-security test.
Initialization and the Scan invocation each have a 1-second VM deadline inside
the outer process limits. The VM is not a security sandbox. Status is derived from
whether the handler wrote a verdict, including `no_verdict_emitted` when it did
not. A written blank verdict fails explicitly rather than becoming missing coverage;
empty input is not assumed unscanned just because it is empty. Each observation
must expose a nonblank string category; an unsupported class assignment fails
rather than silently dropping `native_category`. The facade does not parse HTML
attributes: secret visibility is unknown until the handler assigns `hidden`. The scanner
may call its known-carrier decoder; recovered content remains inert output.

Aletharsis: the compiled native CLI audits the original bytes at a fixed sandbox
path, without mounting comparator sources, probe scripts, Python or emoji. Only Juriku
receives the supplied Python and emoji mounts; HIBERIUS does not. A binary identification failure is retained as failed, not no findings.
External detectors receive strict decoded text with BOM and line endings retained;
that explicit transformation must not be mistaken for their file-parser coverage.

`adjudicate.py` records every difference against independently authored point/offset
expectations. Visible punctuation and emoji policies differ legitimately. No
majority-vote truth, generic AI attribution, or “watermark found” claim is inferred.
Run twice with fresh output directories and compare every raw stdout/stderr byte;
per-run command paths, CPU measurements and retained-byte counts may differ.
All other recorded environment fields must match, including native/interpreter
binary hashes, versions, platform and the full probe hash map. Both runs’ probe
hashes are checked against the checkout. CI verifies retained raw identities,
independent coordinate expectations, interpretation and comparison reproducibility
without downloading or executing these upstream tools.

Probe identities cover the explicit executable/configuration file list in `PROBE_FILES`;
README and PLAN are documentation, not executed probe inputs. Comparator inventory
and visible-emoji exceptions are keyed by reviewed case and code point; leading-BOM
policy is likewise restricted to the reviewed BOM cases. A new context does not
inherit an exception merely by reusing a code point. Unknown inventory misses
remain unadjudicated; unexpected native omissions and candidate offset mismatches
require investigation. Non-completed HIBERIUS scans cannot establish absence.
The DOM facade rejects unsupported document events and requires visualization/verdict
writes for nonempty input; empty-input early return remains explicitly observable.

Runtime version queries also run through the bounded executor in offline, cleared-
environment namespaces inside the enclosing study cgroup. The supplied Python
runtime is mounted read-only at `/python`; it is not executed directly on the host
by the metadata query. The launcher and deliberately provisioned host libraries
remain trusted prerequisites. Comparator-tree coverage must name exactly Juriku,
HIBERIUS and emoji. `/probe` contains only the individually mounted `PROBE_FILES`,
so an added unlisted helper cannot execute unnoticed from the checkout.

The hidden-state facade accepts direct boolean assignments only. A non-boolean
assignment fails the probe rather than becoming unknown visibility. HTML attributes,
CSS visibility and browser rendering remain outside this facade's coverage.

The current evidence includes a fresh offline Go 1.27.1 build at the recorded
native commit; see `review-rebuild.stderr` and the current environment binary hash.
Its executable bytes differ from the historical build, while every raw audit result
matches. Historical build logs remain historical, not evidence of binary identity
with the new run. The runner’s `retained_bytes` measures case/corpus/results files
before `environment.json` is written; the final manifest separately hashes the
complete retained artifact set, including environment and resource logs.

### Repacked source archives

Archive hashes identify the exact historical downloads, not a promise that GitHub
will always serve identical compressed bytes. If a source tarball changes while
its pinned contents remain identical, explicitly pass `--allow-repacked-source`
to `provision.py`. Size/type/path restrictions still apply, and the entire extracted
file inventory must match `input-trees.json` exactly before success. Missing, added
or changed files fail. The emoji wheel still requires its exact archive hash.
`provisioning.json` records observed and historical archive hashes and successful
tree verification; retain it with a reproduction using this fallback. Do not
rewrite the historical registry or study archive identity to claim byte equality.
The runner independently verifies tree identity again before executing probes.

All probe JSON escapes non-ASCII, including decoded carrier content; parsing it
recovers the original strings. A possible coordinate mismatch is an additional
signal on a difference, never a replacement for its omission classification.
Juriku's input-integrity result is observed and enforced even under optimized Python.

Library preconditions and payload/source size bounds use explicit failures rather
than optimization-sensitive assertions. Juriku reports the observed pathspec
availability. HIBERIUS rejects a handler that changes the supplied input or a
second listener for an already registered element event; silently replacing a
listener is outside this facade's contract. Retained-evidence checks require the
complete set of non-completed native cases to be exactly `controls`, and the
complete set of non-completed HIBERIUS cases to be exactly `empty`. Any new loss
of coverage requires review even if its rows have parser/coverage explanations.
