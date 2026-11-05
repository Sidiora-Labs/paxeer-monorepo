#!/bin/sh
set -eu

build_dir=${1:-build}
source_only=0
if [ "$build_dir" = --source-only ]; then source_only=1; fi
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
python3 - <<'PY'
import pathlib
import re
import sys

tokens = re.compile(
    r'//[^\n]*|/\*[\s\S]*?\*/|"(?:\\[\s\S]|[^"\\])*"'
    r"|'(?:\\[\s\S]|[^'\\])*'"
)
floating = re.compile(
    r'(^|[^a-zA-Z0-9_])(float|double|long\s+double)([^a-zA-Z0-9_]|$)'
    r'|#\s*include\s*[<"]math\.h[>"]'
    r'|(^|[^a-zA-Z0-9_])(([0-9]+\.[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?'
    r'|[0-9]+[eE][+-]?[0-9]+|0[xX][0-9a-fA-F]+(\.[0-9a-fA-F]*)?'
    r'[pP][+-]?[0-9]+)[fFlL]?([^a-zA-Z0-9_]|$)'
)


def strip_comment(match):
    token = match.group()
    if token.startswith(('//', '/*')):
        return re.sub(r'[^\n]', ' ', token)
    return token


found = False
inspected = 0
for directory in ('include/layerx', 'src'):
    if not pathlib.Path(directory).is_dir():
        sys.exit('expected consensus source directory is unavailable')
    for path in sorted(pathlib.Path(directory).rglob('*')):
        if not path.is_file() or path.suffix not in ('.c', '.h'):
            continue
        inspected += 1
        source = re.sub(r'\\\n', '', path.read_text())
        source = tokens.sub(strip_comment, source)
        for number, line in enumerate(source.splitlines(), 1):
            if floating.search(line):
                print(f'{path}:{number}:{line}')
                found = True
if not inspected:
    sys.exit('consensus source inventory is empty')
if found:
    print('consensus source contains floating-point syntax', file=sys.stderr)
    sys.exit(1)
PY

printf '%s\n' "consensus source syntax inspection passed"
if [ "$source_only" -eq 1 ]; then
    echo "source-only inspection; binary instructions and imports were not inspected"
    exit 0
fi
python3 "$script_dir/audit_object_checks.py" float "$build_dir"
printf '%s\n' "no floating-point syntax, instructions or libm imports in inspected consensus paths"
