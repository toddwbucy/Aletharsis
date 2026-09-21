# Located XML part evidence — XP-001

Status: proposed parser increment under #14, stacked on DP-001 (#64).
`internal/xmlparts` preserves XML structure and locations within an independently
identified decompressed part. It supplies a prerequisite for OPC/ODF identification
and context producers; it does not enable DOCX/ODT CLI support or assess profiles.

## Identity and location contract

`Parse` receives already acquired part bytes, their expected SHA-256, a context and
explicit limits. It verifies that identity before parsing and returns the digest,
parser version `xml-parts/1`, encoding, BOM flag, ordered elements and ordered tokens.
No path, URL, filesystem, subprocess or external resource resolver is involved.
Callers must not mutate input during parsing; output values are independently owned.

The host must retain the package source digest and exact part name alongside this
part digest. Equal hashes in two different package parts do not merge their identity.
Every span is half-open and measures original **part bytes**, never ZIP-container
bytes, Unicode scalars, UTF-16 units, rendered characters or document reading order.

Each element has a stable collection index, parent index (`-1` for the root), lexical
prefix, local name, resolved namespace URI, ordered attributes, start-tag span,
end-tag span and full-element span. Attributes retain names, namespace-declaration
status and normalized XML values. Their source anchor is the containing start tag;
this version does not claim attribute-value character spans.

Tokens retain start/end events, text, comments, declarations and processing
instructions in input order, with parent/element references. Self-closing tags produce
a zero-width `synthetic_end` at the end of their start tag; no closing bytes are invented.
The optional leading UTF-8 BOM is explicit and shifts all offsets by three bytes.
Remaining token spans cover all original bytes, including outside-root whitespace.

Text mapping quality is deliberately separate from token location:

- `exact_utf8`: decoded character data equals its original byte slice exactly.
  A downstream scalar map can be derived within that verified literal token.
- `token_only`: XML references, CDATA delimiters or XML line-ending normalization
  changed the representation, or this is a non-text token. The token remains exactly
  located, but its decoded characters do not inherit one-to-one source-byte offsets.
- `synthetic_end`: the parser-generated end event for a self-closing element.

For example `&#x200B;` yields a decoded zero-width space with a token-level anchor;
it does not pretend that the eight ASCII source bytes are a literal three-byte ZWSP.
CDATA remains distinguishable through its original token span. Comments and processing
instructions are untrusted evidence values, never executable instructions.

## Parsing and namespace policy

The admitted subset is UTF-8 XML 1.0, optionally BOM-prefixed, without DTDs or custom
entities. Other encodings fail explicitly; no transcoding can silently change offsets.
Unknown entities and malformed input produce no partial semantic document. The standard
predefined references and legal numeric character references are decoded normally.

The implementation uses Go `encoding/xml.RawToken` and explicitly checks the document
root, stack, lexical QName matching, namespace scopes, undeclared prefixes, reserved
bindings and duplicate expanded attributes. Default namespaces apply to elements but
not unprefixed ordinary attributes. Prefix shadowing is scoped and restored on exit.
Required attribute whitespace, declaration placement/order, QName component starts,
XML character validity and outside-root content receive additional checks. The Go
lexer determines admitted XML name characters; this is not a certification of full
XML 1.0 Fifth Edition name support or an Office schema validator.

Attribute values apply XML literal-whitespace normalization before decoding references:
a literal tab/CR/LF becomes a space (CRLF counts once), while `&#x9;` remains a tab.
Raw start-tag bytes remain available through their anchor, so normalization is neither
source mutation nor loss of forensic evidence. No DTD-dependent attribute typing,
default attributes, validation, XInclude, stylesheets or external entities execute.
All directives, including internal/external DOCTYPE declarations, are unsupported.
Namespace URIs are compared as exact strings; they are never fetched or treated as trust.

## Bounds and failure semantics

Default/maximum limits: 4 MiB part bytes, 100,000 tokens, 50,000 elements, depth 128,
128 attributes per element and 8 MiB retained name/namespace/value bytes. Callers may
lower limits. The source cap also bounds individual lexer work and token allocations;
attribute/token counts are checked after lexical tokenization and before storage.
Struct/slice overhead, original bytes and lexer scratch space are additional memory:
these limits are not a hard RSS promise. Cancellation is checked between tokens;
a single bounded lexer call is not preempted and no wall-clock deadline is guaranteed.

Typed sentinel errors distinguish invalid admitted XML, unsupported features/encoding,
source identity mismatch and resource limits; context cancellation is preserved.
Every error returns no partial Document. A later format parser must retain original
part evidence and explicitly report failed/unavailable coverage. It must not classify
a failed part, unsupported encoding or DTD-bearing package as clean or expected.

## Verification and remaining gates

Tests use manually specified byte spans for supplementary emoji, combining characters,
ZWSP, BOM, entity references, CDATA and CRLF. They verify token coverage, element nesting,
self-closing events, namespace shadowing, ordinary/qualified attributes, literal versus
referenced whitespace, malicious instruction text, malformed grammar, duplicate
attributes, unsupported DTD/encoding, identity, bounds, cancellation, owned output and
determinism. The four retained DOCX/ODT packages from DP-001 are parsed end to end through
the package reader; hidden-sequence tokens resolve to exactly 192 original part bytes.
These checks preserve structural facts without declaring a watermark or trusted content.

```sh
go test ./internal/xmlparts
go test -race ./internal/xmlparts
go test ./...
go vet ./...
go test ./internal/xmlparts -run '^$' -fuzz FuzzXMLPart -fuzztime 3s -parallel 2
```

Next: OPC/ODF format identification, relationship resolution within package boundaries,
format-specific text/hidden-content/metadata extraction, exact derived text mappings,
context producers and reviewed profiles. Unknown XML does not become expected merely
because it parses. Report/import migration, forensic explanations and CLI integration
remain separate requirements. PDF uses a separate parser path. #14 remains open.

References: [Go encoding/xml](https://pkg.go.dev/encoding/xml),
[XML 1.0](https://www.w3.org/TR/REC-xml/) and
[Namespaces in XML](https://www.w3.org/TR/xml-names/). These specify the lexer and
semantic boundaries; successful parsing here does not establish document validity,
credential authenticity, human intent or absence of concealed content.

## Increment validation record

Linux amd64 / Go 1.27.1: full Go suite, `go vet ./...` and package race checks passed.
The final three-second, two-worker fuzz configuration completed 191,482 executions
without failure (about four seconds including shutdown). The 265 offline contract
tests passed. This is bounded implementation evidence, not XML/Office conformance
certification. No network resource, external parser, model or Office application ran.
