import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def _assert_lockfile_resolves_locked(directory):
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--format-version",
            "1",
            "--manifest-path",
            f"services/agent/{directory}/Cargo.toml",
        ],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert result.returncode == 0, f"{directory}: {result.stderr}"


def test_fuzz_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("fuzz")


def test_qualification_pay2_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("qualification/pay2")


def test_tests_boundary_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/boundary")


def test_tests_isolation_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/isolation")


def test_tests_parity_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/parity")


def test_tests_qualify_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/qualify")


def test_tools_audit_verify_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tools/audit-verify")


def test_tools_policy_harness_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tools/policy-harness")
