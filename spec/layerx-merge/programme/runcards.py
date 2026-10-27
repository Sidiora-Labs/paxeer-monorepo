from __future__ import annotations

from collections import Counter
from collections.abc import Iterable, Mapping
from functools import cache
from os import PathLike
from pathlib import Path

from .kvx import KvxError, KvxValue, load


class CardError(ValueError):
    pass


def load_card(path: str | PathLike[str]) -> dict[str, KvxValue]:
    try:
        sections = load(path)
    except KvxError as error:
        raise CardError(str(error)) from error
    if "card" not in sections:
        raise CardError(f"{path}: missing [card] section")
    return sections["card"]


@cache
def _domain_roles() -> dict[str, str]:
    sections = load(Path(__file__).resolve().parents[1] / "spec.kvx")
    roles: dict[str, str] = {}
    for section, fields in sections.items():
        if section.startswith("feature."):
            slug = fields["name"]
            if not isinstance(slug, str) or not slug:
                raise CardError(f"{section}: invalid domain name")
            roles[section.removeprefix("feature.")] = f"fm-{slug}"
    return roles


def check_card(card: Mapping[str, KvxValue], schema: Mapping[str, KvxValue]) -> None:
    required = schema.get("required")
    if not isinstance(required, list) or not required or not all(
        isinstance(key, str) and key for key in required
    ):
        raise CardError("schema: required must be a nonempty list of key names")
    missing = [key for key in required if key not in card]
    if missing:
        raise CardError(f"missing required card keys: {', '.join(missing)}")

    domain = card["domain"]
    if type(domain) not in (int, str) or str(domain) not in _domain_roles():
        raise CardError(f"domain: unknown domain {domain!r}")
    expected = _domain_roles()[str(domain)]
    if card["role"] != expected:
        raise CardError(f"role: domain {domain} requires {expected}, got {card['role']!r}")


def check_against_spec(
    cards: Iterable[Mapping[str, KvxValue]], spec_path: str | PathLike[str]
) -> list[dict[str, KvxValue]]:
    try:
        sections = load(spec_path)
    except KvxError as error:
        raise CardError(str(error)) from error

    domains: dict[str, dict[str, dict[str, KvxValue]]] = {}
    for name, fields in sections.items():
        if name.startswith("task."):
            task_id = name.removeprefix("task.")
            domain = task_id.split(".", 1)[0]
            domains.setdefault(domain, {})[task_id] = fields

    checked: list[dict[str, KvxValue]] = []
    for card in cards:
        domain = card.get("domain")
        if type(domain) not in (int, str):
            raise CardError(f"domain: invalid domain {domain!r}")
        anchors = card.get("anchors")
        if not isinstance(anchors, list) or not anchors or not all(
            isinstance(anchor, str) and anchor for anchor in anchors
        ):
            raise CardError(f"domain {domain}: anchors must be a nonempty list of task ids")
        missing = [anchor for anchor in anchors if f"task.{anchor}" not in sections]
        if missing:
            raise CardError(f"domain {domain}: missing anchors: {', '.join(missing)}")

        tasks = domains.get(str(domain))
        if not tasks:
            raise CardError(f"domain {domain}: no tasks in {spec_path}")
        outside = [anchor for anchor in anchors if anchor not in tasks]
        if outside:
            raise CardError(
                f"domain {domain}: anchors belong to another domain: {', '.join(outside)}"
            )

        waves: Counter[int] = Counter()
        profiles: Counter[str] = Counter()
        for task_id, task in tasks.items():
            wave = task.get("wave")
            profile = task.get("sandbox_profile")
            if type(wave) is not int or wave < 0:
                raise CardError(f"task.{task_id}: wave must be a nonnegative integer")
            if not isinstance(profile, str) or not profile:
                raise CardError(
                    f"task.{task_id}: sandbox_profile must be a nonempty string"
                )
            waves[wave] += 1
            profiles[profile] += 1

        largest = max(profiles.values())
        expected = [profile for profile, count in profiles.items() if count == largest]
        if card.get("sandbox_profile") not in expected:
            raise CardError(
                f"sandbox_profile: domain {domain} requires one of {expected!r}, "
                f"got {card.get('sandbox_profile')!r}"
            )
        checked.append({**card, "max_parallel": max(waves.values())})
    return checked
