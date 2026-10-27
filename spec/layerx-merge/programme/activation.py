from __future__ import annotations

import re
from collections.abc import Sequence
from os import PathLike
from pathlib import Path
from typing import NotRequired, TypedDict

from .kvx import KvxValue, load

PLAN_PATH = Path(__file__).resolve().parent / "data/activation-plan.kvx"
ANCHOR_STORE_DELETE = "v7.0-anchor-store-delete"
KNOWN_UPGRADE_PLANS = ("v6.9", "v6.10", "v6.12.0", "v7.0")
PRECOMPILE_GATES = {
    "v6.9": "activationPrecompileGate",
    "v7.0": "mergePrecompileGate",
}
_GO_TOKEN = re.compile(
    r'//[^\n]*|/\*[\s\S]*?\*/|"(?:\\.|[^"\\])*"|`[^`]*`|'
    r"'(?:\\.|[^'\\])*'|[A-Za-z_][A-Za-z_0-9]*|\n|[^\S\n]+|.",
)


class PlanError(ValueError):
    pass


class Plan(TypedDict):
    name: str
    handler: str
    store_added: list[str]
    store_deleted: list[str]
    precompile_gate: NotRequired[str]


def constant_sites(root: str | PathLike[str]) -> list[str]:
    root = Path(root)
    sites: list[str] = []
    for path in sorted(root.rglob("*.go")):
        source = path.read_text(encoding="utf-8")
        if '"v7.0"' not in source and "`v7.0`" not in source:
            continue
        tokens: list[str] = []
        for match in _GO_TOKEN.finditer(source):
            token = match.group()
            if token.startswith(("//", "/*")):
                if "\n" in token:
                    tokens.append("\n")
            elif token == "\n" or not token.isspace():
                tokens.append(token)
        in_const = False
        grouped = False
        depth = 0
        previous = ""
        for token in tokens:
            if token == "const":
                in_const = True
                grouped = False
            elif in_const:
                if token in ('"v7.0"', "`v7.0`"):
                    sites.append(path.relative_to(root).as_posix())
                    break
                if token == "(":
                    if previous == "const":
                        grouped = True
                    depth += 1
                elif token == ")":
                    depth -= 1
                    if grouped and depth == 0:
                        in_const = False
                elif depth == 0 and (
                    token == ";"
                    or (
                        token == "\n"
                        and previous
                        and (previous[-1].isalnum() or previous[-1] in "_\"`')]}")
                        and previous != "const"
                    )
                ):
                    in_const = False
            if token != "\n":
                previous = token
    return sites


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


def check_gates(plans: Sequence[Plan]) -> None:
    validate(plans)
    for plan in plans:
        name = plan["name"]
        expected = PRECOMPILE_GATES.get(name)
        if expected is not None:
            if plan.get("precompile_gate") != expected:
                raise PlanError(f"plan {name}: precompile_gate must name {expected}")
        elif "precompile_gate" in plan:
            raise PlanError(f"plan {name}: no precompile_gate is declared in node")


def pending_rule(plans: Sequence[Plan]) -> dict[str, str]:
    rules: dict[str, str] = {}
    for plan in plans:
        name = plan["name"]
        if name not in KNOWN_UPGRADE_PLANS:
            rules[name] = "never applied"
        elif name == "v6.9":
            rules[name] = "done_height == 0 and not activation_modules_present"
        else:
            rules[name] = "done_height == 0"
    return rules


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
        plan = Plan(name=name, handler=handler, store_added=added, store_deleted=deleted)
        if "precompile_gate" in fields:
            gate = fields["precompile_gate"]
            if not isinstance(gate, str) or not gate.strip():
                raise PlanError(f"plan {name}: precompile_gate must name a gate function")
            plan["precompile_gate"] = gate
        plans.append(plan)
    validate(plans)
    return plans
