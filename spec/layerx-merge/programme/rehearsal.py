from __future__ import annotations

import re
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


def to_finding(result: RehearsalResult, task_id: str) -> str:
    if result.passed:
        return ""
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", task_id):
        raise ValueError("task_id must be a valid KVX section identifier")
    if result.failed_step is None or not 1 <= result.failed_step <= len(result.evidence):
        raise ValueError("failed result must identify a step with recorded evidence")
    fields = {
        "task": task_id,
        "file": "spec/layerx-merge/programme/rehearsal.py",
        "symbol": "run_steps",
        "observed": (
            f"Rehearsal step {result.failed_step} failed: "
            f"{result.evidence[result.failed_step - 1]}."
        ),
        "assumption": (
            "The failed rehearsal step is outside this task's scope and remains unresolved."
        ),
        "severity": "blocker",
    }
    lines = [f"[finding.{task_id}]"]
    for key, value in fields.items():
        if any(character in value for character in "\r\n\t"):
            raise ValueError("finding values must fit on one KVX line without tabs")
        quoted = value.replace('"', r'\"')
        lines.append(f'{key} = "{quoted}"')
    return "\n".join(lines) + "\n"
