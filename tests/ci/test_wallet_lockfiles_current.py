"""Prove each wallet workspace installs against its committed pnpm lockfile."""

import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def assert_wallet_lockfile_installs_frozen(directory, *options):
    command = [
        "pnpm", "install", *options, "--frozen-lockfile", "--ignore-scripts",
    ]
    result = subprocess.run(
        command,
        cwd=ROOT / directory,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"{directory}: {' '.join(command)} exited {result.returncode}\n"
        f"{result.stdout}\n{result.stderr}"
    )


def test_sdk_wallet_lockfile_installs_frozen():
    assert_wallet_lockfile_installs_frozen("sdk/wallet", "--ignore-workspace")


def test_services_wallet_lockfile_installs_frozen():
    assert_wallet_lockfile_installs_frozen("services/wallet")


def test_apps_wallet_lockfile_installs_frozen():
    assert_wallet_lockfile_installs_frozen("apps/wallet")
