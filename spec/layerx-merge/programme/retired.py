from __future__ import annotations

import json
import re
import subprocess
from collections.abc import Iterable
from fnmatch import fnmatchcase
from os import PathLike
from pathlib import Path, PurePosixPath

from .kvx import KvxValue, load

INVENTORY_PATH = Path(__file__).resolve().parent / "data" / "retired.kvx"
DOMAIN_PREFIXES: dict[int, tuple[str, ...]] = {
    1: (
        "include/layerx",
        "src",
        "modules/lxphost",
        "layerxproof",
        "custodyproof",
        "tests/arith",
        "tests/codec",
        "tests/crypto",
        "tests/fuzz",
    ),
    2: tuple(
        f"{parent}/{name}"
        for parent in ("modules", "precompiles", "api")
        for name in ("layerxledger", "layerxauth", "layerxpay", "layerxagent")
    ),
    3: (
        *(
            f"{parent}/{name}"
            for parent in ("modules", "precompiles", "api")
            for name in (
                "perps",
                "spot",
                "pricefeed",
                "oracle",
                "mint",
                "epoch",
                "layerxexchange",
            )
        ),
        "apps/perps",
        "services/platform/perps-data",
    ),
    4: tuple(
        f"{parent}/launchpad"
        for parent in (
            "modules",
            "precompiles",
            "services",
            "apps",
            "sdk",
            "tools",
            "contracts",
            "api",
        )
    ),
    5: (
        "node",
        "modules/sequencer",
        "modules/store",
        "storage",
        "engine",
        "consensus",
        "wasm-runtime",
        "cmd",
        "daemon",
        "admin",
        "tests/chain",
        "tests/daemon",
        "occ_tests",
    ),
    6: (
        *(
            f"{parent}/{name}"
            for parent in ("modules", "api")
            for name in (
                "evm",
                "tokenfactory",
                "layerxgov",
                "layerxbridge",
                "layerxcustody",
                "layerxanchor",
                "xweb",
            )
        ),
        "precompiles",
        "interchain",
        "wasm",
        "contracts",
        "tests/bridge",
    ),
    7: ("rpc", "api/eth", "api/pax", "sdk/proto"),
    8: ("services/platform", "tools/platform", "sdk/platform"),
    9: ("services/agent",),
    10: ("services/human",),
    11: ("services/interop",),
    12: ("services/wallet", "apps/wallet", "sdk/wallet"),
    13: ("programs", "precompiles/layerxprograms"),
    14: (
        "apps/web",
        "apps/explorer",
        "apps/bridge",
        "services/explorer",
        "services/bridge",
        "sdk/js",
    ),
    15: ("sdk",),
    16: ("tools", "deploy", "docker", "scripts", ".github", "tests"),
    17: ("spec", "docs"),
}
DOMAIN_ROOT_FILES: dict[int, tuple[str, ...]] = {
    5: ("go.mod", "go.sum"),
    7: ("buf.*",),
    16: (
        "Makefile",
        "chain.mk",
        "package.json",
        "pnpm-workspace.yaml",
        "rust-toolchain.toml",
        "foundry*.toml",
        "AGENTS.md",
        "CLAUDE.md",
    ),
    17: ("*.md", "GOTCHA.kvx", "codify.kvx"),
}


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


def callers(component: str, root: str | PathLike[str]) -> list[str]:
    result = subprocess.run(
        [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component",
            component,
        ],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode not in (0, 1):
        result.check_returncode()
    locations: dict[str, None] = {}
    for line in result.stdout.splitlines():
        match = re.fullmatch(rf"(.+):([1-9][0-9]*): {re.escape(component)}: .+", line)
        if match is None:
            raise ValueError(f"malformed retired caller: {line!r}")
        locations[f"{match[1]}:{match[2]}"] = None
    if bool(locations) != (result.returncode == 1):
        raise ValueError("retired caller findings disagree with scanner exit status")
    return list(locations)


def owners(callers: Iterable[str]) -> dict[str, int]:
    result: dict[str, int] = {}
    for caller in callers:
        location = re.fullmatch(r"(.+):[1-9][0-9]*", caller)
        path = location[1] if location else caller
        relative = PurePosixPath(path)
        if (
            not path
            or relative.is_absolute()
            or ".." in relative.parts
            or str(relative) != path
            or path == "."
        ):
            raise ValueError(f"invalid caller path: {path!r}")
        matches = [
            (len(prefix), domain)
            for domain, prefixes in DOMAIN_PREFIXES.items()
            for prefix in prefixes
            if path == prefix or path.startswith(prefix + "/")
        ]
        if path.startswith("tests/explorer"):
            matches.append((len("tests/explorer"), 14))
        if relative.parent == PurePosixPath("api") and (
            relative.suffix == ".yaml" or relative.name == "README.md"
        ):
            matches.append((len(path), 7))
        if relative.parent == PurePosixPath("."):
            for domain, patterns in DOMAIN_ROOT_FILES.items():
                if any(fnmatchcase(path, pattern) for pattern in patterns):
                    matches.append((len(path), domain))
                    break
        if not matches:
            raise ValueError(f"no owning domain for caller: {path}")
        result[path] = max(matches)[1]
    return result
