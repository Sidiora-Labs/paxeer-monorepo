"""Checks the hosting definition against the dashboard's runtime sources."""

from __future__ import annotations

import ast
import fnmatch
import re
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/dashboard.kvx"
DOCKERFILE = ROOT / "docker/platform-dashboard/Dockerfile"
FILES_TABLE = ROOT / "docker/platform-dashboard/files.tsv"
PROJECT_PREFIX = "PLATFORM_DASHBOARD_"
INPUT_WORDS = {"generated", "provider", "chain", "operator"}


def parse_value(text: str) -> object:
    text = text.strip()
    if text.startswith('"'):
        return ast.literal_eval(text)
    if text.startswith("["):
        return ast.literal_eval(text)
    if re.fullmatch(r"-?\d+", text):
        return int(text)
    raise AssertionError(f"unsupported KVX value: {text}")


def parse_kvx(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    current: dict[str, object] | None = None
    for number, raw in enumerate(path.read_text().splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        match = re.fullmatch(r"\[([A-Za-z0-9_.-]+)\]", line)
        if match:
            if match.group(1) in sections:
                raise AssertionError(f"duplicate section at {path}:{number}")
            current = {}
            sections[match.group(1)] = current
            continue
        if current is None or "=" not in line:
            raise AssertionError(f"invalid KVX line at {path}:{number}")
        key, value = line.split("=", 1)
        key = key.strip()
        if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key):
            raise AssertionError(f"invalid key at {path}:{number}")
        if key in current:
            raise AssertionError(f"duplicate key at {path}:{number}")
        current[key] = parse_value(value)
    return sections


def function_body(source: str, name: str) -> str:
    match = re.search(rf"(?:pub(?:\([^)]*\))?\s+)?fn\s+{name}\s*\([^{{]*\)\s*(?:->[^{{]+)?\{{", source)
    if not match:
        raise AssertionError(f"function {name} is missing")
    depth = 0
    for index in range(match.end() - 1, len(source)):
        if source[index] == "{":
            depth += 1
        elif source[index] == "}":
            depth -= 1
            if depth == 0:
                return source[match.end() - 1 : index + 1]
    raise AssertionError(f"function {name} is unterminated")


def env_names(source: str) -> set[str]:
    return set(re.findall(r'"([A-Z][A-Z0-9_]+)"', source))


def source_variables() -> set[str]:
    names: set[str] = set()
    for path in (ROOT / "services/platform/hosted/dashboard/src").glob("*.rs"):
        names.update(env_names(path.read_text()))

    webhooks = ROOT / "services/platform/hosted/webhooks/src"
    hosted = webhooks / "hosted.rs"
    hosted_source = hosted.read_text()
    names.update(env_names(function_body(hosted_source, "from_environment")))
    retry = function_body(hosted_source, "configured_retry_policy")
    optional = set(re.findall(r'\b(?:read|number)\(\s*"([A-Z][A-Z0-9_]+)"\s*,', retry))
    names.difference_update(optional)

    trusted = (webhooks / "trusted.rs").read_text()
    names.update(env_names(function_body(trusted, "from_dashboard_environment")))
    return {
        name
        for name in names
        if name.startswith(("LAYERX_DASHBOARD_", "LAYERX_WEBHOOKS_"))
    }


def table_variables() -> set[str]:
    return {
        row.split("\t", 1)[0]
        for row in FILES_TABLE.read_text().splitlines()
        if row and not row.startswith("#")
    }


def docker_copy_sources() -> list[str]:
    sources: list[str] = []
    for raw in DOCKERFILE.read_text().splitlines():
        fields = raw.split()
        if not fields or fields[0] != "COPY" or any(
            field == "--from=build" for field in fields[1:]
        ):
            continue
        fields = [field for field in fields[1:] if not field.startswith("--")]
        sources.extend(fields[:-1])
    return sources


def cargo_source_directories() -> set[Path]:
    docker_text = DOCKERFILE.read_text()
    manifest = re.search(r"--manifest-path\s+(\S+)", docker_text)
    assert manifest, "the build recipe must name its Cargo manifest"
    manifest_path = ROOT / manifest[1]
    assert manifest_path.is_file(), manifest_path
    workspace = tomllib.loads(manifest_path.read_text())["workspace"]
    pending = [manifest_path]
    pending.extend(manifest_path.parent / member / "Cargo.toml" for member in workspace["members"])
    directories: set[Path] = set()
    visited: set[Path] = set()
    while pending:
        path = pending.pop().resolve()
        if path in visited:
            continue
        visited.add(path)
        assert path.is_file(), path
        directories.add(path.parent)
        cargo = tomllib.loads(path.read_text())
        groups = [cargo.get("dependencies", {}), cargo.get("build-dependencies", {})]
        groups.append(cargo.get("workspace", {}).get("dependencies", {}))
        for group in groups:
            for dependency in group.values():
                if isinstance(dependency, dict) and "path" in dependency:
                    pending.append(path.parent / dependency["path"] / "Cargo.toml")
    return directories


def test_platform_dashboard_health_port_matches_listener() -> None:
    variables = parse_kvx(DEFINITION)["vars.dashboard"]
    listener = variables["LAYERX_DASHBOARD_LISTEN"]
    assert isinstance(listener, str)
    assert variables["PORT"] == listener.rsplit(":", 1)[1] == "9445"


def test_platform_dashboard_definition() -> None:
    sections = parse_kvx(DEFINITION)
    service = sections["service.dashboard"]
    variables = sections["vars.dashboard"]
    inputs = sections["inputs.dashboard"]

    assert service["kind"] == "repo"
    dockerfile = ROOT / str(service["dockerfile"])
    assert dockerfile == DOCKERFILE and dockerfile.is_file()
    assert (ROOT / str(service["root"])).is_dir()
    watch = service["watch"]
    assert isinstance(watch, list) and watch
    for pattern in watch:
        assert any(fnmatch.fnmatch(path.as_posix(), str(pattern)) for path in [
            Path(str(pattern).removesuffix("/**")),
            Path(str(pattern).removesuffix("/**")) / "Dockerfile",
        ]), f"watch pattern does not cover a source path: {pattern}"
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["order"] == 8
    assert service["needs"] == ["redis-gateway", "redis-internal", "identity"]

    expected = source_variables() | table_variables() | {"PORT"}
    if variables.get("LAYERX_DASHBOARD_LISTENER") == "plain":
        dashboard_main = (ROOT / "services/platform/hosted/dashboard/src/main.rs").read_text()
        expected -= env_names(function_body(dashboard_main, "tls_config"))
    assert expected <= set(variables), sorted(expected - set(variables))
    assert set(variables) <= expected, sorted(set(variables) - expected)

    env_rows = {value[4:] for value in variables.values() if isinstance(value, str) and value.startswith("env:")}
    assert env_rows <= set(inputs)
    assert all(name.startswith(PROJECT_PREFIX) for name in env_rows)
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in env_rows)
    assert all(value in INPUT_WORDS for value in inputs.values())

    definition_text = DEFINITION.read_text().splitlines()
    for index, line in enumerate(definition_text):
        if any(line.startswith(name + " =") for name in inputs):
            assert index and definition_text[index - 1].lstrip().startswith("#")

    patterns = [str(pattern).lstrip("/") for pattern in watch]

    def watched(path: Path) -> bool:
        return any(fnmatch.fnmatchcase(path.as_posix(), pattern) for pattern in patterns)

    recipe_directories = {DOCKERFILE.parent}
    for source in docker_copy_sources():
        path = Path(source)
        if source in (".", "..") or path.is_absolute():
            continue
        source_path = ROOT / path
        assert source_path.exists(), source
        assert watched(path), path
        recipe_directories.add(source_path if source_path.is_dir() else source_path.parent)
    for directory in recipe_directories:
        for path in directory.rglob("*"):
            if path.is_file():
                assert watched(path.relative_to(ROOT)), path

    for directory in cargo_source_directories():
        manifest = directory.relative_to(ROOT) / "Cargo.toml"
        assert watched(manifest), directory

