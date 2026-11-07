#!/usr/bin/env python3
"""Checked publication and restart receipts for the periodic snapshot runner."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile
import tomllib

MAX_HEIGHT = 2**63 - 1


def require(condition, message):
    if not condition:
        raise ValueError(message)


def positive(value):
    require(re.fullmatch(r"[1-9][0-9]*", str(value)) is not None, "expected a positive height")
    result = int(value)
    require(result < MAX_HEIGHT, "height exceeds the native range")
    return result


def next_height(height, interval):
    return positive(positive(height) + positive(interval))


def sync_directory(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def atomic_write(path, data):
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix=".snapshot-write-")
    try:
        with os.fdopen(fd, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        sync_directory(path.parent)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def committed(path, height, chain):
    value = json.loads(Path(path).read_text())
    require(value["chain_id"] == chain, "export chain differs from the requested chain")
    require(positive(value["initial_height"]) == positive(height) + 1, "latest committed height differs from target")
    require(isinstance(value["app_state"], dict) and value["app_state"], "missing application state")
    return value


def configure(path, height):
    path = Path(path)
    before = path.read_text()
    parsed = tomllib.loads(before)
    wanted = {
        "pruning": "custom", "pruning-keep-recent": "1", "pruning-keep-every": "0",
        "pruning-interval": "1", "halt-height": positive(height),
    }
    after = before
    for key, value in wanted.items():
        require(key in parsed, f"missing configuration key: {key}")
        after, count = re.subn(
            r"(?m)^" + re.escape(key) + r"\s*=.*$", key + " = " + json.dumps(value), after,
        )
        require(count == 1, f"ambiguous configuration key: {key}")
    expected = dict(parsed, **wanted)
    require(tomllib.loads(after) == expected, "configuration edit changed unrelated settings")
    atomic_write(path, after.encode())


def file_hashes(root):
    result = {}
    require(root.is_dir() and not root.is_symlink(), "snapshot root must be a real directory")
    for path in sorted(root.rglob("*")):
        mode = path.lstat().st_mode
        require(stat.S_ISDIR(mode) or stat.S_ISREG(mode), "snapshot contains a special file")
        if stat.S_ISREG(mode):
            with path.open("rb") as stream:
                result[str(path.relative_to(root))] = hashlib.file_digest(stream, "sha256").hexdigest()
    return result


def verify_snapshot(root, height):
    hashes = file_hashes(root)
    formats = list((root / "snapshot" / str(positive(height))).iterdir())
    require(len(formats) == 1 and formats[0].name.isdecimal(), "missing or ambiguous snapshot format")
    chunks = list(formats[0].iterdir())
    require(chunks and all(p.name.isdecimal() for p in chunks), "missing snapshot chunks")
    require(sorted(int(p.name) for p in chunks) == list(range(len(chunks))), "snapshot chunks have gaps")
    require(all(p.is_file() and p.stat().st_size > 0 for p in chunks), "empty snapshot chunk")
    metadata = root / "snapshot" / "metadata.db"
    require(metadata.is_dir() and any(p.is_file() and p.stat().st_size for p in metadata.iterdir()), "missing snapshot metadata")
    hashes.pop("receipt.json", None)
    return hashes


def receipt(root, initial, interval, chain):
    value = json.loads((root / "receipt.json").read_text())
    require(value["chain_id"] == chain and value["initial_height"] == positive(initial), "snapshot receipt belongs to another run")
    require(value["interval"] == positive(interval), "snapshot interval differs from receipt")
    require(value["next_height"] == next_height(value["height"], interval), "invalid next snapshot height")
    require(value["height"] >= positive(initial) and (value["height"] - positive(initial)) % positive(interval) == 0, "invalid snapshot sequence")
    require(value["files"] == verify_snapshot(root, value["height"]), "published snapshot failed integrity verification")
    committed(root / "committed.json", value["height"], chain)
    return value


def resume(directory, initial, interval, chain):
    directory = Path(directory)
    positive(initial)
    next_height(initial, interval)
    # Published directories are the journal: a rename is the single commit point.
    published = [p for p in directory.iterdir() if p.name.isdecimal()]
    if not published:
        return positive(initial)
    published.sort(key=lambda path: int(path.name))
    require([p.name for p in published] == [str(positive(initial) + i * positive(interval)) for i in range(len(published))], "snapshot journal has gaps")
    # Rehash the latest committed publication; older snapshots need no reread on
    # every cycle, which would grow quadratically with retained history.
    latest = receipt(published[-1], initial, interval, chain)
    require(latest["height"] == int(published[-1].name), "snapshot receipt height differs from directory")
    return latest["next_height"]


def publish(staging, directory, height, interval, chain):
    staging, directory = Path(staging), Path(directory)
    require(staging.parent.resolve() == directory.resolve(), "staging must be inside the snapshot directory")
    require(not (directory / str(positive(height))).exists(), "snapshot is already published")
    # A clean child exit and successful native Create are enforced by the shell.
    committed(staging / "committed.json", height, chain)
    hashes = verify_snapshot(staging, height)
    hashes.pop("receipt.json", None)
    initial = positive(height)
    existing = sorted((p for p in directory.iterdir() if p.name.isdecimal()), key=lambda p: int(p.name))
    if existing:
        old = json.loads((existing[0] / "receipt.json").read_text())
        initial = old["initial_height"]
        require(resume(directory, initial, interval, chain) == positive(height), "snapshot is out of sequence")
    value = {
        "chain_id": chain, "height": positive(height), "initial_height": initial,
        "interval": positive(interval), "next_height": next_height(height, interval), "files": hashes,
    }
    atomic_write(staging / "receipt.json", json.dumps(value, sort_keys=True).encode())
    # Verify against the exact file set, excluding the self-referential receipt.
    for path in staging.rglob("*"):
        if path.is_file():
            with path.open("rb") as stream:
                os.fsync(stream.fileno())
    for path in sorted((p for p in staging.rglob("*") if p.is_dir()), reverse=True):
        sync_directory(path)
    sync_directory(staging)
    os.rename(staging, directory / str(positive(height)))
    sync_directory(directory)
    return value["next_height"]


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("resume", "configure", "committed", "publish"))
    parser.add_argument("values", nargs="+")
    args = parser.parse_args(argv)
    result = globals()[args.operation](*args.values)
    if args.operation in ("resume", "publish"):
        print(result)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, KeyError, TypeError) as error:
        sys.exit(f"snapshot: {error}")
