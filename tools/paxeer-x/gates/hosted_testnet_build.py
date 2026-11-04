"""Build and verify source-bound artifacts for the hosted testnet gate.

Producer: python3 tools/paxeer-x/gates/hosted_testnet_build.py PRIVATE_DIRECTORY
Consumer: LAYERX_TESTNET_BUILD_MANIFEST=.../build-manifest.json gate 104.17.3
"""

import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys

SCHEMA = "layerx.hosted-testnet-build.v1"
TARGETS = {
    "LAYERX_TESTNET_LIB_TEST_BINARY": (
        "services/platform/hosted/testnet/Cargo.toml",
        "layerx_platform_testnet",
        "lib",
    ),
    "LAYERX_TESTNET_CONTROL_TEST_BINARY": (
        "services/platform/hosted/testnet/Cargo.toml",
        "layerx-testnet-control",
        "bin",
    ),
    "LAYERX_FAUCET_TEST_BINARY": (
        "services/platform/hosted/faucet/Cargo.toml",
        "layerx-faucet",
        "bin",
    ),
}


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def private(path, directory=False):
    path = Path(path)
    info = path.lstat()
    require(
        path.is_absolute()
        and path.resolve(strict=True) == path
        and info.st_uid == os.geteuid()
        and (directory or info.st_nlink == 1),
        "caller-owned canonical evidence path required",
    )
    require(
        not info.st_mode & 0o077
        and (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)),
        "private regular evidence required",
    )


def duplicate_free(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, "duplicate manifest field")
        value[key] = item
    return value


def load(path):
    private(path.parent, True)
    private(path)
    require(path.stat().st_size <= 64 * 1024 * 1024, "bounded manifest required")
    return json.loads(path.read_text(), object_pairs_hook=duplicate_free)


def save(path, value):
    private(path.parent, True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())


def source_hashes(root, names):
    hashes = {}
    for name in sorted(set(names)):
        relative = Path(name)
        require(
            not relative.is_absolute() and ".." not in relative.parts,
            "repository-relative source required",
        )
        if any(part == ".env" or part.startswith(".env.") for part in relative.parts):
            continue
        path = root / relative
        if path.is_symlink():
            require(path.resolve().is_relative_to(root), "source link escapes checkout")
            hashes[name] = {"link": os.readlink(path), "sha256": digest(path)}
        else:
            require(path.is_file(), "source closure contains unavailable input: " + name)
            hashes[name] = digest(path)
    require(hashes, "complete source closure required")
    return hashes


def git(root, *args):
    return subprocess.check_output(["git", *args], cwd=root, timeout=60)


def source_identity(root):
    # Every tracked and nonignored input, including extensionless build files,
    # fixtures, lockfiles, workspace manifests and this producer/consumer code.
    names = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    return {
        "revision": git(root, "rev-parse", "HEAD").decode().strip(),
        "tree": git(root, "rev-parse", "HEAD^{tree}").decode().strip(),
        "hashes": source_hashes(root, [os.fsdecode(n) for n in names.split(b"\0") if n]),
    }


def compiler_artifacts(root, rows):
    selected = {}
    for row in rows:
        if row.get("reason") != "compiler-artifact" or row.get("profile", {}).get("test") is not True:
            continue
        for name, (manifest, target, kind) in TARGETS.items():
            if (
                row.get("manifest_path") == str(root / manifest)
                and row.get("target", {}).get("name") == target
                and row.get("target", {}).get("kind") == [kind]
                and row.get("executable")
            ):
                require(name not in selected, "duplicate compiler test target")
                selected[name] = row
    require(set(selected) == set(TARGETS), "complete genuine compiler test artifacts required")
    return selected


def artifact(path):
    path = Path(path)
    require(
        path.is_absolute() and not path.is_symlink() and path.is_file()
        and os.access(path, os.X_OK),
        "actual executable compiler artifact required",
    )
    return {"path": str(path), "sha256": digest(path)}


def checked_artifact(row, compiler_path):
    require(artifact(compiler_path) == row, "compiled executable digest changed")
    return Path(row["path"])


def build(root, directory):
    require(directory.is_absolute() and not directory.is_relative_to(root), "private evidence outside checkout required")
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    private(directory, True)
    require(not (directory / "build-manifest.json").exists(), "build already recorded")
    before = source_identity(root)
    argv = [
        "cargo", "test", "--locked", "--manifest-path", "services/platform/Cargo.toml",
        "-p", "layerx-platform-testnet", "-p", "layerx-platform-faucet",
        "--lib", "--bins", "--no-run", "--message-format=json",
    ]
    log = directory / "compiler.jsonl"
    fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as stream:
        result = subprocess.run(
            argv, cwd=root,
            env=dict(os.environ, CARGO_TARGET_DIR=str(directory / "target")),
            stdin=subprocess.DEVNULL, stdout=stream, stderr=subprocess.STDOUT,
            timeout=1800,
        )
    require(result.returncode == 0, "hosted compiler failed; see " + str(log))
    rows = [json.loads(line) for line in log.read_text().splitlines() if line.startswith("{")]
    selected = compiler_artifacts(root, rows)
    binaries = {name: artifact(row["executable"]) for name, row in selected.items()}
    require(source_identity(root) == before, "source changed during compilation")
    save(directory / "build-manifest.json", {
        "schema": SCHEMA, "task": "104.17.3", "evidence_dimension": "private-profile",
        "identity": before, "build_exit_code": 0, "command": argv,
        "compiler_log": {"path": str(log), "sha256": digest(log)},
        "compiler_artifacts": selected, "binaries": binaries,
    })
    return directory / "build-manifest.json"


def verify_artifacts(root, filename):
    require(filename.is_absolute() and not filename.resolve().is_relative_to(root), "private build manifest outside checkout required")
    built = load(filename)
    require(
        built["schema"] == SCHEMA and built["task"] == "104.17.3"
        and built["evidence_dimension"] == "private-profile"
        and type(built["build_exit_code"]) is int and built["build_exit_code"] == 0,
        "successful private compiler-produced manifest required",
    )
    require(built["identity"] == source_identity(root), "compiled revision/tree/source closure changed")
    log = Path(built["compiler_log"]["path"])
    private(log)
    require(digest(log) == built["compiler_log"]["sha256"], "compiler transcript changed")
    rows = [json.loads(line) for line in log.read_text().splitlines() if line.startswith("{")]
    selected = compiler_artifacts(root, rows)
    require(selected == built["compiler_artifacts"], "compiler transcript binding changed")
    require(set(built["binaries"]) == set(TARGETS), "complete binary roster required")
    for name, row in selected.items():
        checked_artifact(built["binaries"][name], row["executable"])
    return built


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: hosted_testnet_build.py PRIVATE_DIRECTORY")
    print(build(Path(__file__).resolve().parents[3], Path(sys.argv[1])))
