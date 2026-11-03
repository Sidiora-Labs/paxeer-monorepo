from __future__ import annotations

import re
from collections.abc import Mapping
from os import PathLike
from pathlib import Path
from typing import Literal, NotRequired, TypedDict

from programme.kvx import KvxError, KvxValue, load


class Decision(TypedDict):
    carry: Literal["kept", "rewritten", "superseded"]
    record: str
    binds: list[str]
    basis: NotRequired[str]
    basis_revision: NotRequired[str]
    value: NotRequired[str]


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
        identifier = int(match[1])
        decision: Decision = {"carry": carry, "record": record, "binds": binds}
        if identifier <= 33 or "basis" in entry or "basis_revision" in entry:
            basis = entry.get("basis")
            revision = entry.get("basis_revision")
            if not isinstance(basis, str) or not basis.strip():
                raise KvxError(f"{path}: [{name}] needs a nonempty basis string")
            if not isinstance(revision, str) or not re.fullmatch(r"[0-9a-f]{40}", revision):
                raise KvxError(f"{path}: [{name}] needs a full basis revision")
            decision["basis"] = basis
            decision["basis_revision"] = revision
        if "value" in entry:
            value = entry["value"]
            if not isinstance(value, str) or not value.strip():
                raise KvxError(f"{path}: [{name}] needs a nonempty value string")
            decision["value"] = value
        index[identifier] = decision
    return index


def render_record(entry: Decision, spec: Mapping[str, Mapping[str, KvxValue]]) -> str:
    identifier = int(Path(entry["record"]).stem.removeprefix("D"))
    name = f"decision.{identifier}"
    decision = spec[name]
    value = entry["value"] if "value" in entry else decision["value"]
    targets = decision["affects"]
    if not isinstance(targets, list):
        raise KvxError(f"[{name}] needs an affects string list")
    affects = "\n".join(f"- {target}" for target in targets)
    binds = "\n".join(f"- `task.{task}`" for task in entry["binds"])
    return (
        f"# D{identifier}: {decision['title']}\n\n"
        f"Carry status: `{entry['carry']}`.\n\n"
        f"Source: `[{name}]` in [spec.kvx](../../spec.kvx). "
        "Carry status, code basis, bound tasks and any rewritten value come from "
        "[the carry-forward index](../../programme/data/decisions.kvx).\n\n"
        f"## Value\n\n{value}\n\n"
        f"## Code basis\n\nBasis revision: `{entry['basis_revision']}`.\n\n"
        f"{entry['basis']}\n\n"
        f"## Affects\n\n{affects}\n\n"
        f"## Bound tasks\n\n{binds}\n\n"
        "[Decision index](INDEX.md)\n"
    )


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
