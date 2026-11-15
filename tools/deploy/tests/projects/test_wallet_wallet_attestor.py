"""Checks the wallet attestor project row against its build and runtime inputs."""

from __future__ import annotations

import ast
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/wallet/wallet-attestor.kvx"
DOCKERFILE = ROOT / "docker/wallet-attestor/Dockerfile"
SOURCE_ROOT = ROOT / "services/wallet/attestor"
FILES_TABLE = ROOT / "docker/wallet-attestor/files.tsv"


def _parse_definition(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    current: dict[str, object] | None = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("[") and line.endswith("]"):
            current = sections.setdefault(line[1:-1], {})
            continue
        if current is None or "=" not in line:
            raise AssertionError(f"invalid definition line: {raw}")
        key, value = (part.strip() for part in line.split("=", 1))
        current[key] = ast.literal_eval(value)
    return sections


def _source_environment_names() -> set[str]:
    names: set[str] = set()
    constant = re.compile(r"\bEnv[A-Za-z0-9_]+\s*=\s*\"([A-Z][A-Z0-9_]*)\"")
    for source in SOURCE_ROOT.rglob("*.go"):
        names.update(constant.findall(source.read_text(encoding="utf-8")))
    for line in FILES_TABLE.read_text(encoding="utf-8").splitlines():
        if line and not line.startswith("#"):
            names.add(line.split("\t", 1)[0])
    return names


def _required_environment_names() -> set[str]:
    constant = re.compile(r"\b(Env[A-Za-z0-9_]+)\s*=\s*\"([A-Z][A-Z0-9_]*)\"")
    required: set[str] = set()
    for source in SOURCE_ROOT.rglob("*.go"):
        text = source.read_text(encoding="utf-8")
        names = {identifier: value for identifier, value in constant.findall(text)}
        for identifier in re.findall(r"\brequire\(\s*(Env[A-Za-z0-9_]+)\s*\)", text):
            if identifier in names:
                required.add(names[identifier])
    return required


def _env_references(vars_row: dict[str, object]) -> dict[str, str]:
    references: dict[str, str] = {}
    for name, value in vars_row.items():
        if isinstance(value, str) and value.startswith("env:"):
            references[name] = value[4:]
    return references


def test_wallet_attestor_definition_tracks_sources() -> None:
    sections = _parse_definition(DEFINITION)
    service = sections["service.wallet-attestor"]
    variables = sections["vars.wallet-attestor"]
    inputs = sections["inputs.wallet-attestor"]

    assert service["kind"] == "repo"
    assert service["container"] == "docker/wallet-attestor/Dockerfile"
    assert service["root"] == "."
    assert service["health"] == "/health"
    assert service["volume"] == "/data"
    assert DOCKERFILE.is_file()

    for pattern in service["watch"]:
        assert list(ROOT.glob(pattern)), pattern

    source_names = _source_environment_names()
    references = _env_references(variables)
    assert set(variables) <= source_names
    assert set(references.values()) <= {
        f"WALLET_WALLET_ATTESTOR_{name}" for name in source_names
    }
    assert set(inputs) == set(references.values())
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in inputs)
    assert set(inputs) <= {
        f"WALLET_WALLET_ATTESTOR_{name}" for name in source_names
    }

    # Every variable that the configuration requires is represented by a row.
    required = _required_environment_names()
    assert required <= set(variables)

