"""Search every selected input; rg exit 1 is a no-match, all other errors refuse."""

import argparse
import os
from pathlib import Path
import re
import stat
import subprocess
import sys


class SearchError(ValueError):
    pass


def classify(returncode):
    if returncode not in (0, 1):
        raise SearchError('selected publication input could not be searched')
    return returncode == 0


def selected_files(data, exclude=None):
    if data and not data.endswith(b'\0'):
        raise SearchError('publication inventory is not NUL terminated')
    pattern = re.compile(exclude) if exclude else None
    return [os.fsdecode(name) for name in data.split(b'\0') if name
            and not (pattern and pattern.search(os.fsdecode(name)))]


def readable_input(name):
    path = Path(name)
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or not info.st_mode & 0o444:
        raise SearchError('selected publication input is inaccessible')
    with path.open('rb') as stream:
        stream.read(1)


def scan(files, arguments):
    outputs = []
    matched = False
    for name in files:
        readable_input(name)
    for offset in range(0, len(files), 200):
        batch = files[offset:offset + 200]
        result = subprocess.run(
            ['rg', *arguments[:-1], '--', arguments[-1], *batch], stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=False,
        )
        matched |= classify(result.returncode)
        outputs.append(result.stdout)
    return matched, b''.join(outputs)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--exclude')
    parser.add_argument('--no-match-ok', action='store_true')
    parser.add_argument('--select-only', action='store_true')
    parser.add_argument('--gitlinks')
    parser.add_argument('rg_arguments', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    arguments = args.rg_arguments
    if arguments[:1] == ['--']:
        arguments = arguments[1:]
    try:
        files = selected_files(sys.stdin.buffer.read(), args.exclude)
        if args.gitlinks:
            gitlinks = set(Path(args.gitlinks).read_text().splitlines())
            files = [name for name in files if name not in gitlinks]
        if args.select_only:
            sys.stdout.buffer.write(b''.join(os.fsencode(name) + b'\0' for name in files))
            return 0
        matched, output = scan(files, arguments)
    except (SearchError, OSError, ValueError):
        # Never echo rg's stderr, a filename or a partial match on an error.
        print('publication scan refused: selected input or search tool unavailable', file=sys.stderr)
        return 2
    sys.stdout.buffer.write(output)
    return 0 if matched or args.no_match_ok else 1


if __name__ == '__main__':
    sys.exit(main())
