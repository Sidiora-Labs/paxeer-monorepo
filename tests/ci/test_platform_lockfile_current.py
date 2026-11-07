import subprocess
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_platform_lockfile_resolves_locked():
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--offline",
            "--format-version",
            "1",
            "--manifest-path",
            "services/platform/Cargo.toml",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr


def test_runtime_clock_is_a_platform_member():
    with (ROOT / "services/platform/Cargo.lock").open("rb") as lockfile:
        lock = tomllib.load(lockfile)
    clock_packages = [
        package
        for package in lock["package"]
        if package["name"] == "layerx-runtime-clock"
    ]
    assert len(clock_packages) == 1
