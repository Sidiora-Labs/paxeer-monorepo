#!/usr/bin/env python3
from __future__ import annotations

import argparse
import shlex
import subprocess
import sys
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
    for index, command in enumerate(commands, start=1):
        log_path = directory / f"wave-{wave}-{index}.log"
        with log_path.open("w") as log:
            completed = subprocess.run(
                list(command),
                cwd=ROOT,
                shell=False,
                check=False,
                stdout=log,
                stderr=log,
            )
        print(
            f"revision={revision} command={shlex.join(command)} "
            f"exit={completed.returncode} log={log_path}",
            flush=True,
        )
        if completed.returncode != 0:
            return completed.returncode
    return 0


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
