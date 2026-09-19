from ..identify import encoding_for
from ..models import DocumentEvidence, TextEvidence


class TextParser:
    name = "text-source-v1"

    def parse(self, data: bytes) -> DocumentEvidence:
        encoding, bom = encoding_for(data)
        text = data.decode(encoding, errors="strict")
        offsets = []
        offset = 0
        for character in text:
            offsets.append(offset)
            offset += len(character.encode(encoding))
        # Include EOF for unambiguous half-open spans, including empty files.
        offsets.append(offset)
        crlf = text.count("\r\n")
        return DocumentEvidence(texts=[TextEvidence(
            source="file", text=text, encoding=encoding, byte_offsets=offsets, bom=bom,
            line_endings={"crlf": crlf, "lf": text.count("\n") - crlf,
                          "cr": text.count("\r") - crlf},
        )], structure={"inspection": "literal source text", "byte_length": len(data)})
