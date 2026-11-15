"""The signature provider definition follows its container and settings readers."""

import fnmatch
import os
import re
import shlex
import subprocess
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/explorer/sig-provider.kvx"
CONTAINER = ROOT / "docker/explorer-sig-provider"
SOURCE = ROOT / "services/explorer/services/sig-provider"
LAUNCHER = ROOT / "services/explorer/services/libs/blockscout-service-launcher/src"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def settings_environment():
    source = (SOURCE / "sig-provider-server/src/settings.rs").read_text()
    prefix = re.search(r'const SERVICE_NAME:.*?=\s*"([A-Z_]+)";', source).group(1)
    structs = {}
    for path in (
        SOURCE / "sig-provider-server/src/settings.rs",
        LAUNCHER / "launcher/settings.rs",
        LAUNCHER / "tracing/settings.rs",
    ):
        for match in re.finditer(
            r"(?P<attributes>(?:^#\[[^\n]+\]\n)+)"
            r"pub struct (?P<name>\w+)\s*\{(?P<body>.*?)^\}",
            path.read_text(),
            re.MULTILINE | re.DOTALL,
        ):
            structs[match["name"]] = match["attributes"], match["body"]

    known, required = set(), set()

    def visit(type_name, components):
        attributes, body = structs[type_name]
        container_default = bool(re.search(r"#\[serde\([^]]*\bdefault\b", attributes))
        fields = list(
            re.finditer(
                r"(?P<attributes>(?:\s*#\[[^\n]*\]\s*)*)"
                r"\s*pub (?P<name>\w+):\s*(?P<type>[^,\n]+),",
                body,
            )
        )
        assert fields, type_name
        for field in fields:
            rename = re.search(r'rename\s*=\s*"([^"]+)"', field["attributes"])
            name = rename[1] if rename else field["name"]
            path = [*components, name.upper()]
            field_type = field["type"].strip()
            if field_type in structs:
                visit(field_type, path)
            else:
                variable = "__".join([prefix, *path])
                known.add(variable)
                optional = (
                    container_default
                    or re.search(r"#\[serde\([^]]*\bdefault\b", field["attributes"])
                    or field_type.startswith("Option<")
                )
                if not optional:
                    required.add(variable)

    visit("Settings", [])
    assert known
    reader = (LAUNCHER / "launcher/settings.rs").read_text()
    suffix = re.search(r'format!\("\{\}([^"\n]+)", Self::SERVICE_NAME\)', reader)[1]
    known.add(prefix + suffix)
    assert 'Environment::with_prefix(Self::SERVICE_NAME).separator("__")' in reader

    for path in SOURCE.rglob("*.rs"):
        if "tests" in path.parts or "test" in path.stem:
            continue
        text = path.read_text()
        for match in re.finditer(
            r'(?:std::)?env::var(?:_os)?\(\s*"([A-Z0-9_]+)"\s*\)', text
        ):
            known.add(match[1])
            expression = text[match.end():text.find(";", match.end())]
            if not re.search(r"\.(?:unwrap_or\w*|ok|is_ok|is_err)\s*\(", expression):
                required.add(match[1])
    return prefix, known, required


def container_environment():
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    arguments = set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)", dockerfile, re.MULTILINE))
    entrypoint = (CONTAINER / "entrypoint.sh").read_text()
    shell_reads = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)(?=[:?+\-]|\})", entrypoint))
    required = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)\:?\?", entrypoint))
    for table in CONTAINER.glob("*.tsv"):
        for line in table.read_text().splitlines():
            if line.strip() and not line.startswith("#"):
                name = line.split()[0]
                assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
                shell_reads.add(name)
                required.add(name)
    return arguments | shell_reads, arguments | required


def test_service_container_and_watch_paths():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == {DEFINITION.stem}
    service = data["service"][DEFINITION.stem]
    assert service["kind"] == "repo"
    assert service["root"] == "/"
    assert ROOT / service["dockerfile"] == CONTAINER / "Dockerfile"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["order"] == 1
    assert service["needs"] == []
    assert service.get("public", 0) == 0
    assert "volume" not in service
    assert "target" not in service
    watches = service["watch"]
    for pattern in watches:
        assert pattern.endswith("/**")
        assert (ROOT / pattern.lstrip("/")[:-3]).is_dir(), pattern

    def watched(path):
        relative = path.relative_to(ROOT).as_posix()
        return any(fnmatch.fnmatch(relative + "/", p.lstrip("/")) for p in watches)

    assert watched(CONTAINER)
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    ignore = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for line in dockerfile.splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        words = [word for word in shlex.split(line)[1:] if not word.startswith("--")]
        for source in words[:-1]:
            path = ROOT / source
            assert path.exists(), source
            assert watched(path), source
            assert any(
                pattern.startswith("!")
                and (source == pattern[1:] or source.startswith(pattern[1:] + "/"))
                for pattern in ignore
            ), source
    assert "USER app" in dockerfile
    assert (
        "COPY --chmod=755 docker/explorer-sig-provider/entrypoint.sh /app/entrypoint.sh"
        in dockerfile
    )
    assert 'ENTRYPOINT ["/app/entrypoint.sh"]' in dockerfile
    assert 'CMD ["./sig-provider-server"]' in dockerfile


def test_variables_follow_the_source_readers():
    data = definition()
    prefix, known, required = settings_environment()
    container_known, container_required = container_environment()
    variables = data["vars"][DEFINITION.stem]
    assert required | container_required <= variables.keys()
    assert variables.keys() <= known | container_known
    assert "PORT" in variables
    address = prefix + "__SERVER__HTTP__ADDR"
    assert address in known
    assert variables[address] == "[::]:${{PORT}}"
    assert variables["PORT"].isdecimal()
    assert 1 <= int(variables["PORT"]) <= 65535
    main = (SOURCE / "sig-provider-server/src/main.rs").read_text()
    assert "Settings::build()" in main
    assert "sig_provider(settings).await" in main


def test_operator_inputs_are_named_and_described():
    data = definition()
    variables = data["vars"][DEFINITION.stem]
    inputs = data["inputs"][DEFINITION.stem]
    uses = {}
    for variable, value in variables.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", variable)
        assert isinstance(value, str)
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            expected = f"paxeer-x-{DEFINITION.parent.name}_{DEFINITION.stem}_{variable}"
            assert name == expected.upper().replace("-", "_")
            uses[name] = variable
    assert set(inputs) == set(uses)
    lines = DEFINITION.read_text().splitlines()
    for name, kind in inputs.items():
        assert kind in {"generated", "provider", "chain", "operator"}
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 5


def run_entrypoint(port=None, address=None):
    environment = {
        name: value
        for name, value in os.environ.items()
        if name != "PORT" and not name.startswith("SIG_PROVIDER__")
    }
    prefix, _, _ = settings_environment()
    if port is not None:
        environment["PORT"] = port
    if address is not None:
        environment[prefix + "__SERVER__HTTP__ADDR"] = address
    return subprocess.run(
        ["/bin/sh", str(CONTAINER / "entrypoint.sh"), "/usr/bin/env", "-0"],
        env=environment,
        capture_output=True,
        timeout=5,
        check=False,
    )


@pytest.mark.parametrize("port", ["1", "43127", "65535"])
def test_entrypoint_passes_the_hosting_port_to_the_server(port):
    prefix, _, _ = settings_environment()
    result = run_entrypoint(port)
    assert result.returncode == 0, result.stderr.decode()
    environment = dict(item.split(b"=", 1) for item in result.stdout.split(b"\0") if item)
    assert environment[b"PORT"] == port.encode()
    assert (
        environment[(prefix + "__SERVER__HTTP__ADDR").encode()]
        == f"[::]:{port}".encode()
    )


def test_manifest_port_and_health_match_the_server():
    data = definition()
    prefix, _, _ = settings_environment()
    variables = data["vars"][DEFINITION.stem]
    address_name = prefix + "__SERVER__HTTP__ADDR"
    address = variables[address_name].replace("${{PORT}}", variables["PORT"])
    result = run_entrypoint(variables["PORT"], address)
    assert result.returncode == 0, result.stderr.decode()
    assert f"{address_name}={address}".encode() in result.stdout.split(b"\0")
    result = run_entrypoint()
    assert result.returncode == 0, result.stderr.decode()
    assert f"PORT={variables['PORT']}".encode() in result.stdout.split(b"\0")
    mapping = (SOURCE / "sig-provider-proto/proto/api_config_http.yaml").read_text()
    route = re.search(r"Health\.Check\s+get:\s*(/\S+)", mapping)[1]
    assert data["service"][DEFINITION.stem]["health"] == route
    server = (SOURCE / "sig-provider-server/src/run.rs").read_text()
    assert "route_health(config, self.health.clone())" in server
    health = (SOURCE / "sig-provider-server/src/health.rs").read_text()
    assert "ServingStatus::Serving" in health


@pytest.mark.parametrize("port", ["", "0", "01", "65536", "123456", "-1", "80x", "80\n"])
def test_entrypoint_rejects_invalid_ports(port):
    result = run_entrypoint(port)
    assert result.returncode == 1
    assert result.stdout == b""
    assert result.stderr == b"PORT must be a decimal integer between 1 and 65535\n"


def test_entrypoint_preserves_an_explicit_listener():
    prefix, _, _ = settings_environment()
    address = "[::]:43127"
    result = run_entrypoint("43127", address)
    assert result.returncode == 0, result.stderr.decode()
    assert f"{prefix}__SERVER__HTTP__ADDR={address}".encode() in result.stdout.split(b"\0")
