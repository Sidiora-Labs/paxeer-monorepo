#!/usr/bin/env bash
# paxeer-x-services: private-profile
set -euo pipefail
exec python3 - <<'PY'
import json
import os
from pathlib import Path
import re
import subprocess
import sys

root = Path.cwd()
evidence = Path(os.environ['PAXEER_X_EVIDENCE_DIR'])
tests = 0

def execute(argv, name, timeout=180):
    path = evidence / ('104.17.4-' + name + '.log')
    with path.open('w') as log:
        result = subprocess.run(argv, cwd=root, stdout=log, stderr=subprocess.STDOUT, timeout=timeout)
    print('exit=' + str(result.returncode) + ' log=' + str(path), flush=True)
    if result.returncode:
        sys.exit(result.returncode)
    return path.read_text()

for name in ('LAYERX_GATEWAY_LIB_TEST_BINARY', 'LAYERX_GATEWAY_TEST_BINARY'):
    binary = os.environ.get(name)
    if not binary or not Path(binary).is_absolute() or not Path(binary).is_file() or not os.access(binary, os.X_OK):
        print('missing prerequisite: genuine prebuilt ' + name, file=sys.stderr)
        sys.exit(78)
    inventory = execute([binary, '--list'], name.lower() + '-inventory')
    declared = set(re.findall(r'^(.+): test$', inventory, re.M))
    if not declared:
        raise RuntimeError('required real Rust test inventory is empty: ' + name)
    output = execute([binary, '--nocapture', '--test-threads=1'], name.lower())
    results = re.findall(r'test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;', output)
    if len(results) != 1 or int(results[0][0]) != len(declared) or results[0][1:] != ('0', '0'):
        raise RuntimeError('real Rust test corpus omitted a required case: ' + name)
    tests += int(results[0][0])

output = execute(['python3', 'services/platform/hosted/gateway/tests/key-store.py'], 'key-store')
records = re.findall(r'^GATEWAY_KEY_STORE_RESULT (.+)$', output, re.M)
if len(records) != 1:
    raise RuntimeError('real Redis key-store corpus evidence is missing')
record = json.loads(records[0])
if not isinstance(record.get('tests'), int) or record['tests'] <= 0 or record.get('skipped') != 0:
    raise RuntimeError('real Redis key-store corpus is empty or skipped')
tests += record['tests']

print('PAXEER_X_GATE tests=' + str(tests) + ' skipped=0', flush=True)
PY
