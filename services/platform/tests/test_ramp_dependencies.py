import json
import subprocess
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


def test_dependency_policy_bans_paxeer_client(platform_metadata, tmp_path):
    package_names = {package["name"] for package in platform_metadata["packages"]}
    assert RETIRED_PACKAGES <= package_names
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
    assert result.returncode == 1, (result.stdout, result.stderr)
    assert "platform dependency policy: forbidden boundary" in result.stderr
    refused_packages = set(result.stderr.splitlines()[1:])
    assert RETIRED_PACKAGES <= refused_packages, result.stderr
