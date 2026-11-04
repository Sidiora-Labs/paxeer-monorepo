"""Private compiler provenance consumed before any prebuilt corpus is executed."""

import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess


SCHEMA = "paxeer-x.compiler-manifest.v1"
PLANS = {
    "104.17.4": ("services/platform/Cargo.toml", "layerx-platform-gateway",
                 {"layerx_platform_gateway": 1, "layerx_gateway": 1}),
    "104.24.1": ("services/interop/Cargo.toml", "layerx-migrate",
                 {"layerx_migrate": 2, "operator": 1, "funded_operator": 1,
                  "migration_v2_connected": 1, "testnets": 1}),
    "104.24.2": ("services/interop/Cargo.toml", "layerx-migrate",
                 {"migration_v2_connected": 1}),
}


def require(value, reason):
    if not value:
        raise ValueError(reason)


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def private_bytes(path, root, maximum=32 * 1024 * 1024):
    path = Path(path)
    require(path.is_absolute() and path.resolve() == path
            and path != root and root not in path.parents,
            "canonical evidence outside source required")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and not info.st_mode & 0o077 and info.st_nlink == 1
                and 0 < info.st_size <= maximum, "bounded private owner file required")
        data = stream.read(maximum + 1)
        require(len(data) <= maximum, "private evidence grew beyond bound")
        return data


def private_json(path, root):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate evidence key")
            result[key] = value
        return result
    return json.loads(private_bytes(path, root), object_pairs_hook=unique)


def write_private(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as stream:
        json.dump(value, stream, sort_keys=True)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())


def source_identity(root):
    def git(*args):
        return subprocess.check_output(["git", *args], cwd=root)
    require(not git("status", "--porcelain", "--untracked-files=all"),
            "clean published compiler source required")
    files = {}
    # Bind the entire tracked closure, including workspace/path dependencies,
    # build scripts, lockfiles, native headers, fixtures and gate consumers.
    for raw in git("ls-files", "--stage", "-z").split(b"\0"):
        if not raw:
            continue
        header, raw_name = raw.split(b"\t", 1)
        mode, object_id, stage = header.split()
        require(stage == b"0", "unmerged compiler source")
        name = os.fsdecode(raw_name)
        path = root / name
        if mode == b"160000":
            files[name] = {"gitlink": object_id.decode()}
        elif mode == b"120000":
            require(path.is_symlink(), "source link replaced")
            files[name] = {"symlink": os.readlink(path)}
        else:
            require(path.resolve() == path and path.is_file(),
                    "noncanonical compiler input: " + name)
            files[name] = digest(path)
    require(files, "empty compiler source closure")
    return {"revision": git("rev-parse", "HEAD").decode().strip(),
            "tree": git("rev-parse", "HEAD^{tree}").decode().strip(), "files": files}


def command_for(gate):
    manifest, package, _ = PLANS[gate]
    command = ["cargo", "test", "--locked", "--manifest-path", manifest,
               "-p", package, "--no-run", "--message-format=json"]
    if gate == "104.24.2":
        command += ["--test", "migration_v2_connected"]
    else:
        command += ["--lib", "--bins"] if gate == "104.17.4" else ["--tests"]
    return command


def executable_identity(raw):
    path = Path(raw)
    require(path.is_absolute() and path.resolve() == path and path.is_file()
            and os.access(path, os.X_OK), "canonical compiled executable required")
    with path.open("rb") as stream:
        require(stream.read(4) == b"\x7fELF", "actual native compiler executable required")
    return {"path": str(path), "sha256": digest(path)}


def compiler_outputs(data, gate):
    rows = {}
    finished = []
    expected = PLANS[gate][2]
    for line in data.decode().splitlines():
        event = json.loads(line)
        if event.get("reason") == "build-finished":
            finished.append(event.get("success"))
        if (event.get("reason") != "compiler-artifact"
                or not event.get("profile", {}).get("test") or not event.get("executable")):
            continue
        target = event["target"]["name"].replace("-", "_")
        if target not in expected:
            continue
        path = event["executable"]
        require(path not in rows, "duplicate compiler executable")
        rows[path] = {"target": target, **executable_identity(path)}
    require(finished == [True], "complete successful compiler transcript required")
    require(Counter(row["target"] for row in rows.values()) == Counter(expected),
            "complete exact compiler target inventory required")
    return rows


def verify_document(document, root, gate, executables, source):
    require(set(document) == {"schema", "gate", "source", "command", "exit_code",
                              "compiler_log", "executables"}, "closed compiler manifest required")
    require(document["schema"] == SCHEMA and document["gate"] == gate
            and type(document["exit_code"]) is int and document["exit_code"] == 0
            and document["source"] == source and document["command"] == command_for(gate),
            "compiler manifest is not bound to current source and command")
    log = document["compiler_log"]
    require(set(log) == {"path", "sha256"}, "closed compiler transcript identity required")
    data = private_bytes(log["path"], root)
    require(hashlib.sha256(data).hexdigest() == log["sha256"], "compiler transcript changed")
    rows = compiler_outputs(data, gate)
    require(document["executables"] == rows, "compiler executable digest changed")
    require(len(executables) == len(set(map(str, executables)))
            and set(map(str, executables)) == set(rows), "selected corpus differs from compiler outputs")
    return rows


def require_manifest(root, gate, executables):
    raw = os.environ.get("PAXEER_X_COMPILER_MANIFEST")
    require(raw, "private compiler-produced manifest required before corpus execution")
    document = private_json(raw, root)
    return verify_document(document, root, gate, executables, source_identity(root))


def build(root, gate, directory):
    require(directory.is_absolute() and directory.resolve() == directory
            and directory != root and root not in directory.parents, "private compiler evidence required")
    info = directory.stat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
            and not info.st_mode & 0o077, "owned private compiler evidence required")
    source = source_identity(root)
    command = command_for(gate)
    log = directory / (gate + "-compiler.jsonl")
    fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "wb") as stream:
        result = subprocess.run(command, cwd=root, stdin=subprocess.DEVNULL,
                                stdout=stream, stderr=subprocess.PIPE, timeout=1200)
    require(result.returncode == 0, "compiler failed; private transcript: " + str(log))
    rows = compiler_outputs(private_bytes(log, root), gate)
    require(source_identity(root) == source, "source changed during compilation")
    document = {"schema": SCHEMA, "gate": gate, "source": source, "command": command,
                "exit_code": 0, "compiler_log": {"path": str(log), "sha256": digest(log)},
                "executables": rows}
    path = directory / (gate + "-compiler-manifest.json")
    write_private(path, document)
    return path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("gate", choices=PLANS)
    parser.add_argument("--evidence", required=True, type=Path)
    arguments = parser.parse_args()
    root = Path(__file__).resolve().parents[3]
    print(build(root, arguments.gate, arguments.evidence))


if __name__ == "__main__":
    main()
