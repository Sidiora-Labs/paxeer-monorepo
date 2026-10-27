from __future__ import annotations

import json
from os import PathLike
from pathlib import Path

from .kvx import KvxValue, load

INVENTORY_PATH = Path(__file__).resolve().parent / "data" / "retired.kvx"


def load_components() -> dict[str, dict[str, KvxValue]]:
    return {
        section.removeprefix("component."): component
        for section, component in load(INVENTORY_PATH).items()
        if section.startswith("component.")
    }


def diff_against_json(
    json_path: str | PathLike[str],
) -> tuple[set[str], set[str]]:
    """Return names missing from the inventory, then names missing from JSON."""
    with Path(json_path).open(encoding="utf-8") as source:
        json_names = set(json.load(source)["components"])
    inventory_names = set(load_components())
    return json_names - inventory_names, inventory_names - json_names
