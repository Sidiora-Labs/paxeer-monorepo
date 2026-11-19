"""Checks the hosted faucet definition against its container and source."""

from __future__ import annotations

import ast
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/faucet.kvx"
DOCKER_DIR = ROOT / "docker/platform-faucet"
SOURCE = ROOT / "services/platform/hosted/faucet/src/main.rs"


def _parse_kvx(path: Path) -> tuple[dict[str, dict[str, object]], dict[str, list[str]]]:
    sections: dict[str, dict[str, object]] = {}
    comments: dict[str, list[str]] = {}
    section = ""
    pending_comments: list[str] = []
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line:
            pending_comments = []
            continue
        if line.startswith("#"):
            pending_comments.append(line[1:].strip())
            continue
        if line.startswith("[") and line.endswith("]"):
            section = line[1:-1]
            sections[section] = {}
            pending_comments = []
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        parsed: object
        try:
            parsed = ast.literal_eval(value.strip())
        except (SyntaxError, ValueError):
            parsed = value.strip()
        sections[section][key] = parsed
        comments[f"{section}.{key}"] = pending_comments
        pending_comments = []
    return sections, comments


def _source_variables() -> tuple[set[str], set[str]]:
    source = SOURCE.read_text()
    direct = set(re.findall(r'env::var(?:_os)?\("([A-Z][A-Z0-9_]*)"', source))
    calls = set(
        re.findall(r"(?:parse_u64|read_secret|read_optional_secret)\(\"([A-Z][A-Z0-9_]*)\"", source)
    )
    defaults = set(re.findall(r'parse_u64\("([A-Z][A-Z0-9_]*)"\s*,', source))
    return direct | calls, defaults


def _input_references(vars_row: dict[str, object]) -> set[str]:
    return {
        value.removeprefix("env:")
        for value in vars_row.values()
        if isinstance(value, str) and value.startswith("env:")
    }


def test_platform_faucet_definition_matches_sources() -> None:
    sections, comments = _parse_kvx(DEFINITION)
    service = sections["service.faucet"]
    vars_row = sections["vars.faucet"]
    inputs = sections["inputs.faucet"]

    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/platform-faucet/Dockerfile"
    assert service["root"] == "."
    assert service["health"] == "/livez"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 3
    assert service["public"] == 1
    assert service["order"] == 15
    assert {"router", "core-boundary", "testnet-control"}.issubset(service["needs"])

    source_variables, defaults = _source_variables()
    required = source_variables - defaults
    assert required <= set(vars_row), sorted(required - set(vars_row))
    assert set(vars_row) <= source_variables | {"PORT"}

    references = _input_references(vars_row)
    assert references == set(inputs)
    assert references
    for name, kind in inputs.items():
        assert re.fullmatch(r"PLATFORM_FAUCET_[A-Z0-9_]+", name)
        assert kind in {"generated", "provider", "chain", "operator"}
        assert comments[f"inputs.faucet.{name}"], name

    for pattern in service["watch"]:
        candidate = ROOT / pattern.removeprefix("/").removesuffix("/**")
        assert candidate.exists(), pattern


def test_platform_faucet_container_materializes_runtime_inputs() -> None:
    dockerfile = (DOCKER_DIR / "Dockerfile").read_text()
    ignore = (DOCKER_DIR / "Dockerfile.dockerignore").read_text()
    entrypoint = (DOCKER_DIR / "entrypoint.sh").read_text()
    files = (DOCKER_DIR / "files.tsv").read_text()

    for copied in (
        "docker/common/env-files.sh",
        "docker/platform-faucet/files.tsv",
        "docker/platform-faucet/entrypoint.sh",
    ):
        assert f"COPY" in dockerfile and copied in dockerfile
        assert copied in ignore
        assert (ROOT / copied).exists()

    sections, _ = _parse_kvx(DEFINITION)
    vars_row = sections["vars.faucet"]
    file_variables = {
        line.split("\t", 1)[0]
        for line in files.splitlines()
        if line and not line.startswith("#")
    }
    assert file_variables <= set(vars_row)
    assert all("\t/run/layerx/" in line for line in files.splitlines() if line and not line.startswith("#"))
    assert "chown -R 4020:4020 /run/layerx" in entrypoint
    assert "su-exec 4020:4020 /usr/local/bin/layerx-faucet" in entrypoint
    assert "LAYERX_FAUCET_LISTEN=\"[::]:${PORT:-9443}\"" in entrypoint

    source = SOURCE.read_text()
    assert 'request.path == "/livez"' in source
    assert 'request.path == "/readyz"' in source
    assert 'LAYERX_FAUCET_LISTEN = "[::]:${{PORT}}"' in DEFINITION.read_text()
