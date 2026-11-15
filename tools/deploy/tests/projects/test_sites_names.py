from __future__ import annotations

import ast
import fnmatch
import re
import shlex
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
APP = ROOT / "apps" / "names"
DOCKERFILE = ROOT / "docker" / "names-web" / "Dockerfile"
DEFINITION = ROOT / "deploy" / "projects" / "sites" / "names.kvx"


def _sections(path: Path) -> tuple[dict[str, dict[str, object]], list[str]]:
    sections: dict[str, dict[str, object]] = {}
    raw = path.read_text().splitlines()
    current: dict[str, object] | None = None
    for line in raw:
        match = re.fullmatch(r"\[([^]]+)\]", line.strip())
        if match:
            current = sections.setdefault(match.group(1), {})
            continue
        if current is None or not line.strip() or line.lstrip().startswith("#"):
            continue
        key, separator, value = line.partition("=")
        if not separator:
            continue
        current[key.strip()] = ast.literal_eval(value.strip())
    return sections, raw


def _source_env_names() -> set[str]:
    names: set[str] = set()
    for source in APP.joinpath("src").rglob("*"):
        if source.suffix not in {".ts", ".tsx", ".js", ".mjs", ".cjs"}:
            continue
        text = source.read_text()
        names.update(re.findall(r"process\.env\.([A-Z][A-Z0-9_]*)", text))
        names.update(re.findall(r"process\.env\[['\"]([A-Z][A-Z0-9_]*)['\"]\]", text))
        names.update(re.findall(r"\brequired\(['\"]([A-Z][A-Z0-9_]*)['\"]\)", text))
    return names


def _docker_copy_sources() -> set[str]:
    sources: set[str] = set()
    for line in DOCKERFILE.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        instruction, _, rest = line.partition(" ")
        if instruction.upper() != "COPY":
            continue
        words = shlex.split(rest)
        if any(word.startswith("--from=") for word in words):
            continue
        while words and words[0].startswith("--"):
            words.pop(0)
        if len(words) < 2:
            continue
        sources.update(words[:-1])
    return sources


def _preceded_by_comment(raw: list[str], key: str) -> bool:
    for index, line in enumerate(raw):
        if re.match(rf"^{re.escape(key)}\s*=", line):
            previous = index - 1
            while previous >= 0 and not raw[previous].strip():
                previous -= 1
            return previous >= 0 and raw[previous].lstrip().startswith("#")
    return False


def test_names_project_definition() -> None:
    sections, raw = _sections(DEFINITION)
    service = sections["service.names"]
    variables = sections["vars.names"]
    inputs = sections["inputs.names"]

    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/names-web/Dockerfile"
    assert service["root"] == "/"
    assert service["health"] == "/"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["needs"] == []

    watch = service["watch"]
    assert isinstance(watch, list)
    assert all(isinstance(pattern, str) for pattern in watch)
    assert DOCKERFILE.exists()
    assert all((ROOT / source).exists() for source in _docker_copy_sources())
    for source in {"docker/names-web/Dockerfile", *(_docker_copy_sources())}:
        candidates = {source}
        if (ROOT / source).is_dir():
            candidates.add(f"{source}/.sentinel")
        assert any(
            fnmatch.fnmatch(candidate, pattern)
            for candidate in candidates
            for pattern in watch
        ), f"Build-context source is not watched: {source}"

    source_names = _source_env_names()
    docker_args = set(
        re.findall(r"^\s*ARG\s+([A-Z][A-Z0-9_]*)", DOCKERFILE.read_text(), re.MULTILINE)
    )
    runtime = re.split(r"^FROM\s+", DOCKERFILE.read_text(), flags=re.MULTILINE)[-1]
    env_match = re.search(r"\b(PORT)=(\d+)\b", runtime)
    assert env_match
    expected_names = source_names | docker_args | {env_match.group(1)}
    assert set(variables) == expected_names
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in variables)

    for name in source_names | docker_args:
        assert name in variables
    for name, value in variables.items():
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            env_name = value[4:]
            assert env_name in inputs
            assert env_name == f"SITES_NAMES_{name}"
    assert set(inputs) == {
        value[4:]
        for value in variables.values()
        if isinstance(value, str) and value.startswith("env:")
    }
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in inputs)
    assert all(inputs[name] in {"generated", "provider", "chain", "operator"} for name in inputs)
    assert all(_preceded_by_comment(raw, name) for name in inputs)

    assert variables[env_match.group(1)] == env_match.group(2)
    assert re.search(
        rf"^EXPOSE\s+{re.escape(env_match.group(2))}\s*$", runtime, re.MULTILINE
    )
