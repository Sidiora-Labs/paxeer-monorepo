import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/lxp_test_epoch_budget_rollback.c",
    "tests/daemon/lxp_test_epoch_escrow_timeout.c",
    "tests/daemon/lxp_test_epoch_modules.h",
    "tests/daemon/lxp_test_epoch_service_acceptance.c",
    "tests/daemon/lxp_test_module_custody.c",
    "tests/daemon/lxp_test_budget_lifecycle.c",
)


def test_daemon_tests_epoch_leaves_the_build():
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
