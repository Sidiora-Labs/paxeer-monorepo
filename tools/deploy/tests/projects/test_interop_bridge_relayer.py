"""Compare the bridge relayer definition with its container and runtime inputs."""

import fnmatch
import json
import re
import shlex
import subprocess
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
CONTAINER = ROOT / "docker/bridge-relayer"
CRATES = ROOT / "services/interop/crates"
DEFINITION = ROOT / "deploy/projects/interop/bridge-relayer.kvx"
SERVICE = "bridge-relayer"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def shell_source(path):
    return "\n".join(
        line
        for line in path.read_text().splitlines()
        if not line.lstrip().startswith("#")
    )


def shell_reads(source):
    names = {
        braced or plain
        for braced, plain in re.findall(
            r"\$(?:\{([A-Z][A-Z0-9_]*)(?:[^}]*)\}|([A-Z][A-Z0-9_]*))",
            source,
        )
    }
    for words in re.findall(r"for\s+\w+\s+in\s+([^;\n]+);?\s*do", source):
        names.update(re.findall(r"\b[A-Z][A-Z0-9_]+\b", words))
    return names


def file_rows():
    rows = []
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, guest, mode, roles = line.split("\t")
            rows.append((name, guest, mode, roles.split(",")))
    assert rows
    return rows


def rust_environment():
    sources = [
        path.read_text()
        for crate in ("layerx-bridge-relayer", "layerx-mirror-signer")
        for path in sorted((CRATES / crate / "src").rglob("*.rs"))
    ]
    source = "\n".join(sources)
    constants = dict(re.findall(
        r'\bconst\s+(\w+)\s*:\s*&str\s*=\s*"([^"]+)"\s*;', source
    ))
    required = set()
    for argument in re.findall(
        r'\b(?:std::)?env::var(?:_os)?\(\s*("[^"]+"|[A-Z][A-Z0-9_]*)\s*\)',
        source,
    ):
        if argument.startswith('"'):
            required.add(json.loads(argument))
        else:
            assert argument in constants, argument
            required.add(constants[argument])
    signer = (CRATES / "layerx-mirror-signer/src/lib.rs").read_text()
    bridge_options = signer.split("impl BridgeOptions {", 1)[1].split(
        "enum ChainKey", 1
    )[0]
    optional = set()
    for match in re.finditer(
        r'\benvironment\(\s*("[^"]+"|[A-Z][A-Z0-9_]*)\s*\)', bridge_options
    ):
        argument = match.group(1)
        if argument.startswith('"'):
            name = json.loads(argument)
        else:
            assert argument in constants, argument
            name = constants[argument]
        prefix = bridge_options[:match.start()]
        if re.search(r"if let Some\(\w+\)\s*=\s*$", prefix):
            optional.add(name)
        else:
            required.add(name)
    assert required
    assert optional
    return required, optional


def docker_copies():
    for line in (CONTAINER / "Dockerfile").read_text().splitlines():
        if not line.startswith("COPY "):
            continue
        words = shlex.split(line)[1:]
        if any(word.startswith("--from=") for word in words):
            continue
        words = [word for word in words if not word.startswith("--")]
        yield from words[:-1]


def watched(path, patterns):
    relative = path.resolve().relative_to(ROOT).as_posix()
    return any(
        fnmatch.fnmatchcase(relative, pattern.lstrip("/")) for pattern in patterns
    )


def test_bridge_relayer_service_row():
    document = definition()
    assert set(document) == {"service", "vars", "inputs"}
    for section in document.values():
        assert set(section) == {SERVICE}
    service = document["service"][SERVICE]
    assert service["kind"] == "repo"
    assert (ROOT / service["root"]).resolve() == ROOT
    assert (ROOT / service["dockerfile"]).resolve() == CONTAINER / "Dockerfile"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    deployments = sorted(
        (ROOT / "services/interop/deploy/bridge-relayer").glob("relayer-*.toml")
    )
    assert deployments
    for path in deployments:
        deployment = tomllib.loads(path.read_text())
        assert deployment["build"]["dockerfile"] == service["dockerfile"]
        assert {mount["destination"] for mount in deployment["mounts"]} == {
            service["volume"]
        }
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    if SERVICE in rollout:
        assert service["order"] == rollout[SERVICE]["order"]
    assert service["order"] >= 1
    assert service["needs"] == []


def test_bridge_relayer_tcp_transport_and_health():
    service = definition()["service"][SERVICE]
    transport = (CRATES / "layerx-bridge-relayer/src/cosign.rs").read_text()
    assert "TcpListener::bind(self.listen)" in transport
    assert "SslVerifyMode::PEER | SslVerifyMode::FAIL_IF_NO_PEER_CERT" in transport
    assert "tls.read_exact(&mut prefix)" in transport
    assert "tls.read_exact(&mut frame)" in transport
    assert service["target"] == "other"
    lines = DEFINITION.read_text().splitlines()
    target = next(
        index for index, line in enumerate(lines) if line.startswith("target =")
    )
    assert lines[target - 1].startswith("#")
    assert "TLS" in lines[target - 1] and "TCP" in lines[target - 1]
    api = (CRATES / "layerx-bridge-relayer/src/attestation_api.rs").read_text()
    routes = re.search(r'route @ \(([^)]+)\)', api)
    assert routes
    assert "health" not in routes.group(1)
    assert "health" not in service
    assert service["public"] == 0
    variables = definition()["vars"][SERVICE]
    assert "PORT" not in variables


def test_bridge_relayer_watch_covers_container_and_build_sources():
    service = definition()["service"][SERVICE]
    patterns = service["watch"]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        base = re.split(r"[*?\[]", pattern.lstrip("/"), maxsplit=1)[0].rstrip("/")
        assert (ROOT / base).exists(), pattern
    copies = list(docker_copies())
    assert copies
    for source in copies:
        path = ROOT / source
        assert path.exists(), source
        if path.is_file():
            assert watched(path, patterns), source
    for path in CONTAINER.iterdir():
        if path.is_file():
            assert watched(path, patterns), path
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    manifests = re.findall(r"--manifest-path\s+(\S+)", dockerfile)
    assert manifests
    visited = set()

    def visit(manifest):
        manifest = manifest.resolve()
        if manifest in visited:
            return
        visited.add(manifest)
        assert manifest.is_file(), manifest
        assert watched(manifest, patterns), manifest
        for path in (manifest.parent / "src").rglob("*"):
            if path.is_file():
                assert watched(path, patterns), path
        build = manifest.parent / "build.rs"
        if build.is_file():
            assert watched(build, patterns), build
            for relative in re.findall(
                r'crate_dir\.join\("([^"]+)"\)', build.read_text()
            ):
                dependency = (manifest.parent / relative).resolve()
                assert dependency.exists(), dependency
                assert watched(dependency, patterns), dependency
        cargo = tomllib.loads(manifest.read_text())
        workspace_path = next(
            parent / "Cargo.toml"
            for parent in manifest.parents
            if (parent / "Cargo.toml").is_file()
            and "workspace" in tomllib.loads((parent / "Cargo.toml").read_text())
        )
        workspace = tomllib.loads(workspace_path.read_text())
        for section in ("dependencies", "build-dependencies"):
            for name, settings in cargo.get(section, {}).items():
                if not isinstance(settings, dict):
                    continue
                directory = manifest.parent
                if settings.get("workspace"):
                    settings = workspace["workspace"]["dependencies"][name]
                    directory = workspace_path.parent
                if isinstance(settings, dict) and "path" in settings:
                    visit(directory / settings["path"] / "Cargo.toml")

    for manifest in set(manifests):
        workspace_path = ROOT / manifest
        assert watched(workspace_path, patterns)
        cargo = tomllib.loads(workspace_path.read_text())
        for member in cargo["workspace"]["members"]:
            member_path = workspace_path.parent / member / "Cargo.toml"
            package = tomllib.loads(member_path.read_text())["package"]["name"]
            if package in re.findall(r"--package\s+(\S+)", dockerfile):
                visit(member_path)
    assert visited


def test_bridge_relayer_variables_follow_source_reads():
    variables = definition()["vars"][SERVICE]
    entry = shell_source(CONTAINER / "entrypoint.sh")
    bootstrap = shell_source(CONTAINER / "env-entrypoint.sh")
    common = shell_source(ROOT / "docker/common/env-files.sh")
    shell_required = shell_reads(entry) | shell_reads(bootstrap)
    internal = set(re.findall(r"^export\s+([A-Z][A-Z0-9_]*)=", bootstrap, re.M))
    shell_required -= internal
    rust_required, rust_optional = rust_environment()
    file_names = {row[0] for row in file_rows()}
    arguments = set(re.findall(
        r"^ARG\s+([A-Za-z_][A-Za-z0-9_]*)", (CONTAINER / "Dockerfile").read_text(), re.M
    ))
    required = shell_required | rust_required | file_names | arguments
    allowed = required | rust_optional | shell_reads(common)
    assert required <= variables.keys(), sorted(required - variables.keys())
    assert variables.keys() <= allowed, sorted(variables.keys() - allowed)
    deployment = tomllib.loads(
        (ROOT / "services/interop/deploy/bridge-relayer/relayer-1.toml").read_text()
    )
    for name, value in variables.items():
        assert isinstance(value, str) and value
        if not value.startswith("env:"):
            assert deployment["env"][name] == value, name
    config = (CRATES / "layerx-bridge-relayer/src/config.rs").read_text()
    for constant, value in re.findall(r"pub const (ATTESTOR_\w+): \w+ = (\d+);", config):
        name = next(name for name in variables if name.endswith(constant))
        assert variables[name] == value


def test_bridge_relayer_inputs_have_names_categories_and_formats():
    document = definition()
    variables = document["vars"][SERVICE]
    inputs = document["inputs"][SERVICE]
    expected = set()
    lines = DEFINITION.read_text().splitlines()
    table = {name: guest for name, guest, _, _ in file_rows()}
    for name, value in variables.items():
        if not value.startswith("env:"):
            continue
        external = value.removeprefix("env:")
        assert external == f"INTEROP_{SERVICE.upper().replace('-', '_')}_{name}"
        assert external not in expected
        expected.add(external)
        assert inputs[external] in {"generated", "provider", "chain", "operator"}
        line = next(
            index for index, text in enumerate(lines)
            if text.startswith(external + " =")
        )
        comment = lines[line - 1]
        assert comment.startswith("# ") and ", as " in comment
        if name in table:
            assert "base64-encoded" in comment
            if "/signer/" in table[name]:
                assert inputs[external] == "chain"
    assert inputs.keys() == expected


def test_bridge_relayer_file_table_matches_all_attestor_mounts():
    rows = file_rows()
    paths = ROOT / "services/interop/deploy/bridge-relayer"
    policy = json.loads((paths / "signer-keys.json").read_text())
    for path in sorted(paths.glob("relayer-*.toml")):
        deployment = tomllib.loads(path.read_text())
        index = deployment["env"]["LAYERX_BRIDGE_ATTESTOR_INDEX"]
        selected = [
            (name, guest, mode) for name, guest, mode, roles in rows
            if "*" in roles or f"bridge-relayer-{index}" in roles
        ]
        actual = {name: guest for name, guest, _ in selected}
        assert len(actual) == len(selected)
        assert len(set(actual.values())) == len(selected)
        assert actual == {
            row["secret_name"]: row["guest_path"] for row in deployment["files"]
        }
        assert all(int(mode or "0600", 8) & 0o077 == 0 for _, _, mode in selected)
    mounts = {guest for _, guest, _, _ in rows}
    assert all(key["key_file"] in mounts for key in policy["keys"])


def test_bridge_relayer_container_materializes_files_before_privilege_drop():
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    ignores = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for source in docker_copies():
        if source.startswith("docker/"):
            assert "!" + source in ignores
    assert "ca-certificates jq libssl3 openssl util-linux" in dockerfile
    entrypoint = json.loads(next(
        line.removeprefix("ENTRYPOINT ") for line in dockerfile.splitlines()
        if line.startswith("ENTRYPOINT ")
    ))
    bootstrap = (CONTAINER / "env-entrypoint.sh").read_text()
    assert entrypoint == ["/usr/local/bin/bridge-relayer-env-entrypoint"]
    assert 'export LAYERX_ROLE="bridge-relayer-$index"' in bootstrap
    assert "exec /usr/local/bin/layerx-env-files /usr/local/bin/bridge-relayer-entrypoint" in bootstrap
    entry = (CONTAINER / "entrypoint.sh").read_text()
    assert 'chown 4101:4100 "$signer_keys"' in entry
    assert 'chmod 0400 "$signer_keys"' in entry
    assert entry.index('chmod 0400 "$signer_keys"') < entry.index("setpriv --reuid=4101")
    assert "setpriv --reuid=4102 --regid=4102 --groups=4100" in entry
    assert "setpriv --reuid=4102 --regid=4102 --clear-groups" in entry
    assert "trap 'stop; exit 0' TERM INT" in entry
    assert 'install -d -o 4102 -g 4102 -m 0700 /data/relayer /data/cosign' in entry


def test_bridge_relayer_requires_attestor_index_and_file_inputs():
    for value in (None, "", "0", "01", "-1", "abc", "1/2"):
        environment = {"PATH": "/usr/bin:/bin"}
        if value is not None:
            environment["LAYERX_BRIDGE_ATTESTOR_INDEX"] = value
        result = subprocess.run(
            ["/bin/sh", str(CONTAINER / "env-entrypoint.sh")],
            env=environment,
            capture_output=True,
            text=True,
            timeout=5,
        )
        assert result.returncode != 0
        assert "attestor index" in result.stderr
    result = subprocess.run(
        ["/bin/sh", str(ROOT / "docker/common/env-files.sh"), "/usr/bin/true"],
        env={
            "PATH": "/usr/bin:/bin",
            "LAYERX_FILES_TABLE": str(CONTAINER / "files.tsv"),
        },
        capture_output=True,
        text=True,
        timeout=5,
    )
    assert result.returncode == 1
    assert f"required variable {file_rows()[0][0]} is not set" in result.stderr
