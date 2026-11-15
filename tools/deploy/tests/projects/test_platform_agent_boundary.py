"""Check the platform agent-boundary definition against its build inputs."""

from __future__ import annotations

import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/agent-boundary.kvx"
DOCKERFILE = ROOT / "docker/platform-agent-boundary/Dockerfile"
SOURCE = ROOT / "services/platform/hosted/agent-boundary/src"
FILES_TABLE = ROOT / "docker/platform-agent-boundary/files.tsv"
INIT_SCRIPT = ROOT / "docker/platform-agent-boundary/init.sh"


def _value(text: str):
    text = text.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        if re.fullmatch(r"-?[0-9]+", text):
            return int(text)
        raise AssertionError(f"unsupported KVX value: {text}")


def _definition() -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    section: dict[str, object] | None = None
    for line_number, raw in enumerate(DEFINITION.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        match = re.fullmatch(r"\[([a-z0-9_.-]+)\]", line)
        if match:
            name = match.group(1)
            if name in sections:
                raise AssertionError(f"duplicate section at line {line_number}: {name}")
            section = sections[name] = {}
            continue
        if section is None:
            raise AssertionError(f"assignment outside a section at line {line_number}")
        key, separator, value = line.partition("=")
        if separator != "=":
            raise AssertionError(f"invalid assignment at line {line_number}")
        key = key.strip()
        if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key) or key in section:
            raise AssertionError(f"invalid or duplicate key at line {line_number}")
        section[key] = _value(value)
    return sections


def _rust_variables() -> tuple[set[str], set[str]]:
    required: set[str] = set()
    optional: set[str] = set()
    for source in SOURCE.glob("*.rs"):
        text = source.read_text(encoding="utf-8")
        for match in re.finditer(r'env::var\("([A-Z][A-Z0-9_]*)"\)', text):
            name = match.group(1)
            context = text[max(0, match.start() - 32) : match.start()]
            (optional if re.search(r"match\s*$", context) else required).add(name)
    return required, optional


def _file_inputs() -> set[str]:
    names: set[str] = set()
    for raw in FILES_TABLE.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if line and not line.startswith("#"):
            fields = line.split("\t")
            assert len(fields) == 4, f"file mapping is not four columns: {line!r}"
            names.add(fields[0])
    return names


def _shell_inputs() -> set[str]:
    text = INIT_SCRIPT.read_text(encoding="utf-8")
    return set(re.findall(r"\$\{?([A-Z][A-Z0-9_]*)", text))


def _build_arguments() -> set[str]:
    return set(re.findall(r"^\s*ARG\s+([A-Z][A-Z0-9_]*)", DOCKERFILE.read_text(encoding="utf-8"), re.MULTILINE))


def _covered(path: str, patterns: list[str]) -> bool:
    candidate = (ROOT / path).resolve()
    for pattern in patterns:
        base = (ROOT / pattern.removesuffix("/**")).resolve()
        if pattern.endswith("/**") and (candidate == base or base in candidate.parents):
            return True
        if not pattern.endswith("/**") and candidate == (ROOT / pattern).resolve():
            return True
    return False


def test_platform_agent_boundary_definition() -> None:
    sections = _definition()
    service = sections["service.agent-boundary"]
    variables = sections["vars.agent-boundary"]
    inputs = sections["inputs.agent-boundary"]

    assert service["kind"] == "repo"
    container = ROOT / str(service["container"])
    assert container == DOCKERFILE and container.is_file()
    assert service["root"] == "."
    patterns = service["watch"]
    assert isinstance(patterns, list) and patterns
    for pattern in patterns:
        assert isinstance(pattern, str)
        assert list(ROOT.glob(pattern)), f"watch pattern has no repository path: {pattern}"
    assert service["health"] == "/livez"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["order"] >= 1
    assert service["needs"] == ["identity", "archive"]

    required, optional = _rust_variables()
    source_variables = required | _file_inputs() | _shell_inputs() | _build_arguments()
    assert optional <= {"LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER"}
    assert required <= set(variables)
    assert set(variables) <= source_variables

    env_references = set()
    for name, value in variables.items():
        assert isinstance(name, str) and re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert isinstance(value, str)
        if value.startswith("env:"):
            env_name = value[4:]
            assert re.fullmatch(r"[A-Z][A-Z0-9_]*", env_name), env_name
            assert env_name in inputs, f"missing input row for {env_name}"
            env_references.add(env_name)
        else:
            assert value
    assert set(inputs) == env_references
    for name, word in inputs.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert word in {"generated", "provider", "chain", "operator"}

    docker = DOCKERFILE.read_text(encoding="utf-8")
    assert "docker/common/env-files.sh" in docker
    assert "docker/platform-agent-boundary/files.tsv" in docker
    assert "docker/platform-agent-boundary/init.sh" in docker
    assert "PORT" in INIT_SCRIPT.read_text(encoding="utf-8")
    assert "LAYERX_AGENT_BOUNDARY_LISTEN" in INIT_SCRIPT.read_text(encoding="utf-8")
    assert 'path == "/livez"' in (SOURCE / "main.rs").read_text(encoding="utf-8")

    for raw in docker.splitlines():
        if "--from=" in raw:
            continue
        match = re.match(r"\s*COPY\s+(?:--[^ ]+\s+)*(\S+)", raw)
        if match and not match.group(1).startswith("--") and match.group(1) != ".":
            source = match.group(1)
            if source.startswith("--from="):
                continue
            assert (ROOT / source).exists(), source
            assert _covered(source, patterns), f"Docker source is outside watch patterns: {source}"
