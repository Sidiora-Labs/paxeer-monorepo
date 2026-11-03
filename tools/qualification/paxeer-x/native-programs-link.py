#!/usr/bin/env python3
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
TASK = "104.35.28"
SCHEMA = "layerx.native-programs-link.v1"
DECLARED = "tools/qualification/paxeer-x/native-programs-link.py"
PRODUCTION = (
    "layerxd",
    "layerx-genesis-build",
    "layerx-verify",
    "layerx-module-registry",
    "layerx-archive-codec",
)


def digest(path):
    with Path(path).open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def source_inputs():
    inventory = subprocess.run(
        [
            "git",
            "ls-files",
            "-z",
            "--",
            "Makefile",
            "src",
            "include",
            "modules/sequencer",
            "node/sequencer",
            "rpc",
            "custodyproof",
            "precompiles",
            "sdk/x/params",
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
    paths = {os.fsdecode(path) for path in inventory if path}
    return {
        path: digest(ROOT / path)
        for path in sorted(paths)
        if Path(path).suffix
        in {
            ".c",
            ".h",
            ".rs",
            ".go",
            ".proto",
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


def inputs():
    source = source_inputs()
    inventory = subprocess.run(
        ["git", "ls-files", "-z", "--", "cmd", "*.go", "go.mod", "go.sum"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    ).stdout.split(b"\0")
    paths = {os.fsdecode(path) for path in inventory if path} | {DECLARED}
    source.update(
        {
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
        }
    )
    return source


def evidence_directory():
    directory = Path(
        os.environ.get(
            "PAXEER_X_NATIVE_PROGRAMS_LINK_EVIDENCE",
            "/root/lx-ops/qualification/native-programs-link",
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


def build(arguments, evidence):
    manifest = evidence / "native-programs-artifacts.json"
    manifest.unlink(missing_ok=True)
    source = inputs()
    native_dir = Path(arguments.native_dir).resolve()
    programs_dir = Path(arguments.programs_dir).resolve()
    native_make = evidence / "build-native.mk"
    targets = " ".join("$(BUILD_DIR)/bin/" + name for name in PRODUCTION)
    native_make.write_text(
        ".PHONY: paxeer-x-native-programs-link-build\n"
        "paxeer-x-native-programs-link-build: "
        + targets
        + "\n"
    )
    native_make.chmod(0o600)
    command = [
        "make",
        "-f",
        "Makefile",
        "-f",
        str(native_make),
        "-j4",
        "-W",
        str(programs_dir / "debug/liblayerx_programs_sandbox.a"),
        "BUILD_DIR=" + str(native_dir),
        "PROGRAMS_TARGET_DIR=" + str(programs_dir),
        "PROGRAMS_RUNTIME_LIB="
        + str(programs_dir / "debug/liblayerx_programs_sandbox.a"),
        "paxeer-x-native-programs-link-build",
    ]
    environment = dict(
        os.environ, PATH="/root/.cargo/bin:" + os.environ.get("PATH", "")
    )
    lock_path = Path("/root/lx-cargo/native-build.lock")
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        with (evidence / "build-native.log").open("w") as output:
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
    if inputs() != source:
        raise RuntimeError("unchanged complete source required")
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    production = {name: artifact(native_dir / "bin" / name) for name in PRODUCTION}
    save(
        manifest,
        {
            "schema": SCHEMA,
            "task": TASK,
            "revision": revision,
            "inputs": source,
            "production": production,
        },
    )
    print("ARTIFACTS " + str(manifest), flush=True)


def qualify(evidence):
    manifest = json.loads((evidence / "native-programs-artifacts.json").read_text())
    if (
        manifest.get("schema") != SCHEMA
        or manifest.get("task") != TASK
        or manifest.get("inputs") != inputs()
    ):
        raise RuntimeError("production artifacts do not bind current complete source")
    if set(manifest.get("production", {})) != set(PRODUCTION):
        raise RuntimeError("every Programs-linked production executable required")
    dependencies = {}
    with (evidence / "verify-production-links.log").open("w") as output:
        for name in PRODUCTION:
            path = checked(manifest["production"][name])
            with path.open("rb") as binary:
                if binary.read(4) != b"\x7fELF":
                    raise RuntimeError("actual native ELF executable required: " + name)
            command = ["readelf", "--dynamic", str(path)]
            result = subprocess.run(
                command, check=True, capture_output=True, text=True, timeout=30
            )
            output.write(result.stdout)
            dependencies[name] = [
                line.strip()
                for line in result.stdout.splitlines()
                if "(NEEDED)" in line
            ]
            command = ["ldd", "-r", str(path)]
            result = subprocess.run(command, capture_output=True, text=True, timeout=30)
            output.write(result.stdout)
            output.write(result.stderr)
            output.flush()
            if result.returncode:
                raise subprocess.CalledProcessError(result.returncode, command)
            if (
                "not found" in result.stdout + result.stderr
                or "undefined symbol:" in result.stdout + result.stderr
            ):
                raise RuntimeError("unresolved production native dependency: " + name)
    if not any("libssl.so" in line for line in dependencies["layerxd"]):
        raise RuntimeError("daemon Programs-to-Client TLS dependency missing")
    if inputs() != manifest["inputs"]:
        raise RuntimeError("source changed during focused native qualification")
    for value in manifest["production"].values():
        checked(value)
    save(
        evidence / "native-programs-result.json",
        {
            "task": TASK,
            "revision": manifest["revision"],
            "command": "timeout 10m python3 " + DECLARED,
            "exit": 0,
            "production_dependencies": dependencies,
        },
    )
    print("VERIFIED " + str(evidence / "native-programs-result.json"), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--build", action="store_true")
    parser.add_argument("--cargo", default="/root/.cargo/bin/cargo")
    parser.add_argument(
        "--native-dir", default="/root/lx-target/arbiter-prestate/native"
    )
    parser.add_argument(
        "--programs-dir", default="/root/lx-target/arbiter-prestate/rust"
    )
    parser.add_argument("--agent-dir", default="/root/lx-target/agent")
    arguments = parser.parse_args()
    evidence = evidence_directory()
    if arguments.build:
        build(arguments, evidence)
    else:
        qualify(evidence)


if __name__ == "__main__":
    try:
        main()
    except (
        OSError,
        ValueError,
        RuntimeError,
        subprocess.SubprocessError,
    ) as error:
        print("FAILED " + str(error), file=sys.stderr)
        sys.exit(
            error.returncode if isinstance(error, subprocess.CalledProcessError) else 1
        )
