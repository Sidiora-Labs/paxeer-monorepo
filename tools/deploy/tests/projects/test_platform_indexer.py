import fnmatch
import json
import os
import re
import shlex
import subprocess
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/indexer.kvx"
CONTAINER = ROOT / "docker/platform-indexer"
PACKAGE = ROOT / "services/platform/hosted/indexer"


def document(path):
    return tomllib.loads(path.read_text())


def production(path):
    return re.split(r"#\[cfg\(test\)\]\s*mod\s", path.read_text())[0]


def rust_sources():
    pending = [PACKAGE / "src/main.rs"]
    sources = {}
    while pending:
        path = pending.pop()
        if path in sources:
            continue
        source = production(path)
        sources[path] = source
        for module in re.findall(r"\b(?:layerx_indexer|crate)::([a-z]\w*)", source):
            dependency = PACKAGE / "src" / f"{module}.rs"
            if dependency.is_file():
                pending.append(dependency)
    return sources


def variable_reads():
    reads, required = set(), set()
    sources = rust_sources()
    direct = re.compile(
        r'\b(?:lookup|(?:std::)?env::var(?:_os)?)\(\s*"([A-Z][A-Z0-9_]*)"\s*\)'
    )
    for source in sources.values():
        for match in direct.finditer(source):
            name = match[1]
            reads.add(name)
            expression = source[match.end() :].split(";", 1)[0]
            mandatory = ".ok_or" in expression or (
                "env::var" in match[0] and ".map_err" in expression
            )
            if (
                expression.lstrip().startswith(".")
                and mandatory
                and ".unwrap_or" not in expression
            ):
                required.add(name)
        reads.update(
            re.findall(
                r'\b(?:number|optional_number)\(\s*lookup,\s*"([A-Z][A-Z0-9_]*)"',
                source,
            )
        )
        for pair in re.findall(
            r'\bendpoint\(\s*"([A-Z][A-Z0-9_]*)",\s*"([A-Z][A-Z0-9_]*)"',
            source,
        ):
            reads.update(pair)
        for prefix in re.findall(
            r'lookup\(&format!\("([A-Z0-9_]+)\{key\}"\)\)', source
        ):
            keys = re.search(r"let keys = (\[[^;]+\]);", source)
            assert keys, "Formatted environment names must have a source key array"
            reads.update(prefix + key for key in json.loads(keys[1]))
        for prefix in re.findall(r'tls::server_config\("([A-Z0-9_]+)"\)', source):
            tls = production(ROOT / "services/platform/hosted/internal/src/tls.rs")
            body = tls.split("pub fn server_config(", 1)[1].split("\n///", 1)[0]
            for match in re.finditer(
                r'env::var\(format!\("\{prefix\}([A-Z0-9_]+)"\)\)', body
            ):
                name = prefix + match[1]
                reads.add(name)
                if body[match.end() :].lstrip().startswith(".map_err"):
                    required.add(name)

    scripts = [
        *CONTAINER.glob("*.sh"),
        PACKAGE / "env-init.sh",
        ROOT / "docker/common/env-files.sh",
    ]
    for path in scripts:
        source = "\n".join(
            line for line in path.read_text().splitlines()
            if not line.lstrip().startswith("#")
        )
        for match in re.finditer(
            r"\$(?:\{([A-Z][A-Z0-9_]*)([^}]*)\}|([A-Z][A-Z0-9_]*))", source
        ):
            name = match[1] or match[3]
            reads.add(name)
            if not any(
                operator in (match[2] or "") for operator in (":-", ":+", "-", "+")
            ):
                required.add(name)

    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, guest, mode, roles = line.split("\t")
            assert Path(guest).is_absolute()
            assert not mode or re.fullmatch(r"[0-7]{3,4}", mode)
            assert roles == "*"
            reads.add(name)
            required.add(name)
    for name in re.findall(
        r"^ARG\s+([A-Z][A-Z0-9_]*)", (CONTAINER / "Dockerfile").read_text(), re.M
    ):
        reads.add(name)
        required.add(name)
    assert reads and required
    return reads, required


def watched(path, patterns):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern) for pattern in patterns)


def cargo_sources():
    pending = [PACKAGE / "Cargo.toml"]
    seen = set()
    while pending:
        manifest = pending.pop().resolve()
        if manifest in seen:
            continue
        seen.add(manifest)
        data = document(manifest)
        workspace = next(
            ancestor / "Cargo.toml"
            for ancestor in manifest.parents
            if (ancestor / "Cargo.toml").is_file()
            and "workspace" in document(ancestor / "Cargo.toml")
        )
        inherited = document(workspace)["workspace"].get("dependencies", {})
        yield manifest
        yield workspace
        if workspace.with_name("Cargo.lock").is_file():
            yield workspace.with_name("Cargo.lock")
        if (manifest.parent / "src").is_dir():
            yield from (manifest.parent / "src").rglob("*")
        groups = [data, *data.get("target", {}).values()]
        for group in groups:
            for kind in ("dependencies", "build-dependencies"):
                for name, dependency in group.get(kind, {}).items():
                    if not isinstance(dependency, dict):
                        continue
                    base = manifest.parent
                    if dependency.get("workspace"):
                        dependency = inherited[name]
                        base = workspace.parent
                    if isinstance(dependency, dict) and "path" in dependency:
                        pending.append(base / dependency["path"] / "Cargo.toml")


def test_indexer_service_matches_rollout_and_storage():
    data = document(DEFINITION)
    assert set(data) == {"service", "vars", "inputs"}
    assert all(set(section) == {"indexer"} for section in data.values())
    service = data["service"]["indexer"]
    rollout = document(ROOT / "deploy/rollout.kvx")["service"]
    assert rollout["indexer"]["railway_service"] == "indexer"
    assert service["kind"] == "repo"
    assert service["health"] == rollout["indexer"]["health"]
    assert service["order"] == rollout["indexer"]["order"]
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 0
    assert service["root"] == "/"
    assert service["volume"] == "/data"
    example = (PACKAGE / "railway.env.example").read_text()
    assert "No public domain" in example and "one replica, volume at /data" in example
    assert service["needs"] == ["archive"]
    assert all(rollout[name]["order"] < service["order"] for name in service["needs"])
    variables = data["vars"]["indexer"]
    assert Path(variables["LAYERX_INDEXER_DB"]).is_relative_to(service["volume"])
    assert "backfill_cursors" in (PACKAGE / "env-init.sh").read_text()


def test_indexer_build_inputs_exist_and_are_watched():
    service = document(DEFINITION)["service"]["indexer"]
    dockerfile = ROOT / service["dockerfile"]
    assert dockerfile == CONTAINER / "Dockerfile" and dockerfile.is_file()
    patterns = service["watch"]
    assert patterns and all(pattern.startswith("/") for pattern in patterns)
    assert all(list(ROOT.glob(pattern.lstrip("/"))) for pattern in patterns)
    assert watched(dockerfile, patterns)
    ignore = dockerfile.with_name(
        dockerfile.name + ".dockerignore"
    ).read_text().splitlines()
    for line in dockerfile.read_text().splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        paths = [
            part for part in shlex.split(line)[1:] if not part.startswith("--")
        ][:-1]
        for relative in paths:
            path = ROOT / relative
            assert path.exists(), relative
            if relative == ".":
                continue
            assert watched(path, patterns), relative
            included = True
            for rule in ignore:
                if not rule or rule.startswith("#"):
                    continue
                pattern = rule.removeprefix("!").strip("/")
                ancestors = [
                    relative,
                    *(
                        parent.as_posix() for parent in Path(relative).parents
                        if parent != Path(".")
                    ),
                ]
                if any(
                    fnmatch.fnmatchcase(candidate, pattern) for candidate in ancestors
                ):
                    included = rule.startswith("!")
            assert included, relative
    for path in cargo_sources():
        assert path.exists() and watched(path, patterns), path.relative_to(ROOT)
    assert watched(
        ROOT / "precompiles", [pattern.removesuffix("/**") for pattern in patterns]
    )
    assert "COPY . ." in dockerfile.read_text()
    assert "package-abis.sh precompiles /abi" in dockerfile.read_text()


def test_indexer_variables_follow_source_reads():
    variables = document(DEFINITION)["vars"]["indexer"]
    reads, required = variable_reads()
    assert required <= variables.keys(), (
        f"Missing required rows: {sorted(required - variables.keys())}"
    )
    assert variables.keys() <= reads, (
        f"Rows without readers: {sorted(variables.keys() - reads)}"
    )
    assert all(isinstance(value, str) and value for value in variables.values())
    for name, value in variables.items():
        if not value.startswith("env:"):
            assert (
                value.startswith(("/", "${{"))
                or value.isdecimal()
                or re.fullmatch(r"\[::\]:\d+", value)
            ), name


def test_indexer_inputs_name_and_describe_each_external_value():
    data = document(DEFINITION)
    service_name = next(iter(data["service"]))
    prefix = f"{DEFINITION.parent.name}_{service_name}".upper().replace("-", "_")
    inputs = data["inputs"][service_name]
    expected = set()
    for variable, value in data["vars"][service_name].items():
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            assert name == f"{prefix}_{variable}"
            expected.add(name)
    assert set(inputs) == expected
    lines = DEFINITION.read_text().splitlines()
    for name, category in inputs.items():
        assert category in {"generated", "provider", "chain", "operator"}
        index = next(
            index for index, line in enumerate(lines) if line.startswith(name + " =")
        )
        assert index > 0 and lines[index - 1].startswith("# ")
        assert re.search(r"\b(as|format)\b", lines[index - 1])


def test_indexer_file_inputs_match_runtime_paths():
    variables = document(DEFINITION)["vars"]["indexer"]
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        name, guest, _, _ = line.split("\t")
        assert variables[name].startswith("env:")
        assert variables["LAYERX_" + name] == guest
    assert variables["LAYERX_INDEXER_HEALTH_SERVER_NAME"] == "${{RAILWAY_PRIVATE_DOMAIN}}"
    assert 'tls::server_config("LAYERX_INDEXER")' in production(PACKAGE / "src/main.rs")


def render_health(**overrides):
    variables = document(DEFINITION)["vars"]["indexer"]
    environment = {"PATH": os.environ["PATH"], **variables}
    environment["LAYERX_INDEXER_HEALTH_SERVER_NAME"] = "localhost"
    environment.update(overrides)
    return subprocess.run(
        ["sh", str(CONTAINER / "health-config.sh")],
        env=environment,
        text=True,
        capture_output=True,
        timeout=5,
    )


@pytest.mark.parametrize("port", ["1024", "8080", "65535"])
def test_indexer_http_health_reaches_verified_tls_route(port):
    result = render_health(PORT=port)
    assert result.returncode == 0, result.stderr
    service = document(DEFINITION)["service"]["indexer"]
    variables = document(DEFINITION)["vars"]["indexer"]
    tls_port = variables["LAYERX_INDEXER_LISTEN"].rsplit(":", 1)[1]
    assert f"listen [::]:{port} ipv6only=off;" in result.stdout
    assert f"location = {service['health']} {{" in result.stdout
    assert f'if path == "{service["health"]}"' in production(PACKAGE / "src/api.rs")
    assert f"proxy_pass https://127.0.0.1:{tls_port};" in result.stdout
    assert "proxy_http_version 1.1;" in result.stdout
    assert "proxy_ssl_verify on;" in result.stdout
    assert "proxy_ssl_trusted_certificate /run/indexer-health/ca.pem;" in result.stdout
    assert "proxy_ssl_name localhost;" in result.stdout
    assert "location / { return 404; }" in result.stdout


@pytest.mark.parametrize("port", ["", "0", "65536", "-1", "8095", "8080; return 200"])
def test_indexer_health_rejects_invalid_or_conflicting_ports(port):
    result = render_health(PORT=port)
    assert result.returncode != 0
    assert result.stdout == ""
    assert "port" in result.stderr.lower()


@pytest.mark.parametrize(
    "name", ["", "localhost;", "localhost\nserver {}", ".localhost", "localhost."]
)
def test_indexer_health_rejects_invalid_tls_names(name):
    result = render_health(LAYERX_INDEXER_HEALTH_SERVER_NAME=name)
    assert result.returncode != 0
    assert result.stdout == ""
    assert "name" in result.stderr.lower()
