"""Pin an operator-authenticated paxd TrustRoot for the registry consumer."""

import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import tempfile


MAX_ROOT_BYTES = 524_288
MAX_TOTAL_POWER = (2**63 - 1) // 8


def read_private(path, uid):
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_uid != uid or before.st_mode & 0o077:
        raise ValueError("chain trust root must be an owner-private regular file")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as stream:
        after = os.fstat(stream.fileno())
        if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_mode, before.st_uid) != (
            after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_mode, after.st_uid
        ) or not 0 < after.st_size <= MAX_ROOT_BYTES:
            raise ValueError("chain trust root changed or exceeds its bounds")
        data = stream.read(MAX_ROOT_BYTES + 1)
        final = os.fstat(stream.fileno())
    if len(data) != after.st_size or (after.st_size, after.st_mtime_ns, after.st_ctime_ns) != (
        final.st_size, final.st_mtime_ns, final.st_ctime_ns
    ):
        raise ValueError("chain trust root changed during read")
    return data


def validate_root(data, chain_id):
    """Check TrustRoot::canonical_bytes framing; native consumers verify commits."""
    offset = 5
    if data[:5] != b"LXTR\x01":
        raise ValueError("expected a committed chain TrustRoot, not sequencer history")

    def take(size):
        nonlocal offset
        value = data[offset:offset + size]
        if len(value) != size:
            raise ValueError("truncated chain trust root")
        offset += size
        return value

    def field(size=None):
        length = int.from_bytes(take(4), "big")
        if length > MAX_ROOT_BYTES or (size is not None and length != size):
            raise ValueError("noncanonical chain trust root field")
        return take(length)

    actual_chain = field().decode("utf-8")
    height = int.from_bytes(field(8), "big")
    field(8)  # committed header time_ms
    field(32)  # committed header hash
    if not chain_id or actual_chain != chain_id or height == 0:
        raise ValueError("chain trust root chain or committed height differs")
    count = int.from_bytes(take(4), "big")
    if count == 0 or count > (len(data) - offset) // 72:
        raise ValueError("invalid chain validator count")
    previous = b""
    total = 0
    for _ in range(count):
        address, public = field(20), field(32)
        power = int.from_bytes(field(8), "big")
        if address <= previous or address != hashlib.sha256(public).digest()[:20]:
            raise ValueError("noncanonical chain validator address")
        previous = address
        total += power
    if offset != len(data) or not 0 < total <= MAX_TOTAL_POWER:
        raise ValueError("invalid chain validator power or trailing bytes")


def sync_directory(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def pin_trust_root(source, destination, chain_id, expected_sha256, uid=None, gid=None):
    """Publish exactly the configured checkpoint bytes in an immutable generation."""
    uid = os.geteuid() if uid is None else uid
    gid = os.getegid() if gid is None else gid
    source, destination = Path(source), Path(destination)
    if not source.is_absolute() or not destination.is_absolute():
        raise ValueError("chain trust paths must be absolute")
    if not re.fullmatch(r"[0-9a-f]{64}", expected_sha256):
        raise ValueError("an independently authenticated checkpoint SHA256 is required")
    data = read_private(source, os.geteuid())
    if hashlib.sha256(data).hexdigest() != expected_sha256:
        raise ValueError("chain trust root differs from deployment checkpoint pin")
    validate_root(data, chain_id)
    info = destination.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != uid or stat.S_IMODE(info.st_mode) != 0o700:
        raise ValueError("registry generation store must be a private owned directory")
    lock_fd = os.open(destination / ".lock", os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600)
    with os.fdopen(lock_fd, "rb") as lock:
        info = os.fstat(lock.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o077:
            raise ValueError("invalid registry generation lock")
        fcntl.flock(lock, fcntl.LOCK_EX)
        selected = destination / expected_sha256
        if os.path.lexists(selected):
            info = selected.lstat()
            if not stat.S_ISDIR(info.st_mode) or info.st_uid != uid or stat.S_IMODE(info.st_mode) != 0o700:
                raise ValueError("invalid retained chain generation")
            if {p.name for p in selected.iterdir()} != {"trust-root"} or read_private(selected / "trust-root", uid) != data:
                raise ValueError("retained chain generation differs")
        else:
            pending = Path(tempfile.mkdtemp(prefix=".pending-", dir=destination))
            try:
                with (pending / "trust-root").open("xb") as stream:
                    os.fchmod(stream.fileno(), 0o400)
                    os.fchown(stream.fileno(), uid, gid)
                    stream.write(data)
                    stream.flush()
                    os.fsync(stream.fileno())
                os.chown(pending, uid, gid)
                sync_directory(pending)
                os.rename(pending, selected)
                sync_directory(destination)
            finally:
                if pending.exists():
                    shutil.rmtree(pending)
    return {"directory": str(selected), "generation": expected_sha256}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source")
    parser.add_argument("destination")
    parser.add_argument("chain_id")
    parser.add_argument("sha256")
    parser.add_argument("--uid", type=int, required=True)
    parser.add_argument("--gid", type=int, required=True)
    args = parser.parse_args()
    try:
        result = pin_trust_root(args.source, args.destination, args.chain_id, args.sha256, args.uid, args.gid)
    except (OSError, ValueError) as error:
        parser.exit(1, f"registry chain trust refused: {error}\n")
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
