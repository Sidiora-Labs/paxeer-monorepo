import json
import subprocess
from copy import deepcopy
from pathlib import Path

import pytest


REPO_ROOT = Path(__file__).resolve().parents[3]
RETIRED_PACKAGES = {"layerx-paxeer-client", "layerx-intents"}


@pytest.fixture(scope="module")
def platform_metadata():
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--format-version",
            "1",
            "--manifest-path",
            str(REPO_ROOT / "services/platform/Cargo.toml"),
        ],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def test_ramp_crates_do_not_depend_on_paxeer_client(platform_metadata):
    packages = {package["name"]: package for package in platform_metadata["packages"]}
    for name in ("layerx-ramp-toolkit", "layerx-reference-ramp"):
        dependencies = {dependency["name"] for dependency in packages[name]["dependencies"]}
        assert dependencies.isdisjoint(RETIRED_PACKAGES), (name, dependencies)


def assert_dependency_policy_refuses(
    platform_metadata, tmp_path, ramp_name, dependency_name, transitive
):
    package_names = {package["name"] for package in platform_metadata["packages"]}
    assert RETIRED_PACKAGES <= package_names
    metadata = deepcopy(platform_metadata)
    packages = {package["name"]: package for package in metadata["packages"]}
    dependency_package = packages[dependency_name]
    nodes = {node["id"]: node for node in metadata["resolve"]["nodes"]}
    ramp_node = nodes[packages[ramp_name]["id"]]
    parent_node = nodes[ramp_node["dependencies"][0]] if transitive else ramp_node
    parent_node["dependencies"].append(dependency_package["id"])
    parent_node["deps"].append(
        {
            "name": dependency_name.replace("-", "_"),
            "pkg": dependency_package["id"],
            "dep_kinds": [{"kind": None, "target": None}],
        }
    )
    metadata_path = tmp_path / "metadata.json"
    metadata_path.write_text(json.dumps(metadata))
    result = subprocess.run(
        [
            "sh",
            str(REPO_ROOT / "services/platform/tools/dependency-policy.sh"),
            str(metadata_path),
        ],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, (result.stdout, result.stderr)
    assert "platform dependency policy: forbidden ramp dependency" in result.stderr
    assert f"{ramp_name} depends on {dependency_name}" in result.stderr


@pytest.mark.parametrize("ramp_name", ("layerx-ramp-toolkit", "layerx-reference-ramp"))
@pytest.mark.parametrize("dependency_name", sorted(RETIRED_PACKAGES))
@pytest.mark.parametrize("transitive", (False, True))
def test_dependency_policy_bans_paxeer_client(
    platform_metadata, tmp_path, ramp_name, dependency_name, transitive
):
    assert_dependency_policy_refuses(
        platform_metadata, tmp_path, ramp_name, dependency_name, transitive
    )


@pytest.mark.parametrize("ramp_name", ("layerx-ramp-toolkit", "layerx-reference-ramp"))
@pytest.mark.parametrize("dependency_name", ("rusqlite", "libsqlite3-sys"))
@pytest.mark.parametrize("transitive", (False, True))
def test_dependency_policy_bans_sqlite(
    platform_metadata, tmp_path, ramp_name, dependency_name, transitive
):
    assert_dependency_policy_refuses(
        platform_metadata, tmp_path, ramp_name, dependency_name, transitive
    )


def test_dependency_policy_passes_live_metadata(platform_metadata, tmp_path):
    package_names = {package["name"] for package in platform_metadata["packages"]}
    assert RETIRED_PACKAGES | {"rusqlite", "libsqlite3-sys"} <= package_names
    metadata_path = tmp_path / "metadata.json"
    metadata_path.write_text(json.dumps(platform_metadata))
    result = subprocess.run(
        [
            "sh",
            str(REPO_ROOT / "services/platform/tools/dependency-policy.sh"),
            str(metadata_path),
        ],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, (result.stdout, result.stderr)
    assert "platform dependency and unsafe policies passed" in result.stdout
