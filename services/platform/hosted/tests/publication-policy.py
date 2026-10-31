import json
import os
from pathlib import Path
import re
import sys


def hexadecimal(value, length, prefix=False):
    pattern = ('0x' if prefix else '') + '[0-9a-fA-F]{' + str(length * 2) + '}'
    if not re.fullmatch(pattern, value) or int(value, 16) == 0:
        raise ValueError('publication policy hexadecimal field invalid')
    return (value[2:] if prefix else value).lower()


def integer(value, limit):
    if not value.isdecimal() or not 0 < int(value) < limit:
        raise ValueError('publication policy domain invalid')
    return int(value)


def options(arguments):
    if any(item.startswith('--') for item in arguments):
        raise ValueError('publication policy option invalid')
    return arguments


def main(arguments):
    arguments = options(arguments)
    if len(arguments) == 5 and arguments[0] == 'treasury':
        _, output, network, asset, recipient = arguments
        value = dict(version=1, network_id=integer(network, 2**32),
                     asset_id=hexadecimal(asset, 32), recipient=hexadecimal(recipient, 20))
    else:
        raise ValueError('publication policy arguments invalid')
    path = Path(output)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'w') as destination:
        json.dump(value, destination, sort_keys=True)
        destination.write('\n')
        destination.flush()
        os.fsync(destination.fileno())


if __name__ == '__main__':
    try:
        main(sys.argv[1:])
    except (OSError, ValueError):
        raise SystemExit('publication policy creation refused') from None
