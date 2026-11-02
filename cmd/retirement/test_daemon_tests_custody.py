import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/custody-chain-contract.py",
    "tests/daemon/custody_chain.py",
    "tests/daemon/withdraw-custody.py",
    "tests/daemon/movement_proof.py",
    "tests/daemon/owner-production.py",
    "tests/daemon/owner_checkpoint.py",
    "tests/daemon/lxp_test_owner_rotation.c",
    "tests/daemon/lxp_test_replica_prefix.c",
    "tests/daemon/retain-replica-prefix.py",
)


def test_daemon_tests_custody_leaves_the_build():
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
