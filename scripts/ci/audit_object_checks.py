"""Fail-closed consensus object inventory and inspection checks."""

import os
from pathlib import Path
import re
import stat
import subprocess
import sys


EXCLUDED_SOURCES = {
    'src/storage/lxp_projection.c',
    'src/network/lx_oracle_adapter.c',
    'src/sequencer/lx_oracle_root.c',
    'src/paxeer/lxp_da_challenge_record.c',
    'src/guarantor/lxp_da_challenge.c',
}
OS_SYMBOLS = re.compile(
    r'^(time|clock_gettime|gettimeofday|rand|random|getenv|getpid|gethostname|stat|'
    r'socket|connect|accept|bind|listen|send|sendto|recv|recvfrom|getaddrinfo|curl_easy_perform)(@.*)?$'
)
LIBM_SYMBOLS = re.compile(
    r'^(acos|asin|atan|atan2|ceil|cos|cosh|exp|fabs|floor|fmod|frexp|ldexp|log|log10|'
    r'modf|pow|sin|sinh|sqrt|tan|tanh)(f|l)?(@.*)?$'
)
BALANCE_SYMBOLS = re.compile(r'^(lxp_apply_transfer|lxp_apply_transfer_set|lxp_balance_apply_leg)$')
FLOAT_INSTRUCTIONS = re.compile(
    r'\s(f(add|sub|mul|div|ld|st|com|ucom|sqrt|sin|cos|tan)|'
    r'v?(add|sub|mul|div|max|min|sqrt|com|ucom|cvt|round|mov)(ss|sd|ps|pd))\s'
)
EXECUTION_DIRS = {'protocol', 'codec', 'state', 'ledger', 'modules'}
CONSENSUS_DIRS = {'protocol', 'codec', 'state', 'storage', 'modules', 'sequencer'}


class InspectionError(ValueError):
    pass


def readable_regular(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or not info.st_mode & 0o444:
        raise InspectionError('inspection input is not a readable regular file')
    with path.open('rb') as stream:
        stream.read(1)


def walk_files(directory):
    if (not directory.is_dir() or directory.is_symlink()
            or not directory.stat().st_mode & 0o444 or not directory.stat().st_mode & 0o111):
        raise InspectionError('expected inventory directory is unavailable')
    def failed(error):
        raise InspectionError('inventory enumeration failed') from error
    for base, directories, files in os.walk(directory, onerror=failed):
        for name in directories:
            child = Path(base) / name
            if child.is_symlink() or not child.stat().st_mode & 0o444:
                raise InspectionError('inventory directory is not inspectable')
        for name in files:
            yield Path(base) / name


def object_inventory(root, build):
    sources = sorted(
        path.relative_to(root) for path in walk_files(root / 'src')
        if path.suffix == '.c' and path.relative_to(root).as_posix() not in EXCLUDED_SOURCES
    )
    if not sources:
        raise InspectionError('expected consensus source inventory is empty')
    for source in sources:
        readable_regular(root / source)
    expected = {build / 'obj' / path.with_suffix('.o') for path in sources}
    actual = {path for path in walk_files(build / 'obj' / 'src') if path.suffix == '.o'}
    if actual != expected:
        raise InspectionError('built object inventory differs from the current source inventory')
    for path in sorted(actual):
        readable_regular(path)
        if path.stat().st_size == 0:
            raise InspectionError('built object is empty')
    return sorted(actual)


def checked_output(tool, returncode, output):
    if returncode != 0:
        raise InspectionError(f'{tool} inspection failed (exit {returncode})')
    return output


def inspect(tool, *arguments):
    try:
        result = subprocess.run(
            [tool, *map(str, arguments)], check=False, text=True,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
    except OSError as error:
        raise InspectionError(f'{tool} inspection unavailable') from error
    return checked_output(tool, result.returncode, result.stdout)


def undefined_symbols(output):
    return [parts[-1] for line in output.splitlines()
            if len(parts := line.split()) >= 2 and parts[-2] in ('U', 'w', 'v')]


def check_float(disassembly, nm_output):
    if not disassembly.strip():
        raise InspectionError('objdump produced no disassembly')
    if FLOAT_INSTRUCTIONS.search(disassembly):
        raise InspectionError('consensus object contains floating-point instructions')
    if any(LIBM_SYMBOLS.fullmatch(symbol) for symbol in undefined_symbols(nm_output)):
        raise InspectionError('consensus object imports a libm floating-point symbol')


def check_symbols(group, output):
    for symbol in undefined_symbols(output):
        if group in EXECUTION_DIRS and OS_SYMBOLS.fullmatch(symbol):
            raise InspectionError('consensus object imports a forbidden operating-system symbol')
        if group in CONSENSUS_DIRS and symbol.startswith('sqlite3_'):
            raise InspectionError('consensus object imports forbidden SQLite projection symbols')
        if group == 'modules' and BALANCE_SYMBOLS.fullmatch(symbol):
            raise InspectionError('module object imports a ledger balance-apply internal')


def main():
    mode, build = sys.argv[1:]
    root = Path.cwd()
    build = Path(build).absolute()
    try:
        objects = object_inventory(root, build)
        for path in objects:
            group = path.relative_to(build / 'obj' / 'src').parts[0]
            if mode == 'float':
                check_float(inspect('objdump', '-d', path), inspect('nm', '-u', path))
            elif mode == 'symbols':
                check_symbols(group, inspect('nm', '-u', path))
            else:
                raise InspectionError('unknown inspection mode')
    except (OSError, InspectionError) as error:
        print(f'consensus inspection refused: {error}', file=sys.stderr)
        return 1
    print(f'consensus {mode} inspection: {len(objects)} expected objects inspected')
    return 0


if __name__ == '__main__':
    sys.exit(main())
