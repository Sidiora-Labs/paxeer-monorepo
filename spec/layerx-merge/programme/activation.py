from __future__ import annotations

import re
from collections.abc import Mapping, Sequence
from os import PathLike
from pathlib import Path
from typing import NotRequired, TypedDict

from .kvx import KvxValue, load

PLAN_PATH = Path(__file__).resolve().parent / "data/activation-plan.kvx"
GOLDEN_PATH = PLAN_PATH.parent / "goldens/merge-bringup.kvx"
RUNBOOK_PATH = PLAN_PATH.parents[2] / "docs/activation-runbook.md"
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
        if section == "meta":
            if (
                type(fields.get("format_version")) is not int
                or fields["format_version"] != 1
            ):
                raise ValueError("activation plan format_version must be integer 1")
            continue
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


def _runbook_value(value: KvxValue) -> str:
    text = ", ".join(value) if isinstance(value, list) else str(value)
    return text.replace("|", "\\|").replace("\n", "<br>")


def render_runbook(
    plans: Sequence[Plan], goldens: Mapping[str, Mapping[str, KvxValue]]
) -> str:
    """Render the activation inventory and bring-up expectations as Markdown."""
    check_gates(plans)
    rules = pending_rule(plans)
    lines = [
        "# Activation runbook",
        "",
        "Generated from `programme/data/activation-plan.kvx` and",
        "`programme/data/goldens/merge-bringup.kvx`. Do not edit this file by hand.",
        "From the repository root, regenerate it with:",
        "",
        "```sh",
        "PYTHONPATH=spec/layerx-merge python3 -m programme.activation",
        "```",
        "",
        "## Plans",
        "",
        "Apply the four plans recognized by `node/upgrades.go:knownUpgradePlans`",
        "in the data order below. The upgrade info file names the plan and its",
        "activation height; the store loader and handler must use that same plan.",
        "The file height must equal the last committed height plus one at startup.",
        "",
        "| Plan | Handler | Stores added | Stores deleted | Precompile gate | Pending rule |",
        "| --- | --- | --- | --- | --- | --- |",
    ]
    deferred: list[Plan] = []
    for plan in plans:
        if plan["name"] not in KNOWN_UPGRADE_PLANS:
            deferred.append(plan)
            continue
        cells = [
            plan["name"],
            plan["handler"],
            ", ".join(plan["store_added"]) or "none",
            ", ".join(plan["store_deleted"]) or "none",
            plan.get("precompile_gate", "none"),
            rules[plan["name"]],
        ]
        lines.append("| " + " | ".join(_runbook_value(cell) for cell in cells) + " |")
    if deferred:
        lines.extend(["", "Additional plan data outside `knownUpgradePlans`:", ""])
        for plan in deferred:
            lines.append(
                f"- `{plan['name']}`: handler `{plan['handler']}`; "
                f"stores added: {_runbook_value(plan['store_added']) or 'none'}; "
                f"stores deleted: {_runbook_value(plan['store_deleted']) or 'none'}; "
                f"pending rule: {rules[plan['name']]}."
            )
    lines.extend(
        [
            "",
            "## Bring-up checks",
            "",
            "These are golden test expectations, not evidence of a live activation.",
            "Run each command from the repository root and check its expected exit.",
            "Refusal tests exit successfully when the handler rejects the bad plan.",
        ]
    )
    for section, fields in goldens.items():
        if not section.startswith("golden."):
            raise ValueError(f"unexpected bring-up golden section: {section}")
        case = section.removeprefix("golden.")
        lines.extend(
            [
                "",
                f"### {case}",
                "",
                f"Test: `{fields['test']}`. Expected test exit: `{fields['expect_exit']}`.",
                "",
                "```sh",
                str(fields["command"]),
                "```",
                "",
            ]
        )
        if "expect_text" in fields:
            lines.append(f"Expected handler error contains: `{fields['expect_text']}`.")
        else:
            lines.append("Expected outcome: the merge plan is accepted.")
        inputs = {
            key: value
            for key, value in fields.items()
            if key not in {"test", "command", "expect_exit", "expect_text"}
        }
        if inputs:
            lines.extend(["", "| Golden input | Value |", "| --- | --- |"])
            for key, value in inputs.items():
                lines.append(f"| `{key}` | `{_runbook_value(value)}` |")
    lines.extend(
        [
            "",
            "## Refusals",
            "",
            "The merge handler refuses invalid sequencer genesis, an empty leader",
            "list, a nonzero lastSeq, or missing receipt-key coverage at H_act + 1.",
            "A refusal stops the block that would apply the plan.",
            "The four golden error fragments are:",
            "",
            "| Case | Handler error contains |",
            "| --- | --- |",
        ]
    )
    for section, fields in goldens.items():
        if "expect_text" in fields:
            lines.append(
                f"| {section.removeprefix('golden.')} | "
                f"`{_runbook_value(fields['expect_text'])}` |"
            )
    return "\n".join(lines) + "\n"


if __name__ == "__main__":
    RUNBOOK_PATH.write_bytes(
        render_runbook(load_plans(), load(GOLDEN_PATH)).encode("utf-8")
    )
