"""Checks the interop project definition against its image and service sources."""

from __future__ import annotations

import ast
import fnmatch
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/interop/interop.kvx"
DOCKERFILE = ROOT / "docker/interop-gateway/Dockerfile"
FILE_TABLE = ROOT / "docker/interop-gateway/files.tsv"
CONFIG_SOURCE = ROOT / "services/interop/crates/layerx-interop-service/src/config.rs"
GATEWAY_ROUTES = ROOT / "services/interop/crates/layerx-interop-gateway/src/server.rs"
ENTRYPOINT = ROOT / "docker/common/env-files.sh"


def _parse_kvx(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    section: dict[str, object] | None = None
    pending_key: str | None = None
    pending_value: list[str] = []
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if pending_key is not None:
            pending_value.append(stripped)
            if stripped.endswith("]"):
                assert section is not None
                section[pending_key] = ast.literal_eval(" ".join(pending_value))
                pending_key = None
                pending_value = []
            continue
        if stripped.startswith("[") and stripped.endswith("]"):
            section = sections.setdefault(stripped[1:-1], {})
            continue
        assert section is not None, f"key outside section: {line}"
        key, raw = (part.strip() for part in stripped.split("=", 1))
        if raw.startswith("[") and not raw.endswith("]"):
            pending_key = key
            pending_value = [raw]
        else:
            section[key] = ast.literal_eval(raw)
    assert pending_key is None, f"unterminated value: {pending_key}"
    return sections


def _source_env_names() -> set[str]:
    text = CONFIG_SOURCE.read_text()
    names = set(
        re.findall(
            r'(?:env::var|env::var_os|read_file|read_secret|required_label)'
            r'\(\s*"([A-Z][A-Z0-9_]*)"',
            text,
        )
    )
    names.update(
        re.findall(
            r'\["(LAYERX_INTEROP_[A-Z0-9_]+)"(?:,\s*"[A-Z0-9_]+")?\]', text
        )
    )
    for body in re.split(r"\n(?=(?:pub )?fn )", text):
        for binding, name in re.findall(
            r'\blet\s+(\w+)\s*=\s*"([A-Z][A-Z0-9_]*)"\s*;', body
        ):
            if re.search(
                rf"\benv::(?:var|var_os)\(\s*{re.escape(binding)}\s*\)", body
            ):
                names.add(name)
    return names


def _file_table_names() -> set[str]:
    names = set()
    for line in FILE_TABLE.read_text().splitlines():
        if line and not line.startswith("#"):
            name, _, _, roles = line.split("\t")
            assert "*" in roles.split(",")
            names.add(name)
    return names


def _copy_sources() -> set[str]:
    sources = set()
    for line in DOCKERFILE.read_text().splitlines():
        match = re.match(r"COPY(?:\s+--[^ ]+)*\s+([^ ]+)(?:\s+[^ ]+)?$", line)
        if match and not match.group(1).startswith("--"):
            source = match.group(1)
            if source != "." and not source.startswith("/src/"):
                sources.add(source)
    return sources


def test_interop_definition_matches_sources() -> None:
    sections = _parse_kvx(DEFINITION)
    service = sections["service.interop"]
    variables = sections["vars.interop"]
    inputs = sections["inputs.interop"]

    assert service["kind"] == "repo"
    container = ROOT / str(service["dockerfile"])
    assert container.is_file()
    assert str(service["root"]) == "."
    watch = [str(pattern) for pattern in service["watch"]]
    assert all(
        (ROOT / pattern.lstrip("/").split("*", 1)[0]).exists() for pattern in watch
    )
    assert all(
        any(fnmatch.fnmatch(source, pattern.lstrip("/")) for pattern in watch)
        for source in _copy_sources()
    )
    assert "/docker/interop-gateway/**" in watch
    assert "/docker/common/**" in watch
    assert "/services/interop/**" in watch
    assert service["health"] in GATEWAY_ROUTES.read_text()
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["order"] == 11
    assert set(service["needs"]) == {"gateway", "redis-internal", "sequencer-primary"}

    assert variables["PORT"] == "9443"
    assert variables["LAYERX_INTEROP_LISTEN"] == "[::]:${{PORT}}"

    source_names = _source_env_names()
    source_text = CONFIG_SOURCE.read_text()
    if variables.get("LAYERX_INTEROP_LISTENER") == "plain":
        tls_names = set(
            re.findall(
                r"LISTENER_CERTIFICATE_VARIABLES:.*?=\s*\[([^]]+)\]",
                source_text,
                re.DOTALL,
            )
        )
        source_names -= set(
            re.findall(r'"(LAYERX_INTEROP_[A-Z0-9_]+)"', " ".join(tls_names))
        )
    migration = re.search(
        r"fn migration_v2_config\(\).*?(?=\n}\n\nfn )", source_text, re.DOTALL
    )
    if migration:
        optional_migration = re.search(
            r'env::var\("(LAYERX_INTEROP_[A-Z0-9_]+)"\).*?NotPresent',
            migration.group(0),
            re.DOTALL,
        )
        if optional_migration:
            source_names.discard(optional_migration.group(1))
    file_names = _file_table_names()
    entrypoint_names = set(
        re.findall(r"\$\{?([A-Z][A-Z0-9_]*)", ENTRYPOINT.read_text())
    )
    assert "role=${LAYERX_ROLE:-}" in ENTRYPOINT.read_text()
    assert "LAYERX_ROLE" not in variables
    expected_names = source_names | file_names | {"LAYERX_FILES_TABLE", "PORT"}
    declared = set(variables)
    assert expected_names <= declared
    assert declared <= expected_names
    assert file_names <= declared

    for name, value in variables.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert isinstance(value, str)
        if value.startswith("env:"):
            env_name = value[4:]
            assert env_name == f"INTEROP_INTEROP_{name}"
            assert env_name in inputs
        elif name in entrypoint_names:
            assert value

    assert set(inputs) == {
        value[4:]
        for value in variables.values()
        if isinstance(value, str) and value.startswith("env:")
    }
    assert set(inputs) <= {
        line.split("=", 1)[0].strip()
        for line in DEFINITION.read_text().splitlines()
        if line.strip() and not line.lstrip().startswith("#") and "=" in line
    }
