#!/bin/sh
set -eu

build_dir=${1:-build}
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
for directory in ('include/layerx', 'src'):
    for path in sorted(pathlib.Path(directory).rglob('*')):
        if not path.is_file() or path.suffix not in ('.c', '.h'):
            continue
        source = re.sub(r'\\\n', '', path.read_text())
        source = tokens.sub(strip_comment, source)
        for number, line in enumerate(source.splitlines(), 1):
            if floating.search(line):
                print(f'{path}:{number}:{line}')
                found = True
if found:
    print('consensus source contains floating-point syntax', file=sys.stderr)
    sys.exit(1)
PY

objects=$(find "$build_dir/obj/src" -type f -name '*.o' 2>/dev/null || true)
if [ -n "$objects" ] && objdump -d $objects | \
    grep -E '[[:space:]](f(add|sub|mul|div|ld|st|com|ucom|sqrt|sin|cos|tan)|v?(add|sub|mul|div|max|min|sqrt|com|ucom|cvt|round|mov)(ss|sd|ps|pd))[[:space:]]'; then
    echo "consensus object contains floating-point instructions" >&2
    exit 1
fi

if [ -n "$objects" ] && nm -u $objects | \
    grep -E '[[:space:]]U[[:space:]]+(acos|asin|atan|atan2|ceil|cos|cosh|exp|fabs|floor|fmod|frexp|ldexp|log|log10|modf|pow|sin|sinh|sqrt|tan|tanh)(f|l)?(@.*)?$'; then
    echo "consensus object imports a libm floating-point symbol" >&2
    exit 1
fi

printf '%s\n' "no floating-point syntax, instructions or libm imports in consensus paths"
