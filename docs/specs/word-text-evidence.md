# WordprocessingML text and direct context — WT-001

Status: proposed #14 extraction increment, stacked on XM-001 (#69).
`wordtext.Extract` selects text from one identity-checked XML part. It retains the
entire mapped XML document, including character data it does not select. It neither
identifies a package as DOCX nor changes report schemas or CLI format support.

## Selection and evidence

Accepted roots are namespace-qualified `document`, `hdr`, `ftr`, `comments`,
`footnotes` and `endnotes` in the transitional or strict WordprocessingML namespace.
A document must have exactly one direct `body`; other roots use the root as scope.
Selected character-data segments must belong to `t`, `delText`, `instrText` or
`delInstrText`, directly inside a same-namespace run, with a paragraph ancestor
inside the story scope. Text elements containing child elements are unsupported.
This is bounded structural selection, not full OOXML schema validation.

Every selected segment records its XML element, direct run, nearest paragraph,
text role and XM-001 segment index. XML entities, CDATA, supplementary characters,
combining characters and line endings retain XM-001's exact scalar-to-part-byte
maps. Multiple character-data tokens in one text element remain separate segments;
empty text elements remain in the XML tree without invented text. No inter-run or
paragraph concatenation, whitespace trimming, Unicode normalization or synthetic
separator is applied. Direct `xml:space` declarations are retained; inheritance and
rendered whitespace behavior are not computed. Unknown values produce a partial
result.

Context includes revision ancestors (`ins`, `del`, `moveFrom`, `moveTo`), foreign
namespace ancestors and text-box ancestry. Ancestor lists run nearest to farthest.
Revision authors and other attributes remain accessible through XML element links.
Foreign wrappers, including markup-compatibility alternatives, produce a partial
result: the extractor does not choose a rendered branch. It preserves selected text
from each branch. Field instructions are inert text, never evaluated.

Character data not selected is retained by segment reference and reason:
`syntax_whitespace`, `unclassified_character_data` or `unsupported_text_structure`.
Non-whitespace unclassified data and unsupported text structures produce issues and
a partial result. Every mapped character-data segment is selected or unselected
exactly once. This does not claim coverage of every possible Word text carrier:
attributes, symbols, drawings and embedded objects still require other analyzers.

## Direct visibility observations

For each selected run, direct `rPr/vanish` and `rPr/webHidden` observations retain
property element references and one of `on`, `off`, `unspecified` or `unknown`.
Missing `val` means on; `true`/`1` and `false`/`0` are recognized. Transitional
`on`/`off` are also admitted; this implementation conservatively leaves those
spellings unknown in strict parts. Unexpected attributes, child elements, nonempty
text, duplicate toggles or multiple direct run-property containers are unknown.

Paragraph run properties, styles and historical properties nested under
`rPrChange` do not become direct run flags. Neither `off` nor `unspecified` means
visible: style inheritance, property revision effects and rendering are unresolved
and explicitly listed as limitations. `webHidden` is recorded separately from
`vanish`. These facts alone are not watermark classifications.

Run `tab`, `br` and `cr` elements are separate control observations, each retaining
its full source span and element/run/paragraph references. Their attributes remain
in the XML evidence; no decoded character is manufactured for them. Paragraph tab
stop declarations under `tabs` are formatting, not text controls. Unsupported
control structures and foreign wrapper context yield issues. Controls' additional
context can be recovered through the retained XML ancestry.

## Authority, bounds and failures

The host supplies the exact part bytes and SHA-256, and must bind them to package
source identity, part name and verified relationship context. Part byte offsets
must never be relabeled ZIP byte offsets. Extracted text hashes identify decoded
segments, not original lexical bytes. This helper grants no cleanup authority.

The result records parser version `word-text/1`, story, namespace, retained XML,
ordered observations/issues, and limitations. `completed` means this selection
pass completed within its declared scope; it does not mean complete rendered-text
coverage. Unknown roots/namespaces or ambiguous main-body structure fail with
`ErrStructure`; XML, identity, cancellation and resource errors propagate with no
partial result.

XP-001 defaults bound parsing (4 MiB input, 100,000 tokens, 50,000 elements, depth
128, 128 attributes per element, 8 MiB retained values); XM-001 adds a shared
200,000-scalar mapping cap. WT-001 caps the sum of retained revision, foreign-wrapper
and direct-property references across text observations at 200,000. Context walks
are bounded by XML depth. These allocation/work budgets are not an RSS guarantee.
Cancellation is checked in parsing/mapping and between selected segments and
control candidates. The caller must not mutate source bytes during extraction;
the extractor never writes them or accesses paths, networks, renderers or programs.

## Validation and remaining work

Tests cover direct versus inherited/historical flags, ambiguous properties,
namespace spoofing, strict/transitional admission, all supported story roots,
foreign wrappers and text boxes, deleted/instruction text, tab-stop formatting,
controls, preserved whitespace, entity/CDATA source maps, segment coverage,
immutability, deterministic output, cancellation and mapping/context limits.
A retained DOCX fixture exercises ZIP-part acquisition through hidden-run extraction
and checks all 64 zero-width characters against original part bytes.

```sh
go test ./internal/wordtext
go test -race ./internal/wordtext
go test ./...
go vet ./...
go test ./internal/wordtext -run '^$' -fuzz FuzzExtract -fuzztime 3s -parallel 2
```

Remaining #14 work includes story/relationship assembly, cross-run analyzer scopes,
ODT text extraction, metadata and other structural observations, effective style
resolution or explicit coverage gaps, reviewed profile producers, report/import
mapping, CLI integration and PDF support. No claim of end-to-end DOCX audit support
or Office renderer equivalence follows from this increment.

Validation record: Linux amd64 / Go 1.27.1 full Go suite, vet and wordtext race
checks passed, along with 265 offline contract tests. A three-second two-worker
fuzz configuration exercised 64,950 inputs without failure (about four seconds
including shutdown). These checks do not establish full OOXML conformance.
