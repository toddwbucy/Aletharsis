# Single-file reveal CLI

This increment connects #15's package boundaries for single-file exports.
Directory scanning and source-relative publication are specified separately in
[directory reveal integration](directory-reveal.md). Review acceptance remains
a separate delivery gate.

```sh
aletharsis audit notes.txt --reveal-out review-new --json
aletharsis audit notes.txt --reveal-out review-v2 --schema-version 2.0 --json
```

`--reveal-out` accepts a new directory, only for the full `audit` view. It cannot
be combined with `--output`. Its parent must already exist. Parent aliases are
resolved to physical paths and the opened parent identity is checked before writing;
the final destination is never followed. Existing destinations, including source aliases, fail exclusively.
Directory sources, unsupported formats and failed audits produce no derivatives;
the report remains available on stdout with exit 4. Successful calls retain the
ordinary severity exit code, including 1–3 for findings. Failure diagnostics use
fixed messages and stable stage codes, never inspected instructions or filenames.

## Acquisition and evidence

Schema 1 uses `InspectSnapshot`; schema 2 opts into `RetainSnapshot`. Both keep the
one no-atime buffer already acquired for analysis. There is no second source read.
Schema 2's native findings and source buffer are internal owned service fields,
excluded from JSON; ordinary callers do not retain them by default. The report
wire schemas and ordinary CLI defaults remain unchanged.

The full native findings feed `reveal.Compare`, which verifies the source hash
and complete decoded scalar/byte map before rendering. No format parser or
analyzer is duplicated. The source is not reopened after this point. External
changes after acquisition do not change which historical snapshot the bundle
represents. Hash verification on later import is still required.

## Bundle

| File | Meaning |
| --- | --- |
| `report.json` | Exact normal report JSON, also printed with `--json` |
| `revealed.txt` | Unicode reveal preserving visible text and line endings |
| `comparison.json` | Decoded text, verified reveal, faithful/display comparisons and coordinate maps |
| `faithful.diff` | Exact decoded-UTF-8 to revealed-UTF-8 unified diff; active characters retained |
| `display.diff` | Separately labeled ASCII-escaped comparison |
| `manifest.json` | Source/report digest chain and sorted artifact names/sizes/hashes |

No timestamps or random IDs appear in artifacts. Source path is retained in the
ordinary report, so moving the source may change the report and manifest hashes.
The diff is presentation only, never an approved cleanup plan or a patch against
encoded source bytes. Render artifacts as inert text, never executable markup.

The CLI uses a new 0700 directory and exclusive 0600 files under an opened root.
The caller must control the output namespace: this is not a sandbox against
hostile processes running as the same user. Publication rolls back new files on
failure and only removes an empty failed directory. It never recursively deletes
output. Crash durability and atomic directory publication are not promised.
Consumers must validate the manifest and every artifact. Stdout failure after
publication returns 4 and explicitly reports that the bundle remains published.

The source limit remains 8 MiB. Reveal output is capped at 16 MiB, comparisons at
32 MiB per representation pair, total diff text at 32 MiB, combined lines at
200,000 and escape/occurrence counts at 100,000. Complete publication, including
mapping JSON, is capped at 64 MiB. Inputs below 8 MiB can exceed these budgets and
fail without published derivatives. Report-schema limits still apply separately.

## Validation

CLI tests compare stdout byte-for-byte with ordinary audits under both schemas;
verify every manifest digest; repeat bundles; inspect exact clean/zero-width
output; exercise binary patterns, multilingual text and UTF-16/32; and compare
source bytes and native timestamps before/after. Separate tests reject directory,
symlink and hard-link collisions, unsafe ancestors, invalid flags and malformed
input. Acquisition tests prove one reader invocation. Publication and comparison
packages retain their own bounds, rollback and mapping tests.

## Compiled-binary demonstration

The built CLI was run with schema 2.0, JSON stdout and a distinct new reveal
output directory for each of these committed fixtures. All five listed artifact
sizes/hashes and the report hash were verified against the actual files; stdout
matched `report.json` byte-for-byte. Source access/modification/change timestamps
were equal immediately before and after execution, and source bytes/hashes were
unchanged. These are single-file demonstrations, not a directory-scan claim.

| Fixture | Exit | Findings | Reveal occurrences | Source SHA-256 before and after |
| --- | ---: | ---: | ---: | --- |
| `clean_ascii.txt` | 0 | 0 | 0 | `1d0d5e404f224d3fb3adfd664d0e7a17efcfba0f0e9282f7501b2952e26dc9d6` |
| `isolated_zwsp.txt` | 1 | 1 | 1 | `d887c905011994da13e5e0f3a525a66ee5523cd1a0d2b1d13f8e341d6771e11a` |
| `binary_zero_width.txt` | 3 | 3 | 64 | `ba294cd82318821110ff59d8677318c1a84f3d462a1365b369eced80087d127c` |
| `normal_utf8.txt` | 0 | 0 | 0 | `25bf36bfba515f584d31641588eccbe4bf453dc12881eb06421efb8c263814da` |

For the isolated character the revealed artifact is:

```text
An isolated⟦U+200B ZERO WIDTH SPACE⟧ space.
```

The corresponding ASCII display diff is:

```diff
--- a/escaped-decoded.txt
+++ b/escaped-revealed.txt
@@ -1,1 +1,1 @@
-An isolated\u{200B} space.
+An isolated\u{27E6}U+200B ZERO WIDTH SPACE\u{27E7} space.
```

The report still contains the native `unicode.zero_width` finding and original
coordinate evidence; the marker does not replace that report. Clean ASCII and
normal UTF-8 yielded unchanged revealed text and empty faithful diffs. Reproduce
with `go build -trimpath -o bin/aletharsis ./cmd/aletharsis`, then, for example:

```sh
bin/aletharsis audit tests/fixtures/isolated_zwsp.txt \
  --schema-version 2.0 --json --reveal-out review-isolated-new
```
