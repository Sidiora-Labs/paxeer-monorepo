"""Checks the wallet gas-station hosting definition against its sources."""

from __future__ import annotations

import re
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/wallet/gas-station.kvx"
INIT = ROOT / "docker/gas-station/init.sh"
DOCKERFILE = ROOT / "docker/gas-station/Dockerfile"
ROLLOUT = ROOT / "deploy/rollout.kvx"


def _definition() -> dict:
    with DEFINITION.open("rb") as stream:
        return tomllib.load(stream)


def _runtime_sources() -> tuple[str, ...]:
    return (
        INIT.read_text(encoding="utf-8"),
        (ROOT / "services/interop/crates/layerx-gas-station/src/config.rs").read_text(
            encoding="utf-8"
        ),
        (ROOT / "services/interop/crates/layerx-gas-station/src/service.rs").read_text(
            encoding="utf-8"
        ),
        (ROOT / "services/interop/crates/layerx-gas-station/src/station.rs").read_text(
            encoding="utf-8"
        ),
    )


def _required_env_names(definition: dict) -> set[str]:
    """Derive required names from shell `need` calls and production env reads."""

    init, config, service, station = _runtime_sources()
    config = config.split("#[cfg(test)]", 1)[0]
    names = set(re.findall(r"^need ([A-Z][A-Z0-9_]*)\b", init, re.MULTILINE))
    names.update(re.findall(r'std::env::var(?:_os)?\("([A-Z][A-Z0-9_]*)"\)', config))
    names.update(re.findall(r'std::env::var(?:_os)?\("([A-Z][A-Z0-9_]*)"\)', service))
    names.update(re.findall(r'std::env::var(?:_os)?\("([A-Z][A-Z0-9_]*)"\)', station))

    # The init script validates the names selected by these two settings and
    # the binaries then read the selected variables for their signing keys.
    variables = definition["vars"]["gas-station"]
    for selector in re.findall(r'^need "\$([A-Z][A-Z0-9_]*)"', init, re.MULTILINE):
        selected = variables.get(selector)
        if isinstance(selected, str) and re.fullmatch(r"[A-Z][A-Z0-9_]*", selected):
            names.add(selected)
    return names


def _paths_cover(patterns: list[str], path: str) -> bool:
    for pattern in patterns:
        if pattern.endswith("/**") and (path == pattern[:-3] or path.startswith(pattern[:-2])):
            return True
        if pattern == path:
            return True
    return False


def test_wallet_gas_station_definition_matches_rollout_and_image() -> None:
    definition = _definition()
    service = definition["service"]["gas-station"]
    rollout = ROLLOUT.read_text(encoding="utf-8")

    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/gas-station/Dockerfile"
    assert service["root"] == "."
    assert DOCKERFILE.is_file()
    assert _paths_cover(service["watch"], "docker/gas-station/Dockerfile")
    assert _paths_cover(service["watch"], "docker/gas-station/init.sh")
    assert _paths_cover(service["watch"], "docker/common/env-files.sh")
    assert _paths_cover(service["watch"], "services/interop/Cargo.toml")
    assert _paths_cover(service["watch"], "services/interop/crates/layerx-gas-station/src/main.rs")
    assert _paths_cover(service["watch"], "services/agent/crates/layerx-types/src/lib.rs")
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["volume"] == "/data"
    assert service["public"] == 1
    assert service["order"] == 11
    assert service["needs"] == []
    assert re.search(r"\[service\.gas-station\][\s\S]*?health = \"/healthz\"", rollout)

    service_source = (ROOT / "services/interop/crates/layerx-gas-station/src/service.rs").read_text(
        encoding="utf-8"
    )
    assert 'head.path == "/healthz"' in service_source


def test_wallet_gas_station_variables_are_source_backed_and_inputs_are_named() -> None:
    definition = _definition()
    variables = definition["vars"]["gas-station"]
    inputs = definition["inputs"]["gas-station"]
    required = _required_env_names(definition)

    assert required <= set(variables)
    assert variables
    env_names = {
        value.removeprefix("env:")
        for value in variables.values()
        if isinstance(value, str) and value.startswith("env:")
    }
    assert env_names == set(inputs)
    for name, value in inputs.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert value in {"generated", "provider", "chain", "operator"}
    assert all(not name.startswith("GAS_STATION_") for name in inputs)

    expected_prefix = "WALLET_GAS_STATION_"
    assert all(name.startswith(expected_prefix) for name in inputs)
    assert variables["GAS_STATION_RELAYER_KEY_ENV"] == "GAS_STATION_RELAYER_KEY"
    assert variables["GAS_STATION_RATE_OWNER_KEY_ENV"] == "GAS_STATION_RATE_OWNER_KEY"


def test_wallet_gas_station_definition_contains_no_deployment_values() -> None:
    definition_text = DEFINITION.read_text(encoding="utf-8")
    assert "https://" not in definition_text
    assert not re.search(r"0x[0-9a-fA-F]{40}", definition_text)
    assert not re.search(r"(?:0x)?[0-9a-fA-F]{64}", definition_text)
