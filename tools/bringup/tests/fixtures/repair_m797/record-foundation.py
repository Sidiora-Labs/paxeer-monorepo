#!/usr/bin/env python3
"""Record only the provenance wire contract, not a native foundation deployment."""
import importlib.util
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[5]
sys.path.insert(0, str(ROOT / 'tools/bringup/tests'))
spec = importlib.util.spec_from_file_location('foundation', ROOT / 'tools/bringup/tests/paxeer-x-fixture-foundation.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
destination = Path(__file__).resolve().parent
revision = module.ART.git('rev-parse', 'HEAD')
foundation = module.Foundation(destination, revision, ROOT / 'tools/bringup/runtime-bundle.json', 'sha256:' + '1' * 64)
raw = foundation.producer(['git', '-C', str(ROOT), 'ls-tree', '-r', '--full-tree', '-z', revision,
                           '--', 'tools/bringup/runtime-bundle.json'])
(destination / 'source-tree.bin').write_bytes(raw)
(destination / 'invocations.json').write_text(json.dumps(foundation.producers, indent=2) + '\n')
(destination / 'source.json').write_text(json.dumps(dict(revision=revision,
    source_binding=module.hashlib.sha256(raw).hexdigest(),
    source_paths=['tools/bringup/runtime-bundle.json']), indent=2) + '\n')
