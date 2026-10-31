import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/bootstrap-send.sh",
    "tests/daemon/finality-authority-chain.py",
    "tests/daemon/finality-authority-chain.sh",
    "tests/daemon/fixtures/finality-authority/rpc.json",
    "tests/daemon/handover-authority.py",
    "tests/daemon/handover-chain.py",
    "tests/daemon/handover-peers.py",
    "tests/daemon/handover-programs.py",
    "tests/daemon/lxp_test_finality_authority.c",
    "tests/daemon/lxp_test_finality_json.c",
)


def test_daemon_tests_handover_finality_leaves_the_build():
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
