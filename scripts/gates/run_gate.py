#!/usr/bin/env python3
from __future__ import annotations

import argparse
import errno
import os
import shlex
import subprocess
import sys
import uuid
from collections import deque
from collections.abc import Sequence
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
USAGE = "run_gate.py --wave N|programme --log-dir DIR -- COMMAND"


def wave_identifier(value: str) -> int | str:
    if value == "programme":
        return value
    try:
        wave = int(value)
    except ValueError:
        raise argparse.ArgumentTypeError(
            "wave must be a non-negative integer or programme"
        ) from None
    if wave < 0:
        raise argparse.ArgumentTypeError(
            "wave must be a non-negative integer or programme"
        )
    return wave


# Only untracked output is exempt: a tracked file is always source.
BUILD_OUTPUT_DIRECTORIES = frozenset({"__pycache__", ".pytest_cache", "node_modules"})
ROOT_OUTPUT_DIRECTORIES = frozenset(
    {"build", "out", "cache", "target", ".lane-target", ".logs", ".codegraph"}
)
SOURCE_REFUSAL = 125
EVIDENCE_REFUSAL = 73


def permitted_output(path: Path, evidence_root: Path) -> bool:
    relative = path.relative_to(ROOT)
    if (
        ROOT in evidence_root.parents
        and path.is_relative_to(evidence_root)
    ):
        return True
    parts = relative.parts
    if any(part in BUILD_OUTPUT_DIRECTORIES for part in parts[:-1]):
        return True
    if len(parts) > 1 and parts[0] in ROOT_OUTPUT_DIRECTORIES:
        return True
    for index, part in enumerate(parts[:-1]):
        parent = ROOT.joinpath(*parts[:index])
        if part == "target" and (parent / "Cargo.toml").is_file():
            return True
        if part == "target" and (parent / "pom.xml").is_file():
            return True
        if part == ".build" and (parent / "Package.swift").is_file():
            return True
        if part in {"obj", "bin"} and any(
            project.is_file() for project in parent.glob("*.csproj")
        ):
            return True
        if part in {"dist", ".next"} and (parent / "package.json").is_file():
            return True
        if part in {"artifacts", "cache"} and any(
            (parent / f"hardhat.config.{suffix}").is_file()
            for suffix in ("js", "ts", "cjs", "mjs")
        ):
            return True
    return False


def source_identity_error(revision: str, evidence_root: Path) -> str | None:
    try:
        current = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        if current != revision:
            return f"HEAD changed: expected={revision} observed={current}"
        status = subprocess.run(
            [
                "git", "status", "--porcelain=v1", "-z",
                "--untracked-files=all", "--ignored=matching",
            ],
            cwd=ROOT,
            check=True,
            capture_output=True,
        ).stdout.split(b"\0")
        changes = []
        entries = iter(status)
        for entry in entries:
            if not entry:
                continue
            state = entry[:2]
            name = os.fsdecode(entry[3:])
            # Porcelain -z emits a second path for a rename or copy.
            if b"R" in state or b"C" in state:
                next(entries)
            if state in {b"??", b"!!"}:
                path = ROOT / name
                # Keep a trailing directory separator meaningful to output policy.
                candidate = path / "__output__" if name.endswith("/") else path
                if permitted_output(candidate, evidence_root):
                    continue
            changes.append(f"{os.fsdecode(state)} {name!r}")
        if changes:
            return "source tree differs from HEAD:\n" + "\n".join(changes)
    except (OSError, subprocess.CalledProcessError) as error:
        return f"source identity check failed: {error}"
    return None


def emit_evidence(
    revision: str,
    command: Sequence[str],
    code: int,
    log_path: Path,
    status: str = "",
) -> None:
    print(
        f"revision={revision} command={shlex.join(command)} "
        f"exit={code} log={log_path}" + (f" {status}" if status else ""),
        flush=True,
    )
    if code != 0:
        print(f"=== last 200 lines of {log_path} ===", flush=True)
        with log_path.open(encoding="utf-8", errors="replace") as log:
            print("".join(deque(log, maxlen=200)), end="", flush=True)


def run_gate(
    wave: int | str, commands: Sequence[Sequence[str]], log_dir: str | Path
) -> int:
    if not commands or any(not command for command in commands):
        raise ValueError("commands must contain nonempty argument lists")
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        shell=False,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    directory = Path(log_dir).resolve()
    directory.mkdir(parents=True, exist_ok=True)
    attempt = directory / f"attempt-{uuid.uuid4().hex}"
    try:
        attempt.mkdir(mode=0o700)
    except FileExistsError as error:
        print(
            f"revision={revision} status=evidence-refusal exit={EVIDENCE_REFUSAL} "
            f"log={attempt} error={error}",
            flush=True,
        )
        return EVIDENCE_REFUSAL
    try:
        for index, command in enumerate(commands, start=1):
            log_path = attempt / f"wave-{wave}-{index}.log"
            try:
                log = log_path.open("x", encoding="utf-8")
            except FileExistsError:
                print(
                    f"revision={revision} command={shlex.join(command)} "
                    f"exit={EVIDENCE_REFUSAL} log={log_path} status=evidence-refusal",
                    flush=True,
                )
                return EVIDENCE_REFUSAL
            code, status = 0, ""
            with log:
                problem = source_identity_error(revision, directory)
                if problem is not None:
                    code = SOURCE_REFUSAL
                    status = "status=source-identity-refusal phase=before"
                    log.write(
                        f"source-identity-refusal phase=before expected={revision}\n"
                        f"{problem}\n"
                    )
                else:
                    try:
                        completed = subprocess.run(
                            list(command),
                            cwd=ROOT,
                            shell=False,
                            check=False,
                            stdout=log,
                            stderr=log,
                        )
                        code = completed.returncode
                    except OSError as error:
                        code = 127 if error.errno == errno.ENOENT else 126
                        status = "status=launch-failure"
                        log.write(
                            f"launch-failure errno={error.errno} error={error}\n"
                        )
                    problem = source_identity_error(revision, directory)
                    if problem is not None:
                        log.write(
                            f"\nsource-identity-refusal phase=after expected={revision} "
                            f"command_exit={code}\n{problem}\n"
                        )
                        code = SOURCE_REFUSAL
                        status = "status=source-identity-refusal phase=after"
            log_path.chmod(0o400)
            emit_evidence(revision, command, code, log_path, status)
            if code != 0:
                return code
        return 0
    finally:
        attempt.chmod(0o500)


def main(argv: Sequence[str] | None = None) -> int:
    arguments = list(sys.argv[1:] if argv is None else argv)
    parser = argparse.ArgumentParser(prog="run_gate.py", usage=USAGE)
    parser.add_argument(
        "--wave", required=True, type=wave_identifier, metavar="N|programme"
    )
    parser.add_argument("--log-dir", required=True, type=Path, metavar="DIR")
    separator = arguments.index("--") if "--" in arguments else len(arguments)
    options = parser.parse_args(arguments[:separator])
    command_arguments = arguments[separator + 1 :]
    commands: list[list[str]] = [[]]
    for argument in command_arguments:
        if argument == "--":
            commands.append([])
        else:
            commands[-1].append(argument)
    if any(not command for command in commands):
        parser.error("a nonempty command is required after each --")
    return run_gate(options.wave, commands, options.log_dir)


if __name__ == "__main__":
    sys.exit(main())
