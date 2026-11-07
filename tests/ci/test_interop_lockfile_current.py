import subprocess
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_interop_lockfile_resolves_locked():
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--format-version",
            "1",
            "--manifest-path",
            "services/interop/Cargo.toml",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr


def test_committed_proof_is_a_member():
    with (ROOT / "services/interop/Cargo.lock").open("rb") as lockfile:
        lock = tomllib.load(lockfile)
    committed_proof_packages = [
        package
        for package in lock["package"]
        if package["name"] == "layerx-committed-proof"
    ]
    assert len(committed_proof_packages) == 1
