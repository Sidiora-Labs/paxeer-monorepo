import fnmatch
import os
import re
import shlex
import subprocess
import tomllib
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/ramp.kvx"
DOCKERFILE = ROOT / "docker/ramps/Dockerfile"
MAIN = ROOT / "services/platform/ramps/reference/src/main.rs"
INIT = ROOT / "services/platform/ramps/env-init.sh"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def docker_copies():
    copies = []
    for line in DOCKERFILE.read_text().splitlines():
        words = shlex.split(line)
        if not words or words[0] != "COPY":
            continue
        if any(word.startswith("--from=") for word in words):
            continue
        copies.extend(word for word in words[1:-1] if not word.startswith("--"))
    return copies


def file_inputs():
    tables = [ROOT / source for source in docker_copies() if source.endswith(".tsv")]
    assert tables
    result = {}
    for table in tables:
        for line in table.read_text().splitlines():
            if not line or line.startswith("#"):
                continue
            name, destination, mode, roles = line.split("\t")
            assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
            assert destination.startswith("/")
            assert not mode or re.fullmatch(r"0?[0-7]{3}", mode)
            assert roles == "*"
            assert name not in result
            result[name] = destination
    return result


def source_variables():
    required = set(file_inputs())
    optional = set()
    for source in docker_copies():
        if not source.endswith(".sh"):
            continue
        text = (ROOT / source).read_text()
        required.update(re.findall(r"^\s*need\s+([A-Z][A-Z0-9_]*)\b", text, re.M))
        required.update(re.findall(r"\benv\.([A-Z][A-Z0-9_]*)\b", text))
        required.update(re.findall(r"\$([A-Z][A-Z0-9_]*)\b", text))
        for name, operator in re.findall(
            r"\$\{([A-Z][A-Z0-9_]*)(?:(:?[?+\-])|(?=\}))", text
        ):
            (optional if operator in ("-", ":-", "+", ":+") else required).add(name)
    rust = MAIN.read_text()
    required.update(re.findall(r'(?<!option_)\benv!\(\s*"([A-Z][A-Z0-9_]*)"', rust))
    optional.update(re.findall(r'\boption_env!\(\s*"([A-Z][A-Z0-9_]*)"', rust))
    for match in re.finditer(
        r'\benv::var(?:_os)?\(\s*"([A-Z][A-Z0-9_]*)"\s*\)', rust
    ):
        tail = rust[match.end():].split(";", 1)[0]
        target = (
            optional
            if re.search(r"\.(?:unwrap_or(?:_else|_default)?|ok)\(", tail)
            else required
        )
        target.add(match.group(1))
    required.update(
        re.findall(r"^ARG\s+([A-Za-z_][A-Za-z0-9_]*)", DOCKERFILE.read_text(), re.M)
    )
    return required, optional


def watched(path, patterns):
    relative = path.resolve().relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern.lstrip("/")) for pattern in patterns)


def cargo_inputs(manifest, seen=None):
    seen = set() if seen is None else seen
    manifest = manifest.resolve()
    if manifest in seen:
        return seen
    seen.add(manifest)
    data = tomllib.loads(manifest.read_text())
    workspace_root, workspace = manifest.parent, data.get("workspace")
    if workspace is None:
        for parent in manifest.parents:
            candidate = parent / "Cargo.toml"
            if candidate.is_file():
                workspace = tomllib.loads(candidate.read_text()).get("workspace")
                if workspace is not None:
                    workspace_root = parent
                    seen.add(candidate)
                    break
    assert workspace is not None, manifest
    tables = [data]
    tables.extend(data.get("target", {}).values())
    for table in tables:
        for kind in ("dependencies", "build-dependencies", "dev-dependencies"):
            for value in table.get(kind, {}).values():
                if not isinstance(value, dict):
                    continue
                if "path" in value:
                    cargo_inputs(manifest.parent / value["path"] / "Cargo.toml", seen)
    for value in workspace.get("dependencies", {}).values():
        if isinstance(value, dict) and "path" in value:
            cargo_inputs(workspace_root / value["path"] / "Cargo.toml", seen)
    for member in data.get("workspace", {}).get("members", []):
        for directory in manifest.parent.glob(member):
            cargo_inputs(directory / "Cargo.toml", seen)
    return seen


def test_ramp_container_and_watch_sources():
    service = definition()["service"]["ramp"]
    assert service["kind"] == "repo"
    assert ROOT / service["dockerfile"] == DOCKERFILE
    assert (ROOT / service["root"]).resolve() == ROOT
    assert "COPY . ." in DOCKERFILE.read_text()
    patterns = service["watch"]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert not PurePosixPath(pattern).is_absolute()
        assert ".." not in PurePosixPath(pattern).parts
        assert list(ROOT.glob(pattern)), pattern
    assert watched(DOCKERFILE, patterns)
    ignore = DOCKERFILE.with_name("Dockerfile.dockerignore")
    assert watched(ignore, patterns)
    for source in docker_copies():
        path = ROOT / source
        assert path.exists(), source
        if path.is_file():
            assert watched(path, patterns), source
        if source.startswith("docker/"):
            assert "!" + source in ignore.read_text().splitlines(), source
    manifest = re.search(r"--manifest-path\s+(\S+)", DOCKERFILE.read_text()).group(1)
    for path in cargo_inputs(ROOT / manifest):
        assert watched(path, patterns), path
        assert watched(path.parent / "src/main.rs", patterns), path


def test_ramp_variables_follow_source_reads():
    required, optional = source_variables()
    variables = definition()["vars"]["ramp"]
    assert required
    assert required <= set(variables), (
        f"missing variables: {sorted(required - set(variables))}"
    )
    assert set(variables) <= required | optional, (
        f"unread variables: {sorted(set(variables) - required - optional)}"
    )
    assert not (set(variables) & (optional - required)), (
        "optional defaults do not need overrides"
    )
    helper = (ROOT / "docker/common/env-files.sh").read_text()
    assert "base64 -d" in helper
    assert 'fail "required variable $name is not set' in helper
    assert set(file_inputs()) <= set(variables)


def test_ramp_inputs_are_named_classified_and_documented():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert all(set(section) == {"ramp"} for section in data.values())
    variables = data["vars"]["ramp"]
    inputs = data["inputs"]["ramp"]
    used = set()
    for name, value in variables.items():
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            env = value.removeprefix("env:")
            assert env == f"PLATFORM_RAMP_{name}"
            assert re.fullmatch(r"[A-Z][A-Z0-9_]*", env)
            used.add(env)
        else:
            assert name == "PORT" or "${{" in value
    assert used == set(inputs)
    lines = DEFINITION.read_text().splitlines()
    for env, word in inputs.items():
        assert word in {"generated", "provider", "chain", "operator"}
        index = next(
            i for i, line in enumerate(lines) if line.startswith(env + " =")
        )
        assert index > 0 and lines[index - 1].startswith("# "), env
        assert len(lines[index - 1].split()) >= 5, env
    for name in file_inputs():
        env = variables[name].removeprefix("env:")
        index = next(
            i for i, line in enumerate(lines) if line.startswith(env + " =")
        )
        assert "base64" in lines[index - 1], env


def test_ramp_health_dependencies_and_journal():
    service = definition()["service"]["ramp"]
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert service["railway_service"] == rollout["ramp"]["railway_service"]
    assert service["order"] == rollout["ramp"]["order"]
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    deployment = (INIT.parent / "deployment.yaml").read_text()
    replicas = int(re.search(r"^  replicas: (\d+)$", deployment, re.M).group(1))
    assert service["replicas"] == replicas
    health = re.search(r"readinessProbe:.*path: (/[a-z]+)", deployment).group(1)
    assert service["health"] == health
    assert f'request.path == "{health}"' in MAIN.read_text()
    assert '"local-journal"' in MAIN.read_text()
    assert "kind: Ingress" in deployment
    assert len(service["needs"]) == len(set(service["needs"]))
    assert service["needs"]
    for name in service["needs"]:
        assert name != "ramp" and name in rollout
        assert rollout[name]["order"] <= service["order"]
    journal_dir = re.search(r"^journal_dir=(/\S+)$", INIT.read_text(), re.M).group(1)
    assert PurePosixPath(service["volume"]) in PurePosixPath(journal_dir).parents
    assert '"$journal_dir/journal.jsonl"' in INIT.read_text()
    assert (
        'chown -R 4020:4020 "$secrets" "$config_dir" "$journal_dir"' in INIT.read_text()
    )
    assert 'exec setpriv --reuid=4020 --regid=4020' in INIT.read_text()


def test_ramp_port_reaches_the_plain_listener():
    variables = definition()["vars"]["ramp"]
    port = int(variables["PORT"])
    assert 1 <= port <= 65535
    assert variables["RAMP_LISTEN"] == "[::]:${{PORT}}"
    entrypoint = ROOT / "docker/ramps/entrypoint.sh"
    text = entrypoint.read_text()
    assert 'export RAMP_LISTEN="[::]:$PORT"' in text
    assert (
        'exec /usr/local/bin/layerx-env-files /usr/local/bin/ramp-env-init "$@"' in text
    )
    assert 'ENTRYPOINT ["/usr/local/bin/ramp-entrypoint"]' in DOCKERFILE.read_text()
    assert (
        "COPY --chmod=0555 docker/ramps/entrypoint.sh /usr/local/bin/ramp-entrypoint"
        in DOCKERFILE.read_text()
    )
    init = INIT.read_text()
    assert "listen: env.RAMP_LISTEN" in init
    assert 'listener: "plain"' in init
    assert "TcpListener::bind(&config.listen)" in MAIN.read_text()
    for value in (None, "", "zero", "-1", "0", "65536", "80\n", "1" * 100):
        environment = dict(os.environ)
        environment.pop("PORT", None)
        if value is not None:
            environment["PORT"] = value
        result = subprocess.run(
            ["/bin/sh", str(entrypoint)], env=environment, capture_output=True, text=True
        )
        assert result.returncode != 0, value
        assert "PORT" in result.stderr, value
