#!/usr/bin/env python3
"""Pipe/lifetime fixture: transport an actual producer file or hold/fill pipes.

This is a child peer of the subprocess wrapper, not a replacement KMS producer.
It never creates owner identities, signs requests or emits an owner-export schema.
"""
import json
import os
from pathlib import Path
import signal
import sys

sys.dont_write_bytecode = True
root = Path(__file__).resolve().parent
configuration = json.loads((root / 'peer-control.json').read_bytes())
assert sys.argv[1:] == ['prepare']
assert json.loads(sys.stdin.buffer.read()) == configuration['request']
(root / 'peer-pid').write_text(str(os.getpid()))
if configuration['mode'] == 'stall':
    signal.pause()
elif configuration['mode'] == 'closed-pipes':
    os.close(1)
    os.close(2)
    signal.pause()
elif configuration['mode'] in ('stdout', 'stderr'):
    descriptor = 1 if configuration['mode'] == 'stdout' else 2
    while True:
        os.write(descriptor, b'x' * 8192)
elif configuration['mode'] == 'success':
    with Path(configuration['producer_output']).open('rb') as source:
        sys.stdout.buffer.write(source.read())
elif configuration['mode'] == 'refuse':
    sys.exit(1)
else:
    raise AssertionError('unknown child peer case')
