"""Check the hosted explorer frontend definition against its public sources."""

from __future__ import annotations

import ast
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/explorer/frontend.kvx"
DOCKERFILE = ROOT / "docker/explorer-frontend/Dockerfile"
ENTRYPOINT = ROOT / "apps/explorer/deploy/scripts/entrypoint.sh"
APP_SOURCES = ROOT / "apps/explorer"
REQUIRED_SCHEMA_SOURCES = (
    APP_SOURCES / "deploy/tools/envs-validator/schema.ts",
    APP_SOURCES / "deploy/tools/envs-validator/schema_multichain.ts",
    APP_SOURCES / "deploy/tools/envs-validator/schemas/apis.ts",
    APP_SOURCES / "deploy/tools/envs-validator/schemas/chain.ts",
)
ENV_NAME = re.compile(r"^[A-Z][A-Z0-9_]*$")
INPUT_NAME = re.compile(r"^EXPLORER_FRONTEND_[A-Z][A-Z0-9_]*$")
KV_HEAD = re.compile(r"^\[([a-z0-9_.-]+)\]$")
KV_LINE = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$")


def _parse_definition() -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    current: dict[str, object] | None = None
    for raw in DEFINITION.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        heading = KV_HEAD.fullmatch(line)
        if heading:
            current = sections.setdefault(heading.group(1), {})
            continue
        match = KV_LINE.fullmatch(line)
        assert match and current is not None, f"invalid definition line: {raw}"
        key, encoded = match.groups()
        current[key] = ast.literal_eval(encoded)
    return sections


def _source_env_names() -> set[str]:
    names: set[str] = set()
    source_files = [
        DOCKERFILE,
        ENTRYPOINT,
        APP_SOURCES / "deploy/scripts/validate_envs.sh",
        APP_SOURCES / "deploy/scripts/make_envs_script.sh",
        APP_SOURCES / "deploy/scripts/download_assets.sh",
        APP_SOURCES / "deploy/scripts/favicon_generator.sh",
        APP_SOURCES / "deploy/scripts/build_sprite.sh",
        APP_SOURCES / "deploy/scripts/og_image_generator.js",
        APP_SOURCES / "deploy/scripts/sitemap_generator.sh",
        APP_SOURCES / "deploy/tools/favicon-generator/index.js",
    ]
    source_files.extend(APP_SOURCES.rglob("*.ts"))
    source_files.extend(APP_SOURCES.rglob("*.tsx"))
    source_files.extend(APP_SOURCES.rglob("*.js"))
    for path in source_files:
        if not path.is_file() or "node_modules" in path.parts or ".next" in path.parts:
            continue
        text = path.read_text()
        names.update(re.findall(r"getEnvValue\(\s*['\"]([A-Z][A-Z0-9_]*)['\"]", text))
        names.update(re.findall(r"process\.env\.([A-Z][A-Z0-9_]*)", text))
        names.update(re.findall(r"process\.env\[['\"]([A-Z][A-Z0-9_]*)['\"]\]", text))
        names.update(re.findall(r"\b(?:export\s+)?(?:local\s+)?([A-Z][A-Z0-9_]*)=\$", text))
        names.update(re.findall(r"\$\{([A-Z][A-Z0-9_]*)", text))
    for path in REQUIRED_SCHEMA_SOURCES:
        text = path.read_text()
        names.update(re.findall(r"\b(NEXT_PUBLIC_[A-Z0-9_]+)\s*:\s*[^\n]*\.required\(\)", text))
    return names


def _docker_build_args() -> set[str]:
    return set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)\s*$", DOCKERFILE.read_text(), re.MULTILINE))


def _docker_runtime_names() -> set[str]:
    names = set(re.findall(r"^ENV\s+([A-Z][A-Z0-9_]*)\s*=", DOCKERFILE.read_text(), re.MULTILINE))
    names.update(re.findall(r"^ENV\s+([A-Z][A-Z0-9_]*)\s+", DOCKERFILE.read_text(), re.MULTILINE))
    return names


def test_frontend_service_matches_container_and_deployment_sources():
    sections = _parse_definition()
    assert set(sections) == {"service.frontend", "vars.frontend", "inputs.frontend"}
    service = sections["service.frontend"]
    assert service == {
        "kind": "repo",
        "dockerfile": "docker/explorer-frontend/Dockerfile",
        "root": ".",
        "watch": ["docker/explorer-frontend/**", "docker/common/**", "apps/explorer/**", "docs/explorer/ENVS.md"],
        "health": "/explorer/api/healthz",
        "restart": "ON_FAILURE",
        "replicas": 1,
        "public": 1,
        "order": 2,
        "needs": ["backend"],
    }
    for pattern in service["watch"]:
        assert list(ROOT.glob(pattern)), pattern
    assert (ROOT / service["dockerfile"]).is_file()
    dockerfile = DOCKERFILE.read_text()
    assert "EXPOSE 3000" in dockerfile
    assert 'ENV PORT=3000' in dockerfile
    assert 'COPY --chmod=755 apps/explorer/deploy/scripts/entrypoint.sh .' in dockerfile
    assert "/explorer/api/healthz" in (APP_SOURCES / "pages/api/healthz.tsx").read_text() or (
        (APP_SOURCES / "next.config.js").read_text().find("basePath: '/explorer'") >= 0
        and (APP_SOURCES / "pages/api/healthz.tsx").is_file()
    )


def test_frontend_variables_follow_source_reads_and_inputs():
    sections = _parse_definition()
    variables = sections["vars.frontend"]
    inputs = sections["inputs.frontend"]
    source_names = _source_env_names()
    required = {"ENVS_PRESET", "PORT", *_docker_build_args()}
    required.update(name for name in source_names if name in {"NEXT_PUBLIC_APP_HOST", "NEXT_PUBLIC_API_HOST", "NEXT_PUBLIC_NETWORK_ID", "NEXT_PUBLIC_NETWORK_NAME"})
    assert required <= variables.keys()
    assert set(variables) <= source_names | _docker_build_args() | _docker_runtime_names() | {"ENVS_PRESET", "PORT"}
    for name, value in variables.items():
        assert ENV_NAME.fullmatch(name), name
        assert isinstance(value, str)
        if value.startswith("env:"):
            input_name = value[4:]
            assert INPUT_NAME.fullmatch(input_name), input_name
            assert input_name in inputs
        elif value.startswith("${{"):
            assert value.endswith("}}"), value
    assert set(inputs)
    assert set(inputs) == {value[4:] for value in variables.values() if value.startswith("env:")}
    assert all(INPUT_NAME.fullmatch(name) for name in inputs)
    assert all(value in {"generated", "provider", "chain", "operator"} for value in inputs.values())
    definition_text = DEFINITION.read_text()
    for name in inputs:
        assert re.search(rf"(?m)^# .+\n{name} = \"(?:generated|provider|chain|operator)\"$", definition_text), name
    variable_text = definition_text.split("[vars.frontend]", 1)[1].split("[inputs.frontend]", 1)[0]
    assert not re.search(r"(?:password|secret|token|key)\s*=\s*\"(?!env:|\$\{\{)[^\"]+", variable_text, re.IGNORECASE)


def test_frontend_server_settings_use_deployment_inputs():
    sections = _parse_definition()
    variables = sections["vars.frontend"]
    inputs = sections["inputs.frontend"]
    expected = {
        "EXPLORER_INFERENCE_NODE_URL": (
            "EXPLORER_FRONTEND_EXPLORER_INFERENCE_NODE_URL", "chain"
        ),
        "EXPLORER_ASSISTANT_BASE_URL": (
            "EXPLORER_FRONTEND_EXPLORER_ASSISTANT_BASE_URL", "provider"
        ),
        "EXPLORER_ASSISTANT_API_KEY": (
            "EXPLORER_FRONTEND_EXPLORER_ASSISTANT_API_KEY", "provider"
        ),
    }
    assert expected.keys() <= _source_env_names()
    for name, (input_name, category) in expected.items():
        assert variables[name] == f"env:{input_name}"
        assert inputs[input_name] == category
    assert "EXPLORER_ASSISTANT_MODEL" not in variables
    assert not any("ASSISTANT_MODEL" in name for name in inputs)
