"""Keep core build and API identities aligned with the release version file."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess


ROOT = Path(__file__).resolve().parents[2]
VERSION_FILE = Path("tools/release/version.json")
IMAGE_TABLE = ROOT / "services/platform/hosted/tests/beta-images.sh"
CORE_IMAGES = {
    "paxd-node": "docker/paxeer/Dockerfile.paxd-node",
    "paxd": "docker/paxeer/Dockerfile.paxd",
    "kernel": "docker/kernel/Dockerfile",
    "layerx": "docker/layerx/Dockerfile",
    "localnode": "docker/localnode/Dockerfile",
    "rpcnode": "docker/rpcnode/Dockerfile",
}


def _release_version():
    return json.loads((ROOT / VERSION_FILE).read_text())["version"]


def _run(args, cwd, *, input=None, env=None):
    return subprocess.run(
        args,
        cwd=cwd,
        input=input,
        env=env,
        text=True,
        capture_output=True,
        check=True,
        timeout=30,
    ).stdout.strip()


def _image_sources(cwd, repo_root=None):
    env = dict(os.environ)
    env.pop("REPO_ROOT", None)
    if repo_root is not None:
        env["REPO_ROOT"] = str(repo_root)
    output = _run(
        [
            "bash",
            "-euo",
            "pipefail",
            "-c",
            'source "$1"; shift; for name in "$@"; do '
            'image_source "$name"; printf "\\n"; done',
            "core-images",
            str(IMAGE_TABLE),
            *CORE_IMAGES,
        ],
        cwd,
        env=env,
    )
    return dict(zip(CORE_IMAGES, output.splitlines(), strict=True))


def test_core_image_tags_match_version_json(tmp_path):
    version = _release_version()
    assert version == "v7.0.0"
    expected = {
        name: f"ghcr.io/sidiora-labs/{name}:{version} {dockerfile}"
        for name, dockerfile in CORE_IMAGES.items()
    }
    assert _image_sources(tmp_path) == expected
    assert _image_sources(tmp_path, ROOT) == expected

    alternate = tmp_path / "release source"
    (alternate / VERSION_FILE).parent.mkdir(parents=True)
    (alternate / VERSION_FILE).write_text(json.dumps({"version": "v7.1.2"}))
    assert _image_sources(tmp_path, alternate) == {
        name: f"ghcr.io/sidiora-labs/{name}:v7.1.2 {dockerfile}"
        for name, dockerfile in CORE_IMAGES.items()
    }
    (alternate / VERSION_FILE).write_text("{}")
    result = subprocess.run(
        ["bash", "-c", 'source "$1"; image_source paxd', "images", str(IMAGE_TABLE)],
        cwd=tmp_path,
        env=dict(os.environ, REPO_ROOT=str(alternate)),
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode != 0
    assert result.stdout == ""


def _git(repo, *args):
    return _run(
        [
            "git",
            "-c",
            "user.name=Version test",
            "-c",
            "user.email=version-test@example.invalid",
            "-c",
            "commit.gpgSign=false",
            "-c",
            "tag.gpgSign=false",
            *args,
        ],
        repo,
    )


def _make_version(repo, *overrides):
    output = _run(
        [
            "make",
            "--no-print-directory",
            "-s",
            "-f",
            "chain.mk",
            "-f",
            "-",
            "LEDGER_ENABLED=false",
            *overrides,
            "print-release-identity",
        ],
        repo,
        input=".PHONY: print-release-identity\nprint-release-identity:\n"
        "\t@printf '%s\\n' '$(VERSION)' '$(ldflags)'\n",
        env=dict(os.environ, GOTOOLCHAIN="local"),
    )
    version, flags = output.splitlines()
    assert (
        f"-X github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version.Version={version}"
        in flags
    )
    return version


def test_chain_mk_version_falls_back_to_version_json(tmp_path):
    repo = tmp_path / "chain"
    repo.mkdir()
    shutil.copy2(ROOT / "chain.mk", repo / "chain.mk")
    (repo / VERSION_FILE).parent.mkdir(parents=True)
    shutil.copy2(ROOT / VERSION_FILE, repo / VERSION_FILE)
    _git(repo, "init", "-q", "-b", "development")
    _git(repo, "add", ".")
    _git(repo, "commit", "-qm", "Initial release identity")
    assert _make_version(repo) == _release_version()
    _git(repo, "tag", "unrelated/v99.0.0")
    assert _make_version(repo) == _release_version()
    _git(repo, "checkout", "-q", "--detach")
    assert _make_version(repo) == _release_version()

    (repo / VERSION_FILE).write_text(json.dumps({"version": "v7.1.2"}))
    assert _make_version(repo) == "v7.1.2"
    _git(repo, "checkout", "-qb", "release/v8.0.0")
    assert _make_version(repo) == "v7.1.2"
    _git(repo, "tag", "paxeer-network/v7.0.1")
    assert _make_version(repo) == "v8.0.0"
    _git(repo, "checkout", "-q", "development")
    assert _make_version(repo) == "v7.0.1"
    _git(repo, "checkout", "-qb", "release/v7.0.1-rc1")
    assert _make_version(repo) == "v7.0.1"
    _git(repo, "checkout", "-qb", "release/v6.0.0")
    assert _make_version(repo) == "v7.0.1"
    assert _make_version(repo, "VERSION=v7.1.2") == "v7.1.2"

    makefile = (ROOT / "Makefile").read_text()
    recipe = makefile.split("paxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "jq -er .version tools/release/version.json" in recipe
    assert '$(PAXEER_MAKE) build VERSION="$$version"' in recipe


def test_node_api_documents_carry_release_version():
    expected = _release_version().removeprefix("v")
    for relative in (
        "consensus/rpc/openapi/openapi.yaml",
        "tools/paxctl/sidecar/api/openapi.yaml",
    ):
        document = (ROOT / relative).read_text()
        info = document.split("\ninfo:\n", 1)[1].split("\n\nservers:", 1)[0]
        versions = re.findall(r"^  version: [\"']?([^\"'\s]+)[\"']?$", info, re.M)
        assert versions == [expected], relative
    sidecar = (ROOT / "tools/paxctl/sidecar/api/openapi.yaml").read_text()
    assert re.search(r"^  title: Paxeer paxctl sidecar API$", sidecar, re.M)


def test_paxd_dockerfile_default_tag_matches():
    dockerfile = (ROOT / "docker/paxeer/Dockerfile.paxd").read_text()
    assert dockerfile.splitlines()[0] == (
        f"ARG PAXD_IMAGE=ghcr.io/sidiora-labs/paxd-node:{_release_version()}"
    )
    assert dockerfile.splitlines()[1] == "FROM ${PAXD_IMAGE}"
