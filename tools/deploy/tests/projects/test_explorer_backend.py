"""The explorer backend definition follows its release and environment readers."""

import fnmatch
import re
import shlex
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/explorer/backend.kvx"
CONTAINER = ROOT / "docker/explorer-backend"
SOURCE = ROOT / "services/explorer/backend"
ENV_NAME = re.compile(r"[A-Z][A-Z0-9_]*\Z")


def definition():
    return tomllib.loads(DEFINITION.read_text())


def source_environment():
    """Collect names from production configuration rather than maintaining a catalogue."""
    paths = [
        SOURCE / "config/config_helper.exs",
        SOURCE / "config/runtime.exs",
        SOURCE / "config/runtime/prod.exs",
        SOURCE / "apps/block_scout_web/config/config.exs",
        SOURCE / "apps/explorer/config/config.exs",
    ]
    paths.extend(SOURCE.glob("apps/*/config/prod/*.exs"))
    paths.extend(SOURCE.glob("apps/*/config/prod.exs"))
    paths.extend(SOURCE.glob("apps/*/config/*.exs"))
    paths.append(SOURCE / "apps/block_scout_web/lib/block_scout_web/endpoint.ex")
    paths.append(SOURCE / "apps/block_scout_web/lib/block_scout_web/health_endpoint.ex")
    reads = set()
    required = set()
    call = re.compile(
        r"(?:System\.get_env|safe_get_env|parse_[a-z_]+_env_var)\s*\(\s*"
        r'"([A-Z][A-Z0-9_]*)"'
    )
    for path in paths:
        text = path.read_text()
        reads.update(match.group(1) for match in call.finditer(text))
        required.update(
            match.group(1)
            for match in re.finditer(
                r"System\.get_env\(\s*\"([A-Z][A-Z0-9_]*)\"\s*\)\s*\|\|\s*raise",
                text,
            )
        )

    helper = (SOURCE / "config/config_helper.exs").read_text()
    reads.update(re.findall(r'defp?\s+define_urls_vars\([^)]*\).*?\{"([A-Z][A-Z0-9_]*)",\s*"([A-Z][A-Z0-9_]*)"\}', helper, re.DOTALL))
    reads = {item for value in reads for item in (value if isinstance(value, tuple) else (value,))}
    required.add("DATABASE_URL")
    required.add("SECRET_KEY_BASE")
    required.add("ETHEREUM_JSONRPC_HTTP_URL")
    required.add("PORT")
    return reads, required


def container_environment():
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    arguments = set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)", dockerfile, re.MULTILINE))
    return arguments, arguments


def watched(path, patterns):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(
        fnmatch.fnmatchcase(relative, pattern)
        or fnmatch.fnmatchcase(relative + "/", pattern)
        for pattern in patterns
    )


def test_service_container_and_watch_paths():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == {DEFINITION.stem}
    service = data["service"][DEFINITION.stem]
    assert service["kind"] == "repo"
    assert service["root"] == "/"
    assert ROOT / service["dockerfile"] == CONTAINER / "Dockerfile"
    assert service["health"] == "/api/health/liveness"
    assert service["restart"] == "ON_FAILURE"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["order"] == 1
    assert service["needs"] == []

    patterns = service["watch"]
    assert patterns and all(pattern.startswith("/") and pattern.endswith("/**") for pattern in patterns)
    for pattern in patterns:
        assert list(ROOT.glob(pattern.lstrip("/"))), pattern
    assert watched(CONTAINER, patterns)

    dockerfile = (CONTAINER / "Dockerfile").read_text()
    for line in dockerfile.splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        words = [word for word in shlex.split(line)[1:] if not word.startswith("--")]
        for source in words[:-1]:
            path = ROOT / source
            assert path.exists(), source
            assert watched(path, patterns), source
    assert 'CMD ["/bin/sh", "-c", "bin/blockscout eval' in dockerfile
    assert "USER ${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP}" in dockerfile


def test_variables_follow_the_source_readers():
    data = definition()
    variables = data["vars"][DEFINITION.stem]
    source_reads, source_required = source_environment()
    container_reads, container_required = container_environment()
    reads = source_reads | container_reads
    required = source_required | container_required
    assert required <= variables.keys(), sorted(required - variables.keys())
    assert variables.keys() <= reads, sorted(variables.keys() - reads)
    assert "PORT" in variables and variables["PORT"].isdecimal()
    assert 1 <= int(variables["PORT"]) <= 65535
    for name, value in variables.items():
        assert ENV_NAME.fullmatch(name)
        assert isinstance(value, str) and value


def test_external_inputs_are_named_and_described():
    data = definition()
    variables = data["vars"][DEFINITION.stem]
    inputs = data["inputs"][DEFINITION.stem]
    expected_prefix = "EXPLORER_BACKEND_"
    referenced = set()
    for variable, value in variables.items():
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            assert name == expected_prefix + variable
            referenced.add(name)
    assert set(inputs) == referenced
    lines = DEFINITION.read_text().splitlines()
    for name, category in inputs.items():
        assert category in {"generated", "provider", "chain", "operator"}
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 5


def test_health_route_and_port_match_the_release():
    data = definition()
    service = data["service"][DEFINITION.stem]
    variables = data["vars"][DEFINITION.stem]
    endpoint = (SOURCE / "apps/block_scout_web/lib/block_scout_web/endpoint.ex").read_text()
    assert 'System.get_env("PORT") || raise' in endpoint
    router = (SOURCE / "apps/block_scout_web/lib/block_scout_web/health_router.ex").read_text()
    assert 'get("/liveness"' in router
    api_router = (SOURCE / "apps/block_scout_web/lib/block_scout_web/routers/api_router.ex").read_text()
    assert 'scope "/health"' in api_router
    assert service["health"] == "/api/health/liveness"
    assert variables["PORT"].isdecimal()
