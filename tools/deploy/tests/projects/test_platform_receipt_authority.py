"""Receipt authority deployment settings follow the executable sources."""

import base64
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import stat
import subprocess
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/receipt-authority.kvx"
CONTAINER = ROOT / "docker/platform-authority"
AUTHORITY = ROOT / "services/platform/hosted/authority/src"
CLOCK = ROOT / "services/platform/hosted/runtime-clock/src/main.rs"
SERVICE = "receipt-authority"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def functions(source):
    """Locate Rust bodies without treating quoted braces as delimiters."""
    masked = re.sub(
        r'r(?P<hashes>#{0,255})"[\s\S]*?"(?P=hashes)'
        r'|"(?:\\.|[^"\\])*"|//[^\n]*|/\*[\s\S]*?\*/|(?:b)?\'(?:\\.|[^\'\\])\'',
        lambda match: " " * len(match[0]),
        source,
    )
    result = {}
    signature = (
        r"\bfn\s+(\w+)(?:<[^;{}]*>)?\s*\([^{}]*?\)"
        r"(?:[^;{}\[\]]|\[[^\]]*\])*\{"
    )
    for match in re.finditer(signature, masked):
        start = match.end()
        depth = 1
        end = start
        while depth:
            assert end < len(masked), match[1]
            depth += (masked[end] == "{") - (masked[end] == "}")
            end += 1
        result[match[1]] = source[start : end - 1]
    return result


def environment_reads(source):
    """Find direct reads and calls to helpers that forward an env name."""
    bodies = functions(source)
    read = re.compile(
        r'\benv::(var(?:_os)?)\(\s*(?:"([A-Z][A-Z0-9_]*)"|(\w+))\s*\)'
    )
    helpers = {
        name
        for name, body in bodies.items()
        if any(match[3] for match in read.finditer(body))
    }
    required = set()
    optional = set()
    for body in bodies.values():
        for match in read.finditer(body):
            if match[2] is None:
                continue
            expression = body[match.end() :].split(";", 1)[0]
            default = re.search(r"\.unwrap_or(?:_else|_default)?\b", expression)
            if match[1] == "var_os" or default:
                optional.add(match[2])
            else:
                required.add(match[2])
        for helper in helpers:
            required.update(
                re.findall(rf'\b{helper}\(\s*"([A-Z][A-Z0-9_]*)"\s*\)', body)
            )
    return required - optional, optional


def file_rows():
    rows = []
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, path, mode, roles = line.split("\t")
            assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
            assert Path(path).is_absolute()
            assert mode == "0600"
            assert roles == "*"
            rows.append((name, path, mode, roles))
    assert rows
    assert len({row[0] for row in rows}) == len(rows)
    assert len({row[1] for row in rows}) == len(rows)
    return rows


def watched(path, service):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern) for pattern in service["watch"])


def local_cargo_manifests():
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
        tables = [document, workspace]
        tables.extend(document.get("target", {}).values())
        for table in tables:
            for kind in ("dependencies", "dev-dependencies", "build-dependencies"):
                for dependency in table.get(kind, {}).values():
                    if isinstance(dependency, dict) and "path" in dependency:
                        pending.append(path.parent / dependency["path"] / "Cargo.toml")
        for table in document.get("patch", {}).values():
            for dependency in table.values():
                if "path" in dependency:
                    pending.append(path.parent / dependency["path"] / "Cargo.toml")
    return seen


def test_repository_build_inputs_and_watch_patterns():
    service = definition()["service"][SERVICE]
    assert service["kind"] == "repo"
    assert service["root"] == "/"
    dockerfile = ROOT / service["dockerfile"]
    assert dockerfile == CONTAINER / "Dockerfile"
    assert dockerfile.is_file()
    for pattern in service["watch"]:
        assert pattern.startswith("/") and pattern.endswith("/**")
        assert (ROOT / pattern[1:-3]).is_dir(), pattern
    inputs = set(CONTAINER.iterdir())
    rules = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for line in dockerfile.read_text().splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        fields = [word for word in shlex.split(line)[1:] if not word.startswith("--")]
        for name in fields[:-1]:
            if name == ".":
                continue
            path = ROOT / name
            assert path.is_file(), name
            included = True
            relative = path.relative_to(ROOT).as_posix()
            parts = relative.split("/")
            parents = ["/".join(parts[:i]) for i in range(1, len(parts) + 1)]
            for rule in rules:
                if rule and not rule.startswith("#"):
                    if any(
                        fnmatch.fnmatchcase(parent, rule.lstrip("!"))
                        for parent in parents
                    ):
                        included = rule.startswith("!")
            assert included, name
            inputs.add(path)
    for manifest in local_cargo_manifests():
        inputs.add(manifest)
        inputs.add(manifest.parent / "src" / "lib.rs")
    for path in inputs:
        assert watched(path, service), path.relative_to(ROOT)
    assert "--locked" in dockerfile.read_text()
    assert watched(ROOT / "services/platform/Cargo.lock", service)


def test_required_variables_follow_runtime_readers_and_build_arguments():
    variables = definition()["vars"][SERVICE]
    required = set()
    for path in AUTHORITY.rglob("*.rs"):
        if path.name == "human.rs" or "human" in path.relative_to(AUTHORITY).parts:
            continue
        source = path.read_text().split("#[cfg(test)]", 1)[0]
        needed, _ = environment_reads(source)
        required.update(needed)
    assert required
    entrypoint = (CONTAINER / "entrypoint.sh").read_text()
    required.update(re.findall(r"\$\{([A-Z][A-Z0-9_]*):\?", entrypoint))
    arguments = set(re.findall(
        r"^ARG\s+([A-Z][A-Z0-9_]*)", (CONTAINER / "Dockerfile").read_text(), re.M
    ))
    required.update(arguments)
    replica_settings = re.search(
        r"const REPLICA_SETTINGS:[^=]+=\s*\[([^]]+)\];",
        (AUTHORITY / "main.rs").read_text(),
    )
    assert replica_settings is not None
    replica_names = set(re.findall(r'"([A-Z][A-Z0-9_]*)"', replica_settings[1]))
    assert len(replica_names) == 5
    required.difference_update(replica_names)
    material = {row[0] for row in file_rows()}
    assert variables.keys() == required | material
    human = functions((AUTHORITY / "human.rs").read_text())["load"]
    trigger = 'std::env::var_os("LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE").is_none()'
    assert trigger in human
    assert "Ok(None)" in human
    assert "volume" not in definition()["service"][SERVICE]


def test_operator_inputs_have_names_types_and_format_comments():
    document = definition()
    assert set(document) == {"service", "vars", "inputs"}
    for section in document.values():
        assert set(section) == {SERVICE}
    variables = document["vars"][SERVICE]
    inputs = document["inputs"][SERVICE]
    external = set()
    for name, value in variables.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            expected = f"PLATFORM_{SERVICE.upper().replace('-', '_')}_{name}"
            assert value == "env:" + expected
            external.add(expected)
        else:
            assert value.startswith("/run/layerx-authority/") or name in {
                "PORT", "LAYERX_AUTHORITY_LISTEN"
            }
    assert set(inputs) == external
    lines = DEFINITION.read_text().splitlines()
    for name, word in inputs.items():
        assert word in {"generated", "provider", "chain", "operator"}
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 8


def test_port_health_topology_and_clock_supervision():
    document = definition()
    service = document["service"][SERVICE]
    variables = document["vars"][SERVICE]
    source = (AUTHORITY / "main.rs").read_text()
    bodies = functions(source)
    assert variables["LAYERX_AUTHORITY_LISTEN"] == "[::]:${{PORT}}"
    assert 1 <= int(variables["PORT"]) <= 65535
    assert 'env::var("LAYERX_AUTHORITY_LISTEN")' in bodies["config"]
    assert ".parse::<SocketAddr>()" in bodies["config"]
    assert "TcpListener::bind(config.listen)" in bodies["serve"]
    assert f'if path == "{service["health"]}"' in bodies["route"]
    assert "return readiness(config);" in bodies["route"]
    assert "dependencies.replica.ready && dependencies.chain.ready" in bodies["readiness"]
    assert "ServerConnection::new" in bodies["handle_connection"]
    assert service["target"] == "other"
    target_comment = (
        "# The TLS listener and loopback-only node and replica peers "
        "require a shared network namespace.\ntarget"
    )
    assert target_comment in DEFINITION.read_text()
    assert ".and_then(loopback_http)" in bodies["replica_config"]
    loopback = functions((AUTHORITY / "lib.rs").read_text())["loopback_http"]
    chain = functions((AUTHORITY / "chain_receipt.rs").read_text())["load"]
    assert "_ => return None" in loopback
    assert "super::loopback_http" in chain
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert service["needs"] == ["sequencer-primary"]
    assert all(rollout[name]["order"] < service["order"] for name in service["needs"])
    assert service["order"] < rollout["gateway"]["order"]
    assert (service["restart"], service["replicas"], service["public"]) == ("ALWAYS", 1, 0)
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    assert "USER 4020:4020" in dockerfile
    assert f'EXPOSE {variables["PORT"]}' in dockerfile
    assert 'ENTRYPOINT ["/usr/local/bin/authority-entrypoint"]' in dockerfile
    entrypoint = (CONTAINER / "entrypoint.sh").read_text()
    assert '"$listen" != "[::]:$port"' in entrypoint
    assert (
        "exec /usr/local/bin/layerx-env-files /usr/local/bin/layerx-runtime-clock"
        in entrypoint
    )
    assert (
        "--runtime-dir /tmp -- /usr/local/bin/layerx-receipt-authority" in entrypoint
    )
    clock = CLOCK.read_text()
    bindings = set(re.findall(r'\.env\(\s*"([A-Z][A-Z0-9_]*)"', clock))
    client = ROOT / "services/agent/crates/layerx-client/src/runtime_clock.rs"
    needed, _ = environment_reads(client.read_text())
    assert needed <= bindings
    assert bindings.isdisjoint(variables)
    assert '"--pdeathsig", "KILL"' in clock


def test_protected_material_is_wired_to_runtime_file_readers():
    variables = definition()["vars"][SERVICE]
    rows = file_rows()
    staged = {row[1] for row in rows}
    paths = {
        path
        for value in variables.values()
        if value.startswith("/run/layerx-authority/")
        for path in value.split(":")
    }
    assert staged == paths
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    for operation in ("mkdir -p", "chown 4020:4020", "chmod 0700"):
        assert operation + " /run/layerx-authority" in dockerfile
    assert (
        "COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files"
        in dockerfile
    )
    assert "COPY docker/platform-authority/files.tsv /etc/layerx/files.tsv" in dockerfile
    assert all(variables[name].startswith("env:") for name, *_ in rows)


def staged_environment(tmp_path):
    environment = {"PATH": os.environ["PATH"]}
    rows = file_rows()
    material = {}
    table = tmp_path / "files.tsv"
    lines = []
    for name, path, mode, roles in rows:
        destination = tmp_path / Path(path).name
        data = secrets.token_bytes(96)
        material[destination] = data
        environment[name] = base64.b64encode(data).decode("ascii")
        lines.append("\t".join((name, str(destination), mode, roles)))
    table.write_text("\n".join(lines) + "\n")
    environment["LAYERX_FILES_TABLE"] = str(table)
    return environment, material


def test_env_file_transport_preserves_bytes_permissions_and_unsets_inputs(tmp_path):
    environment, material = staged_environment(tmp_path)
    consumer = (
        "import hashlib,json,os,pathlib,sys; "
        "print(json.dumps({p: [hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest(), "
        "pathlib.Path(p).stat().st_mode & 0o777, pathlib.Path(p).stat().st_uid] "
        "for p in sys.argv[1:]})); "
        "assert not any(n.endswith('_B64') for n in os.environ)"
    )
    result = subprocess.run(
        [
            "sh", str(ROOT / "docker/common/env-files.sh"),
            sys.executable, "-c", consumer, *map(str, material),
        ],
        env=environment,
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode == 0, result.stderr
    assert not result.stderr
    assert json.loads(result.stdout) == {
        str(path): [
            hashlib.sha256(data).hexdigest(), stat.S_IRUSR | stat.S_IWUSR,
            os.geteuid(),
        ]
        for path, data in material.items()
    }


@pytest.mark.parametrize("invalid", [False, True])
def test_env_file_transport_requires_valid_material(tmp_path, invalid):
    environment, material = staged_environment(tmp_path)
    name = file_rows()[0][0]
    if invalid:
        environment[name] = "!"
    else:
        del environment[name]
    result = subprocess.run(
        ["sh", str(ROOT / "docker/common/env-files.sh"), "cat", *map(str, material)],
        env=environment,
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode == 1
    assert not result.stdout
    assert name in result.stderr
    assert ("not valid base64" if invalid else "not set") in result.stderr
    assert not any(path.exists() for path in material)


@pytest.mark.parametrize("port,listen", [
    ("0", "[::]:0"), ("65536", "[::]:65536"),
    ("http", "[::]:http"), ("9445", "[::]:8080"),
])
def test_entrypoint_requires_matching_tcp_port(port, listen):
    result = subprocess.run(
        ["sh", str(CONTAINER / "entrypoint.sh")],
        env={
            "PATH": os.environ["PATH"], "PORT": port,
            "LAYERX_AUTHORITY_LISTEN": listen,
        },
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode == 1
    assert not result.stdout
    assert result.stderr.startswith("authority startup:")
