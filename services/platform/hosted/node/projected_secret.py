#!/usr/bin/env python3
"""Adopt a Kubernetes projected key into an owner-controlled regular file."""
import argparse
import os
from pathlib import Path
import stat

from native_runtime import write_file
from data_directory import open_directory, _private_directory


def adopt_projected_secret(source, destination):
    # AtomicWriter deliberately makes the source a symlink. Only the private
    # destination is passed to providers that enforce O_NOFOLLOW.
    descriptor = os.open(source, os.O_RDONLY | os.O_NONBLOCK)
    try:
        info = os.fstat(descriptor)
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
                or info.st_uid not in (0, os.geteuid()) or info.st_mode & 0o027
                or not 1 <= info.st_size <= 4096):
            raise ValueError('unsafe projected secret')
        raw = os.read(descriptor, 4097)
        if len(raw) != info.st_size:
            raise ValueError('projected secret changed while reading')
    finally:
        os.close(descriptor)
    destination = Path(destination)
    _, parent = open_directory(destination.parent, create=True)
    try:
        _private_directory(parent)
    finally:
        os.close(parent)
    write_file(destination, raw)


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', required=True)
    parser.add_argument('--destination', required=True)
    args = parser.parse_args(argv)
    adopt_projected_secret(args.source, args.destination)


if __name__ == '__main__':
    main()
