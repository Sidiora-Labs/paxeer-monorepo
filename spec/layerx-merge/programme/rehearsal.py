from __future__ import annotations

import shlex
import subprocess
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from os import PathLike
from pathlib import Path

from . import evidence
from .kvx import KvxValue


@dataclass
class RehearsalResult:
    passed: bool
    failed_step: int | None
    evidence: list[str]

    @property
    def exit_code(self) -> int:
        return 0 if self.passed else 1


def run_steps(
    steps: Sequence[Mapping[str, KvxValue]],
    root: str | PathLike[str],
    log_dir: str | PathLike[str],
) -> RehearsalResult:
    root = Path(root).resolve()
    directory = (root / log_dir).resolve()
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    failed_step = None
    lines: list[str] = []
    for number, step in enumerate(steps, start=1):
        command = step["command"]
        stop_on_failure = step["stop_on_failure"]
        if not isinstance(command, str) or not command.strip():
            raise ValueError(f"step {number}: command must be a nonempty string")
        if not isinstance(stop_on_failure, bool):
            raise TypeError(f"step {number}: stop_on_failure must be a boolean")
        arguments = shlex.split(command)
        if not arguments:
            raise ValueError(f"step {number}: command must contain an executable")
        log_path = directory / f"step-{number}.log"
        exit_code = evidence.run(
            [
                "bash",
                "-c",
                'cd -- "$1" && shift && exec "$@"',
                "rehearsal",
                str(root),
                *arguments,
            ],
            log_path,
        )
        line = evidence.evidence_line(revision, command, exit_code, log_path)
        with (directory / "evidence.log").open("a", encoding="utf-8") as ledger:
            ledger.write(line + "\n")
        lines.append(line)
        print(line, flush=True)
        if exit_code != 0:
            if failed_step is None:
                failed_step = number
            if stop_on_failure:
                break
    return RehearsalResult(
        passed=failed_step is None, failed_step=failed_step, evidence=lines
    )
