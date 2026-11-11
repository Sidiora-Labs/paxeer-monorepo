import os
from pathlib import Path
import re
import runpy
import shlex
from string import Template
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
COMPOSE_FILES = (
    "docker/docker-compose.yml",
    "docker/docker-compose.monitoring.yml",
    "services/explorer/deploy/docker-compose.local.yml",
    "tools/ops/monitoring/docker-compose.yml",
)
NODE_REPOSITORY = "ghcr.io/sidiora-labs/localnode"
WEBSEARCH_REPOSITORY = "ghcr.io/sidiora-labs/x-websearch"


def workflow(name):
    return yaml.load((WORKFLOWS / name).read_text(), Loader=yaml.BaseLoader)


@pytest.mark.parametrize("owner", ["Sidiora-Labs", "Another-OWNER", "lowercase"])
def test_ghcr_tags_use_repository_owner(tmp_path, owner):
    steps = workflow("paxeer-docker-publish.yml")["jobs"]["pax-images"]["steps"]
    image = next(step for step in steps if step.get("id") == "image")
    assert image["env"] == {"REPOSITORY_OWNER": "${{ github.repository_owner }}"}
    environment_file = tmp_path / "github-env"
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", "-c", image["run"]],
        env={
            **os.environ,
            "REPOSITORY_OWNER": owner,
            "GITHUB_ENV": str(environment_file),
        },
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode == 0, result.stderr
    assert environment_file.read_text() == f"IMAGE_NAME=ghcr.io/{owner.lower()}/pax\n"
    build = next(step for step in steps if step.get("id") == "build_push_image")
    assert steps.index(image) < steps.index(build)
    assert build["with"]["push"] == "true"
    assert build["with"]["tags"].splitlines() == [
        "${{ env.IMAGE_NAME }}:${{ env.MAJOR_VERSION }}",
        "${{ env.IMAGE_NAME }}:${{ env.MAJOR_VERSION }}.${{ env.MINOR_VERSION }}",
        "${{ env.IMAGE_NAME }}:${{ env.MAJOR_VERSION }}.${{ env.MINOR_VERSION }}.${{ env.PATCH_VERSION }}",
        "${{ env.IMAGE_NAME }}:v${{ env.MAJOR_VERSION }}.${{ env.MINOR_VERSION }}.${{ env.PATCH_VERSION }}",
    ]
    record = next(
        step for step in steps if step["name"] == "Record image digest in release notes"
    )
    assert steps.index(build) < steps.index(record)
    assert "`%s:v%s@%s`" in record["run"]
    assert '"$IMAGE_NAME" "$VERSION" "$IMAGE_DIGEST" >> "$notes"' in record["run"]
    runbook = (ROOT / "docs/release/7.0.0-validator-runbook.md").read_text()
    assert "ghcr.io/sidiora-labs/pax:7.0.0" in runbook
    assert "ghcr.io/paxeer-network/" not in runbook.lower()


def test_no_unused_registry_login():
    steps = workflow("paxeer-docker-publish.yml")["jobs"]["pax-images"]["steps"]
    logins = [
        step for step in steps if step.get("uses", "").startswith("docker/login-action@")
    ]
    assert len(logins) == 1
    assert logins[0]["with"] == {
        "registry": "ghcr.io",
        "username": "${{ github.repository_owner }}",
        "password": "${{ secrets.GITHUB_TOKEN }}",
    }
    text = (WORKFLOWS / "paxeer-docker-publish.yml").read_text()
    assert "DOCKERHUB" not in text
    assert "DOCKER_IMAGES" not in text
    assert "/packages/container/pax" not in text


def test_image_source_label_uses_github_repository():
    steps = workflow("paxeer-hpx-registry.yml")["jobs"]["release"]["steps"]
    publish = next(step for step in steps if "ko build" in step.get("run", ""))
    assert (
        '--image-label="org.opencontainers.image.source=${{ github.server_url }}/${{ github.repository }}"'
        in publish["run"]
    )


def test_no_old_repository_name_in_workflows():
    # The root Go module github.com/Sidiora-Labs/Paxeer-X-Network and its
    # storage/state_db package retain their import names until an owner-approved
    # module rename. Exempt these Go imports in paxeer-db-tests.yml and
    # paxeer-go-test.yml; image/tag fields and docker build/tag commands are
    # registry references and remain checked, including outside YAML fields.
    go_import = re.compile(
        r"(?<![\w./:-])github\.com/Sidiora-Labs/Paxeer-X-Network"
        r"(?:/[A-Za-z0-9_./-]+)?(?=[\s\"']|$)"
    )
    registry = runpy.run_path(str(WORKFLOWS / "scripts/integration_registry.py"))
    images = registry["image_environment"](
        registry["registry_config"](), "Sidiora-Labs/paxeer-monorepo", "1"
    )
    assert images["GHCR_OWNER"] == "sidiora-labs"
    assert all(
        image.startswith("ghcr.io/sidiora-labs/")
        for name, image in images.items()
        if name != "GHCR_OWNER"
    )
    paths = [path for path in WORKFLOWS.rglob("*") if path.is_file()]
    assert paths
    for path in paths:
        text = path.read_text()
        relative = path.relative_to(ROOT)
        for namespace in re.findall(r"ghcr\.io/(\$\{[^}]+\}|[^/\s]+)", text):
            if namespace == "${REPOSITORY_OWNER,,}":
                assert path.name == "paxeer-docker-publish.yml", relative
                assert "REPOSITORY_OWNER: ${{ github.repository_owner }}" in text
            elif namespace == "{config['owner']}":
                assert path == WORKFLOWS / "scripts/integration_registry.py", relative
            else:
                assert namespace == "sidiora-labs", (relative, namespace)
        if path.name in {"paxeer-db-tests.yml", "paxeer-go-test.yml"}:
            text = go_import.sub("", text)
        assert "paxeer-x-network" not in text.lower(), relative
        assert "ghcr.io/paxeer-network/" not in text.lower(), relative


def image_environment(tmp_path):
    environment = dict(os.environ)
    for name in (
        "LOCALNODE_IMAGE_TAG",
        "X_WEBSEARCH_IMAGE_TAG",
        "MAKEFLAGS",
        "MFLAGS",
        "COMPOSE_FILE",
        "COMPOSE_ENV_FILES",
    ):
        environment.pop(name, None)
    environment.update(
        PROJECT_HOME=str(ROOT),
        GO_PKG_PATH=str(tmp_path / "go"),
        GOCACHE=str(tmp_path / "gocache"),
        X_WEBSEARCH_HOME=str(tmp_path / "websearch"),
        USERID=str(os.getuid()),
        GROUPID=str(os.getgid()),
        GRAFANA_ADMIN_PASSWORD_FILE=str(tmp_path / "grafana-password"),
    )
    return environment


def make_commands(target, environment, overrides=()):
    # Only print the real callers and producers, including stop prerequisites.
    result = subprocess.run(
        [
            "make", "--no-print-directory", "--dry-run", "-f", "chain.mk",
            target, *overrides,
        ],
        cwd=ROOT,
        env=environment,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    return [
        shlex.split(line)
        for line in result.stdout.replace("\\\n", " ").splitlines()
        if line.strip()
    ]


def build_alias(commands):
    builds = [command for command in commands if command[:2] == ["docker", "build"]]
    assert len(builds) == 1, commands
    tags = [
        builds[0][index + 1]
        for index, token in enumerate(builds[0])
        if token == "--tag"
    ]
    assert "pax-chain/localnode" in tags
    aliases = [tag for tag in tags if tag.startswith(NODE_REPOSITORY + ":")]
    assert len(aliases) == 1, tags
    assert builds[0][builds[0].index("-f") + 1] == "docker/localnode/Dockerfile"
    return aliases[0]


def ci_image_contract(run_id):
    integration = workflow("paxeer-integration-test.yml")
    registry = runpy.run_path(str(WORKFLOWS / "scripts/integration_registry.py"))
    config = registry["registry_config"]()
    assert config["packages"]["localnode"] == "pax-chain-integration-test-localnode"
    environment = registry["image_environment"](
        config, "Sidiora-Labs/paxeer-monorepo", run_id
    )
    tag = integration["env"]["LOCALNODE_IMAGE_TAG"].replace(
        "${{ github.run_id }}", str(run_id)
    )
    assert tag == f"ci-{run_id}"
    environment["LOCALNODE_IMAGE_TAG"] = tag
    steps = integration["jobs"]["integration-tests"]["steps"]
    load = next(
        step
        for step in steps
        if step.get("name") == "Load prebuilt paxd and pull Docker images"
    )
    commands = [
        shlex.split(Template(line).substitute(environment))
        for line in load["run"].splitlines()
    ]
    source = environment["GHCR_LOCALNODE_RUN_IMAGE"]
    aliases = [
        command[3]
        for command in commands
        if command[:3] == ["docker", "tag", source]
        and command[3].startswith(NODE_REPOSITORY + ":")
    ]
    assert len(aliases) == 1, commands
    alias = aliases[0]
    assert alias == f"{NODE_REPOSITORY}:{tag}"
    assert commands.index(["docker", "pull", source]) < commands.index(
        ["docker", "tag", source, alias]
    )
    assert ["docker", "tag", source, "pax-chain/localnode"] in commands
    assert ["docker", "pull", environment["GHCR_RPCNODE_RUN_IMAGE"]] in commands
    assert ["docker", "tag", environment["GHCR_RPCNODE_RUN_IMAGE"], "pax-chain/rpcnode"] in commands
    start = next(
        step for step in steps
        if step.get("name") == "Start 4 node docker cluster"
    )
    assert steps.index(load) < steps.index(start)
    assert "make -f chain.mk docker-cluster-start-ci" in start["run"]
    prepare = integration["jobs"]["prepare-cluster"]["steps"]
    build = next(
        step for step in prepare
        if step.get("name") == "Build localnode image (cached)"
    )
    tags = build["with"]["tags"].replace(
        "${{ env.LOCALNODE_IMAGE_TAG }}", tag
    ).splitlines()
    assert "pax-chain/localnode" in tags
    assert alias in tags
    return environment, alias


def test_cluster_image_identity_contract(tmp_path):
    environment = image_environment(tmp_path)
    revision = subprocess.check_output(
        ["git", "log", "-1", "--format=%H"], cwd=ROOT, text=True
    ).strip()
    alias = build_alias(make_commands("build-docker-node", environment))
    assert alias == f"{NODE_REPOSITORY}:local-{revision}"
    websearch = yaml.safe_load((ROOT / "docker/docker-compose.yml").read_text())["services"]["x-websearch"]
    assert websearch["build"] == {
        "context": "..",
        "dockerfile": "docker/x-websearch/Dockerfile",
    }
    _, first = ci_image_contract("101")
    _, second = ci_image_contract("202")
    assert first != second


@pytest.mark.parametrize(
    "target,overlay,run_id,overrides",
    [
        ("docker-cluster-start", None, None, ()),
        ("docker-cluster-start-skipbuild", None, None, ()),
        ("docker-cluster-start-ci", None, "101", ()),
        ("docker-cluster-start-ci", None, "202", ()),
        ("docker-cluster-start-monitoring", "monitoring", None, ()),
        ("docker-cluster-start-giga-mixed", "giga-mixed", None, ()),
        ("docker-cluster-stop", None, None, ()),
        ("docker-cluster-stop-monitoring", "monitoring", None, ()),
        (
            "docker-cluster-start",
            None,
            None,
            (
                "LOCALNODE_IMAGE_TAG=local-experiment",
                "X_WEBSEARCH_IMAGE_TAG=search-experiment",
            ),
        ),
    ],
)
def test_compose_image_identity_contract(tmp_path, target, overlay, run_id, overrides):
    environment = image_environment(tmp_path)
    assert "LOCALNODE_IMAGE_TAG" not in environment
    assert "X_WEBSEARCH_IMAGE_TAG" not in environment
    if run_id:
        ci_environment, expected_node = ci_image_contract(run_id)
        environment.update(ci_environment)
    else:
        expected_node = build_alias(
            make_commands("build-docker-node", environment, overrides)
        )
    revision = subprocess.check_output(
        ["git", "log", "-1", "--format=%H"], cwd=ROOT, text=True
    ).strip()
    expected_websearch = f"{WEBSEARCH_REPOSITORY}:local-{revision}"
    if overrides:
        expected_websearch = f"{WEBSEARCH_REPOSITORY}:search-experiment"
    commands = make_commands(target, environment, overrides)
    callers = []
    for command in commands:
        for index in range(len(command) - 1):
            if command[index : index + 2] != ["docker", "compose"]:
                continue
            assignments = {}
            cursor = index - 1
            while cursor >= 0 and re.match(r"[A-Z_][A-Z0-9_]*=", command[cursor]):
                name, value = command[cursor].split("=", 1)
                assignments[name] = value
                cursor -= 1
            assert assignments["LOCALNODE_IMAGE_TAG"] == expected_node.split(":")[-1]
            assert assignments["X_WEBSEARCH_IMAGE_TAG"] == expected_websearch.split(":")[-1]
            action = next(
                token for token in command[index + 2 :] if token in {"up", "down"}
            )
            options = command[index + 2 : command.index(action, index + 2)]
            callers.append((action, options))
            # Run the real Compose executable in config-only mode for every service.
            # Per-service rendering also proves all four nodes, even if --images
            # deduplicates identical references in a full-project listing.
            for service, expected in [
                *[(f"node{node}", expected_node) for node in range(4)],
                ("x-websearch", expected_websearch),
            ]:
                result = subprocess.run(
                    ["docker", "compose", *options, "config", "--images", service],
                    cwd=ROOT / "docker",
                    env={**environment, **assignments},
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                assert result.returncode == 0, result.stderr
                assert result.stdout.splitlines() == [expected], (
                    target, service, result.stdout
                )
    if "start" in target:
        assert [action for action, _ in callers] == ["down", "up"]
        if not run_id:
            assert build_alias(commands) == expected_node
    else:
        assert [action for action, _ in callers] == ["down"]
    if overlay:
        assert callers[-1][1] == [
            "-f", "docker-compose.yml", "-f", f"docker-compose.{overlay}.yml"
        ]


@pytest.mark.parametrize("missing", ["LOCALNODE_IMAGE_TAG", "X_WEBSEARCH_IMAGE_TAG"])
@pytest.mark.parametrize("value", [None, ""])
def test_direct_compose_requires_nonempty_image_tags(tmp_path, missing, value):
    environment = image_environment(tmp_path)
    environment.update(
        LOCALNODE_IMAGE_TAG="local-explicit",
        X_WEBSEARCH_IMAGE_TAG="search-explicit",
    )
    if value is None:
        environment.pop(missing)
    else:
        environment[missing] = value
    result = subprocess.run(
        ["docker", "compose", "-f", "docker-compose.yml", "config", "--images"],
        cwd=ROOT / "docker",
        env=environment,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode != 0
    assert missing in result.stderr
    assert "required variable" in result.stderr


@pytest.mark.parametrize("relative", COMPOSE_FILES)
def test_compose_first_party_images_use_ghcr_namespace(relative):
    services = yaml.safe_load((ROOT / relative).read_text())["services"]
    expected = {}
    if relative == "docker/docker-compose.yml":
        expected = {f"node{i}": "localnode" for i in range(4)}
        expected["x-websearch"] = "x-websearch"
    elif relative == "services/explorer/deploy/docker-compose.local.yml":
        expected = {
            name: f"paxeer-x-explorer-{name}"
            for name in (
                "smart-contract-verifier",
                "sig-provider",
                "backend",
                "frontend",
            )
        }
    for name, repository in expected.items():
        assert services[name]["image"].startswith(
            f"ghcr.io/sidiora-labs/{repository}:"
        ), (relative, name, services[name]["image"])
    images = [service["image"] for service in services.values() if "image" in service]
    assert images, relative
    for image in images:
        assert not image.startswith("pax-chain/"), (relative, image)
        if image.startswith("ghcr.io/"):
            assert image.startswith("ghcr.io/sidiora-labs/"), (relative, image)


@pytest.mark.parametrize("relative", COMPOSE_FILES)
def test_compose_images_have_explicit_tags(relative):
    services = yaml.safe_load((ROOT / relative).read_text())["services"]
    images = {
        name: service["image"]
        for name, service in services.items()
        if "image" in service
    }
    assert images, relative
    for name, image in images.items():
        reference = re.fullmatch(r"[^:\s]+:(.+)", image)
        assert reference, (relative, name, image)
        tag = reference[1]
        if image.startswith("ghcr.io/sidiora-labs/"):
            required = re.fullmatch(r"\$\{([A-Z][A-Z0-9_]*):\?[^{}]+\}", tag)
            assert required, (relative, name, image)
            if relative == "services/explorer/deploy/docker-compose.local.yml":
                assert required[1] == "EXPLORER_IMAGE_TAG", (name, image)
            elif relative == "docker/docker-compose.yml":
                variable = (
                    "X_WEBSEARCH_IMAGE_TAG"
                    if name == "x-websearch"
                    else "LOCALNODE_IMAGE_TAG"
                )
                assert required[1] == variable, (name, image)
        else:
            assert re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}", tag), (
                relative,
                name,
                image,
            )
            assert tag.lower() not in {
                "latest",
                "stable",
                "main",
                "edge",
                "rolling",
                "beta",
            }, (relative, name, image)

    if relative == "docker/docker-compose.monitoring.yml":
        ops = yaml.safe_load(
            (ROOT / "tools/ops/monitoring/docker-compose.yml").read_text()
        )["services"]
        assert images["prometheus"] == ops["prometheus"]["image"]
        assert images == {
            "prometheus": "prom/prometheus:v2.55.1",
            "grafana": "grafana/grafana:11.3.0",
        }
    elif relative == "tools/ops/monitoring/docker-compose.yml":
        assert images == {
            "prometheus": "prom/prometheus:v2.55.1",
            "alertmanager": "prom/alertmanager:v0.27.0",
            "blackbox": "prom/blackbox-exporter:v0.25.0",
            "node-exporter": "prom/node-exporter:v1.8.2",
        }
