from __future__ import annotations

import re
from collections.abc import Mapping
from os import PathLike
from pathlib import Path
from typing import TypedDict

from programme.kvx import KvxError, check_format, load

CATALOGUE_PATH = Path(__file__).resolve().parent / "data/services.kvx"
SPEC_PATH = Path(__file__).resolve().parents[1] / "spec.kvx"
UNDECLARED_READINESS = "undeclared"


class Service(TypedDict):
    domain: int
    route: str
    readiness: str
    completion_task: str
    operation_task: str


def load_catalogue(
    path: str | PathLike[str] = CATALOGUE_PATH,
) -> dict[str, Service]:
    sections = check_format(path)
    catalogue: dict[str, Service] = {}
    fields = {"domain", "route", "readiness", "completion_task", "operation_task"}
    for name, entry in sections.items():
        if name == "meta":
            continue
        match = re.fullmatch(r"service\.([a-z0-9]+(?:-[a-z0-9]+)*)", name)
        if match is None:
            raise KvxError(f"{path}: invalid service section [{name}]")
        if set(entry) != fields:
            raise KvxError(f"{path}: [{name}] must declare {sorted(fields)}")
        domain = entry["domain"]
        if type(domain) is not int or domain <= 0:
            raise KvxError(f"{path}: [{name}] domain must be a positive integer")
        for field in fields - {"domain"}:
            if not isinstance(entry[field], str) or not entry[field].strip():
                raise KvxError(f"{path}: [{name}] {field} must be a nonempty string")
        readiness = entry["readiness"]
        if readiness != UNDECLARED_READINESS and not readiness.startswith("/"):
            raise KvxError(
                f"{path}: [{name}] readiness must be a probe path or undeclared"
            )
        for field in ("completion_task", "operation_task"):
            if re.fullmatch(r"[1-9][0-9]*\.[0-9]+\.[0-9]+", entry[field]) is None:
                raise KvxError(f"{path}: [{name}] {field} must be a task id")
        catalogue[match[1]] = {
            "domain": domain,
            "route": entry["route"],
            "readiness": readiness,
            "completion_task": entry["completion_task"],
            "operation_task": entry["operation_task"],
        }
    if not catalogue:
        raise KvxError(f"{path}: catalogue must contain service sections")
    return catalogue


def diff_spec(
    catalogue: Mapping[str, Service],
    spec_path: str | PathLike[str] = SPEC_PATH,
) -> tuple[set[str], set[str]]:
    spec_slugs = {
        name.removeprefix("service.")
        for name in load(spec_path)
        if name.startswith("service.")
    }
    catalogue_slugs = set(catalogue)
    return spec_slugs - catalogue_slugs, catalogue_slugs - spec_slugs
