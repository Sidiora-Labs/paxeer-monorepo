"""Publish journal seeds once, without overwriting retained runtime records."""

import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys


MARKER = '.bootstrap.json'
MAX_RECORD = 524288
MAX_RECORDS = 128


def directory(path, private=False):
    if Path(os.path.realpath(path)) != path:
        raise ValueError('journal directory alias')
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        info = os.fstat(fd)
        if (info.st_uid not in (0, os.geteuid()) or info.st_gid != os.getegid()
                or info.st_mode & 0o022):
            raise ValueError('journal directory ownership or mode')
        if private and (info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700):
            raise ValueError('private journal directory ownership or mode')
    except BaseException:
        os.close(fd)
        raise
    return fd


def read_record(fd, name, private=False):
    record = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
    with os.fdopen(record, 'rb') as stream:
        before = os.fstat(stream.fileno())
        owners = (os.geteuid(),) if private else (0, os.geteuid())
        modes = (0o600,) if private else (0o400, 0o440, 0o600)
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or before.st_uid not in owners or before.st_gid != os.getegid()
                or stat.S_IMODE(before.st_mode) not in modes or not 0 < before.st_size <= MAX_RECORD):
            raise ValueError('journal record ownership, type or bound')
        raw = stream.read(MAX_RECORD + 1)
        after = os.fstat(stream.fileno())
        identity = lambda value: (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns)
        if len(raw) != before.st_size or identity(before) != identity(after):
            raise ValueError('journal record changed while reading')
        return raw


def create_record(fd, name, raw):
    record = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    with os.fdopen(record, 'wb') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def bootstrap_journal(source, private):
    source, private = Path(source), Path(private)
    parent = directory(private, private=True)
    try:
        fcntl.flock(parent, fcntl.LOCK_EX)
        seeds = directory(source)
        try:
            names = sorted(os.listdir(seeds))
            if not 2 <= len(names) <= MAX_RECORDS:
                raise ValueError('journal seed count')
            for name in names:
                if not re.fullmatch(r'[0-9a-f]{64}\.(admission|deployment)', name):
                    raise ValueError('journal seed name')
                if name.split('.')[0] + ('.deployment' if name.endswith('.admission') else '.admission') not in names:
                    raise ValueError('journal seed pair missing')
            records = {name: read_record(seeds, name) for name in names}
            if sum(map(len, records.values())) > MAX_RECORD:
                raise ValueError('journal seed total bound')
            if sorted(os.listdir(seeds)) != names:
                raise ValueError('journal seed inventory changed')
        finally:
            os.close(seeds)
        manifest = json.dumps({'version': 1, 'seeds': {
            name: hashlib.sha256(raw).hexdigest() for name, raw in records.items()
        }}, sort_keys=True, separators=(',', ':')).encode()
        target = private / 'journal'
        if os.path.lexists(target):
            journal = directory(target, private=True)
            try:
                if MARKER in os.listdir(journal):
                    if read_record(journal, MARKER, private=True) != manifest:
                        raise ValueError('journal bootstrap seed conflict; reconciliation required')
                    for name in names:
                        read_record(journal, name, private=True)
                    return
                # Adoption is explicit and cannot fill gaps or replace newer bytes.
                for name, raw in records.items():
                    if read_record(journal, name, private=True) != raw:
                        raise ValueError('unmarked journal conflicts with seed; reconciliation required')
                create_record(journal, '.bootstrap.pending', manifest)
                os.rename('.bootstrap.pending', MARKER, src_dir_fd=journal, dst_dir_fd=journal)
                os.fsync(journal)
            finally:
                os.close(journal)
            os.fsync(parent)
            return
        # A crash before publication leaves staging for explicit reconciliation.
        staging = private / '.journal-bootstrap'
        os.mkdir(staging, 0o700)
        journal = directory(staging, private=True)
        try:
            for name, raw in records.items():
                create_record(journal, name, raw)
            create_record(journal, MARKER, manifest)
            os.fsync(journal)
        finally:
            os.close(journal)
        os.rename(staging, target)
        os.fsync(parent)
    finally:
        os.close(parent)


if __name__ == '__main__':
    try:
        bootstrap_journal(*sys.argv[1:])
    except (OSError, ValueError) as error:
        raise SystemExit('Human journal bootstrap refused: ' + str(error))
