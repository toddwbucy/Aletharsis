# Go v2 evidence graph and coverage validation

| Attribute | Value |
| --- | --- |
| Status | Implementation proposed for review; default CLI output remains schema 1.0 |
| Depends on | [EC-002](report-v2-wire-and-import.md), [execution records](go-v2-records-registry.md) |
| Tracking | [#17](https://github.com/toddwbucy/Aletharsis/issues/17), [#21 G1](https://github.com/toddwbucy/Aletharsis/issues/21), [#7](https://github.com/toddwbucy/Aletharsis/issues/7) |

## Evidence records

`internal/evidence/v2` adds typed artifact, representation, transformation,
content-reference, mapping, anchor and structural-result records. Producers validate
before serialization. Explicit `DecodeArtifact`, `DecodeMapping`, `DecodeAnchor`
and `DecodeResult` entry points enforce closed wire variants and the existing
record byte/depth/node limits. Plain `json.Unmarshal` is not the import boundary.
Text-pointer indices use canonical nonnegative decimal notation.

Artifact absence distinguishes not acquired, extraction unavailable, not retained
and redacted. Retained blob references contain digests, never filesystem paths;
this package neither resolves blobs nor opens files or URLs. A blob's declared
digest does not prove its bytes are available or authentic.

Anchors have separate text, structural-object, credential, statistical-sample and
legacy-unknown locator variants. Text locators carry paired scalar/source-byte
spans and selection identities. Statistical locators name samples and scopes,
without invented watermark spans. The only accepted result payload remains
`structural_scan/1`; vendor probabilities or verification results require separately
reviewed contracts.

## Graph checks

`Graph.Validate(file, document)` checks the execution/evidence subgraph:

- Unique typed references and complete capability planning.
- A single source artifact whose identity agrees with the file record.
- Parent-before-child lineage, transform inputs, mappings and content references.
- Retained UTF-8 text digests and lengths; native UTF-8/16/32 coordinate widths.
- Requested/analyzed/excluded bounds, partial-coverage accounting, disabled and
  unavailable operation states, and diagnostic linkage.
- Text anchor coordinates, selection digests and containment in analyzed scope.
  Byte scopes refer to artifact UTF-8 bytes; locator byte spans refer to original
  source bytes. Those coordinate systems are never substituted for each other.
- Credential/sample references and structural result eligibility, scope and anchor
  linkage. Every analyzed structural scope needs a result; partial execution needs
  usable results.

`Graph.AggregateStatus(file, document)` validates first, then applies EC-001's
operational precedence. Disabled declarations do not cause failure. Acquisition or
parsing failure wins over independent results. Partial coverage, requested optional
failure, required unavailability and cancellation cannot silently become completed
analysis. Status is independent of a future finding-view filter.

These checks establish internal consistency, not authenticity. Unknown mapping
methods are descriptive claims and grant no exact-navigation or edit authority.
Source-byte verification still uses the independent identity API and authorized
acquisition. No credential signatures or vendor responses are verified here.

## Validation and remaining work

All eight accepted report fixtures exercise the new Go records and graph checks,
including clean/unavailable, structural observation, filtered observation, required
unavailability, acquisition/decode failure, cancellation and partial timeout.
Mutation tests cover broken identities, coordinates, lineage, scope accounting,
missing results, unsupported result semantics and malformed closed records.
Bounded fuzzing starts from accepted records and invalid inputs.

This graph is not a full report validator or a general workbench importer. It does
not yet validate finding payloads/linkage, summary counts or the report envelope,
assign deterministic references, coordinate native execution, or emit CLI v2 output.
Whole-report resource budgets and import retention remain separate from the small
record decoder budgets. In-memory graph construction is a Go service boundary, not
an unbounded JSON import entry point.

The next increment must assemble native reports, validate finding linkage and
summary/view semantics, assign deterministic identities and expose opt-in v2 JSON
and human coverage reporting. Existing schema 1.0 behavior and frozen artifacts
remain unchanged. Issue #17, Epic #21 G1 and Issue #7 stay open.
