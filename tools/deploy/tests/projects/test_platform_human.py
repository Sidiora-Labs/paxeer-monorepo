import fnmatch
import os
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/human.kvx"
CONTAINER = ROOT / "docker/human-service"
SERVICE = ROOT / "services/human/crates/layerx-human-service/src"
MAIN = (SERVICE / "main.rs").read_text()
ENTRY = (CONTAINER / "entrypoint.sh").read_text()
DOCKERFILE = (CONTAINER / "Dockerfile").read_text()


def definition():
    return tomllib.loads(DEFINITION.read_text())


def rust_function(source, name):
    match = re.search(
        rf"(?ms)^(?:pub(?:\([^)]*\))?\s+)?fn {name}\b.*?^\}}", source
    )
    assert match, f"function {name} must exist"
    return match.group()


def reader_names(source, readers):
    return set(
        re.findall(
            rf'\b(?:{readers})(?:\s*::\s*<[^>]+>)?\s*\(\s*"([A-Z][A-Z0-9_]*)"',
            source,
        )
    )


def shell_environment(source):
    names = {
        left or right
        for left, right in re.findall(
            r"\$\{#?([A-Z][A-Z0-9_]*)(?:[^}]*)\}|\$([A-Z][A-Z0-9_]*)\b",
            source,
        )
    }
    defaults = dict(
        re.findall(r"\$\{([A-Z][A-Z0-9_]*):?-([^}]+)\}", source)
    )
    return names - defaults.keys(), defaults


def service_environment():
    main = rust_function(MAIN, "run")
    production = (SERVICE / "server/production_components.rs").read_text()
    web = rust_function(production, "web_identity")
    assert "web_identity" in main and "env::var(name)" in main
    assert "lookup(name)" in web
    required = reader_names(main, "required")
    optional = reader_names(main, "optional|number") | reader_names(web, "value")
    tls = re.search(r"(?ms)Listener::Tls => \{(.*?)^        \}", main)
    assert tls
    tls_required = reader_names(tls.group(1), "required")
    assert tls_required
    if definition()["vars"]["human"]["LAYERX_HUMAN_LISTENER"] == "plain":
        required -= tls_required
        optional -= tls_required
    else:
        required |= tls_required
    direct = reader_names(main, "env::var|env::var_os")
    required |= direct
    clock = (
        ROOT / "services/agent/crates/layerx-client/src/runtime_clock.rs"
    ).read_text()
    supervisor = (
        ROOT / "services/platform/hosted/runtime-clock/src/main.rs"
    ).read_text()
    clock_required = reader_names(clock, "std::env::var_os|number")
    supplied = set(re.findall(r'\.env\(\s*"([A-Z][A-Z0-9_]*)"', supervisor))
    assert clock_required and clock_required <= supplied
    assert "RuntimeClock::from_environment()" in main
    assert "/usr/local/bin/layerx-runtime-clock" in ENTRY
    args = set(re.findall(r"(?m)^ARG ([A-Z][A-Z0-9_]*)(?:=.*)?$", DOCKERFILE))
    port = re.search(r"(?ms)^configure_service_port\(\) \{.*?^\}", ENTRY)
    assert port
    service_branch = re.search(r"(?ms)^    service\)\n(.*?)^        ;;", ENTRY)
    assert service_branch and "configure_service_port" in service_branch.group()
    shell_required, shell_defaults = shell_environment(
        port.group() + service_branch.group()
    )
    return required | args | shell_required, optional | shell_defaults.keys()


def watched(path, patterns):
    relative = path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase("/" + relative, pattern) for pattern in patterns)


def test_platform_human_container_and_watch_inputs():
    config = definition()
    assert set(config) == {"service", "vars", "inputs"}
    assert set(config["service"]) == {"human"}
    service = config["service"]["human"]
    assert service["kind"] == "repo"
    assert ROOT / service["dockerfile"] == CONTAINER / "Dockerfile"
    assert service["root"] == "/"
    assert (ROOT / service["root"].lstrip("/")).is_dir()
    assert service["watch"] and isinstance(service["watch"], list)
    for pattern in service["watch"]:
        assert pattern.startswith("/")
        prefix = re.split(r"[*?\[]", pattern.lstrip("/"), maxsplit=1)[0]
        assert (ROOT / prefix).exists(), pattern
    assert watched(CONTAINER / "Dockerfile", service["watch"])
    instructions = DOCKERFILE.replace("\\\n", " ").splitlines()
    for instruction in instructions:
        if not instruction.startswith("COPY "):
            continue
        words = shlex.split(instruction)[1:]
        if any(word.startswith("--from=") for word in words):
            continue
        sources = [word for word in words[:-1] if not word.startswith("--")]
        assert sources
        for source in sources:
            path = ROOT / source
            assert path.exists(), source
            if source == ".":
                assert "/**" in service["watch"]
            else:
                assert watched(path, service["watch"]), source
    for manifest in re.findall(r"--manifest-path\s+(\S+)", DOCKERFILE):
        assert (ROOT / manifest).is_file()
        assert watched(ROOT / manifest, service["watch"])
    assert re.search(r"(?m)^USER 4020:4020$", DOCKERFILE)
    assert 'ENTRYPOINT ["/usr/local/bin/human-entrypoint", "service"]' in DOCKERFILE


def test_platform_human_variables_follow_the_selected_process():
    variables = definition()["vars"]["human"]
    required, optional = service_environment()
    assert required <= variables.keys(), sorted(required - variables.keys())
    assert variables.keys() <= required | optional, sorted(
        variables.keys() - required - optional
    )
    for value in variables.values():
        assert isinstance(value, str) and value
    for name, default in re.findall(r"(?m)^ARG ([A-Z][A-Z0-9_]*)=([^\n]+)$", DOCKERFILE):
        assert variables[name] == default
    main = rust_function(MAIN, "run")
    for name, default in re.findall(
        r'number\("([A-Z][A-Z0-9_]*)",\s*([0-9_]+)_[a-z0-9]+\)', main
    ):
        if name in variables:
            assert variables[name] != default.replace("_", "")
    _, defaults = shell_environment(ENTRY)
    for name, default in defaults.items():
        if name in variables:
            assert variables[name] != default
    assert "CARGO_PKG_VERSION" in MAIN
    assert not any(name.startswith("LAYERX_RUNTIME_CLOCK_") for name in variables)


def test_platform_human_operator_inputs():
    config = definition()
    variables = config["vars"]["human"]
    inputs = config["inputs"]["human"]
    expected = {}
    for name, value in variables.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        if value.startswith("env:"):
            env = value.removeprefix("env:")
            assert env == f"PLATFORM_HUMAN_{name}"
            assert re.fullmatch(r"[A-Z][A-Z0-9_]*", env)
            expected[env] = name
        else:
            assert not re.search(r"https?://|[A-Za-z0-9_-]{40,}", value)
    assert inputs.keys() == expected.keys()
    lines = DEFINITION.read_text().splitlines()
    for name, category in inputs.items():
        assert category in {"generated", "provider", "chain", "operator"}
        position = next(
            i for i, line in enumerate(lines) if line.startswith(name + " =")
        )
        assert position > 0 and lines[position - 1].startswith("# ")
        assert len(lines[position - 1].split()) >= 6
    assert all(category == "operator" for category in inputs.values())


def test_platform_human_listener_and_health():
    config = definition()
    variables = config["vars"]["human"]
    service = config["service"]["human"]
    listener = (SERVICE / "server.rs").read_text()
    http = (SERVICE / "server/http.rs").read_text()
    assert variables["LAYERX_HUMAN_LISTENER"] == "plain"
    assert 'Some("plain")' in listener and "Ok(Self::Plain)" in listener
    assert not any(name.startswith("LAYERX_HUMAN_TLS_") for name in variables)
    assert variables["LAYERX_HUMAN_BIND"] == "0.0.0.0:${{PORT}}"
    assert 1 <= int(variables["PORT"]) <= 65535
    assert 'required("LAYERX_HUMAN_BIND")' in MAIN
    assert ".parse::<SocketAddr>()" in MAIN
    assert "PlainServer::new(" in MAIN and "bind," in MAIN
    route = re.search(
        r'const (\w+): &str = "' + re.escape(service["health"]) + '";', http
    )
    assert route
    assert f'request.method == "GET" && request.path == {route.group(1)}' in http
    assert "readiness" in http


def test_platform_human_local_component_target():
    config = definition()
    service = config["service"]["human"]
    backend = (SERVICE / "server/backend.rs").read_text()
    component = (SERVICE / "server/component.rs").read_text()
    assert service["target"] == "other"
    lines = DEFINITION.read_text().splitlines()
    target = lines.index('target = "other"')
    assert lines[target - 1].startswith("# ") and "Unix" in lines[target - 1]
    assert 'PathBuf::from(required("LAYERX_HUMAN_COMPONENT_SOCKET")?)' in MAIN
    assert "UnixComponents::new(&component_endpoint" in MAIN
    assert "Uds::connect(&self.endpoint" in backend
    assert "socket_peercred(&stream)" in component
    assert "peer.uid.as_raw() != allowed_uid" in component
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["order"] == 1
    assert service["needs"] == []
    assert "volume" not in service
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())
    assert "human" not in rollout["service"]


def configure_port(environment):
    function = re.search(r"(?ms)^configure_service_port\(\) \{.*?^\}", ENTRY)
    assert function
    return subprocess.run(
        [
            "/bin/sh",
            "-eu",
            "-c",
            function.group()
            + '\nconfigure_service_port\nprintf "%s" "${LAYERX_HUMAN_BIND-}"',
        ],
        env={"PATH": os.defpath, **environment},
        capture_output=True,
        text=True,
        timeout=5,
        check=False,
    )


@pytest.mark.parametrize("port", ["1", "8080", "65535"])
def test_platform_human_entrypoint_uses_port(port):
    result = configure_port({"PORT": port, "LAYERX_HUMAN_BIND": "[::]:9443"})
    assert result.returncode == 0, result.stderr
    assert result.stdout == f"0.0.0.0:{port}"
    assert not result.stderr


def test_platform_human_entrypoint_preserves_explicit_bind_without_port():
    result = configure_port({"LAYERX_HUMAN_BIND": "[::]:9443"})
    assert result.returncode == 0, result.stderr
    assert result.stdout == "[::]:9443"
    assert not result.stderr


@pytest.mark.parametrize(
    "port", ["", "0", "65536", "8x80", "-1", "99999999999999999999"]
)
def test_platform_human_entrypoint_rejects_invalid_port(port):
    result = configure_port({"PORT": port})
    assert result.returncode == 64
    assert result.stderr == "PORT must be an integer from 1 to 65535\n"
    assert not result.stdout
