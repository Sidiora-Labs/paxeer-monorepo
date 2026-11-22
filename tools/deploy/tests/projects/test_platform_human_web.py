import json
import os
import re
import shlex
import stat
import subprocess
import tempfile
import tomllib
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/human-web.kvx"
ENV_READ = re.compile(
    r"process\s*\.\s*env\s*(?:\.\s*([a-zA-Z_]\w*)|"
    r"\[\s*(?:[\"']([a-zA-Z_]\w*)[\"']|([a-zA-Z_]\w*))\s*\])"
)


def definition():
    return tomllib.loads(DEFINITION.read_text())


def docker_instructions():
    text = (ROOT / definition()["service"]["human-web"]["dockerfile"]).read_text()
    return [
        line.strip()
        for line in re.sub(r"\\\n\s*", " ", text).splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


def image_defaults():
    defaults = {}
    for line in docker_instructions():
        if line.startswith("FROM "):
            defaults = {}
        if line.startswith("ENV "):
            defaults.update(token.split("=", 1) for token in shlex.split(line[4:]))
    return defaults


def build_arguments():
    return {
        line.split()[1].split("=", 1)[0]
        for line in docker_instructions()
        if line.startswith("ARG ")
    }


def source_environment():
    reads, required = set(), set()
    for folder in (
        "apps/web/src",
        "apps/web/copy",
        "apps/web/packages",
        "sdk/wallet/src",
        "services/agent/sdk/typescript/src",
    ):
        for path in (ROOT / folder).rglob("*"):
            if path.suffix not in {".ts", ".tsx", ".js", ".mjs"}:
                continue
            if any(
                part in {"node_modules", "dist", "fixtures", "test", "tests"}
                for part in path.parts
            ):
                continue
            if re.search(r"\.(?:test|spec)\.", path.name):
                continue
            source = path.read_text()
            for match in ENV_READ.finditer(source):
                names = {match[1] or match[2]}
                defaulted = set()
                if match[3]:
                    key = re.escape(match[3])
                    choices = re.search(
                        rf"\bconst\s+{key}\s*=\s*(\w+)\s*\?\s*"
                        r'"([A-Z][A-Z0-9_]*)"\s*:\s*"([A-Z][A-Z0-9_]*)"\s*;',
                        source[:match.start()],
                    )
                    assert choices, f"{path}: resolve computed environment key {match[3]}"
                    names = {choices[2], choices[3]}
                    fallback = re.match(
                        r"\s*\?\?\s*\(\s*(\w+)\s*\?\s*(\w+)\s*:\s*undefined\s*\)",
                        source[match.end():],
                    )
                    if fallback and fallback[1] == choices[1]:
                        assert re.search(
                            rf"\bconst\s+{re.escape(fallback[2])}\s*=", source
                        )
                        defaulted.add(choices[2])
                reads.update(names)
                starts = list(re.finditer(
                    r"^(?:export\s+)?(?:async\s+)?function\s+\w+",
                    source[:match.start()], re.M,
                ))
                start = starts[-1].start() if starts else 0
                end = source.find("\n}", match.end())
                block = source[start:end if end >= 0 else len(source)]
                assignment = re.search(
                    r"\bconst\s+(\w+)\s*=\s*$", source[start:match.start()]
                )
                alias = assignment[1] if assignment else None
                optional = False
                if alias:
                    guard = re.search(
                        r"if\s*\(([^\n]*)\)\s*\{?\s*return\s+"
                        r"(DEFAULT_\w+|undefined|Promise\.resolve\(undefined\))\s*;",
                        block,
                    )
                    optional = guard is not None and re.search(
                        rf"\b{re.escape(alias)}\b", guard[1]
                    ) is not None
                    if optional and guard[2].startswith("DEFAULT_"):
                        assert re.search(rf"\bconst\s+{guard[2]}\s*=", source)
                if re.match(
                    r"\s*(?:\?\?|\|\|)\s*['\"\d]", source[match.end():]
                ):
                    optional = True
                if (
                    'return { mode: "unconfigured", reason: "missing-configuration" }'
                    in block and "throw " not in block
                ):
                    optional = True
                if not optional:
                    required.update(names - defaulted)
    return reads, required - image_defaults().keys()


def test_repository_build_and_watch_paths():
    service = definition()["service"]["human-web"]
    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/web/Dockerfile"
    assert service["root"] == "."
    assert (ROOT / service["root"]).is_dir()
    assert (ROOT / service["dockerfile"]).is_file()
    patterns = service["watch"]
    assert len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert not Path(pattern).is_absolute() and ".." not in Path(pattern).parts
        assert list(ROOT.glob(pattern)), pattern
    copied = []
    for line in docker_instructions():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        tokens = [token for token in shlex.split(line[5:]) if not token.startswith("--")]
        copied.extend(tokens[:-1])
    copied.append("docker/web")
    for name in copied:
        path = ROOT / name
        assert path.exists(), name
        expected = f"{name}/**" if path.is_dir() else name
        assert any(
            expected == pattern
            or (
                pattern.endswith("/**")
                and PurePosixPath(name).is_relative_to(pattern[:-3])
            )
            or PurePosixPath(name).match(pattern)
            for pattern in patterns
        ), name


def test_variables_follow_source_readers():
    variables = definition()["vars"]["human-web"]
    reads, required = source_environment()
    script = (ROOT / "docker/web/entrypoint.sh").read_text()
    script_required = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*):\?", script))
    arguments = build_arguments()
    assert required | script_required | arguments <= variables.keys()
    assert variables.keys() <= reads | script_required | arguments | image_defaults().keys()
    for argument in arguments:
        assert any(
            line.startswith("RUN ") and f"${{{argument}:?" in line
            and f'{argument}="${argument}"' in line and "npm run build" in line
            for line in docker_instructions()
        ), argument
    for name, value in variables.items():
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert isinstance(value, str) and value
        if "URL" in name or "ORIGIN" in name:
            assert value.startswith("env:") or re.fullmatch(
                r"https://\$\{\{[a-zA-Z0-9_.-]+\}\}", value
            )


def test_inputs_are_named_classified_and_described():
    config = definition()
    variables = config["vars"]["human-web"]
    inputs = config["inputs"]["human-web"]
    expected = set()
    for name, value in variables.items():
        if value.startswith("env:"):
            env = value.removeprefix("env:")
            assert env == f"PLATFORM_HUMAN_WEB_{name}"
            expected.add(env)
    assert inputs.keys() == expected
    lines = DEFINITION.read_text().splitlines()
    for name, classification in inputs.items():
        assert classification in {"generated", "provider", "chain", "operator"}
        line = next(
            index for index, text in enumerate(lines) if text.startswith(f"{name} =")
        )
        assert line > 0
        assert lines[line - 1].startswith("# ") and len(lines[line - 1]) > 20


def test_listener_and_health_match_web_deployment():
    config = definition()
    service, variables = config["service"]["human-web"], config["vars"]["human-web"]
    deployment = (ROOT / "services/platform/hosted/human/web-deployment.yaml").read_text()
    assert variables["PORT"] == image_defaults()["PORT"]
    assert 0 < int(variables["PORT"]) < 65536
    assert f'containerPort: {variables["PORT"]}' in deployment
    assert image_defaults()["HOSTNAME"] == "0.0.0.0"
    assert image_defaults()["NODE_ENV"] == "production"
    assert service["health"] == re.search(
        r"readinessProbe:\s+httpGet:\s+path: (\S+)", deployment
    )[1]
    route = ROOT / "apps/web/src/app" / service["health"].lstrip("/") / "page.tsx"
    assert route.is_file() and "export default async function" in route.read_text()
    assert service["replicas"] == int(
        re.search(r"^  replicas: (\d+)$", deployment, re.M)[1]
    )
    assert service["restart"] == "ALWAYS"
    assert service["public"] == 1
    instructions = docker_instructions()
    command = json.loads(next(
        line[4:] for line in instructions if line.startswith("CMD ")
    ))
    assert command == ["node", "apps/web/server.js"]
    assert 'output: "standalone"' in (
        ROOT / "apps/web/next.config.mjs"
    ).read_text()


def test_dependencies_use_rollout_names_and_order():
    service = definition()["service"]["human-web"]
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert set(service["needs"]) == {"human", "gateway"}
    assert len(service["needs"]) == len(set(service["needs"]))
    for name in service["needs"]:
        if name in rollout:
            assert rollout[name]["railway_service"] == name
            assert service["order"] > rollout[name]["order"]
        else:
            dependency = tomllib.loads(
                DEFINITION.with_name(f"{name}.kvx").read_text()
            )["service"][name]
            assert service["order"] > dependency["order"]
            assert (ROOT / dependency["dockerfile"]).is_file()
    for value in definition()["vars"]["human-web"].values():
        for reference in re.findall(r"\$\{\{([^}]+)\}\}", value):
            if "." in reference:
                assert reference.split(".", 1)[0] in service["needs"]


def test_file_storage_is_persistent_and_owned_by_runtime_user():
    config = definition()
    service, variables = config["service"]["human-web"], config["vars"]["human-web"]
    volume = PurePosixPath(service["volume"])
    assert volume.is_absolute() and str(volume) != "/"
    script = (ROOT / "docker/web/entrypoint.sh").read_text()
    directories = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*):\?", script))
    assert directories
    for name in directories:
        path = PurePosixPath(variables[name])
        assert path.is_relative_to(volume) and path != volume
        assert ".." not in path.parts
    assert len({variables[name] for name in directories}) == len(directories)
    assert 'mkdir -p "$directory"' in script
    assert 'chmod 0700 "$directory"' in script
    assert 'chown 65532:65532 "$directory"' in script
    assert 'exec gosu 65532:65532 "$@"' in script
    assert 'exec "$@"' in script
    instructions = docker_instructions()
    assert any(line.startswith("RUN ") and " gosu" in line for line in instructions)
    assert (
        'COPY --chmod=0755 docker/web/entrypoint.sh /usr/local/bin/web-entrypoint'
        in instructions
    )
    assert 'ENTRYPOINT ["/usr/local/bin/web-entrypoint"]' in instructions


def test_entrypoint_prepares_private_directories_and_executes_command():
    script = ROOT / "docker/web/entrypoint.sh"
    uid = 65532 if os.getuid() == 0 else os.getuid()
    gid = 65532 if os.getuid() == 0 else os.getgid()
    credentials = (
        {"user": uid, "group": gid, "extra_groups": []} if os.getuid() == 0 else {}
    )
    with tempfile.TemporaryDirectory() as temporary:
        base = Path(temporary)
        base.chmod(0o777)
        environment = dict(os.environ)
        variables = definition()["vars"]["human-web"]
        names = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*):\?", script.read_text()))
        for name in names:
            environment[name] = str(base / PurePosixPath(variables[name]).name)
        for _ in range(2):
            result = subprocess.run(
                ["/bin/sh", str(script), "/usr/bin/id", "-u"],
                env=environment, cwd=base, capture_output=True, text=True,
                timeout=10, **credentials,
            )
            assert result.returncode == 0, result.stderr
            assert result.stdout.strip() == str(uid)
            for name in names:
                metadata = Path(environment[name]).stat()
                assert stat.S_ISDIR(metadata.st_mode)
                assert stat.S_IMODE(metadata.st_mode) == 0o700
                assert (metadata.st_uid, metadata.st_gid) == (uid, gid)
        result = subprocess.run(
            ["/bin/sh", str(script), "/bin/sh", "-c", "exit 23"],
            env=environment, cwd=base, capture_output=True, text=True,
            timeout=10, **credentials,
        )
        assert result.returncode == 23
