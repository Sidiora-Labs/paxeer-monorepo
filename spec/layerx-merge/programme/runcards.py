from __future__ import annotations

from collections.abc import Mapping
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
