from pathlib import Path
import re
import shlex
import tomllib

import pytest
import yaml

from test_image_copy_sources_in_context import (
    ignored,
    ignore_rules,
    instructions,
    source_included,
    source_paths,
    target_copy_sources,
)


ROOT = Path(__file__).resolve().parents[2]
DOCKERFILE = ROOT / "docker/explorer-backend/Dockerfile"
BACKEND = "services/explorer/backend"


@pytest.fixture
def workflow():
    return yaml.safe_load(
        (ROOT / ".github/workflows/explorer-images.yml").read_text(encoding="utf-8")
    )


@pytest.fixture
def backend_row(workflow):
    rows = [
        row
        for row in workflow["jobs"]["publish"]["strategy"]["matrix"]["include"]
        if row["image"] == "paxeer-x-explorer-backend"
    ]
    assert len(rows) == 1
    return rows[0]


def test_backend_copy_sources_exist_and_are_included(backend_row):
    context = ROOT / backend_row["context"]
    dockerfile = ROOT / backend_row["dockerfile"]
    rules = ignore_rules(context, dockerfile)
    stages = sum(operation == "FROM" for _, operation, _ in instructions(dockerfile))
    sources = {
        source
        for stage in range(stages)
        for _, source in target_copy_sources(dockerfile, str(stage), {})
    }
    assert sources, "The backend image must copy its sources from the context"
    for source in sorted(sources):
        paths = source_paths(context, source)
        assert paths, f"COPY source has no matches: {source}"
        for path in paths:
            assert path.exists(), f"Missing COPY source: {source}"
            assert source_included(path, context, rules), f"Excluded COPY source: {source}"


@pytest.mark.parametrize(
    "path",
    [
        "mix.exs",
        "mix.lock",
        "config/config_helper.exs",
        "config/assets/precompiles-arbitrum.json",
        "rel/config.exs",
        "apps/explorer/mix.exs",
        "apps/explorer/lib/explorer.ex",
    ],
)
def test_backend_context_includes_build_inputs(path):
    source = f"{BACKEND}/{path}"
    assert (ROOT / source).is_file()
    assert not ignored(source, ignore_rules(ROOT, DOCKERFILE))


@pytest.mark.parametrize(
    "path",
    [
        "_build/prod/rel/blockscout/bin/blockscout",
        "deps/phoenix/mix.exs",
        "apps/explorer/_build/dev/lib/explorer/ebin/explorer.app",
        "apps/explorer/deps/decimal/mix.exs",
        "apps/explorer/node_modules/package/index.js",
        "apps/block_scout_web/assets/node_modules/package/index.js",
        "test/test_helper.exs",
        "apps/explorer/test/test_helper.exs",
        "apps/explorer/tests/example.exs",
        ".env",
        ".env.production",
        "config/.env",
        "apps/explorer/.env.local",
        "README.md",
    ],
)
def test_backend_context_excludes_non_build_inputs(path):
    assert ignored(f"{BACKEND}/{path}", ignore_rules(ROOT, DOCKERFILE))


@pytest.mark.parametrize(
    "path",
    [
        ".git/config",
        "apps/explorer/package.json",
        "services/explorer/deploy/tools/mix-in-builder.sh",
        "services/explorer/backend-other/mix.exs",
        "docker/explorer-backend/Dockerfile",
    ],
)
def test_backend_context_excludes_other_trees(path):
    assert ignored(path, ignore_rules(ROOT, DOCKERFILE))


def test_backend_publish_deploy_and_dockerfile_use_repository_root(backend_row, workflow):
    project = tomllib.loads(
        (ROOT / "deploy/projects/explorer/backend.kvx").read_text(encoding="utf-8")
    )["service"]["backend"]
    assert (ROOT / backend_row["context"]).resolve() == ROOT
    assert (ROOT / project["root"].lstrip("/")).resolve() == ROOT
    assert ROOT / backend_row["dockerfile"] == ROOT / project["dockerfile"] == DOCKERFILE
    copies = target_copy_sources(DOCKERFILE, "", {})
    assert copies
    assert all(source.startswith(f"{BACKEND}/") for _, source in copies)
    publish_steps = workflow["jobs"]["publish"]["steps"]
    builds = [
        step["with"]
        for step in publish_steps
        if step.get("uses", "").startswith("docker/build-push-action@")
    ]
    assert len(builds) == 1
    assert builds[0]["context"] == "${{ matrix.context }}"
    assert builds[0]["file"] == "${{ matrix.dockerfile }}"


def test_backend_runtime_checks_before_installing(workflow):
    steps = workflow["jobs"]["explorer-backend-test"]["steps"]
    runtimes = [step["run"] for step in steps if "docker version" in step.get("run", "")]
    assert len(runtimes) == 1
    script = runtimes[0]
    missing = re.search(
        r"^if ! docker version >/dev/null 2>&1; then\n(?P<body>.*?)^fi$",
        script,
        re.M | re.S,
    )
    assert missing, "Install the runtime only when docker version cannot reach its server"
    commands = [
        "sudo apt-get update",
        "sudo apt-get install --yes docker.io acl",
        "sudo systemctl start docker",
    ]
    for command in commands:
        assert script.count(command) == 1
        assert command in missing["body"]
        assert script.index("docker version") < script.index(command)
    assert [missing["body"].index(command) for command in commands] == sorted(
        missing["body"].index(command) for command in commands
    )
    socket = re.search(
        r"^  if \[ ! -r /var/run/docker.sock \] \|\| "
        r"\[ ! -w /var/run/docker.sock \]; then\n(?P<body>.*?)^  fi$",
        missing["body"],
        re.M | re.S,
    )
    assert socket, "Grant socket access only when the job user lacks read or write access"
    access = 'sudo setfacl -m "u:$(id -u):rw" /var/run/docker.sock'
    assert script.count(access) == 1
    assert access in socket["body"]
    assert shlex.split(script[missing.end():]) == [
        "docker", "version", "--format", "{{.Server.Version}}"
    ]
