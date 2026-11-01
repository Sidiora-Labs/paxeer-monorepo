import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/paxeer_x_checkpoint_retry.py",
    "tests/daemon/paxeer_x_finality_fixture.py",
    "tests/daemon/paxeer_x_finality_recordings.py",
    "tests/daemon/paxeer_x_historical_finality.py",
    "tests/daemon/paxeer_x_kernel_readiness.py",
    "tests/daemon/paxeer_x_runtime_fixture.py",
    "tests/daemon/paxeer_x_runtime_fixture_test.py",
    "tests/daemon/paxeer_x_worker_configuration.py",
    "tests/daemon/lxp_test_runtime_fixture.c",
)


def test_daemon_tests_runtime_fixture_leaves_the_build():
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
