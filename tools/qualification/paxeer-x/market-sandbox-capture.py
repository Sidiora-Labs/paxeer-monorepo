#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
TASK = "104.35.34"
TEST = "genuine_signed_market_profile_claim_steps_and_cryptographic_refusals"
DECLARED = (
    "programs/crates/layerx-programs-arbiter/tests/market_sandbox_profile.rs",
    "tools/qualification/paxeer-x/market-sandbox-capture.py",
)
TARGET = Path(
    os.environ.get("LAYERX_MARKET_CAPTURE_TARGET", "/root/lx-target/task1043534")
)
EVIDENCE = Path(
    os.environ.get(
        "LAYERX_MARKET_CAPTURE_EVIDENCE",
        "/root/lx-ops/paxeer-x-integration-2026-10-03/qualification/task1043534",
    )
)
DEADLINE = float(
    os.environ.get("PAXEER_X_TASK_DEADLINE_EPOCH", str(time.time() + 1200))
)


def sha(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def sources():
    output = subprocess.check_output(
        [
            "git",
            "ls-files",
            "-z",
            "--",
            "programs",
            "services/agent/crates/layerx-client",
            "services/agent/crates/layerx-proof",
            "services/agent/crates/layerx-types",
            "services/agent/crates/layerx-crypto",
            ".cargo",
        ],
        cwd=ROOT,
    )
    names = {os.fsdecode(value) for value in output.split(b"\0") if value} | set(
        DECLARED
    )
    return {
        name: sha(ROOT / name)
        for name in sorted(names)
        if Path(name).suffix
        in {".c", ".h", ".rs", ".toml", ".lock", ".json", ".py", ".mk", ".inc"}
        or name == "Makefile"
    }


def bound(seconds):
    remaining = min(seconds, DEADLINE - time.time())
    if remaining <= 0:
        raise RuntimeError("original task cutoff exhausted")
    return remaining


def environment():
    return dict(
        os.environ,
        PATH="/root/.cargo/bin:" + os.environ.get("PATH", ""),
        CARGO_TARGET_DIR=str(TARGET / "rust"),
        CARGO_BUILD_JOBS="8",
    )


def run(command, name, env=None, seconds=1100):
    log = EVIDENCE / (name + ".log")
    duration = bound(seconds)
    with log.open("w") as stream:
        process = subprocess.Popen(
            command,
            cwd=ROOT,
            env=env or environment(),
            stdin=subprocess.DEVNULL,
            stdout=stream,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        try:
            code = process.wait(timeout=duration)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            log.with_suffix(".log.exit").write_text("124\n")
            raise
    log.with_suffix(".log.exit").write_text(str(code) + "\n")
    if code:
        raise subprocess.CalledProcessError(code, command)
    return log.read_text()


def row(path):
    path = Path(path).resolve(strict=True)
    return {"path": str(path), "sha256": sha(path)}


def executable(output, name):
    found = set()
    for line in output.splitlines():
        if line.startswith("{"):
            value = json.loads(line)
            if (
                value.get("reason") == "compiler-artifact"
                and value.get("target", {}).get("name") == name
                and value.get("executable")
            ):
                found.add(value["executable"])
    if len(found) != 1:
        raise RuntimeError("one genuine declared compiler executable required: " + name)
    return row(found.pop())


def checked(value):
    path = Path(value["path"])
    if not path.is_file() or path.is_symlink() or sha(path) != value["sha256"]:
        raise RuntimeError("source-bound artifact missing or changed")
    return path


def build():
    inputs = sources()
    cargo = "/root/.cargo/bin/cargo"
    verifier = executable(
        run(
            [
                cargo,
                "test",
                "--locked",
                "--manifest-path",
                "programs/Cargo.toml",
                "-p",
                "layerx-programs-arbiter",
                "--test",
                "market_sandbox_profile",
                "--no-run",
                "--message-format=json",
            ],
            "build-verifier",
        ),
        "market_sandbox_profile",
    )
    if inputs != sources():
        raise RuntimeError("source changed during bounded build")
    manifest = {
        "task": TASK,
        "revision": subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
        ).strip(),
        "inputs": inputs,
        "build_exit": 0,
        "verifier": verifier,
    }
    (EVIDENCE / "artifacts.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print("ARTIFACTS " + str(EVIDENCE / "artifacts.json"), flush=True)


def qualify():
    manifest = json.loads((EVIDENCE / "artifacts.json").read_text())
    if (
        manifest.get("task") != TASK
        or manifest.get("build_exit") != 0
        or manifest["inputs"] != sources()
    ):
        raise RuntimeError("genuine source-bound build required")
    verifier = checked(manifest["verifier"])
    inputs = Path(os.environ["LAYERX_MARKET_SANDBOX_INPUTS"]).resolve(strict=True)
    env = dict(environment(), LAYERX_MARKET_SANDBOX_INPUTS=str(inputs))
    output = run(
        [str(verifier), TEST, "--exact", "--nocapture", "--test-threads=1"],
        "verify",
        env,
        540,
    )
    if "test result: ok. 1 passed; 0 failed; 0 ignored;" not in output:
        raise RuntimeError("genuine signed Market profile case missing or skipped")
    if manifest["inputs"] != sources():
        raise RuntimeError("source changed during verification")
    result = {
        "task": TASK,
        "revision": manifest["revision"],
        "command": "timeout 10m python3 tools/qualification/paxeer-x/market-sandbox-capture.py",
        "exit": 0,
        "fixture_manifest": str(inputs),
        "fixture_sha256": sha(inputs),
        "log": str(EVIDENCE / "verify.log"),
    }
    (EVIDENCE / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    print("VERIFIED " + str(EVIDENCE / "result.json"), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--build", action="store_true")
    args = parser.parse_args()
    EVIDENCE.mkdir(mode=0o700, parents=True, exist_ok=True)
    if EVIDENCE.stat().st_mode & 0o077:
        raise RuntimeError("private evidence directory required")
    if args.build:
        build()
    else:
        qualify()


if __name__ == "__main__":
    try:
        main()
    except (
        OSError,
        ValueError,
        RuntimeError,
        KeyError,
        subprocess.SubprocessError,
    ) as error:
        print("FAILED " + str(error), file=sys.stderr)
        sys.exit(
            error.returncode if isinstance(error, subprocess.CalledProcessError) else 1
        )
