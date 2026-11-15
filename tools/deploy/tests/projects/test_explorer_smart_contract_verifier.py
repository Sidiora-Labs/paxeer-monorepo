"""Checks the explorer smart-contract verifier hosting definition."""

from __future__ import annotations

import re
import shlex
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/explorer/smart-contract-verifier.kvx"
DOCKERFILE = ROOT / "docker/explorer-smart-contract-verifier/Dockerfile"
SERVICE_ROOT = ROOT / "services/explorer/services/smart-contract-verifier"
CONFIG_ENV = SERVICE_ROOT / "smart-contract-verifier-server/config/base.env"
LAUNCHER_SETTINGS = (
    ROOT
    / "services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs"
)
HTTP_ROUTES = SERVICE_ROOT / "smart-contract-verifier-proto/proto/v2/api_config_http.yaml"


def _definition() -> dict:
    return tomllib.loads(DEFINITION.read_text())


def _source_env_names() -> set[str]:
    """Collect names exposed by the service's environment readers."""
    names: set[str] = set()
    for line in CONFIG_ENV.read_text().splitlines():
        match = re.match(r"^\s*#?\s*([A-Z][A-Z0-9_]*(?:__[A-Z0-9_]+)*)\s*=", line)
        if match:
            names.add(match.group(1))
    for path in SERVICE_ROOT.rglob("*.rs"):
        text = path.read_text(errors="replace")
        names.update(re.findall(r"(?:std::env|env)::var(?:_os)?\(\s*\"([A-Z][A-Z0-9_]*)\"", text))
    return names


def _required_explicit_env_names() -> set[str]:
    names: set[str] = set()
    for path in SERVICE_ROOT.rglob("*.rs"):
        text = path.read_text(errors="replace")
        for match in re.finditer(
            r"(?:std::env|env)::var(?:_os)?\(\s*\"([A-Z][A-Z0-9_]*)\"",
            text,
        ):
            following = text[match.end() : match.end() + 120]
            if re.search(r"\.(?:expect|unwrap)\s*\(", following):
                names.add(match.group(1))
    return names


def _copy_sources() -> set[str]:
    sources: set[str] = set()
    for raw in DOCKERFILE.read_text().splitlines():
        if not raw.lstrip().startswith("COPY "):
            continue
        tokens = shlex.split(raw, comments=True)
        tokens = [token for token in tokens[1:] if not token.startswith("--")]
        if len(tokens) < 2:
            continue
        source = tokens[-2]
        if not source.startswith("$") and not source.startswith("/"):
            sources.add(source.rstrip("/"))
    return sources


def _has_watch(definition: dict, path: str) -> bool:
    return any(path == watch.rstrip("/*") or path.startswith(watch.rstrip("*")) for watch in definition["watch"])


def _input_rows() -> tuple[dict, dict[str, str]]:
    definition = _definition()
    inputs = definition.get("inputs", {}).get("smart-contract-verifier", {})
    rows: dict[str, str] = {}
    section = DEFINITION.read_text().splitlines()
    in_section = False
    for index, line in enumerate(section):
        if line.strip() == "[inputs.smart-contract-verifier]":
            in_section = True
            continue
        if in_section and line.startswith("["):
            break
        if in_section:
            match = re.match(r"\s*([A-Z][A-Z0-9_]*)\s*=\s*\"([^\"]+)\"", line)
            if match:
                previous = next((item.strip() for item in reversed(section[:index]) if item.strip()), "")
                assert previous.startswith("#"), f"input {match.group(1)} needs a comment line"
                rows[match.group(1)] = match.group(2)
    return inputs, rows


def test_smart_contract_verifier_definition() -> None:
    definition = _definition()
    service = definition["service"]["smart-contract-verifier"]
    variables = definition["vars"]["smart-contract-verifier"]

    assert service["kind"] == "repo"
    assert service["branch"] == "main"
    assert (ROOT / service["container"]).is_file()
    assert (ROOT / service["root"]).is_dir()
    for watch in service["watch"]:
        prefix = re.split(r"[*?\[]", watch, maxsplit=1)[0].rstrip("/")
        assert (ROOT / prefix).exists(), f"watch path does not exist: {watch}"
    for copied in _copy_sources():
        assert _has_watch(service, copied), f"Dockerfile source is not watched: {copied}"

    env_names = _source_env_names()
    assert set(variables) <= env_names
    assert _required_explicit_env_names() <= set(variables)
    for argument in re.findall(r"^\s*ARG\s+([A-Z][A-Z0-9_]*)", DOCKERFILE.read_text(), re.MULTILINE):
        assert argument in variables

    project_name = definition.get("meta", {}).get("project", "paxeer-x-explorer")
    expected_prefix = f"{project_name}_{service['name'] if 'name' in service else 'smart-contract-verifier'}".upper().replace("-", "_")
    inputs, input_rows = _input_rows()
    referenced_inputs = {value.removeprefix("env:") for value in variables.values() if value.startswith("env:")}
    assert referenced_inputs == set(input_rows)
    assert referenced_inputs == set(inputs)
    for name in referenced_inputs:
        assert name.startswith(expected_prefix + "_")
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)

    assert service["health"] in re.findall(r"^\s*get:\s*(/[^\s#]+)", HTTP_ROUTES.read_text(), re.MULTILINE)
    address = variables["SMART_CONTRACT_VERIFIER__SERVER__HTTP__ADDR"]
    port = re.search(r":(\d+)$", address).group(1)
    assert re.search(rf"0\.0\.0\.0:{port}\b", LAUNCHER_SETTINGS.read_text())
    assert re.search(rf"^EXPOSE\s+{port}\s*$", DOCKERFILE.read_text(), re.MULTILINE)
    assert service["volume"] == "/tmp"
