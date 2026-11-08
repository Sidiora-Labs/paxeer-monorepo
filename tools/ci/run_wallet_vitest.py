#!/usr/bin/env python3
"""Run wallet Vitest with the individually recorded failures owned by 22.39.145."""

import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
WALLET = ROOT / "sdk/wallet"
BASELINE = ROOT / "tests/ci/data/wallet_vitest_baseline.json"


def read_baseline():
    entries = json.loads(BASELINE.read_text(encoding="utf-8"))
    if not isinstance(entries, list):
        raise ValueError(f"{BASELINE}: expected a list of recorded failures")
    seen = set()
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != {
            "file", "full_name", "reason", "retiring_task"
        }:
            raise ValueError(f"{BASELINE}: invalid baseline entry: {entry!r}")
        if any(not isinstance(value, str) or not value.strip() for value in entry.values()):
            raise ValueError(f"{BASELINE}: baseline fields must be nonempty strings")
        path = (ROOT / entry["file"]).resolve()
        if (
            not path.is_relative_to(WALLET / "test")
            or not path.is_file()
            or not path.name.endswith(".test.ts")
            or entry["retiring_task"] != "22.39.145"
        ):
            raise ValueError(f"{BASELINE}: invalid wallet test or retiring task: {entry!r}")
        key = (entry["file"], entry["full_name"])
        if key in seen:
            raise ValueError(f"{BASELINE}: duplicate recorded failure: {key!r}")
        seen.add(key)
    return entries


def baseline_name_pattern(entries):
    # Vitest's name matcher joins suite/test names with spaces, without the file.
    names = [entry["full_name"].replace(" > ", " ") for entry in entries]
    # Escape only ECMAScript regexp metacharacters, not Python-specific escapes.
    escaped = [re.sub(r"([\\^$.*+?()\[\]{}|/])", r"\\\1", name) for name in names]
    return "^(?!(?:" + "|".join(escaped) + ")$)[\\s\\S]*$" if names else "^[\\s\\S]*$"


def wallet_vitest_command(entries, arguments=()):
    return [
        "pnpm", "--ignore-workspace", "exec", "vitest", "run",
        *arguments, "--testNamePattern", baseline_name_pattern(entries),
    ]


def main():
    entries = read_baseline()
    for entry in entries:
        print(
            f"Wallet baseline (skipped, retiring task {entry['retiring_task']}): "
            f"{entry['file']} > {entry['full_name']} — {entry['reason']}",
            flush=True,
        )
    return subprocess.run(wallet_vitest_command(entries, sys.argv[1:]), cwd=WALLET).returncode


if __name__ == "__main__":
    sys.exit(main())
