"""Create and reopen a complete, quiesced recovery image for backup.sh.

The lifecycle hooks own stopping/flushing the actual node and kernel and checking
that their restored committed states agree. This module rejects incomplete byte
captures and verifies every restored member, including required key material.
"""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import stat
import tarfile


MANIFEST = ".paxeer-backup.json"


def archive_name(path):
    path = Path(path)
    if not path.is_absolute() or ".." in path.parts or path == Path("/"):
        raise ValueError(f"source must be a non-root absolute path: {path}")
    if path.resolve() != path:
        raise ValueError(f"source must not traverse symlinks: {path}")
    name = path.as_posix().lstrip("/")
    if name == MANIFEST or name.startswith(MANIFEST + "/"):
        raise ValueError("source collides with recovery manifest")
    return name


def file_fact(path):
    info = path.lstat()
    if stat.S_ISDIR(info.st_mode):
        return {
            "kind": "directory", "mode": stat.S_IMODE(info.st_mode),
            "uid": info.st_uid, "gid": info.st_gid,
        }
    if not stat.S_ISREG(info.st_mode):
        raise ValueError(f"source is not a regular file or directory: {path}")
    before = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    info = path.lstat()
    after = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    if before != after:
        raise ValueError(f"source changed during capture: {path}")
    return {
        "kind": "file",
        "mode": stat.S_IMODE(info.st_mode),
        "uid": info.st_uid,
        "gid": info.st_gid,
        "size": info.st_size,
        "sha256": digest.hexdigest(),
    }


def inventory(sources):
    entries = {}
    for source in sources:
        source = Path(source)
        archive_name(source)
        pending = [source]
        while pending:
            path = pending.pop()
            name = archive_name(path)
            fact = file_fact(path)
            entries[name] = fact
            if fact["kind"] == "directory":
                pending.extend(sorted(path.iterdir(), reverse=True))
    return dict(sorted(entries.items()))


def capture(archive, manifest_path, sources, node_home, kernel_dir, required_keys, dump=None):
    """Called only while every source writer is quiesced; never accept a partial image."""
    archive, manifest_path = Path(archive), Path(manifest_path)
    sources = [Path(source) for source in sources]
    roots = [Path(node_home), Path(kernel_dir)]
    for root in roots:
        archive_name(root)
        if not root.is_dir() or not any(root.is_relative_to(src) for src in sources):
            raise ValueError(f"complete state root must be captured: {root}")
    keys = [
        Path(node_home) / "config/priv_validator_key.json",
        Path(node_home) / "config/node_key.json",
        Path(node_home) / "data/priv_validator_state.json",
        *map(Path, required_keys),
    ]
    for output in (archive, manifest_path):
        if any(output.resolve().is_relative_to(src) for src in sources):
            raise ValueError("capture output must be outside the sources")
    entries = inventory(sources)
    for key in keys:
        name = archive_name(key)
        fact = entries.get(name, {})
        if fact.get("kind") != "file" or fact["size"] == 0:
            raise ValueError(f"required key or signing state missing/empty: {key}")
    # Key JSON must carry actual private material; semantic recovery belongs to
    # the mandatory offline validator for the owning node/backend.
    for key in keys[:2]:
        value = json.loads(key.read_text())
        if not isinstance(value.get("priv_key"), dict) or not value["priv_key"].get("value"):
            raise ValueError(f"private key material missing: {key}")
    manifest = {
        "version": 1,
        "roots": [archive_name(root) for root in roots],
        "required_keys": sorted({archive_name(key) for key in keys}),
        "entries": entries,
    }
    partial = archive.with_name(archive.name + ".part")
    try:
        with partial.open("xb") as output, tarfile.open(
            fileobj=output, mode="w:gz", dereference=False
        ) as image:
            partial.chmod(0o600)
            for name in entries:
                image.add(Path("/") / name, arcname=name, recursive=False)
            if dump is not None:
                dump = Path(dump)
                fact = file_fact(dump)
                if fact["kind"] != "file" or fact["size"] == 0:
                    raise ValueError("database dump is empty or not a regular file")
                if "db" in entries or any(name.startswith("db/") for name in entries):
                    raise ValueError("database dump collides with a source")
                image.add(dump, arcname="db/wallet-db.dump", recursive=False)
                manifest["entries"] = {**entries, "db/wallet-db.dump": fact}
            # Rewalk as well as rehash: additions/deletions and changes to files
            # already read invalidate the entire candidate.
            if inventory(sources) != entries:
                raise ValueError("sources changed during capture")
            encoded = json.dumps(manifest, sort_keys=True).encode()
            member = tarfile.TarInfo(MANIFEST)
            member.size, member.mode = len(encoded), 0o600
            image.addfile(member, io.BytesIO(encoded))
        # Reopen the producer's own image before exposing it for encryption.
        with tarfile.open(partial, "r:gz") as image:
            validate_image(image, manifest)
        manifest_path.write_bytes(encoded)
        manifest_path.chmod(0o600)
        os.replace(partial, archive)
    finally:
        partial.unlink(missing_ok=True)
    return manifest


def validate_image(image, expected=None):
    members = image.getmembers()
    names = [member.name for member in members]
    if len(names) != len(set(names)) or names.count(MANIFEST) != 1:
        raise ValueError("duplicate members or missing recovery manifest")
    for member in members:
        path = PurePosixPath(member.name)
        if (
            path.is_absolute()
            or ".." in path.parts
            or str(path) != member.name
            or not (member.isfile() or member.isdir())
        ):
            raise ValueError(f"unsafe archive member: {member.name}")
    manifest = json.load(image.extractfile(MANIFEST))
    if manifest.get("version") != 1 or (expected is not None and manifest != expected):
        raise ValueError("recovery manifest mismatch")
    entries = manifest["entries"]
    if set(names) != {*entries, MANIFEST}:
        raise ValueError("incomplete archive membership")
    for name in manifest["roots"]:
        if entries.get(name, {}).get("kind") != "directory":
            raise ValueError("missing state root")
    for name in manifest["required_keys"]:
        fact = entries.get(name, {})
        if fact.get("kind") != "file" or fact.get("size", 0) == 0:
            raise ValueError("missing required key or signing state")
    for member in members:
        if member.name == MANIFEST:
            continue
        fact = entries[member.name]
        if (
            member.mode != fact["mode"]
            or member.uid != fact["uid"]
            or member.gid != fact["gid"]
            or member.isdir() != (fact["kind"] == "directory")
        ):
            raise ValueError(f"member type/mode mismatch: {member.name}")
        if member.isfile():
            digest = hashlib.sha256()
            with image.extractfile(member) as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(block)
            if member.size != fact["size"] or digest.hexdigest() != fact["sha256"]:
                raise ValueError(f"member content mismatch: {member.name}")
    return manifest


def restore(archive, destination, expected=None):
    """Validate before extracting into a new, private directory; never touch live state."""
    destination = Path(destination)
    if destination.exists():
        raise ValueError("restore destination must not exist")
    with tarfile.open(archive, "r:gz") as image:
        manifest = validate_image(image, expected)
        destination.mkdir(mode=0o700)
        directories = []
        for member in image.getmembers():
            if member.name == MANIFEST:
                continue
            target = destination / member.name
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            if member.isdir():
                target.mkdir(exist_ok=True, mode=0o700)
                directories.append((target, member))
            else:
                with image.extractfile(member) as source, target.open("xb") as output:
                    for block in iter(lambda: source.read(1024 * 1024), b""):
                        output.write(block)
                os.chown(target, member.uid, member.gid)
                target.chmod(member.mode)
        # Check extracted bytes from disk, including the required private keys.
        for name, fact in manifest["entries"].items():
            target = destination / name
            if fact["kind"] == "file" and file_fact(target) != fact:
                raise ValueError(f"restored content mismatch: {name}")
        for target, member in reversed(directories):
            os.chown(target, member.uid, member.gid)
            target.chmod(member.mode)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    create = commands.add_parser("capture")
    create.add_argument("archive")
    create.add_argument("manifest")
    create.add_argument("--node-home", required=True)
    create.add_argument("--kernel-dir", required=True)
    create.add_argument("--required-key", action="append", default=[])
    create.add_argument("--dump")
    create.add_argument("--source", action="append", required=True)
    reopen = commands.add_parser("restore")
    reopen.add_argument("archive")
    reopen.add_argument("destination")
    reopen.add_argument("--expected-manifest")
    args = parser.parse_args()
    if args.command == "capture":
        capture(
            args.archive, args.manifest, args.source, args.node_home,
            args.kernel_dir, args.required_key, args.dump,
        )
    else:
        expected = None
        if args.expected_manifest:
            expected = json.loads(Path(args.expected_manifest).read_text())
        restore(args.archive, args.destination, expected)


if __name__ == "__main__":
    main()
