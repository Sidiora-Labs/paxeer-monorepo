"""The Paxeer boundary definition follows its executable and image sources."""

import ast
import base64
import configparser
import fnmatch
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tomllib


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/paxeer-boundary.kvx"
CONTAINER = ROOT / "docker/paxeer"
BOUNDARY = ROOT / "services/platform/hosted/paxeer"
SERVICE = "paxeer-boundary"
ENV_NAME = re.compile(r"[A-Z][A-Z0-9_]*\Z")


def definition():
    parser = configparser.ConfigParser(interpolation=None)
    parser.optionxform = str
    parser.read(DEFINITION)
    return {
        section: {key: ast.literal_eval(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def functions(source):
    masked = re.sub(
        r'r(?P<h>#{0,255})"[\s\S]*?"(?P=h)|b?\'(?:\\.|[^\'\\\n])\'|"(?:\\.|[^"\\])*"|//[^\n]*|/\*[\s\S]*?\*/',
        lambda match: " " * len(match[0]),
        source,
    )
    result = {}
    for match in re.finditer(r"\bfn\s+(\w+)(?:<[^;{}]*>)?\s*\([^{}]*\)[^;{}]*\{", masked):
        depth = 1
        end = match.end()
        while depth:
            assert end < len(masked), match[1]
            depth += (masked[end] == "{") - (masked[end] == "}")
            end += 1
        result[match[1]] = source[match.end():end - 1]
    return result


def source_variables():
    required = set()
    optional = set()
    for path in BOUNDARY.glob("src/*.rs"):
        source = path.read_text().split("#[cfg(test)]", 1)[0]
        names = set(re.findall(r'"(LAYERX_[A-Z0-9_]+)"', source))
        bodies = functions(source)
        for body in bodies.values():
            for match in re.finditer(
                r'\benv::var(?:_os)?\(\s*"([A-Z][A-Z0-9_]*)"\s*\)', body
            ):
                name = match[1]
                tail = body[match.end():]
                if re.match(r"\s*\.(?:unwrap_or(?:_else|_default)?|map_or_else)\b", tail):
                    optional.add(name)
                else:
                    required.add(name)
        required.update(names)
    return required - optional, optional


def file_rows():
    rows = []
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, path, mode, roles = line.split("\t")
            rows.append((name, path, mode, roles))
    return rows


def watched(path, patterns):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern) for pattern in patterns)


def cargo_manifests():
    pending = [ROOT / "services/platform/Cargo.toml"]
    seen = set()
    while pending:
        path = pending.pop().resolve()
        if path in seen:
            continue
        assert path.is_file(), path
        seen.add(path)
        document = tomllib.loads(path.read_text())
        workspace = document.get("workspace", {})
        for member in workspace.get("members", []):
            pending.extend(path.parent.glob(member + "/Cargo.toml"))
        for table in [document, workspace, *document.get("target", {}).values()]:
            for kind in ("dependencies", "dev-dependencies", "build-dependencies"):
                for dependency in table.get(kind, {}).values():
                    if isinstance(dependency, dict) and "path" in dependency:
                        pending.append(path.parent / dependency["path"] / "Cargo.toml")
    return seen


def test_paxeer_boundary_definition_matches_container_and_sources():
    document = definition()
    assert set(document) == {
        "service.paxeer-boundary", "vars.paxeer-boundary", "inputs.paxeer-boundary",
    }
    service = document["service." + SERVICE]
    variables = document["vars." + SERVICE]
    required, optional = source_variables()
    staged = {name for name, *_ in file_rows()}
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    build_args = set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)", dockerfile, re.MULTILINE))
    entrypoint = (CONTAINER / "entrypoint.sh").read_text()
    entrypoint_required = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*):\?", entrypoint))
    required |= entrypoint_required | build_args | staged
    assert required <= variables.keys(), sorted(required - variables.keys())
    assert variables.keys() <= required | optional | build_args | staged
    assert all(ENV_NAME.fullmatch(name) for name in variables)


def test_paxeer_boundary_inputs_have_names_and_comments():
    document = definition()
    variables = document["vars." + SERVICE]
    inputs = document["inputs." + SERVICE]
    used = set()
    for name, value in variables.items():
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            expected = f"PLATFORM_{SERVICE.upper().replace('-', '_')}_{name}"
            assert value == "env:" + expected
            used.add(expected)
        else:
            assert not re.search(r"(?:https?|rediss?)://(?!\$\{\{)", value)
    assert set(inputs) == used
    assert set(inputs.values()) <= {"generated", "provider", "chain", "operator"}
    lines = DEFINITION.read_text().splitlines()
    for name in inputs:
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6


def test_paxeer_boundary_watch_patterns_cover_build_inputs():
    service = definition()["service." + SERVICE]
    patterns = service["watch"]
    assert service["kind"] == "repo" and service["root"] == "/"
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert pattern.startswith("/") and pattern.endswith("/**")
        assert (ROOT / pattern[1:-3]).is_dir(), pattern
    dockerfile = CONTAINER / "Dockerfile"
    assert ROOT / service["dockerfile"] == dockerfile
    for line in dockerfile.read_text().splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        fields = [field for field in shlex.split(line)[1:] if not field.startswith("--")]
        for source in fields[:-1]:
            if source == ".":
                continue
            path = ROOT / source
            assert path.exists(), source
            assert watched(path, patterns), source
    for path in (CONTAINER / "Dockerfile", CONTAINER / "Dockerfile.dockerignore",
                 CONTAINER / "entrypoint.sh", CONTAINER / "files.tsv",
                 ROOT / "docker/common/env-files.sh"):
        assert watched(path, patterns), path
    for manifest in cargo_manifests():
        assert watched(manifest, patterns), manifest.relative_to(ROOT)
    assert watched(ROOT / "services/platform/Cargo.lock", patterns)


def test_paxeer_boundary_topology_health_and_file_transport():
    document = definition()
    service = document["service." + SERVICE]
    variables = document["vars." + SERVICE]
    source = (BOUNDARY / "src/main.rs").read_text()
    assert service["target"] == "other"
    assert service["public"] == 0
    assert (service["restart"], service["replicas"]) == ("ALWAYS", 1)
    assert service["needs"] == ["sequencer-primary"]
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert service["order"] > rollout["sequencer-primary"]["order"]
    assert variables["LAYERX_PAXEER_BOUNDARY_LISTEN"] == "0.0.0.0:${{PORT}}"
    assert variables["LAYERX_PAXEER_BOUNDARY_TLS_CERT_DER"].startswith("/run/")
    assert variables["LAYERX_PAXEER_BOUNDARY_TLS_KEY_DER"].startswith("/run/")
    assert f'request.method == "GET" && request.path == "{service["health"]}"' in source
    assert 'request.path == "/livez"' in source
    assert "TcpListener::bind(address)" in source
    exposed = re.search(r"^EXPOSE (.+)$", (CONTAINER / "Dockerfile").read_text(), re.MULTILINE)
    assert exposed and exposed[1].split() == [variables["PORT"]]
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    assert 'COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files' in dockerfile
    assert 'COPY docker/paxeer/files.tsv /etc/layerx/files.tsv' in dockerfile
    assert 'ENTRYPOINT ["/usr/local/bin/paxeer-entrypoint"]' in dockerfile
    rows = file_rows()
    assert {name for name, *_ in rows} == {
        "LAYERX_PAXEER_BOUNDARY_TLS_CERT_B64", "LAYERX_PAXEER_BOUNDARY_TLS_KEY_B64",
    }
    for name, path, mode, roles in rows:
        assert variables[name].startswith("env:")
        assert path in variables.values()
        assert mode == "0600" and roles == "*"


def test_paxeer_boundary_env_file_transport_preserves_bytes(tmp_path):
    rows = file_rows()
    table = tmp_path / "files.tsv"
    destinations = []
    environment = {"PATH": os.environ["PATH"]}
    for name, path, mode, roles in rows:
        destination = tmp_path / Path(path).name
        payload = base64.b64encode((CONTAINER / "Dockerfile").read_bytes()).decode()
        environment[name] = payload
        destinations.append((destination, (CONTAINER / "Dockerfile").read_bytes()))
    table.write_text("\n".join(
        "\t".join((name, str(destination), mode, roles))
        for (name, _, mode, roles), (destination, _) in zip(rows, destinations)
    ) + "\n")
    environment["LAYERX_FILES_TABLE"] = str(table)
    result = subprocess.run(
        ["sh", str(ROOT / "docker/common/env-files.sh"), "true"],
        env=environment, capture_output=True, text=True, timeout=10,
    )
    assert result.returncode == 0, result.stderr
    for destination, payload in destinations:
        assert destination.read_bytes() == payload
        assert destination.stat().st_mode & 0o777 == 0o600
