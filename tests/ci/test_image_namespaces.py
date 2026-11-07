import os
from pathlib import Path
import re
import runpy
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
