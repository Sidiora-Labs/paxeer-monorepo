import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/lxp_test_arbiter_admission.c",
    "tests/daemon/lxp_test_arbiter_native_authority.c",
    "tests/daemon/lxp_test_arbiter_prestate.c",
    "tests/daemon/lxp_test_arbiter_prestate_transport.c",
    "tests/daemon/lxp_test_asset_execution_prestate.c",
    "tests/daemon/lxp_test_asset_inventory.c",
    "tests/daemon/lxp_test_program_replay.c",
    "tests/daemon/lxp_test_replay_authority.c",
    "tests/daemon/lxp_test_replay_catalogue.c",
)


def test_daemon_tests_batch_wal_leaves_the_build():
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
