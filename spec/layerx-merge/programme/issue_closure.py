from __future__ import annotations

import argparse
import re
from os import PathLike
from pathlib import Path


def issue_closure(spec_path: str | PathLike[str]) -> list[str]:
    sections = re.split(
        r"(?m)^[ \t]*\[([A-Za-z0-9_.-]+)\][ \t]*(?:#.*)?$",
        Path(spec_path).read_text(encoding="utf-8"),
    )
    open_tasks = []
    for name, body in zip(sections[1::2], sections[2::2]):
        if not name.startswith("task.18.") or name == "task.18.0.1":
            continue
        status = re.search(
            r'^[ \t]*status[ \t]*=[ \t]*"([^"]*)"', body, re.MULTILINE
        )
        if status is None or status.group(1) not in {"done", "superseded"}:
            open_tasks.append(name.removeprefix("task."))
    return sorted(open_tasks)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spec_path", type=Path)
    args = parser.parse_args()
    open_tasks = issue_closure(args.spec_path)
    print(open_tasks)
    return 1 if open_tasks else 0


if __name__ == "__main__":
    raise SystemExit(main())
