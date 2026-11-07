import subprocess
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def _assert_resolves_locked(manifest):
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--format-version",
            "1",
            "--manifest-path",
            manifest,
        ],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr


def test_human_lockfile_resolves_locked():
    _assert_resolves_locked("services/human/Cargo.toml")


def test_intents_fuzz_lockfile_resolves_locked():
    _assert_resolves_locked("services/human/crates/layerx-intents/fuzz/Cargo.toml")


def test_human_kms_is_a_member():
    with (ROOT / "services/human/Cargo.lock").open("rb") as lockfile:
        lock = tomllib.load(lockfile)
    kms_packages = [
        package
        for package in lock["package"]
        if package["name"] == "layerx-human-kms"
    ]
    assert len(kms_packages) == 1
