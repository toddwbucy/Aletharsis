# G3 Office comparator: static clearance assessment

Status: **pending clearance**, no comparator executed. Tracking #40 / #21.
This assessment does not authorize full oletools installation or adoption.

## Exact acquisition and scope

The pinned upstream revision is `ec10260989dbc48b9109e3d05d22beee19cf2333`.
The archive came from
`https://codeload.github.com/decalage2/oletools/tar.gz/ec10260989dbc48b9109e3d05d22beee19cf2333`.
Its SHA-256 is `8a5e4874f6393ed540c33a1a03367123a5054fd334de3cfca79cab1af1e3e045`
(3,144,228 bytes). The pinned source reports 0.60.3; do not identify it as the
repository's older 0.60.2 release merely because that release appeared in G0.

The [static inventory](office/static-inventory.json) records candidate module,
license and archive identities. Archive reads and Python AST parsing examined
source as data; no upstream imports, installation or invocation occurred.

Root package licensing is BSD-2-Clause with an MIT officeparser notice. The root
explicitly excludes the thirdparty directory. A full installation also declares
`pyparsing`, `olefile`, `easygui`, `colorclass`, conditional `msoffcrypto-tool`, and
`pcodedmp`. The latter's package metadata declares GPL; the vendored `xxxswf`
license contains GPL-3.0 terms. This assessment does not clear that full closure.
Out-of-process execution does not by itself settle redistribution rights.

## Candidate restricted runtime

Use only `ooxml` and read-only `oleobj` helpers, with their statically observed
module closure, plus olefile 0.47. Retain xglob's BSD-2-Clause license separately.
Olefile includes BSD-2-Clause and inherited PIL license text; both its historical
package notice and distribution notice are retained verbatim for review. No
SPDX label here substitutes for examination of those notices.

The olefile wheel is pinned by SHA-256
`543c7da2a7adadf21214938bb79c83ea12b473a4b6ee4ad4bf854e7715e13d1f`.
Source: `https://files.pythonhosted.org/packages/17/d3/b64c356a907242d719fc668b71befd73324e47ab46c8ebbbede252c154b2/olefile-0.47-py2.py3-none-any.whl`.
The selected olefile Python 3 modules import only the standard library. The overall
oletools candidate closure additionally imports pinned olefile, vendored xglob and
optional lxml. Python 2 compatibility modules are not candidates for the proposed
Python 3.12 runtime.
Optional `lxml` must be absent so the upstream standard-library XML fallback is
selected. The recorded AST imports include conditional imports; they are not a
proof that reflection or runtime loading cannot add dependencies.

Before execution, provisioning must enforce a file allowlist with exact hashes,
retain notices, pin the interpreter/stdlib/runtime closure, disable ambient site
packages and bytecode writes, and verify runtime imports under isolation. No
`pip install oletools`, installation hooks, optional extras or audit-time downloads.
This repository retains notices and inventory, not upstream executable source.

## Comparison fidelity and side effects

| #40 target | Proposed independent surface | Limitation to retain |
| --- | --- | --- |
| Package parts | Upstream OOXML ZIP traversal | A shared standard-library ZIP implementation is not a second ZIP implementation; disclose this dependence |
| Core/app metadata | Upstream XML traversal plus an independently authored projection of namespace-qualified properties | `olemeta` is OLE metadata tooling, not a DOCX property oracle; the projection itself needs fixtures and review |
| Relationships | Upstream XML traversal of relationship declarations | `find_external_relationships` selects external types; it alone is not a complete relationship inventory |
| Embedded objects | `oleobj.find_ole` and package target inventory | OLE recognizability is narrower than all embedded packages; record non-OLE and unrecognized targets as unsupported checks |

Do not shrink the comparison to what `find_ole` recognizes. Independent fixture
expectations must cover all four targets and expose missing comparator coverage.
Never derive expected values from Aletharsis or upstream output.

Do not call `oleobj.process_file` or its CLI: its documented behavior writes
extracted objects, including beside the input by default. No decryption/password
helpers, real credentials, customUI execution or external-target fetching.
Only synthetic fixtures; read-only input mounts; cleared environment; no network;
bwrap/prlimit supervision modeled on the Unicode study. Read-only helper names
are not security boundaries: enforce process CPU/time/memory/output restrictions
and retain stderr/cleanup failures. Review returned generators/streams for closure.

## Remaining B0 gates

1. Review exact selected module headers and retained license notices, including
   the interpreter/stdlib/platform dependencies. Any unapproved component stops use.
2. Implement and test provisioning, module allowlisting and import isolation.
3. Validate the proposed comparison projection against independent expected cases,
   including malformed packages and incomplete upstream enumeration.
4. Obtain the appropriate clearance disposition before comparison execution.

Registry archive identity and root-license identification are now factual rather
than unknown. `clearance: pending`, `runtime.assessment: unverified`, proposed
adoption and empty production scope remain accurate. No #40/#21 gate is closed.

## Resources and evidence limits

Preflight used static archive inspection only. No comparator CPU or comparison
artifacts exist. Earlier preflight CPU was not instrumented and is not claimed as
zero; subsequent execution needs measured resource receipts before starting.
The study's original 12-hour engineering, 2-hour CPU, 2-GiB disk, 1-GiB/process,
60-second/case and 32-MiB input/output ceilings remain in force. Do not extend
those budgets silently to accommodate tooling or failed attempts.

## Retained-evidence validation

Each license entry in the static inventory has a repository-relative `retained_path`.
`tests/test_office_clearance.py` verifies those exact bytes and sizes offline,
derives the retained-notice set from the archive notice-member inventory, and checks registry/archive/root-license
linkage. Git disables text conversion for the narrative, inventory and notices.
These checks detect retained-notice drift; they do not recompute upstream module
or archive hashes without their separately provisioned source artifacts. Static
import statements are AST renderings, not verbatim source lines or runtime traces.

## Rights-record follow-up (review pass 2)

The inventory now enumerates every regular `LICENSE*`/`COPYING*` basename
(case-insensitive) in both pinned archives, including documentation copies and
unrelated vendored licenses. Each has an explicit retention decision/reason.
The package-local `oletools/LICENSE.txt` is retained as
[oletools-package-license.txt](office/oletools-package-license.txt), in addition
to the root notice. Documentation copies under `oletools/doc/` and `olefile/doc/`
are inventoried but not provisioned; unrelated thirdparty notices are outside the
candidate closure. Completeness is a pinned-archive inspection claim; offline CI
checks its internal linkage and retained bytes, not an independently downloaded
archive. Both package and root oletools notices include BSD-2-Clause and MIT
material, so neither is shortened to a blanket BSD-only rights record.

Each candidate module records copyright-header observations from the first 100
physical source lines. Header absence does not establish absence of copyright;
codeauthor/changelog credits are not asserted as rights-holder notices. Root
package coverage outside thirdparty is a proposed basis for owner review, not
an invented per-file attribution. Exceptions requiring that explicit decision:

| Module | Observed exception / proposed basis | Other credits (observations only) |
| --- | --- | --- |
| `oletools/__init__.py` | No header attribution; package grant proposed | None in recorded header scope |
| `oletools/ppt_record_parser.py` | BSD disclaimer but no copyright-holder line; package grant proposed | None in recorded header scope |
| `oletools/common/__init__.py` | No header attribution; package grant proposed | None in recorded header scope |
| `oletools/common/io_encoding.py` | Header names msodde (2017–2018 Lagadec), not io_encoding; package grant proposed | None in recorded header scope |
| `oletools/common/log_helper/__init__.py` | No header attribution; package grant proposed | Package codeauthor: Intra2net AG and Philippe Lagadec in log_helper.py:26; not per-file attribution |
| `oletools/common/log_helper/_json_formatter.py` | No header attribution; package grant proposed | Package codeauthor: Intra2net AG and Philippe Lagadec in log_helper.py:26; not per-file attribution |
| `oletools/common/log_helper/_logger_adapter.py` | No header attribution; package grant proposed | Package codeauthor: Intra2net AG and Philippe Lagadec in log_helper.py:26; not per-file attribution |
| `oletools/common/log_helper/_root_logger_wrapper.py` | No header attribution; package grant proposed | Package codeauthor: Intra2net AG and Philippe Lagadec in log_helper.py:26; not per-file attribution |
| `oletools/thirdparty/__init__.py` | Empty initializer, no header; root excludes thirdparty, so no governing notice asserted | None in recorded header scope |
| `oletools/thirdparty/xglob/__init__.py` | No header attribution; separate xglob subpackage notice proposed | None in recorded header scope |

Per-notice records include holders observed in the notice and an SPDX field.
The olefile copies remain `NOASSERTION` for the combined SPDX expression, with
BSD-2-Clause plus the inherited PIL permission notice stated explicitly. The owner
must settle that expression/clearance; this revision does not assume that HPND's
exact template matches both historical copies. Olefile's wheel and both notice
digests are linked from the registry and checked against this inventory.

Provisioning must also exclude write behavior present in the closure:
`OleFileIO.open(..., write_mode=False)` admits a write-mode argument;
`OleFileIO.write_sect` and `OleFileIO.write_stream` write data;
`record_base.OleRecordFile.open(..., **kwargs)` forwards those arguments.
A helper-name allowlist alone is insufficient: read-only input mounts and tests
that reject write-mode calls remain required, alongside the previously excluded
`oleobj.process_file` and CLI extraction paths. No such runtime is cleared here.

## Rights and provisioning evidence (review pass 3)

`ooxml.py:10` credits **Intra2net AG**, and
`common/log_helper/log_helper.py:26` credits **Intra2net AG and Philippe Lagadec**.
Both verbatim codeauthor observations are retained in per-module `other_credits`;
empty lists mean no matching credit in the documented header scope, not no author.
The headerless log_helper siblings do not acquire an inferred per-file holder.
Whether package grants cover these contributions is an explicit owner B0 question.

Proposed provisioning carries **both** root `LICENSE.md` and package
`oletools/LICENSE.txt`, plus xglob and both olefile notices. `provisioning_notices`
lists all five. Non-thirdparty oletools files now propose the package-local notice
as their governing basis; the root notice is additional retained evidence. The two
notices have different dates/bytes; no byte equivalence or clearance is asserted.
The ppt_record_parser BSD disclaimer is recorded verbatim separately from its
absent copyright-holder observation.

`write_entry_points` records archive/member, qualified symbol and definition line,
including `OleFileIO.__init__(write_mode=)` and `io_encoding.uopen(mode=)` as well
as open/write helpers and process_file. Constructor calls can enable writes;
uopen forwards arbitrary modes to builtin open. Gate-2 tests must cover these
paths, alongside enforced read-only mounts; this is not a complete sandbox policy.

Declared setup/wheel license metadata says BSD and is retained as an observation.
It does not override the exact notice text (including MIT and inherited PIL).
Structured registry `runtime.artifacts` binds each archive and retained notice
by its labeled identity, digest, size and retained path. Offline tests verify
retained holder strings and proposed governing-notice scope, but unretained notice
hashes remain static archive observations rather than independently checked CI data.

Review pass 4 records explicit Author labels and __author__ assignments as well
as codeauthor credits. Observation scopes are inventory-level defaults. The
hand-curated write-entry list now includes both private mini-stream writers and
oleobj.main; AST parsing locates definitions, it does not discover a complete
write call graph. Changelog credit to Christian Herdtweck in xglob.py:56 remains
outside the declared header-credit scope and is not inferred to be a rights grant.

The retained oletools notice difference is recorded machine-readably and verified
against both retained files: heading, preamble wrapping and year-range differences are preserved,
not erased by an assertion of legal equivalence. The registry root notice is
identity evidence; the package-local notice remains the proposed governing basis.
Registry license.retained_path is relative to docs/reuse; runtime.artifacts paths
are repository-relative, as are static inventory paths.
