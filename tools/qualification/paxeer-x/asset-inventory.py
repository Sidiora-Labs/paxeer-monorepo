#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
TASK = "104.5.5"
SCHEMA = "layerx.asset-inventory.v1"
TEST = "real_native_effective_asset_transport"
DECLARED = (
    "services/agent/crates/layerx-client/tests/effective_asset.rs",
    "services/agent/crates/layerx-client/src/evidence/caps.rs",
    "services/agent/crates/layerx-client/src/evidence/asset_record.rs",
    "services/agent/crates/layerx-client/src/evidence.rs",
    "tools/qualification/paxeer-x/asset-inventory.py",
)


def digest(path):
    with Path(path).open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def evidence_directory():
    directory = Path(
        os.environ.get(
            "PAXEER_X_ASSET_INVENTORY_EVIDENCE",
            "/root/lx-ops/paxeer-x-integration-2026-10-03/qualification/task10455",
        )
    )
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    directory = directory.resolve(strict=True)
    info = directory.stat()
    if (
        ROOT == directory
        or ROOT in directory.parents
        or info.st_uid != os.geteuid()
        or info.st_mode & 0o077
    ):
        raise RuntimeError("private caller-owned evidence outside checkout required")
    return directory


def inputs():
    inventory = subprocess.run(
        [
            "git",
            "ls-files",
            "-z",
            "--",
            "Makefile",
            "src",
            "include",
            "node/sequencer",
            "modules/sequencer",
            "modules/lxphost",
            "custodyproof",
            "layerxproof",
            "rpc",
            "go.mod",
            "go.sum",
            "services/agent/Cargo.toml",
            "services/agent/Cargo.lock",
            "services/agent/crates/layerx-client",
            "services/agent/crates/layerx-proof",
            "services/agent/crates/layerx-types",
            "services/agent/crates/layerx-crypto",
            "programs/Cargo.toml",
            "programs/Cargo.lock",
            "programs/crates",
            "programs/sdk/rust",
            "tests/programs/test_call_activity.c",
            "tests/bridge/files.h",
            "contracts/config",
            "tools/paxeer-x/build",
            ".cargo",
            "rust-toolchain.toml",
        ],
        cwd=ROOT,
        check=True,
        capture_output=True,
    ).stdout.split(b"\0")
    paths = {os.fsdecode(path) for path in inventory if path} | set(DECLARED)
    return {
        path: digest(ROOT / path)
        for path in sorted(paths)
        if Path(path).suffix
        in {
            ".c",
            ".h",
            ".rs",
            ".go",
            ".mod",
            ".sum",
            ".toml",
            ".lock",
            ".kvx",
            ".json",
            ".mk",
            ".py",
        }
        or path == "Makefile"
    }


def save(path, value):
    with path.open("w") as output:
        json.dump(value, output, indent=2, sort_keys=True)
        output.write("\n")
    path.chmod(0o600)


def artifact(path):
    path = Path(path).resolve(strict=True)
    return {"path": str(path), "sha256": digest(path)}


def checked(value):
    path = Path(value["path"])
    if not path.is_file() or digest(path) != value["sha256"]:
        raise RuntimeError("missing or changed declared artifact")
    return path


def build(arguments, evidence):
    manifest = evidence / "artifacts.json"
    manifest.unlink(missing_ok=True)
    source = inputs()
    command = [
        *shlex.split(arguments.cargo),
        "test",
        "--locked",
        "--manifest-path",
        "services/agent/Cargo.toml",
        "-p",
        "layerx-client",
        "--test",
        "effective_asset",
        "--no-run",
        "--message-format=json",
    ]
    environment = dict(
        os.environ, CARGO_TARGET_DIR="/root/lx-target/agent", CARGO_BUILD_JOBS="4"
    )
    log = evidence / "build-client.log"
    with log.open("w") as output:
        result = subprocess.run(
            command,
            cwd=ROOT,
            env=environment,
            stdin=subprocess.DEVNULL,
            stdout=output,
            stderr=subprocess.STDOUT,
            timeout=1100,
        )
    if result.returncode:
        raise subprocess.CalledProcessError(result.returncode, command)
    executables = set()
    for line in log.read_text().splitlines():
        if not line.startswith("{"):
            continue
        value = json.loads(line)
        if (
            value.get("reason") == "compiler-artifact"
            and value.get("executable")
            and value.get("target", {}).get("name") == "effective_asset"
        ):
            executables.add(value["executable"])
    if len(executables) != 1 or inputs() != source:
        raise RuntimeError(
            "one declared client artifact and unchanged build source required"
        )
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    save(
        manifest,
        {
            "schema": SCHEMA,
            "task": TASK,
            "revision": revision,
            "inputs": source,
            "rust": artifact(executables.pop()),
        },
    )
    print("ARTIFACTS " + str(manifest), flush=True)


def qualify(evidence):
    manifest = json.loads((evidence / "artifacts.json").read_text())
    if (
        manifest.get("schema") != SCHEMA
        or manifest.get("task") != TASK
        or manifest.get("inputs") != inputs()
    ):
        raise RuntimeError("declared artifacts do not bind current source")
    rust = checked(manifest["rust"])
    command = [str(rust), TEST, "--exact", "--nocapture", "--test-threads=1"]
    log = evidence / "verify-client.log"
    with log.open("w") as output:
        result = subprocess.run(
            command,
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            stdout=output,
            stderr=subprocess.STDOUT,
            timeout=120,
        )
    if result.returncode:
        raise subprocess.CalledProcessError(result.returncode, command)
    if (
        "test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out"
        not in log.read_text()
    ):
        raise RuntimeError("genuine client assertions skipped or incomplete")
    if inputs() != manifest["inputs"]:
        raise RuntimeError("source changed during verification")
    save(
        evidence / "result.json",
        {
            "task": TASK,
            "revision": manifest["revision"],
            "command": "timeout 10m python3 tools/qualification/paxeer-x/asset-inventory.py",
            "exit": 0,
            "rust_log": str(log),
        },
    )
    print("VERIFIED " + str(evidence / "result.json"), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--build", action="store_true")
    parser.add_argument("--cargo", default="cargo")
    arguments = parser.parse_args()
    evidence = evidence_directory()
    if arguments.build:
        build(arguments, evidence)
    else:
        qualify(evidence)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print("FAILED " + str(error), file=sys.stderr)
        sys.exit(
            error.returncode if isinstance(error, subprocess.CalledProcessError) else 1
        )
