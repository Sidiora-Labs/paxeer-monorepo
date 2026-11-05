import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/lxp_test_escrow_replay_client.c",
    "tests/daemon/lxp_test_exchange_tif_client.c",
    "tests/daemon/lxp_test_oracle_transport_client.c",
    "tests/daemon/lxp_test_market_sandbox_profile.c",
    "tests/daemon/lxp_test_pay1.c",
    "tests/daemon/pay1.py",
)


def test_daemon_tests_clients_leaves_the_build():
    result = subprocess.run(
        ["make", "-n", "build", "test"],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=120,
        check=False,
    )

    assert result.returncode == 0, result.stdout
    for relative_path in RETIRED_PATHS:
        path = ROOT / relative_path
        assert not path.exists(), f"Retired path still exists: {relative_path}"
        assert not path.is_symlink(), f"Retired path is a symlink: {relative_path}"
        assert relative_path not in result.stdout, (
            f"Build recipe still names {relative_path}:\n{result.stdout}"
        )
