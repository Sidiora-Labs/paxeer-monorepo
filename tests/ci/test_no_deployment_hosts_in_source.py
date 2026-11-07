"""Keep this repair's first-party sources independent of deployment inventory.

The owned paths are explicit: other owners' pages, vendored code and historical
spec records belong to their own hygiene tasks. Deployment data and executable
test fixtures are the only exceptions for private provider names.
"""

import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[2]
OWNED_PATHS = (
    "apps/perps/deploy/README.md",
    "deploy/launchpad/railway-values.json",
    "deploy/launchpad/railway/variables.json",
    "docker/localnode/config/config.toml",
    "docker/rpcnode/config/config.toml",
    "services/interop/deploy/x-websearch",
    "services/platform/hosted/gateway/src/ui_proxy.rs",
    "services/platform/hosted/interop/fly.toml",
    "services/wallet/deploy/env",
    "services/wallet/deploy/human.toml",
    "tools/bringup/ca.sh",
    "tools/bringup/check-live.sh",
    "tools/bringup/check-live.test.sh",
    "tools/bringup/edge.sh",
    "tools/bringup/railway-names.env",
    "tools/hpx/fleet-keygen.sh",
)
DEPLOYMENT_DATA = {
    "tools/bringup/railway-names.env",
    "deploy/launchpad/railway-values.json",
    "deploy/launchpad/railway/variables.json",
}
FIXTURE_FILES = {"tools/bringup/check-live.test.sh"}
PROVIDER_PUBLIC_HOST = re.compile(
    r"(?<![\w${<.-])[a-z0-9-]+(?:\.[a-z0-9-]+)*\."
    r"(?:railway\.app|rlwy\.net|fly\.dev)\b",
    re.IGNORECASE,
)
PRIVATE_PROVIDER_FORM = re.compile(r"\.railway\.internal\b", re.IGNORECASE)
FIXED_PRIVATE_HOST = re.compile(
    r"(?<![\w$.-])(?:[a-z0-9-]+\.)+internal\b", re.IGNORECASE
)
IPV4 = re.compile(r"(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])")
DOCUMENTATION_NETWORKS = tuple(
    ipaddress.ip_network(cidr)
    for cidr in ("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
)
XWEB = "services/interop/deploy/x-websearch"
TEMPLATES = tuple(f"{XWEB}/attestor-{number}.toml" for number in range(1, 5)) + (
    "services/platform/hosted/interop/fly.toml",
    "services/wallet/deploy/human.toml",
)
VARIABLE = re.compile(r"\$\{([A-Z][A-Z0-9_]*)\}")


def _owned_files():
    for name in OWNED_PATHS:
        path = ROOT / name
        assert path.exists(), f"Owned source is missing: {name}"
        if path.is_dir():
            yield from sorted(
                file for file in path.rglob("*")
                if file.is_file() and "__pycache__" not in file.parts
            )
        else:
            yield path


def _fixture_line(path, number, text):
    relative = path.relative_to(ROOT).as_posix()
    if relative in FIXTURE_FILES or "tests" in path.relative_to(ROOT).parts:
        return True
    if path.suffix == ".rs":
        marker = "#[cfg(test)]\nmod tests {"
        if marker in text:
            return number > text[:text.index(marker)].count("\n")
    return False


def test_no_provider_public_hosts_in_source():
    violations = []
    for path in _owned_files():
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if PROVIDER_PUBLIC_HOST.search(line):
                violations.append(f"{path.relative_to(ROOT)}:{number}: {line.strip()}")
    assert not violations, "Use configured origins or reserved example names:\n" + "\n".join(violations)
    gateway = (ROOT / "services/platform/hosted/gateway/src/ui_proxy.rs").read_text()
    assert 'Some("https://explorer-backend.example.net".into())' in gateway


def test_example_addresses_use_documentation_ranges():
    violations = []
    examples = []
    for path in _owned_files():
        text = path.read_text(encoding="utf-8")
        usage = False
        for number, line in enumerate(text.splitlines(), 1):
            if _fixture_line(path, number, text):
                continue
            if "cat <<'EOF'" in line:
                usage = True
            if line == "EOF":
                usage = False
            documentation = path.suffix == ".md" or path.name == "env"
            if not (documentation or usage or line.lstrip().startswith(("#", "//"))):
                continue
            for literal in IPV4.findall(line):
                address = ipaddress.ip_address(literal)
                if any(address in network for network in DOCUMENTATION_NETWORKS):
                    examples.append((path.relative_to(ROOT).as_posix(), literal))
                # Loopback checks describe actual local listeners, and wildcard
                # bind addresses are operational syntax rather than remote examples.
                elif not (address.is_loopback or address.is_unspecified):
                    violations.append(f"{path.relative_to(ROOT)}:{number}: {literal}")
    assert not violations, "Remote examples must use RFC 5737 addresses:\n" + "\n".join(violations)
    for name in (
        "apps/perps/deploy/README.md", "docker/localnode/config/config.toml",
        "docker/rpcnode/config/config.toml", "tools/hpx/fleet-keygen.sh",
        "tools/bringup/edge.sh",
    ):
        assert any(path == name for path, _ in examples), f"Missing documentation example: {name}"


def test_private_host_names_only_in_deploy_data():
    violations = []
    for path in _owned_files():
        relative = path.relative_to(ROOT).as_posix()
        text = path.read_text(encoding="utf-8")
        if relative in DEPLOYMENT_DATA:
            assert "Deployment data:" in "\n".join(text.splitlines()[:4]), relative
            if path.suffix == ".json":
                assert isinstance(json.loads(text), dict), relative
            continue
        for number, line in enumerate(text.splitlines(), 1):
            if _fixture_line(path, number, text):
                continue
            if PRIVATE_PROVIDER_FORM.search(line) or FIXED_PRIVATE_HOST.search(line):
                violations.append(f"{relative}:{number}: {line.strip()}")
    assert not violations, "Private network hosts belong in deployment inputs:\n" + "\n".join(violations)
    manifest = json.loads((ROOT / "deploy/launchpad/railway/variables.json").read_text())
    values = json.loads((ROOT / "deploy/launchpad/railway-values.json").read_text())
    suffix = ".railway" + ".internal:8080"
    for variable, service in (
        ("WEB_API_UPSTREAM", "api"), ("WEB_CHARTS_UPSTREAM", "charts"),
        ("WEB_GATEWAY_UPSTREAM", "gateway"), ("WEB_SOCKETS_UPSTREAM", "api-sockets"),
    ):
        assert values["web"][variable] == f"http://{service}{suffix}"
        assert manifest["web"]["buildArgs"][variable] == values["web"][variable]
    movement = next(line for line in (ROOT / "services/wallet/deploy/env").read_text().splitlines()
                    if line.startswith("LAYERX_HUMAN_MOVEMENT_PROVIDER_PAXEER_RPC_URLS -"))
    assert "production" in movement and "operator" in movement
    assert "localhost" not in movement and "127.0.0.1" not in movement


@pytest.mark.parametrize("name", TEMPLATES)
def test_deployment_templates_require_inputs_and_render_real_toml(name):
    template = (ROOT / name).read_text()
    variables = set(VARIABLE.findall(template)) - {"NAME"}
    assert variables, f"No required deploy variables: {name}"
    command = [sys.executable, str(ROOT / XWEB / "render_deploy_template.py"), str(ROOT / name)]
    environment = {key: value for key, value in os.environ.items() if key not in variables}
    missing = subprocess.run(command, env=environment, capture_output=True, text=True, check=False)
    assert missing.returncode == 1 and "required deployment variable" in missing.stderr
    assert missing.stdout == ""
    for variable in variables:
        if variable.endswith("_HOST"):
            value = variable.lower().replace("_", "-") + ".example.net"
        elif variable.endswith("REDIS_URL"):
            value = "rediss://redis.example.net:6379"
        else:
            value = "https://" + variable.lower().replace("_", "-") + ".example.net"
        environment[variable] = value
    rendered = subprocess.run(command, env=environment, capture_output=True, text=True, check=False)
    assert rendered.returncode == 0, rendered.stderr
    settings = tomllib.loads(rendered.stdout)["env"]
    original = tomllib.loads(template)["env"]
    for key, value in original.items():
        expected = VARIABLE.sub(lambda match: environment[match[1]], value)
        assert settings[key] == expected, (name, key)
    for variable in variables:
        empty_environment = dict(environment, **{variable: ""})
        empty = subprocess.run(command, env=empty_environment, capture_output=True, text=True, check=False)
        assert empty.returncode == 1 and variable in empty.stderr
        assert empty.stdout == ""


@pytest.mark.parametrize("rows,expected", (
    ("IDENTITY=DNS:identity.example.net\n", "identity.example.net"),
    ("IDENTITY=DNS:identity.example.net,DNS:alternate.example.net\n", "identity.example.net"),
    ("OTHER=DNS:other.example.net\n", None),
    ("IDENTITY=\n", None),
    ("IDENTITY=DNS:one.example.net\nIDENTITY=DNS:two.example.net\n", None),
    ("IDENTITY=DNS:bad;command\n", None),
))
def test_bringup_reads_required_private_host_from_deployment_data(tmp_path, rows, expected):
    source = (ROOT / "tools/bringup/check-live.sh").read_text()
    function = "private_host() {" + source.split("private_host() {", 1)[1].split("\n}\n", 1)[0] + "\n}\n"
    names = tmp_path / "names.env"
    names.write_text(rows)
    environment = dict(os.environ, LAYERX_CA_NAMES_FILE=str(names))
    result = subprocess.run(["bash", "-c", function + 'private_host "$1"', "check-live", "identity"],
                            env=environment, capture_output=True, text=True, check=False)
    if expected is None:
        assert result.returncode == 1 and "invalid private network host" in result.stderr
        assert result.stdout == ""
    else:
        assert result.returncode == 0, result.stderr
        assert result.stdout == expected
