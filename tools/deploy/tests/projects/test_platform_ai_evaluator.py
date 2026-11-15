"""Checks the hosting definition against the evaluator sources."""

from __future__ import annotations

import ast
import fnmatch
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/ai-evaluator.kvx"
SERVICE_SOURCE = ROOT / "services/platform/hosted/ai-evaluator/src/main.rs"
EVM_SENDER_SOURCE = ROOT / "services/platform/hosted/internal/src/evm_sender.rs"
TLS_SOURCE = ROOT / "services/platform/hosted/internal/src/tls.rs"
ROLLOUT = ROOT / "deploy/rollout.kvx"
DOCKERFILE = ROOT / "docker/ai-evaluator/Dockerfile"


def read_kvx(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    section: dict[str, object] | None = None
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        match = re.fullmatch(r"\[([^]]+)\]", line)
        if match:
            section = sections.setdefault(match.group(1), {})
            continue
        if section is None or "=" not in line:
            raise AssertionError(f"invalid KVX line in {path}: {raw}")
        name, raw_value = (part.strip() for part in line.split("=", 1))
        section[name] = ast.literal_eval(raw_value)
    return sections


def env_names_from_sources() -> set[str]:
    main = SERVICE_SOURCE.read_text()
    names = set(re.findall(r'"((?:LAYERX|PORT)[A-Z0-9_]*)"', main))
    prefixes = set(re.findall(r'const EVM_SENDER_PREFIX: &str = "([A-Z0-9_]+)"', main))
    sender = EVM_SENDER_SOURCE.read_text()
    for prefix in prefixes:
        names.update(
            f"{prefix}_{suffix}"
            for suffix in re.findall(rf"\{{prefix\}}_([A-Z][A-Z0-9_]*)", sender)
        )
    tls = TLS_SOURCE.read_text()
    start = tls.index("pub fn from_environment(prefix: &str)")
    end = tls.index("\n    }\n}", start)
    names.update(
        f"{prefix}_{suffix}"
        for prefix in prefixes
        for suffix in re.findall(rf"\{{prefix\}}_([A-Z][A-Z0-9_]*)", tls[start:end])
    )
    names.difference_update(prefixes)
    return {name for name in names if name == "PORT" or name.startswith("LAYERX_")}


def input_name(service: str, variable: str) -> str:
    return "_".join(("PLATFORM", service, variable)).upper().replace("-", "_")


def test_platform_ai_evaluator_definition() -> None:
    sections = read_kvx(DEFINITION)
    service = sections["service.ai-evaluator"]
    variables = sections["vars.ai-evaluator"]
    inputs = sections["inputs.ai-evaluator"]

    assert service["kind"] == "repo"
    dockerfile = ROOT / str(service["dockerfile"])
    assert dockerfile == DOCKERFILE and dockerfile.is_file()
    assert service["root"] == "."
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["volume"] == "/var/lib/layerx/ai-evaluator"
    assert service["order"] == 12
    assert service["needs"] == ["router"]
    assert "public" not in service

    watch = [str(pattern) for pattern in service["watch"]]
    for path in (
        "docker/ai-evaluator/Dockerfile",
        "docker/common/env-files.sh",
        "services/platform/hosted/ai-evaluator/src/main.rs",
        "services/platform/hosted/feeder/src/lib.rs",
        "services/platform/hosted/internal/src/evm_sender.rs",
        "programs/crates/layerx-programs-ai-market/src/lib.rs",
    ):
        assert (ROOT / path).exists()
        assert any(fnmatch.fnmatch(path, pattern) for pattern in watch)

    env_names = env_names_from_sources()
    assert set(variables) == env_names
    assert set(inputs) == {input_name("ai-evaluator", name) for name in env_names}
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in inputs)
    assert all(str(value).startswith("env:") for value in variables.values())
    assert all(str(value)[4:] in inputs for value in variables.values())
    assert set(str(value) for value in inputs.values()) <= {
        "generated",
        "provider",
        "chain",
        "operator",
    }

    rollout_services = {
        match.group(1)
        for match in re.finditer(r"^\[service\.([^]]+)\]$", ROLLOUT.read_text(), re.MULTILINE)
    }
    assert set(service["needs"]) <= rollout_services


def test_platform_ai_evaluator_container_contract() -> None:
    recipe = DOCKERFILE.read_text()
    bases = re.findall(r"^FROM\s+([^\s]+)", recipe, re.MULTILINE)
    assert bases and all("@sha256:" in base for base in bases)
    assert re.search(r"^USER\s+4020:4020\s*$", recipe, re.MULTILINE)
    assert "--package layerx-ai-evaluator --bin layerx-ai-evaluator" in recipe
    assert 'ENTRYPOINT ["/usr/local/bin/layerx-ai-evaluator"]' in recipe
    assert "EXPOSE 8080" in recipe
    assert "COPY . ." in recipe
