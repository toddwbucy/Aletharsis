"""Independent Apache-2.0 synthetic vectors; no upstream fixture/source copied."""
import hashlib


def cases():
    # Expected observed code points are explicit, not generated from any detector.
    specs = [
        ('empty','', [], 'Empty literal text; no observations expected.'),
        ('ascii','ordinary text\r\nnext line\n', [], 'ASCII line endings are ordinary; preserve bytes.'),
        ('multilingual','日本語 café فارسی', [], 'Visible multilingual prose is not hidden data.'),
        ('isolated_zwsp','hello\u200bworld', [0x200b], 'Isolated zero-width character; intent undetermined.'),
        ('persian_zwnj','می\u200cروم', [0x200c], 'Legitimate Persian joining context; retain observation without watermark claim.'),
        ('arabic_zwj','ب\u200dب', [0x200d], 'Joining control in Arabic script; context is not universal permission.'),
        ('emoji_zwj','👩\u200d💻', [0x1f469,0x200d,0x1f4bb], 'Emoji plus joiner: source-code review may care; no automatic watermark claim.'),
        ('emoji_selector','❤️', [0x2764,0xfe0f], 'Recognized emoji presentation selector; policy exclusion must remain distinguishable.'),
        ('orphan_selector','x\ufe0f', [0xfe0f], 'Same selector without recognized emoji context.'),
        ('cjk_selector','漢\U000e0100', [0xe0100], 'Ideographic variation sequence; legitimate context must not erase inventory.'),
        ('word_typography','“word”—it’s…\u00a0x\u202fy', [0x201c,0x201d,0x2014,0x2019,0x2026,0xa0,0x202f], 'Visible Word-like punctuation and no-break spaces, not confirmed watermark.'),
        ('bidi','x\u202ey\u202c \u2066z\u2069', [0x202e,0x202c,0x2066,0x2069], 'Bidi formatting and isolation controls with exact positions.'),
        ('arabic_letter_mark','a\u061cb', [0x61c], 'Arabic Letter Mark inventory boundary.'),
        ('controls','a\x00\x1b\x7f\x85b', [0,0x1b,0x7f,0x85], 'Literal control bytes; binary-file identification may reject this sample.'),
        ('soft_combining','co\u00adoperate e\u0301 \u034f', [0xad,0x301,0x34f], 'Soft hyphen and combining marks; do not normalize source.'),
        ('invisible_math','a\u2060\u2061\u2062\u2063\u2064b', [0x2060,0x2061,0x2062,0x2063,0x2064], 'Word joiner and mathematical invisible operators.'),
        ('binary_sequence','x'+('\u200b\u200c'*32)+'y', [0x200b,0x200c], 'Synthetic two-state sequence; possible structural encoding, no vendor attribution.'),
        ('selector_sequence','x'+('\ufe00\ufe01'*8), [0xfe00,0xfe01], 'Repeated selector alphabet; distinguish pattern from isolated selectors.'),
        ('tag_sequence','x\U000e0061\U000e0062\U000e0063\U000e007f', [0xe0061,0xe0062,0xe0063,0xe007f], 'Known ASCII projection abc plus cancel tag; inert evidence only.'),
        ('tag_run', 'x'+''.join(chr(0xe0000+ord(c)) for c in 'abcdefghijklmno')+'\U000e007f', [*range(0xe0061,0xe0070),0xe007f], 'Sixteen tag characters: independent known ASCII projection abcdefghijklmno and cancel tag; compare with short tag case.'),
        ('hangul_fillers','x\u115f\u1160\u3164\uffa0y', [0x115f,0x1160,0x3164,0xffa0], 'Letter-category fillers can be visually blank despite not being format controls.'),
        ('khmer_vowels','x\u17b4\u17b5y', [0x17b4,0x17b5], 'Language-specific invisible marks; preserve legitimate context.'),
        ('mongolian_selectors','x\u180b\u180c\u180dy', [0x180b,0x180c,0x180d], 'Mongolian selectors are combining marks outside generic VS ranges.'),
        ('boundary_offsets','😀e\u0301\u200b\r\n\ufeffz', [0x1f600,0x301,0x200b,0xfeff], 'Non-BMP, decomposed scalar, CRLF and embedded BOM exercise coordinate units.'),
        ('leading_bom','\ufeffa\ufeffb', [0xfeff], 'Leading and embedded BOM retained; comparator may deliberately omit first.'),
        ('literal_marker','⟦U+200B ZERO WIDTH SPACE⟧', [], 'Visible marker-looking text is not actual U+200B.'),
    ]
    specs += [(f'encoded_{encoding}', '\ufeff😀a\u200b\r\n', [0xfeff,0x1f600,0x200b], 'BOM-marked encoding; comparator input is explicit literal decoding, not its file parser.') for encoding in ('utf-16-le','utf-16-be','utf-32-le','utf-32-be')]
    out=[]
    for name,text,points,interpretation in specs:
        encoding=name.removeprefix('encoded_') if name.startswith('encoded_') else 'utf-8'
        raw=text.encode(encoding)
        offsets=[];utf16=[];b=0;u=0
        for char in text:
            offsets.append(b); utf16.append(u)
            b+=len(char.encode(encoding));u+=len(char.encode('utf-16-le'))//2
        offsets.append(b);utf16.append(u)
        observations=[{'scalar':i,'byte_start':offsets[i],'byte_end':offsets[i+1],'utf16_start':utf16[i], 'code_point':f'U+{ord(c):04X}'} for i,c in enumerate(text) if ord(c) in points]
        out.append({'id':name,'license':'Apache-2.0','origin':'independently authored Aletharsis synthetic fixture', 'artifact_kind':'literal source bytes','serialization':encoding,'raw_hex':raw.hex(),'source_sha256':hashlib.sha256(raw).hexdigest(),'decoded_text':text,'decoded_utf8_sha256':hashlib.sha256(text.encode()).hexdigest(),'transformations':[{'operation':'strict literal decoding','from':encoding,'to':'Unicode scalars; no BOM stripping, newline conversion or normalization'}], 'expected_observations':observations,'interpretation':interpretation})
    return out
