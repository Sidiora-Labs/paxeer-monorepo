import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/lxp_test_program_admission.c",
    "tests/daemon/lxp_test_program_artifacts.c",
    "tests/daemon/program-admission.sh",
    "tests/daemon/metered-artifacts.py",
    "tests/daemon/lxp_test_metered_allowance.c",
    "tests/daemon/lxp_test_grant_issuance.c",
    "tests/daemon/grant-fixtures.py",
    "tests/daemon/lxp_test_native_onboarding.c",
    "tests/daemon/post-lxip.py",
    "tests/daemon/governance_lifecycle.py",
    "tests/daemon/lxp_test_paid_withdrawal.c",
)


def test_daemon_tests_program_admission_leaves_the_build():
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
