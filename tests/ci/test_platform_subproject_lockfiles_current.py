import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def assert_lockfile_resolves_locked(directory):
    manifest = f"{directory}/Cargo.toml"
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
    assert result.returncode == 0, f"{manifest}: {result.stderr}"


def test_verify_receipt_sample_lockfile_resolves_locked():
    assert_lockfile_resolves_locked(
        "services/platform/examples/samples/verify-receipt-rust"
    )


def test_node_probe_lockfile_resolves_locked():
    assert_lockfile_resolves_locked("services/platform/hosted/node/tests/probe")


def test_platform_tooling_lockfile_resolves_locked():
    assert_lockfile_resolves_locked("tools/platform")
