#!/usr/bin/env python3
"""Regenerate codec replay bytes for activity types still declared by the kernel."""

import importlib.util
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[3]
OUTPUT = Path(__file__).with_name("replay_corpus.lxb")
RETIRED_BUDGET_PULL = 0x0003000B


def main():
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location(
        "replay_fixture", ROOT / "scripts/replica/lxp_replay_fixture.py"
    )
    producer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(producer)
    headers = (ROOT / "include/layerx").glob("*.h")
    declared = {
        int(value, 16)
        for header in headers
        for value in re.findall(r"\bLX_\w+\s*=\s*(0x[0-9a-fA-F]{8})\b", header.read_text())
    }
    removed = set(producer.REPLAY_ACTIVITY_TYPES) - declared
    if removed - {RETIRED_BUDGET_PULL}:
        raise ValueError(f"unexpected undeclared replay activities: {sorted(removed)}")
    producer.REPLAY_ACTIVITY_TYPES = tuple(
        value for value in producer.REPLAY_ACTIVITY_TYPES if value in declared
    )
    if not producer.REPLAY_ACTIVITY_TYPES:
        raise ValueError("kernel activity table is empty")
    payload = producer.build()
    if not OUTPUT.exists() or OUTPUT.read_bytes() != payload:
        OUTPUT.write_bytes(payload)
    print(f"{OUTPUT.relative_to(ROOT)}: {len(producer.REPLAY_ACTIVITY_TYPES)} activities; retired budget pull excluded")


if __name__ == "__main__":
    main()
