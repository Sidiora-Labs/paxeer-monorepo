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

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/core-boundary.kvx"
CONTAINER = ROOT / "docker/platform-core"
CORE = ROOT / "services/platform/hosted/core/src"
INTERNAL = ROOT / "services/platform/hosted/internal/src"
TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|[A-Za-z_]\w*|::|\S',
    re.DOTALL,
)


def matching(tokens, start):
    opening = tokens[start]
    closing = {"(": ")", "[": "]", "{": "}"}[opening]
    depth = 1
    for index in range(start + 1, len(tokens)):
        if tokens[index] == opening:
            depth += 1
        elif tokens[index] == closing:
            depth -= 1
            if depth == 0:
                return index
    raise AssertionError(f"unclosed {opening}")


def rust_tokens(path):
    tokens = [
        token for token in TOKEN.findall(path.read_text())
        if not token.startswith(("//", "/*"))
    ]
    result = []
    index = 0
    while index < len(tokens):
        if tokens[index:index + 7] == ["#", "[", "cfg", "(", "test", ")", "]"]:
            body = tokens.index("{", index + 7)
            index = matching(tokens, body) + 1
        else:
            result.append(tokens[index])
            index += 1
    return result


def arguments(tokens):
    result = []
    start = index = 0
    while index < len(tokens):
        if tokens[index] in ("(", "[", "{"):
            index = matching(tokens, index) + 1
        elif tokens[index] == ",":
            result.append(tokens[start:index])
            start = index = index + 1
        else:
            index += 1
    if tokens[start:]:
        result.append(tokens[start:])
    return result


def calls(tokens):
    for index in range(len(tokens) - 1):
        if (re.fullmatch(r"[A-Za-z_]\w*", tokens[index])
                and tokens[index + 1] == "("
                and (index == 0 or tokens[index - 1] != "fn")):
            end = matching(tokens, index + 1)
            yield index, tokens[index], arguments(tokens[index + 2:end]), end


def functions(tokens):
    result = {}
    for index, token in enumerate(tokens[:-2]):
        if token != "fn" or tokens[index + 2] != "(":
            continue
        end = matching(tokens, index + 2)
        parameters = [arg[0] for arg in arguments(tokens[index + 3:end])]
        body = tokens.index("{", end)
        result[tokens[index + 1]] = (
            parameters, tokens[body + 1:matching(tokens, body)]
        )
    return result


def required_read(tokens, end):
    return tokens[end + 1:end + 3] in (
        [".", "map_err"], [".", "ok_or"], [".", "ok_or_else"],
    ) or tokens[end + 1:end + 2] == ["?"]


def conditional_call(tokens, index):
    return tokens[max(0, index - 5):index] == ["map", "(", "|", "_", "|"]


def environment_call(tokens, index, name):
    return (name in ("var", "var_os")
            and tokens[max(0, index - 2):index] == ["env", "::"])


def variable_value(argument, constants, prefix=None):
    if len(argument) == 1:
        token = argument[0]
        if token.startswith('"'):
            return ast.literal_eval(token)
        return constants.get(token)
    if prefix is not None and argument[:3] == ["format", "!", "("]:
        template = ast.literal_eval(argument[3])
        return template.replace("{prefix}", prefix)
    return None


def source_variables():
    tokens = []
    constants = {}
    for path in sorted(CORE.glob("*.rs")):
        tokens.extend(rust_tokens(path))
        constants.update(re.findall(
            r'const\s+([A-Z_]\w*)\s*:\s*&str\s*=\s*"([^"]+)"',
            path.read_text(),
        ))
        for name, constant, suffix in re.findall(
            r'let\s+(\w+)\s*=\s*([A-Z_]\w*)\s*\.strip_suffix\("([^"]+)"\)',
            path.read_text(),
        ):
            assert constants[constant].endswith(suffix)
            constants[name] = constants[constant].removesuffix(suffix)
    definitions = functions(tokens)
    parameters = {name: set() for name in definitions}
    readers = {name: set() for name in definitions}
    changed = True
    while changed:
        changed = False
        for name, (names, body) in definitions.items():
            for index, callee, args, end in calls(body):
                read = set(readers.get(callee, set()))
                required = set(parameters.get(callee, set()))
                if environment_call(body, index, callee):
                    read.add(0)
                    if required_read(body, end):
                        required.add(0)
                if conditional_call(body, index):
                    required.clear()
                for mapping, positions in ((readers, read), (parameters, required)):
                    for position in positions:
                        if position < len(args) and len(args[position]) == 1:
                            value = args[position][0]
                            if value in names and names.index(value) not in mapping[name]:
                                mapping[name].add(names.index(value))
                                changed = True

    allowed, required, prefixes = set(), set(), set()
    for index, name, args, end in calls(tokens):
        positions = readers.get(name, set())
        if environment_call(tokens, index, name):
            positions = {0}
        for position in positions:
            if position >= len(args):
                continue
            value = variable_value(args[position], constants)
            if value is not None:
                allowed.add(value)
                if (not conditional_call(tokens, index)
                        and ((environment_call(tokens, index, name)
                              and required_read(tokens, end))
                             or position in parameters.get(name, set()))):
                    required.add(value)
        if (name == "from_env"
                and tokens[max(0, index - 2):index] == ["EvmSender", "::"]):
            prefixes.add(variable_value(args[0], constants))

    signer_groups = []
    sender = rust_tokens(INTERNAL / "evm_sender.rs")
    upstream = functions(rust_tokens(INTERNAL / "tls.rs"))["from_environment"][1]
    for prefix in prefixes:
        assert prefix is not None
        alternatives = set()
        for source in (sender, upstream):
            for index, name, args, _ in calls(source):
                if environment_call(source, index, name):
                    value = variable_value(args[0], constants, prefix)
                    assert value is not None
                    allowed.add(value)
                    if source is sender:
                        alternatives.add(value)
        signer_groups.append(alternatives)
    allowed.discard(None)
    return allowed, required, signer_groups, definitions


def definition():
    parser = configparser.ConfigParser(interpolation=None)
    parser.optionxform = str
    parser.read_string(DEFINITION.read_text())
    return {
        section: {key: ast.literal_eval(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def file_rows():
    return [
        line.split("\t") for line in (CONTAINER / "files.tsv").read_text().splitlines()
        if line and not line.startswith("#")
    ]


def test_core_boundary_variables_follow_source_readers():
    rows = definition()["vars.core-boundary"]
    allowed, required, signer_groups, _ = source_variables()
    staged = {row[0] for row in file_rows()}
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    build_args = set(re.findall(r"^ARG\s+(\w+)", dockerfile, re.MULTILINE))
    assert required <= rows.keys(), f"missing required rows: {required - rows.keys()}"
    assert staged | build_args <= rows.keys()
    assert rows.keys() <= allowed | staged | build_args, "unread variable row"
    selected = set()
    for alternatives in signer_groups:
        configured = rows.keys() & alternatives
        assert len(configured) == 1, "funding selects exactly one signer source"
        selected.update(configured)
    assert rows.keys() == required | staged | build_args | selected


def test_core_boundary_inputs_match_names_and_formats():
    sections = definition()
    assert set(sections) == {
        "service.core-boundary", "vars.core-boundary", "inputs.core-boundary",
    }
    inputs = sections["inputs.core-boundary"]
    used = set()
    for name, value in sections["vars.core-boundary"].items():
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            expected = "PLATFORM_CORE_BOUNDARY_" + name
            assert value == "env:" + expected
            used.add(expected)
        else:
            assert value.startswith("/"), "file and volume settings use container paths"
    assert used == inputs.keys()
    assert set(inputs.values()) <= {"generated", "provider", "chain", "operator"}
    lines = DEFINITION.read_text().splitlines()
    for name in inputs:
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6, "input comments describe value and format"


def test_core_boundary_container_sources_are_watched():
    service = definition()["service.core-boundary"]
    dockerfile = ROOT / service["dockerfile"]
    assert service["kind"] == "repo" and service["root"] == "/"
    assert dockerfile == CONTAINER / "Dockerfile" and dockerfile.is_file()
    patterns = service["watch"]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert pattern.startswith("/")
        assert list(ROOT.glob(pattern.lstrip("/"))), f"watch path does not exist: {pattern}"
    copied = []
    for line in dockerfile.read_text().splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        parts = [part for part in shlex.split(line)[1:] if not part.startswith("--")]
        for source in parts[:-1]:
            path = ROOT / source
            assert path.exists(), f"missing container source: {source}"
            copied.append(source.rstrip("/"))
            sample = source.rstrip("/") + ("/Cargo.toml" if path.is_dir() else "")
            assert any(fnmatch.fnmatchcase("/" + sample, pattern) for pattern in patterns)
    assert any(fnmatch.fnmatchcase("/docker/platform-core/Dockerfile", p) for p in patterns)
    ignored = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for source in copied:
        if source.startswith("docker/"):
            assert "!" + source in ignored
            assert ignored.index("!" + source) > ignored.index("docker")
    for workspace in ("services/platform", "services/agent", "services/human", "services/interop", "programs"):
        assert any(workspace == source or workspace.startswith(source + "/") for source in copied)


def test_core_boundary_host_topology_and_health():
    service = definition()["service.core-boundary"]
    rows = definition()["vars.core-boundary"]
    _, _, _, definitions = source_variables()
    main = (CORE / "main.rs").read_text()
    assert service["target"] == "other"
    assert "UnixStream::connect(&config.supervisor_socket)" in main
    assert "SocketSigner::connect" in main
    assert 'host != "127.0.0.1" && host != "localhost"' in main
    assert service["public"] == 0
    assert service["restart"] == "ALWAYS" and service["replicas"] == 1
    assert service["volume"] == rows["LAYERX_CORE_STATE_DIR"]
    assert service["volume"] in (CONTAINER / "Dockerfile").read_text()
    route = definitions["admin_route"][1]
    assert ['"GET"', ",", json.dumps(service["health"])] == route[
        route.index(json.dumps(service["health"])) - 2:route.index(json.dumps(service["health"])) + 1
    ]
    assert "ServerConnection::new" in main
    listeners = dict(re.findall(r'parse_listen\("([^"]+)",\s*"([^\"]+)"\)', main))
    assert len(listeners) == 2
    ports = {address.rsplit(":", 1)[1] for address in listeners.values()}
    exposed = re.search(r"^EXPOSE (.+)$", (CONTAINER / "Dockerfile").read_text(), re.MULTILINE)
    assert set(exposed[1].split()) == ports
    assert "PORT" not in rows
    rollout = configparser.ConfigParser(interpolation=None)
    rollout.read(ROOT / "deploy/rollout.kvx")
    assert service["needs"]
    for name in service["needs"]:
        assert "service." + name in rollout
        assert service["order"] > int(rollout["service." + name]["order"])


def test_core_boundary_file_inputs_are_connected_to_the_entrypoint():
    rows = definition()["vars.core-boundary"]
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    entrypoint = json.loads(re.search(r"^ENTRYPOINT (.+)$", dockerfile, re.MULTILINE)[1])
    assert entrypoint == ["/usr/local/bin/layerx-env-files", "/usr/local/bin/layerx-core-boundary"]
    assert "COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files" in dockerfile
    assert "COPY docker/platform-core/files.tsv /etc/layerx/files.tsv" in dockerfile
    assert "USER 4020:4020" in dockerfile
    assert "chown 4020:4020 /run/secrets /var/lib/layerx/core" in dockerfile
    table = file_rows()
    assert len({row[0] for row in table}) == len(table)
    paths = set()
    for name, guest, mode, roles in table:
        assert name in rows and rows[name].startswith("env:")
        assert guest in rows.values(), "staged input must be consumed by the service"
        assert mode == "0600" and roles == "*"
        assert guest.startswith("/run/secrets/") and guest not in paths
        paths.add(guest)
    required_paths = {
        value for value in rows.values() if value.startswith("/run/secrets/")
    }
    assert paths == required_paths


def staged_environment(tmp_path):
    payload = (CONTAINER / "Dockerfile").read_bytes()
    rows = file_rows()
    environment = {"PATH": os.environ["PATH"]}
    relocated = []
    destinations = []
    for name, guest, mode, roles in rows:
        destination = tmp_path / Path(guest).name
        destinations.append(destination)
        relocated.append("\t".join((name, str(destination), mode, roles)))
        environment[name] = base64.b64encode(payload).decode("ascii")
    table = tmp_path / "files.tsv"
    table.write_text("\n".join(relocated) + "\n")
    environment["LAYERX_FILES_TABLE"] = str(table)
    return environment, destinations, payload


def test_core_boundary_entrypoint_materializes_private_files(tmp_path):
    environment, destinations, payload = staged_environment(tmp_path)
    command = [
        "sh", str(ROOT / "docker/common/env-files.sh"), sys.executable,
        "-c", "import json, os; print(json.dumps(sorted(os.environ)))",
    ]
    result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=10)
    assert result.returncode == 0, result.stderr
    remaining = json.loads(result.stdout)
    assert not ({row[0] for row in file_rows()} & set(remaining))
    for destination in destinations:
        assert destination.read_bytes() == payload
        assert destination.stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize("invalid", ["absent", "base64"])
def test_core_boundary_entrypoint_requires_valid_file_inputs(tmp_path, invalid):
    environment, destinations, _ = staged_environment(tmp_path)
    name = file_rows()[0][0]
    if invalid == "absent":
        del environment[name]
    else:
        environment[name] = "!"
    result = subprocess.run(
        ["sh", str(ROOT / "docker/common/env-files.sh"), "true"],
        env=environment, capture_output=True, text=True, timeout=10,
    )
    assert result.returncode != 0
    assert name in result.stderr
    assert not any(destination.exists() for destination in destinations)
