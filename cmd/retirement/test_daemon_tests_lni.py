from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "tests/daemon/lni-account.py",
    "tests/daemon/lxp_test_lni_account.c",
    "tests/daemon/lxp_test_lni_head_attestation.c",
    "tests/daemon/lxp_test_lni_module.c",
    "tests/daemon/lxp_test_daemon_allowance.c",
)


def test_daemon_tests_lni_leaves_the_build():
    result = subprocess.run(
        ["make", "-n", "build", "test"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    for path in RETIRED_PATHS:
        assert path not in output, f"Retired path remains in the build: {path}\n{output}"
        assert not (ROOT / path).exists(), f"Retired path still exists: {path}"
        assert not (ROOT / path).is_symlink(), f"Retired path is a symlink: {path}"
