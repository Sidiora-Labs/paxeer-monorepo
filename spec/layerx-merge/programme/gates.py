from __future__ import annotations

import shlex
import subprocess
from os import PathLike
from pathlib import Path

from . import evidence
from .kvx import load

ROOT = Path(__file__).resolve().parents[3]
MANIFEST = Path(__file__).resolve().parent / "data/wave-gates.kvx"


def _wave(n: int | str) -> tuple[str, str]:
    wave = load(MANIFEST)[f"wave.{n}"]
    script = wave["script"]
    evidence_dir = wave["evidence_dir"]
    if not isinstance(script, str) or not isinstance(evidence_dir, str):
        raise TypeError("wave script and evidence_dir must be strings")
    return script, evidence_dir


def run_wave(n: int | str, root: str | PathLike[str]) -> int:
    script, evidence_dir = _wave(n)
    root = Path(root).resolve()
    directory = root / evidence_dir
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    log_path = directory / f"wave-{n}.log"
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    command = [
        "bash",
        "-c",
        'cd -- "$1" && exec bash "$2"',
        "run-wave",
        str(root),
        script,
    ]
    exit_code = evidence.run(command, log_path)
    line = evidence.evidence_line(revision, shlex.join(command), exit_code, log_path)
    with (directory / "evidence.log").open("a", encoding="utf-8") as ledger:
        ledger.write(line + "\n")
    print(line, flush=True)
    return exit_code


def list_wave(n: int | str) -> list[str]:
    script, _ = _wave(n)
    result = subprocess.run(
        ["bash", script, "--list"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.splitlines()
