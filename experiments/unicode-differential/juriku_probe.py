"""Invoke the unchanged pinned read-only line detector; no filesystem CLI path."""
import dataclasses
import importlib.util
import json
from pathlib import Path
import sys

sys.path.insert(0, '/emoji')
spec = importlib.util.spec_from_file_location('juriku_probe_target', '/upstream/hidden-characters-detector.py')
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
assert module.emoji_library_available
assert not module.pathspec_library_available
request = json.load(sys.stdin)
text = request['text']
logger = module.SimpleLogger(use_colors=False, stream=sys.stderr)
results = {}
for mode, word in [('inventory',False), ('word_exclusions',True)]:
    detector = module.UnicodeMarkerDetector(clean_file=False, check_typographic=True,
        check_ivs=True, exclude_word_chars=word, logger=logger)
    unchanged, markers, changed = detector._process_line(text, 1)
    assert unchanged == text and changed is False
    results[mode] = [dict(dataclasses.asdict(m), code_point=f'U+{ord(m.original_char):04X}') for m in markers]
print(json.dumps({'position_units':'Unicode scalar indices in supplied decoded text',
    'emoji_version':module.emoji.__version__, 'pathspec_available':False,
    'input_unchanged':True, 'results':results}, ensure_ascii=True, sort_keys=True))
