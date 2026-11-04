"""Compile market acceptance binaries and the exact guest consumed by runtime tests."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

_spec = importlib.util.spec_from_file_location(
    "market_build_evidence", Path(__file__).with_name("hosted_testnet_build.py")
)
evidence = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(evidence)

SCHEMA = "layerx.market-build.v1"
TARGETS = {
    "LAYERX_MARKET_TEST_BINARY": (
        "programs/crates/layerx-programs-market/Cargo.toml", "layerx_programs_market"
    ),
    "LAYERX_MARKET_RUNTIME_TEST_BINARY": (
        "programs/crates/layerx-programs-runtime/Cargo.toml", "layerx_programs_runtime"
    ),
}
TEST_COMMAND = [
    "cargo", "test", "--locked", "--manifest-path", "programs/Cargo.toml",
    "-p", "layerx-programs-market", "-p", "layerx-programs-runtime", "--lib",
    "--no-run", "--message-format=json",
]
GUEST_COMMAND = [
    "cargo", "build", "--locked", "--manifest-path", "programs/Cargo.toml",
    "-p", "layerx-programs-market", "--release", "--target", "wasm32-unknown-unknown",
    "--message-format=json",
]


def compiler_records(root, test_rows, guest_rows):
    selected = {}
    for name, (manifest, target) in TARGETS.items():
        rows = [row for row in test_rows
                if row.get("reason") == "compiler-artifact"
                and row.get("manifest_path") == str(root / manifest)
                and row.get("target", {}).get("name") == target
                and row.get("profile", {}).get("test") is True
                and row.get("executable")]
        evidence.require(len(rows) == 1, "exact compiler market test target required: " + name)
        selected[name] = rows[0]
    guests = [row for row in guest_rows
              if row.get("reason") == "compiler-artifact"
              and row.get("manifest_path") == str(root / TARGETS["LAYERX_MARKET_TEST_BINARY"][0])
              and row.get("target", {}).get("name") == "layerx_programs_market"
              and row.get("profile", {}).get("test") is False
              and any(path.endswith("/layerx_programs_market.wasm") for path in row.get("filenames", []))]
    evidence.require(len(guests) == 1, "exact compiler-produced market guest required")
    selected["LAYERX_MARKET_WASM"] = guests[0]
    return selected


def rows(path):
    return [json.loads(line) for line in path.read_text().splitlines() if line.startswith("{")]


def guest_artifact(record):
    paths = [Path(path) for path in record["filenames"] if path.endswith("/layerx_programs_market.wasm")]
    evidence.require(len(paths) == 1, "one compiler guest filename required")
    path = paths[0]
    evidence.require(path.is_absolute() and not path.is_symlink() and path.is_file(),
                     "actual regular guest artifact required")
    with path.open("rb") as stream:
        evidence.require(stream.read(8) == b"\0asm\x01\0\0\0", "compiled WebAssembly guest required")
    return {"path": str(path), "sha256": evidence.digest(path)}


def build(root, directory):
    evidence.require(directory.is_absolute() and not directory.resolve().is_relative_to(root),
                     "private compiler evidence outside checkout required")
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    evidence.private(directory, True)
    before = evidence.source_identity(root)
    logs = {}
    for name, command in (("guest", GUEST_COMMAND), ("tests", TEST_COMMAND)):
        log = directory / (name + ".jsonl")
        fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w") as stream:
            result = subprocess.run(command, cwd=root, stdin=subprocess.DEVNULL,
                                    env=dict(os.environ, CARGO_TARGET_DIR=str(directory / "target")),
                                    stdout=stream, stderr=subprocess.STDOUT, timeout=1800)
        evidence.require(result.returncode == 0, "market compiler failed; see " + str(log))
        logs[name] = {"path": str(log), "sha256": evidence.digest(log),
                      "command": command, "exit": result.returncode}
    selected = compiler_records(root, rows(Path(logs["tests"]["path"])), rows(Path(logs["guest"]["path"])))
    artifacts = {name: evidence.artifact(selected[name]["executable"]) for name in TARGETS}
    artifacts["LAYERX_MARKET_WASM"] = guest_artifact(selected["LAYERX_MARKET_WASM"])
    evidence.require(evidence.source_identity(root) == before, "source changed during market compilation")
    manifest = directory / "market-build.json"
    evidence.save(manifest, {"schema": SCHEMA, "identity": before, "logs": logs,
                             "compiler_artifacts": selected, "artifacts": artifacts})
    return manifest


def verify(root, manifest):
    evidence.require(manifest.is_absolute() and not manifest.resolve().is_relative_to(root),
                     "private compiler manifest outside checkout required")
    built = evidence.load(manifest)
    evidence.require(built["schema"] == SCHEMA and built["identity"] == evidence.source_identity(root),
                     "current revision/tree/source-bound compiler manifest required")
    evidence.require(set(built["logs"]) == {"guest", "tests"}, "both compiler transcripts required")
    transcripts = {}
    for name, command in (("guest", GUEST_COMMAND), ("tests", TEST_COMMAND)):
        log = built["logs"][name]
        evidence.private(Path(log["path"]))
        evidence.require(type(log["exit"]) is int and log["exit"] == 0 and log["command"] == command
                         and evidence.digest(log["path"]) == log["sha256"],
                         "successful exact compiler transcript required")
        transcripts[name] = rows(Path(log["path"]))
    selected = compiler_records(root, transcripts["tests"], transcripts["guest"])
    evidence.require(selected == built["compiler_artifacts"], "compiler record binding changed")
    evidence.require(set(built["artifacts"]) == {*TARGETS, "LAYERX_MARKET_WASM"},
                     "both binaries and their guest required")
    for name in TARGETS:
        evidence.checked_artifact(built["artifacts"][name], selected[name]["executable"])
    evidence.require(guest_artifact(selected["LAYERX_MARKET_WASM"]) == built["artifacts"]["LAYERX_MARKET_WASM"],
                     "compiled guest digest changed")
    return built


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: market_build.py PRIVATE_DIRECTORY")
    print(build(Path(__file__).resolve().parents[3], Path(sys.argv[1])))
