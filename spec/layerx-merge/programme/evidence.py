from __future__ import annotations

import subprocess
from collections.abc import Sequence
from os import PathLike
from pathlib import Path


def evidence_line(
    revision: str, command: str, exit_code: int, log_path: str | PathLike[str]
) -> str:
    return f"revision={revision} command={command} exit={exit_code} log={log_path}"


def run(command: Sequence[str], log_path: str | PathLike[str]) -> int:
    if isinstance(command, (str, bytes)):
        raise TypeError("command must be an argument list")
    path = Path(log_path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("wb") as log:
        result = subprocess.run(
            list(command),
            shell=False,
            stdout=log,
            stderr=subprocess.STDOUT,
            check=False,
        )
    return result.returncode
