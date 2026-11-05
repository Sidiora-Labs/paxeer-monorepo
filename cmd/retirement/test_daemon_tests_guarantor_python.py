import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/availability-settlement.py",
    "tests/daemon/guarantor-authorization.py",
    "tests/daemon/guarantor-exchange.py",
    "tests/daemon/guarantor-feedback.py",
    "tests/daemon/guarantor-integration.py",
    "tests/daemon/guarantor-publication-chain.py",
    "tests/daemon/guarantor-publication.py",
    "tests/daemon/guarantor-settlement-chain.py",
    "tests/daemon/guarantor-settlement.py",
)


def test_daemon_tests_guarantor_python_leaves_the_build():
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
