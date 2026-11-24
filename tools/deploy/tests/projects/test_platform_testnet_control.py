"""Keep the private testnet control definition aligned with its executable."""

import ast
import base64
import fnmatch
import importlib.util
import json
import os
import re
import secrets
import shlex
import socket
import subprocess
import sys
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/testnet-control.kvx"
CONTAINER = ROOT / "docker/platform-testnet"
PACKAGE = ROOT / "services/platform/hosted/testnet"
NAME = DEFINITION.stem
DOCUMENT = tomllib.loads(DEFINITION.read_text())
SERVICE = DOCUMENT["service"][NAME]
VARIABLES = DOCUMENT["vars"][NAME]
INPUTS = DOCUMENT["inputs"][NAME]


def production_rust(path):
    source = path.read_text()
    tokens = re.compile(
        r'//[^\n]*|/\*[\s\S]*?\*/|r#*"[\s\S]*?"#*|"(?:\\.|[^"\\])*"|[{}]'
    )
    test_items = re.finditer(r"#\[cfg\(test\)\]\s*(?:mod|fn)\s+\w+", source)
    for match in reversed(list(test_items)):
        depth = 0
        for token in tokens.finditer(source, match.end()):
            if token.group() == "{":
                depth += 1
            elif token.group() == "}":
                depth -= 1
                if depth == 0:
                    source = source[:match.start()] + source[token.end():]
                    break
    return source


def rust_environment():
    required, optional = set(), {}
    for path in (PACKAGE / "src").rglob("*.rs"):
        source = production_rust(path)
        reads = re.finditer(r'(?:std::)?env::var(?:_os)?\(\s*"(\w+)"\s*\)', source)
        for match in reads:
            tail = source[match.end():]
            default = re.match(
                r'\s*\.unwrap_or_else\(\|_\|\s*"([^"]*)"\.to_owned\(\)\)', tail
            )
            if default:
                optional[match[1]] = default[1]
            else:
                required.add(match[1])
        helpers = re.finditer(r"fn\s+(\w+)\(\s*(\w+):\s*&str\s*\)[^{]*\{", source)
        for helper in helpers:
            body = source[helper.end():].split("\n}", 1)[0]
            if re.search(r"env::var\(\s*" + re.escape(helper[2]) + r"\s*\)", body):
                required.update(
                    re.findall(r'\b' + re.escape(helper[1]) + r'\(\s*"(\w+)"', source)
                )
        for variable in re.findall(r'\benv!\(\s*"(\w+)"\s*\)', source):
            if not variable.startswith("CARGO_"):
                required.add(variable)
    assert required and optional
    return required, optional


def python_environment(path):
    required, optional = set(), set()
    for node in ast.walk(ast.parse(path.read_text())):
        if isinstance(node, ast.Subscript) and ast.unparse(node.value) == "os.environ":
            assert isinstance(node.slice, ast.Constant), "environment names must be discoverable"
            required.add(node.slice.value)
        if isinstance(node, ast.Call) and ast.unparse(node.func) in (
            "os.getenv", "os.environ.get"
        ):
            assert node.args and isinstance(node.args[0], ast.Constant)
            optional.add(node.args[0].value)
    return required, optional


def file_rows():
    rows = []
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, path, mode, roles = line.split("\t")
            rows.append((name, path, mode, roles))
    assert rows
    assert len({row[0] for row in rows}) == len(rows)
    assert len({row[1] for row in rows}) == len(rows)
    return rows


def covered(path):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern) for pattern in SERVICE["watch"])


def workspace(manifest):
    for directory in manifest.parents:
        candidate = directory / "Cargo.toml"
        if candidate.is_file():
            data = tomllib.loads(candidate.read_text())
            if "workspace" in data:
                return candidate, data["workspace"]
    raise AssertionError(f"workspace missing for {manifest}")


def build_manifests(manifest, seen=None):
    seen = set() if seen is None else seen
    manifest = manifest.resolve()
    if manifest in seen:
        return seen
    seen.add(manifest)
    data = tomllib.loads(manifest.read_text())
    workspace_path, workspace_data = workspace(manifest)
    assert covered(workspace_path)
    for kind in ("dependencies", "build-dependencies"):
        for name, dependency in data.get(kind, {}).items():
            if not isinstance(dependency, dict):
                continue
            directory = manifest.parent
            if dependency.get("workspace"):
                dependency = workspace_data.get(kind, {}).get(name, {})
                directory = workspace_path.parent
            if isinstance(dependency, dict) and "path" in dependency:
                build_manifests(directory / dependency["path"] / "Cargo.toml", seen)
    return seen


def load_entrypoint():
    spec = importlib.util.spec_from_file_location(
        "testnet_container", CONTAINER / "entrypoint.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_repo_build_and_watch_inputs():
    assert set(DOCUMENT) == {"service", "vars", "inputs"}
    for section in DOCUMENT.values():
        assert set(section) == {NAME}
    assert SERVICE["kind"] == "repo"
    assert SERVICE["railway_service"] == NAME
    assert SERVICE["root"] == "/"
    dockerfile = ROOT / SERVICE["dockerfile"]
    assert dockerfile == CONTAINER / "Dockerfile" and dockerfile.is_file()
    for pattern in SERVICE["watch"]:
        assert next(ROOT.glob(pattern.lstrip("/")), None) is not None, pattern
    for directory in (CONTAINER, ROOT / "docker/common"):
        for path in directory.iterdir():
            if path.is_file():
                assert covered(path), path
    for line in dockerfile.read_text().splitlines():
        if line.startswith("COPY "):
            arguments = shlex.split(line)[1:]
            if any(argument.startswith("--from=") for argument in arguments):
                continue
            for argument in arguments[:-1]:
                if argument.startswith("--"):
                    continue
                path = ROOT / argument
                assert path.exists(), argument
                if path.is_file():
                    assert covered(path), argument
    for manifest in build_manifests(PACKAGE / "Cargo.toml"):
        assert covered(manifest)
        for path in manifest.parent.rglob("*.rs"):
            assert covered(path), path
        build = manifest.parent / "build.rs"
        if build.is_file():
            for name in re.findall(r'crate_dir\.join\("([^"]+)"\)', build.read_text()):
                path = (manifest.parent / name).resolve()
                assert path.is_file() and covered(path), path


def test_variable_rows_follow_source_readers():
    required, optional = rust_environment()
    script = (PACKAGE / "run-testnet.sh").read_text()
    required.update(re.findall(r"\$\{(\w+):\?", script))
    required.update(
        re.findall(r"^ARG\s+(\w+)", (CONTAINER / "Dockerfile").read_text(), re.M)
    )
    python_required, python_optional = python_environment(CONTAINER / "entrypoint.py")
    required.update(python_required)
    required.update(row[0] for row in file_rows())
    readers = required | set(optional) | python_optional
    assert required <= VARIABLES.keys(), sorted(required - VARIABLES.keys())
    assert VARIABLES.keys() <= readers, sorted(VARIABLES.keys() - readers)
    for variable, default in optional.items():
        if variable in VARIABLES:
            assert VARIABLES[variable] != default, variable


def test_inputs_are_named_classified_and_described():
    prefix = f"{DEFINITION.parent.name}_{NAME}".upper().replace("-", "_")
    uses = set()
    for variable, value in VARIABLES.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", variable)
        assert isinstance(value, str)
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            assert name == f"{prefix}_{variable}"
            uses.add(name)
        else:
            assert "://" not in value, variable
    assert INPUTS.keys() == uses
    lines = DEFINITION.read_text().splitlines()
    for name, kind in INPUTS.items():
        assert kind in {"generated", "provider", "chain", "operator"}
        row = next(
            index for index, line in enumerate(lines) if line.startswith(name + " =")
        )
        assert row > 0 and lines[row - 1].startswith("# ")
        assert len(lines[row - 1].split()) >= 8


def test_private_topology_and_liveness():
    deployment = (PACKAGE / "deployment.yaml").read_text()
    block = next(
        block for block in deployment.split("---")
        if "kind: Deployment" in block and "name: layerx-testnet-control," in block
    )
    assert SERVICE["replicas"] == int(re.search(r"replicas:\s*(\d+)", block)[1])
    assert SERVICE["restart"] == "ALWAYS"
    assert SERVICE.get("public", 0) == 0
    assert "volume" not in SERVICE
    assert SERVICE["needs"] == ["identity", "faucet", "gateway"]
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())
    for name in SERVICE["needs"]:
        if name in rollout["service"]:
            assert SERVICE["order"] > rollout["service"][name]["order"]
    module = load_entrypoint()
    source = production_rust(PACKAGE / "src/main.rs")
    assert SERVICE["health"] == module.HEALTH_PATH
    assert re.search(
        re.escape(json.dumps(SERVICE["health"])) + r"\s*=>\s*json_response\(200", source
    )
    assert module.HealthServer.address_family == socket.AF_INET6
    tree = ast.parse((CONTAINER / "entrypoint.py").read_text())
    server = next(
        node for node in ast.walk(tree)
        if isinstance(node, ast.Call) and ast.unparse(node.func) == "HealthServer"
    )
    assert ast.literal_eval(server.args[0].elts[0]) == "::"
    assert ast.unparse(server.args[0].elts[1]) == "port"
    assert 1024 <= int(VARIABLES["PORT"]) <= 65535
    assert int(VARIABLES["PORT"]) not in (9443, 9444)
    assert VARIABLES["LAYERX_TESTNET_PUBLIC_LISTEN"] == "[::]:9443"
    assert VARIABLES["LAYERX_TESTNET_ADMIN_LISTEN"] == "[::]:9444"
    wrapper = (CONTAINER / "entrypoint.py").read_text()
    assert "ssl.PROTOCOL_TLS_CLIENT" in wrapper
    assert "context.load_verify_locations(" in wrapper
    assert "server_hostname=server_name" in wrapper
    assert "CERT_NONE" not in wrapper and "check_hostname = False" not in wrapper
    assert '"/opt/layerx/run-testnet.sh"' in wrapper


def test_container_materializes_all_required_files():
    required, _ = rust_environment()
    file_variables = {
        variable for variable in required
        if re.search(r"(?:_FILE|_DER)$", variable)
    }
    paths = {path for _, path, _, _ in file_rows()}
    assert paths == {VARIABLES[variable] for variable in file_variables}
    for name, path, mode, roles in file_rows():
        assert name in VARIABLES and mode == "0600" and roles == "*"
        assert Path(path).parent == Path("/run/layerx/testnet")
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    assert (
        'ENTRYPOINT ["/usr/local/bin/layerx-env-files", "python3", "-I", '
        '"/opt/layerx/entrypoint.py"]'
    ) in dockerfile
    assert "USER 4020:4020" in dockerfile
    assert "chown 4020:4020 /run/layerx/testnet" in dockerfile
    assert "chmod 0700 /run/layerx/testnet" in dockerfile
    ignore = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    assert "docker" not in ignore
    assert "docker/*" in ignore
    assert "!docker/common" in ignore and "!docker/platform-testnet" in ignore


def test_env_files_decodes_protects_and_removes_material(tmp_path):
    environment = {"PATH": os.environ["PATH"]}
    expected = {}
    rows = []
    for name, path, mode, roles in file_rows():
        destination = tmp_path / Path(path).name
        material = secrets.token_bytes(64)
        expected[str(destination)] = base64.b64encode(material).decode()
        environment[name] = base64.b64encode(material).decode()
        rows.append("\t".join((name, str(destination), mode, roles)))
    table = tmp_path / "files.tsv"
    table.write_text("\n".join(rows) + "\n")
    environment["LAYERX_FILES_TABLE"] = str(table)
    consumer = (
        "import base64,json,os,pathlib,stat; "
        "print(json.dumps({str(p): [base64.b64encode(p.read_bytes()).decode(), "
        "stat.S_IMODE(p.stat().st_mode),p.stat().st_uid,p.stat().st_nlink] "
        "for p in pathlib.Path(__import__('sys').argv[1]).iterdir() if p.name != 'files.tsv'})); "
        "print(json.dumps(sorted(k for k in os.environ if k.endswith('_B64'))))"
    )
    result = subprocess.run(
        ["/bin/sh", str(ROOT / "docker/common/env-files.sh"), sys.executable,
         "-c", consumer, str(tmp_path)],
        env=environment, text=True, capture_output=True, check=False,
    )
    assert result.returncode == 0, result.stderr
    contents, retained = map(json.loads, result.stdout.splitlines())
    assert set(contents) == set(expected)
    for path, data in contents.items():
        assert data == [expected[path], 0o600, os.getuid(), 1]
    assert retained == []


@pytest.mark.parametrize("invalid", [None, "%%%"])
def test_env_files_refuses_missing_or_invalid_material(tmp_path, invalid):
    name, path, mode, roles = file_rows()[0]
    table = tmp_path / "files.tsv"
    destination = tmp_path / Path(path).name
    table.write_text("\t".join((name, str(destination), mode, roles)) + "\n")
    environment = {"PATH": os.environ["PATH"], "LAYERX_FILES_TABLE": str(table)}
    if invalid is not None:
        environment[name] = invalid
    result = subprocess.run(
        ["/bin/sh", str(ROOT / "docker/common/env-files.sh"), sys.executable,
         "-c", "raise SystemExit(93)"],
        env=environment, capture_output=True, text=True, check=False,
    )
    assert result.returncode == 1
    assert not destination.exists()
    assert not list(tmp_path.glob(".layerx-env-files.*"))


@pytest.mark.parametrize("port", ["0", "9443", "9444", "65536", "invalid"])
def test_entrypoint_refuses_invalid_probe_ports(port):
    result = subprocess.run(
        [sys.executable, "-I", str(CONTAINER / "entrypoint.py")],
        env={"PORT": port}, capture_output=True, text=True, check=False,
    )
    assert result.returncode == 2
    assert result.stderr == "testnet control startup refused: invalid runtime inputs\n"
