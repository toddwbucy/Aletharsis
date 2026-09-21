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
rejects oversized or non-regular archive content. Supply a new output directory:

```bash
python3.12 experiments/unicode-differential/provision.py \
  --downloads /absolute/downloads --output /absolute/fresh-inputs
```

Runtime prerequisites: existing Python 3.12.13, Node 26.8.1, Linux bubblewrap,
util-linux prlimit, GNU time and a working systemd user manager. Native binary was
built with Go 1.27.1 from commit `abc513e354e864a38d3d57649667c22ba6080cc7` using
`go build -p=1 -trimpath -o /absolute/aletharsis ./cmd/aletharsis`, in a 512 MiB
cgroup with offline pre-provisioned module/toolchain caches. Pin the host/runtime
for identical reproduction; Node is distro-linked, not a hermetic runtime bundle.
Exact binary/environment identities and initial build resource log are retained.

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
and 4 MiB per output stream (8 MiB combined).
The enclosing cgroup bounds aggregate memory to 512 MiB, has no swap and a
900-second deadline. The runner retains partial results before a failing case
stops execution. Storage is measured after execution, not filesystem-quota enforced;
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
An empty input returns before scanning and is recorded accordingly. The scanner
may call its known-carrier decoder; recovered content remains inert output.

Aletharsis: the compiled native CLI audits the original bytes at a fixed sandbox
path. A binary identification failure is retained as failed, not no findings.
External detectors receive strict decoded text with BOM and line endings retained;
that explicit transformation must not be mistaken for their file-parser coverage.

`adjudicate.py` records every difference against independently authored point/offset
expectations. Visible punctuation and emoji policies differ legitimately. No
majority-vote truth, generic AI attribution, or “watermark found” claim is inferred.
Run twice with fresh output directories and compare every raw stdout/stderr byte;
wall-time/environment paths may differ. CI verifies retained raw identities,
independent coordinate expectations, interpretation and comparison reproducibility
without downloading or executing these upstream tools.
