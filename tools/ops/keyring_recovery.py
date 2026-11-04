"""Validate enumeration and restore only the source keyring from a backup image.

Backend private-key recovery remains in keyring-migrate.sh, using paxd itself.
This module owns the byte comparison and safe, private extraction boundary.
"""

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import stat
import sys
import tarfile


def enumerate_keys(value):
    if not isinstance(value, list):
        raise ValueError("key enumeration must be a JSON list")
    found = {}
    for key in value:
        if not isinstance(key, dict):
            raise ValueError("invalid key record")
        name, address = key.get("name"), key.get("address")
        for field in (name, address):
            if (
                not isinstance(field, str)
                or not field
                or any(char.isspace() or ord(char) < 32 or ord(char) == 127 for char in field)
            ):
                raise ValueError("key name/address is missing or unsafe")
        if name.startswith("-") or name in found:
            raise ValueError("ambiguous key name")
        found[name] = address
    return "\n".join(f"{name} {found[name]}" for name in sorted(found))


def content_fact(path):
    before = path.lstat()
    if stat.S_ISDIR(before.st_mode):
        return {"kind": "directory"}
    if not stat.S_ISREG(before.st_mode):
        raise ValueError(f"keyring contains a link or special file: {path}")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    after = path.lstat()
    identity = lambda info: (
        info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns
    )
    if identity(before) != identity(after):
        raise ValueError("keyring changed while being read")
    return {"kind": "file", "size": before.st_size, "sha256": digest.hexdigest()}


def snapshot(directory):
    directory = Path(directory)
    if directory.is_symlink() or not directory.is_dir():
        raise ValueError("source keyring must be a directory, not a link")
    entries = {".": content_fact(directory)}
    pending = sorted(directory.iterdir())
    while pending:
        path = pending.pop()
        fact = content_fact(path)
        entries[path.relative_to(directory).as_posix()] = fact
        if fact["kind"] == "directory":
            pending.extend(sorted(path.iterdir()))
    return dict(sorted(entries.items()))


def check_source(directory, expected):
    if snapshot(directory) != expected:
        raise ValueError("source keyring contents changed")


def restore_keyring(archive, source_home, destination, expected):
    """Compare archived bytes to the original snapshot before extracting anything."""
    source_home, destination = Path(source_home), Path(destination)
    if not source_home.is_absolute() or source_home == Path("/") or ".." in source_home.parts:
        raise ValueError("source home must be an absolute non-root path")
    if destination.exists() or destination.is_symlink():
        raise ValueError("recovery home must not exist")
    prefix = (source_home / "keyring-test").as_posix().lstrip("/")
    selected = {}
    with tarfile.open(archive, "r:gz") as image:
        seen = set()
        for member in image.getmembers():
            path = PurePosixPath(member.name)
            if (
                member.name in seen
                or path.is_absolute()
                or ".." in path.parts
                or path.as_posix() != member.name
                or not (member.isdir() or member.isfile())
            ):
                raise ValueError("unsafe or duplicate archive member")
            seen.add(member.name)
            if member.name != prefix and not member.name.startswith(prefix + "/"):
                continue
            relative = member.name[len(prefix):].lstrip("/") or "."
            fact = {"kind": "directory"}
            if member.isfile():
                digest = hashlib.sha256()
                with image.extractfile(member) as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b""):
                        digest.update(block)
                fact = {"kind": "file", "size": member.size, "sha256": digest.hexdigest()}
            if expected.get(relative) != fact:
                raise ValueError("backup keyring content mismatch")
            selected[relative] = member
        if set(selected) != set(expected) or expected.get(".") != {"kind": "directory"}:
            raise ValueError("backup keyring membership mismatch")
        destination.mkdir(mode=0o700)
        keyring = destination / "keyring-test"
        keyring.mkdir(mode=0o700)
        for relative, member in sorted(selected.items()):
            if relative == ".":
                continue
            target = keyring / relative
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            if member.isdir():
                target.mkdir(exist_ok=True, mode=0o700)
            else:
                with image.extractfile(member) as stream, target.open("xb") as output:
                    target.chmod(0o600)
                    for block in iter(lambda: stream.read(1024 * 1024), b""):
                        output.write(block)
    check_source(keyring, expected)
    return keyring


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("keys")
    capture = commands.add_parser("snapshot")
    capture.add_argument("directory")
    check = commands.add_parser("check")
    check.add_argument("directory")
    check.add_argument("manifest")
    restore = commands.add_parser("restore")
    restore.add_argument("archive")
    restore.add_argument("source_home")
    restore.add_argument("destination")
    restore.add_argument("manifest")
    args = parser.parse_args()
    if args.command == "keys":
        print(enumerate_keys(json.load(sys.stdin)))
    elif args.command == "snapshot":
        print(json.dumps(snapshot(args.directory), sort_keys=True))
    else:
        expected = json.loads(Path(args.manifest).read_text())
        if args.command == "check":
            check_source(args.directory, expected)
        else:
            restore_keyring(args.archive, args.source_home, args.destination, expected)


if __name__ == "__main__":
    main()
