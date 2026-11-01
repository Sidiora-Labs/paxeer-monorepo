from __future__ import annotations

import re
from collections.abc import Mapping
from os import PathLike
from pathlib import Path
from typing import Literal, TypedDict

from programme.kvx import KvxError, load


class Decision(TypedDict):
    carry: Literal["kept", "rewritten", "superseded"]
    record: str
    binds: list[str]


def load_index(
    path: str | PathLike[str] = Path(__file__).parent / "data/decisions.kvx",
) -> dict[int, Decision]:
    sections = load(path)
    if sections.get("meta", {}).get("format_version") != 1:
        raise KvxError(f"{path}: expected format_version = 1")
    index: dict[int, Decision] = {}
    for name, entry in sections.items():
        if name == "meta":
            continue
        match = re.fullmatch(r"decision\.([1-9][0-9]*)", name)
        if match is None:
            raise KvxError(f"{path}: invalid decision section [{name}]")
        carry = entry.get("carry")
        if carry not in ("kept", "rewritten", "superseded"):
            raise KvxError(f"{path}: [{name}] has invalid carry status")
        record = entry.get("record")
        if (
            not isinstance(record, str)
            or not record
            or Path(record).is_absolute()
            or ".." in Path(record).parts
        ):
            raise KvxError(f"{path}: [{name}] needs a relative record path")
        binds = entry.get("binds")
        if (
            not isinstance(binds, list)
            or not binds
            or any(
                re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", task) is None
                for task in binds
            )
            or len(set(binds)) != len(binds)
        ):
            raise KvxError(f"{path}: [{name}] needs distinct bound task ids")
        index[int(match[1])] = {"carry": carry, "record": record, "binds": binds}
    return index


def records_missing(
    index: Mapping[int, Decision], root: str | PathLike[str]
) -> list[int]:
    root = Path(root)
    return sorted(
        identifier
        for identifier, entry in index.items()
        if not (root / entry["record"]).is_file()
    )


def unbound(spec_path: str | PathLike[str]) -> list[int | str]:
    sections = load(spec_path)
    decisions = sorted(
        identifier
        for identifier, entry in load_index().items()
        if any(f"task.{task}" not in sections for task in entry["binds"])
    )
    resolutions: list[str] = []
    for name, section in sections.items():
        if not name.startswith("resolution.17."):
            continue
        targets = section.get("applies_to")
        if not isinstance(targets, list) or not all(
            isinstance(target, str) for target in targets
        ):
            raise KvxError(f"{spec_path}: [{name}] needs an applies_to string list")
        if any(target not in sections for target in targets):
            resolutions.append(name)
    return [*decisions, *sorted(resolutions)]
