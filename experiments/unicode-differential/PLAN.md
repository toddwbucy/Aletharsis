# Bounded Unicode comparison plan (#39)

G0 is accepted. This development-only study compares the native Aletharsis CLI
with read-only detection surfaces in at least two independently pinned tools.
No upstream code or vectors will be copied into the production module.

Initial targets, exactly as recorded in the G0 registry:

- juriku/hidden-characters-detector, commit
  `c75a379a181750eab788cf2eb668109caa8b88e6` (MIT).
- Hiberius/hiberius-unicode-toolkit, commit
  `1d86b8d951fca3cd7efbe637267f57e391dd7570` (MIT).

Provision only these source archives from GitHub's pinned codeload endpoints.
Download ceiling: 32 MiB/archive, 120 seconds/request. Record exact archive SHA-256;
check root license bytes against the registry before executing any upstream code.
Inspect entry points and dependency closure first. No package installation is
implicit; stop and revise provisioning if a dependency is missing. Use the existing
Python 3.12.13 environment and Node runtime, with exact identities recorded.

Independent synthetic fixtures are authored under Aletharsis's Apache-2.0 license.
Manifest exact UTF-8 or BOM-marked UTF-16/32 bytes, scalar inventory, original byte
and UTF-16 coordinates, legitimate context and expected mechanism before running
comparators. Include Persian/Arabic joiners, emoji, Word typography, zero-width
binary, selectors/tags, bidi/control, encoding/line-ending and supplementary-plane
boundaries. No private documents, model/API calls, StegZero code or successor tool
is included. No claim of universal semantic detection or majority-vote truth.

Execution is offline in a filesystem/network-isolated process with read-only
upstream/input mounts, no home/preferences, and a bounded writable temporary area.
Call detector functions only, not cleaning/encoding/UI actions. DOM adaptation, if
needed, must be documented as an API probe, not a complete browser execution.
Record observed inventory, native coordinate units, mapping and omissions without
turning expected-language exclusions into a negative finding for hidden channels.

Limits from #39: 16 engineering hours, 2 aggregate CPU hours, 2 GiB study disk,
512 MiB/process, 30 seconds/case, 8 MiB input/output, zero GPU/API spend. Use process
and enclosing-job deadlines, memory/CPU/output enforcement, and stop on licensing,
side-effect, resource or prerequisite failure. Retain partial results with explicit
revise/reject recommendation rather than silently extending limits. Full adoption
and native-platform testing are separate G6 gates.

Adjudicate differences as inventory, parser, offset, policy, coverage or defect,
with independently expected evidence and source inspection where necessary.
Preserve raw responses, result and fixture hashes, commands, environment, resource
observations, source-integrity checks and reproduction instructions. Offline CI
verifies retained evidence; production auditing gains no Python/Node dependency.
Feed concrete profile/reveal requirements into #14/#15; update registry/#21 and
submit the study in its own PR. Owner acceptance is still required to close #39.

## Inspected optional dependency plan

Juriku's emoji-presentation exclusion uses optional `emoji>=2.15.0`. Provision
only `emoji==2.15.0`, wheel `emoji-2.15.0-py3-none-any.whl`, SHA-256
`205296793d66a89d88af4688fa57fd6496732eb48917a87175a023c8138995eb` from the
exact files.pythonhosted.org URL recorded in PyPI's versioned metadata. It has no
runtime dependencies (dev extras are excluded). Inspect retained wheel license
before use, unpack without setup/install execution, and mount read-only for the
probe. The optional `pathspec` dependency remains absent: direct line detection
never traverses directories or reads ignore files. Record its import warning and
absence explicitly; no full CLI traversal claim follows. This amendment precedes
any comparator execution and adds no production dependency.
