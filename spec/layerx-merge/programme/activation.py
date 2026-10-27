from __future__ import annotations

from collections.abc import Sequence
from os import PathLike
from pathlib import Path
from typing import TypedDict

from .kvx import KvxValue, load

PLAN_PATH = Path(__file__).resolve().parent / "data/activation-plan.kvx"
ANCHOR_STORE_DELETE = "v7.0-anchor-store-delete"


class Plan(TypedDict):
    name: str
    handler: str
    store_added: list[str]
    store_deleted: list[str]


def _store_names(name: str, key: str, value: KvxValue | None) -> list[str]:
    if not isinstance(value, list) or not all(
        isinstance(store, str) and store.strip() for store in value
    ):
        raise ValueError(f"plan {name}: {key} must be a list of store names")
    return value


def validate(plans: Sequence[Plan]) -> None:
    seen: set[str] = set()
    for plan in plans:
        name = plan.get("name")
        if ANCHOR_STORE_DELETE in seen:
            raise ValueError(f"plan {name} follows {ANCHOR_STORE_DELETE}")
        if not isinstance(name, str) or not name.strip():
            raise ValueError("plan is missing a name")
        if name in seen:
            raise ValueError(f"repeated plan name: {name}")
        handler = plan.get("handler")
        if not isinstance(handler, str) or not handler.strip():
            raise ValueError(f"plan {name} is missing a handler")
        seen.add(name)


def load_plans(path: str | PathLike[str] = PLAN_PATH) -> list[Plan]:
    plans: list[Plan] = []
    for section, fields in load(path).items():
        if not section.startswith("plan."):
            raise ValueError(f"unexpected activation plan section: {section}")
        name = section.removeprefix("plan.")
        handler = fields.get("handler")
        if not isinstance(handler, str) or not handler.strip():
            raise ValueError(f"plan {name} is missing a handler")
        added = _store_names(name, "store_added", fields.get("store_added"))
        deleted = _store_names(name, "store_deleted", fields.get("store_deleted"))
        plans.append(
            Plan(name=name, handler=handler, store_added=added, store_deleted=deleted)
        )
    validate(plans)
    return plans
