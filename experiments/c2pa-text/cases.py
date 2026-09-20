"""Independent synthetic carrier inputs; no signed credentials or copied vectors.

Wire construction follows C2PA 2.4 A.8; HTML cases exercise host syntax, not
cryptographic validity. Expectations intentionally expose upstream discrepancies.
"""
import base64
import hashlib
import struct
import unicodedata


def digest(value):
    return hashlib.sha256(value).hexdigest()


def wrapper(payload, version=1, declared_length=None):
    data = b"C2PATXT\0" + bytes([version]) + struct.pack(">I", len(payload) if declared_length is None else declared_length) + payload
    return "\ufeff" + "".join(chr(0xFE00 + b if b < 16 else 0xE0100 + b - 16) for b in data)


def corpus():
    # A JUMBF-shaped placeholder for extraction tests, NOT a valid C2PA credential.
    payload = b"\0\0\0\x08jumb"
    encoded = base64.b64encode(payload).decode("ascii")
    carrier = wrapper(payload)
    values = []

    def case(name, method, text, expected, rationale):
        raw = text.encode("utf8") if isinstance(text, str) else text
        values.append({"id": name, "method": method, "raw": raw,
                       "expected": expected, "rationale": rationale})

    for name, prefix in [("ascii", "abc"), ("bom", "\ufeffabc"),
                         ("emoji", "a\U0001f600"), ("decomposed", "e\u0301"),
                         ("combining_reorder", "a\u0315\u0300"),
                         ("mixed_endings", "a\r\nb\rc\n")]:
        case("vs_" + name, "vs", prefix + carrier, {
            "manifest_sha256": digest(payload), "offset": len(prefix.encode()),
            "length": len(carrier.encode()),
            "clean_sha256": digest(unicodedata.normalize("NFC", prefix).encode()),
        }, "Extract payload and preserve raw UTF-8 source coordinates; clean text is a separate NFC derivative.")
    case("vs_absent", "vs", "ordinary prose", {"manifest_present": False}, "No carrier present.")
    case("vs_truncated", "vs", "x" + carrier[:-1], {"manifest_present": False}, "Truncated payload must not be extracted as complete.")
    case("vs_bad_version", "vs", wrapper(payload, version=2), {"manifest_present": False}, "Unsupported wrapper version.")
    case("vs_large_declared", "vs", wrapper(payload, declared_length=0xffffffff), {"manifest_present": False}, "Huge declared length, small actual input.")
    case("vs_missing_prefix", "vs", carrier[1:], {"manifest_present": False}, "Missing required wrapper prefix; Unicode inventory still retains selectors.")
    case("vs_multiple", "vs", carrier + "x" + carrier, {"error": "multiple C2PA wrappers detected"}, "Upstream must expose ambiguity; selecting among exclusions needs G1.")
    case("vs_mid_text", "vs", "abc" + carrier + "tail", {"offset": 3, "manifest_sha256": digest(payload)}, "Non-final position remains observable; placement at end is recommended, not mandatory.")
    script = '<script type="application/c2pa">' + encoded + '</script>'
    head = lambda s: "<!doctype html><html><head>" + s + "</head><body></body></html>"
    for name, tag in [("double", script), ("single", script.replace('"', "'")),
                      ("upper", script.replace('script', 'SCRIPT')),
                      ("attribute_space", script.replace('type=', 'type = '))]:
        case("html_" + name, "html", head(tag), {"manifest_sha256": digest(payload)}, "Semantically equivalent HTML script element should expose the same inline carrier.")
    case("html_comment", "html", head("<!--" + script + "-->"), {"association": False}, "A comment is not an HTML manifest association; preserve its literal content separately.")
    case("html_data_type", "html", head(script.replace("type=", "data-type=")), {"association": False}, "data-type is not the required type attribute.")
    case("html_prefix_tag", "html", head(script.replace("<script ", "<scripture ")), {"association": False}, "A different tag name is not script.")
    case("html_body", "html", "<html><head></head><body>" + script + "</body></html>", {"association": False}, "C2PA association belongs in head; a misplaced carrier remains separate forensic evidence.")
    case("html_multiple", "html", head(script + script), {"error": "manifest.html.multipleManifests"}, "Multiple associations are explicit ambiguity.")
    case("html_bad_base64", "html", head('<script type="application/c2pa">!</script>'), {"association": True, "manifest_present": False}, "Carrier is observable even when its payload cannot decode; must not imply a verified credential.")
    case("html_truncated", "html", head(script[:-9]), {"association": False}, "Missing closing script tag; raw evidence still retained.")
    for name, tag in [("reference", '<link rel="c2pa-manifest" href="https://example.invalid/test.c2pa">'),
                      ("reference_single", "<link rel='c2pa-manifest' href='https://example.invalid/test.c2pa'>")]:
        case("html_" + name, "html", head(tag), {"reference": "https://example.invalid/test.c2pa"}, "Discover external association without any network request.")
    begin = "-----BEGIN C2PA MANIFEST-----"
    end = "-----END C2PA MANIFEST-----"
    block = begin + " data:application/c2pa;base64," + encoded + " " + end
    for name, host in [("comment", "# " + block + "\nkey=1\n"),
                       ("front_matter", "---\n" + block + "\n---\nbody"),
                       ("mixed_endings", "\ufeffa\r\n# " + block + "\rb\n")]:
        case("structured_" + name, "structured", host, {"manifest_sha256": digest(payload)}, "Extract bytes; API supplies no source span or host-syntax validation.")
    case("structured_absent", "structured", "key=1", {"error": "manifest.structuredText.noManifest"}, "No delimited block.")
    case("structured_multiple", "structured", block + block, {"error": "manifest.structuredText.multipleReferences"}, "Multiple blocks must remain ambiguous.")
    case("structured_truncated", "structured", begin + " x", {"error": "manifest.structuredText.noManifest"}, "Incomplete delimiter pair cannot yield a complete association.")
    case("structured_empty", "structured", begin + " " + end, {"error": "manifest.structuredText.emptyReference"}, "Empty reference.")
    case("structured_invalid_base64", "structured", begin + " data:application/c2pa;base64,! " + end, {"manifest_present": False}, "Bad base64 is an observed malformed reference, not absence of a carrier.")
    case("structured_uncommented", "structured", block, {"manifest_sha256": digest(payload)}, "Lexical API accepts bare delimiters; a format-aware caller must separately enforce host placement.")
    case("structured_reference", "structured", "# " + begin + " https://example.invalid/test.c2pa " + end, {"reference": "https://example.invalid/test.c2pa"}, "Discover reference; never fetch it during scan.")
    case("invalid_utf8", "vs", b"\xff", {"failure": "invalid_utf8"}, "Probe refuses lossy UTF-8 decoding before upstream sees the sample.")
    case("large_valid", "vs", "a" * (8 * 1024 * 1024) + carrier, {"offset": 8 * 1024 * 1024, "manifest_sha256": digest(payload)}, "Bounded large input with complete small carrier.")
    case("oversized", "vs", b"a" * (16 * 1024 * 1024 + 1), {"failure": "input_limit"}, "Probe rejects input above the issue's 16 MiB ceiling.")
    return values
