"""Check the hpx-registry project definition against its image sources."""

from __future__ import annotations

import ast
import fnmatch
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/hpx-registry.kvx"
DOCKERFILE = ROOT / "docker/hpx-registry/Dockerfile"
SOURCE_FILES = (
    ROOT / "tools/hpx/registry/main.go",
    ROOT / "docker/hpx-registry/entrypoint.sh",
    ROOT / "docker/search-front/real-ip.sh",
)
ENV_NAME = re.compile(r"^PLATFORM_HPX_REGISTRY_[A-Z0-9_]+$")


def _parse_kvx(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    section: dict[str, object] | None = None
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("[") and stripped.endswith("]"):
            section = sections.setdefault(stripped[1:-1], {})
            continue
        if section is None or "=" not in stripped:
            continue
        key, raw = (part.strip() for part in stripped.split("=", 1))
        section[key] = ast.literal_eval(raw)
    return sections


def _source_variables() -> set[str]:
    variables: set[str] = set()
    for line in DOCKERFILE.read_text(encoding="utf-8").splitlines():
        match = re.match(r"\s*ARG\s+([A-Z][A-Z0-9_]*)", line)
        if match:
            variables.add(match.group(1))

    go = SOURCE_FILES[0].read_text(encoding="utf-8")
    variables.update(
        re.findall(r'(?:os\.Getenv|env|envInt)\(\s*"([A-Z][A-Z0-9_]*)"', go)
    )
    for path in SOURCE_FILES[1:]:
        shell = path.read_text(encoding="utf-8")
        variables.update(re.findall(r"\$\{([A-Z][A-Z0-9_]*)", shell))
        variables.update(re.findall(r"\$([A-Z][A-Z0-9_]*)\b", shell))
    return variables


def _copied_sources() -> list[str]:
    copied: list[str] = []
    for line in DOCKERFILE.read_text(encoding="utf-8").splitlines():
        match = re.match(r"\s*COPY\s+(.*)$", line)
        if not match or "--from=" in match.group(1):
            continue
        sources = match.group(1).split()
        sources = [item for item in sources if not item.startswith("--")]
        if len(sources) > 1:
            copied.append(sources[-2])
    return copied


def test_platform_hpx_registry_definition_matches_sources() -> None:
    sections = _parse_kvx(DEFINITION)
    service = sections["service.hpx-registry"]
    variables = sections["vars.hpx-registry"]
    inputs = sections["inputs.hpx-registry"]

    assert service["kind"] == "repo"
    assert service["container"] == "docker/hpx-registry/Dockerfile"
    assert service["root"] == "."
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["volume"] == "/srv/hpx"
    assert service["public"] == 1
    assert service["watch"]
    for pattern in service["watch"]:
        assert isinstance(pattern, str)
        assert (ROOT / pattern.removesuffix("/**")).exists(), pattern

    source_variables = _source_variables()
    assert set(variables) == source_variables
    for name, value in variables.items():
        assert isinstance(value, str) and value.startswith("env:")
        env_name = value.removeprefix("env:")
        assert env_name == f"PLATFORM_HPX_REGISTRY_{name}"
        assert ENV_NAME.fullmatch(env_name)
        assert env_name in inputs
        assert inputs[env_name] in {"generated", "provider", "chain", "operator"}

    assert set(inputs) == {
        value.removeprefix("env:")
        for value in variables.values()
        if isinstance(value, str) and value.startswith("env:")
    }

    for copied in _copied_sources():
        assert any(fnmatch.fnmatch(copied, pattern) for pattern in service["watch"]), copied

    entrypoint = (ROOT / "docker/hpx-registry/entrypoint.sh").read_text(encoding="utf-8")
    proxy_helper = (ROOT / "docker/search-front/real-ip.sh").read_text(encoding="utf-8")
    template = (ROOT / "tools/hpx/hosting/nginx-registry.conf.template").read_text(
        encoding="utf-8"
    )
    main = (ROOT / "tools/hpx/registry/main.go").read_text(encoding="utf-8")
    assert "${PORT}" in template and "${HPX_ADDR}" in template
    assert 'mux.HandleFunc("/healthz"' in main
    assert '"${PORT}"' not in entrypoint
    assert "${PORT" in entrypoint and "${HPX_ADDR" in entrypoint
    assert "LAYERX_TRUSTED_PROXIES" in proxy_helper
    assert "PORT" in variables and "HPX_ADDR" in variables
