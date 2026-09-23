# Report 4.0 attestation mutation gate

This gate checks internal consistency of an imported report. It does not authenticate
a report against unavailable source bytes or replay its declared extractor.

## Corpus and operators

Every JSON fixture in tests/contracts_v4/fixtures participates, including the eight
flat compatibility reports. Invalid baselines fail instead of being skipped. The
current corpus contains 13 fixtures. Generation is exhaustive and deterministic:
sorted object keys, original array order, no randomness, network, or source writes.
A fixture exceeding 12,000 generated candidates fails explicitly; it is not silently
sampled. Identical fixture/operator/pointer/input/mutant combinations are deduplicated.

The JSON/schema walker generates empty strings and arrays, individual array-record
deletions, schema-derived nulls (local references, anyOf/oneOf/allOf and if/then/else; type/enum/const null branches), span collapse,
first/last-byte assessed regions, unsuccessful-state escalation, parent replacement,
and count/summary drift. The Office walker additionally derives a one-byte region
outside retained metadata, relationship and text-origin evidence, where one exists.
XML maps themselves are lexical context, not analytical coverage. A coordinated probe
empties metadata values and origins while narrowing the producing assessed regions.

Each candidate goes through bundled wire validation, DecodeReport, and Encode.
Accepted candidates must preserve their canonical bytes on round trip. Counts are
reported separately for schema, semantic decoding, encoding and accepted cases.

Independent generator tests check nullable traversal, escaped JSON pointers,
determinism, source immutability and required operator coverage.

## Exact acceptance exceptions

internal/evidence/v4/testdata/mutation-exceptions.json is reviewable contract evidence,
not an automatically regenerated golden file. Each entry binds:

- fixture name and SHA-256 of its exact bytes;
- operator and JSON pointer;
- exact before/after values;
- SHA-256 of the complete changed JSON;
- a specific reason for permitting acceptance.

New survivors fail. Changed payloads, missing fixtures, duplicate entries, missing
reasons and stale exceptions fail. There are no wildcard acceptance rules.
Normal tests never write the ledger.

The current ledger contains 927 exact exceptions. Most concern optional prose,
descriptive capability declarations, redundant references, or independently retained
failure projections. Three boundaries deserve explicit review:

1. Flat 4.0 imports deliberately preserve the frozen 2.0 semantics. Some declared
   normalization fields, including formatting_removed_count, are not recomputed.
   Acceptance is a compatibility exception, not an attestation of their accuracy.
2. XML producer linkage requires some nonempty parsing coverage, not source-backed
   replay of every retained XML byte. Identification outcomes may be redundant with
   text/metadata/relationship producers. This is the proposed OC-001 distinction
   between lexical context and analytical coverage.
3. Raw namespaces and other source declarations cannot be authenticated without
   source bytes. They remain constrained where needed for retained relationships,
   metadata, structural coordinates and authority. Acceptance is not proof that a
   producer actually parsed those bytes or followed its claimed grammar.

The review seat should review these exceptions separately from the implementation.
The ledger does not establish independent approval or authorize a merge.

## Discovery and corrected survivors

The initial broad run accepted 1,260 mutations. That was a discovery result, not a
passing gate. It exposed additional consistency gaps beyond the four historical
metadata authority defects:

- detached/missing tag tokens and ancestor reparenting inconsistent with the XML tree;
- deletion of an assessed relationship from its retained inventory;
- empty compressed extents retaining nonempty payload claims;
- signature observations lacking coverage of their supporting prefix;
- embedded-relationship inclusion without a retained relationship reference;
- completed object enumeration dropping candidates explicit in retained evidence.

The remaining review-seat ambiguity is resolved by rejecting both assessed and
excluded regions on failed/canceled/not-run outcomes. Their states, codes and
diagnostics remain available as failure history; they no longer carry inert
exclusions that contradict a successful sibling. Positive tests retain unsuccessful
history without regions, while negative tests cover all three states.

The fixes preserve self-closing XML's legitimate zero-width end token and valid empty
stored ZIP payloads. They do not require full embedded-object payload analysis merely
to inspect a bounded signature prefix.

The accepted gate run generated 8,748 distinct candidates:

| Disposition | Count |
| --- | ---: |
| Rejected by schema | 4,542 |
| Rejected by semantic import | 3,279 |
| Rejected only by Encode | 0 |
| Exact, reasoned acceptance exceptions | 927 |

These are mutation counts, not independent bug counts or a security completeness
score. Additional fixtures and coordinated operators can discover additional gaps.

## Four historical authority classes

TestGeneratedProjectionAuthority derives every ordered pair of direct properties
from the shipped metadata inventory. Each has a valid partial baseline and generated
pooled-exclusion, coincident-sibling, zero-property and detached-property variants:
12 positive baselines and 12 probes per defect class in the current corpus.

These focused probes exercise the authority validator directly, separately from the
full-import walker. That separation is intentional: another import guard must not
make removal of an authority check appear undetectable or make a malformed-schema
rejection masquerade as proof of authority checking.

scripts/check_attestation_faults.py first verifies the unmodified probes pass. It then
uses temporary Go overlays to remove each guard independently. All four experiments
must fail with the corresponding generated semantic survivor, not a compilation error
or unrelated test failure. No checked-out source or frozen fixture is changed.

## Reproduction

Run commands through the repository's bounded launcher (default 1 GiB / 60 seconds):

    python3 scripts/run_bounded.py -- go test -count=1 -p 1 ./internal/evidence/v4 -v
    python3 scripts/run_bounded.py -- python3 scripts/check_attestation_faults.py

The fault runner accepts --go /absolute/path/to/go for a local toolchain.

To collect newly unclassified survivors, set ALETHARSIS_MUTATION_DISCOVERY to a new
scratch output path using env inside the bounded command. It creates that file with
exclusive creation and mode 0600. Discovery still fails if any unclassified survivors
exist; it does not turn them into approved exceptions. Never point it at the ledger.

## Review follow-up: scope and CI boundaries

The broad walker does not by itself falsify all four historical authority
guards. Its current operators cannot transfer exclusions between records or
copy a sibling element's span. The focused authority suite selects its defect
classes explicitly, then generates property pairs within each class. It must
not be described as four defect classes independently discovered by the walker.
Reference substitution, span widening, record insertion, richer state changes
and additional coordinated mutations remain corpus-expansion work; the current
passing gate makes no claim to cover those operators.

The full walker runs in the ordinary CI coverage step. The race step uses
`-short` to omit only that expensive test; the focused authority, structural,
generator and importer tests still run under race instrumentation. The separate
CI fault-experiment step now includes an independent XML single-root fault as
well as the four metadata authority faults. A probe timeout is an explicit failed
experiment, never evidence that a guard detected its fault.

The independent XML case uses two sequential, internally coherent element/token
trees, so removing the root-cardinality check cannot be masked by a different
token-parent consistency check.
