# Report validator dependency

The v2 report implementation proposes a direct dependency on
[`santhosh-tekuri/jsonschema` v6.0.3](https://github.com/santhosh-tekuri/jsonschema/tree/v6.0.3)
for Draft 2020-12 wire validation. The [registry record](jsonschema.json) pins the
revision, module checksum and license digest; the exact upstream
[license](licenses/jsonschema-v6.0.3.txt) is retained for distribution.

This is a schema utility, not a detector adoption or approval of any Epic #21
candidate. It prevents maintaining a second handwritten copy of the 23 published
finding variants. Aletharsis owns strict decoding, resource limits, evidence
semantics, source verification and report interpretation.

The adapter compiles only bundled schemas, disables resource loading, does not
register content decoders, uses Go's regular-expression implementation, and omits
upstream validation error prose from user-facing failures. The library's exact
rational numeric checks receive tokens only after Aletharsis enforces numeric
length/exponent budgets. No document-selected schema or URL is loaded.

The library is pure Go. It uses the existing `golang.org/x/text` pin; `regexp2` is
an upstream test dependency and is not linked into the adapter. No Python, C,
Rust, sidecar or network runtime is introduced. Updating the library requires a
reviewed pin/license-record update and rerunning wire, semantic, import, fuzz,
compatibility and platform checks. Include this license copy with distributions
that link the library.
