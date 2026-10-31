import subprocess

from retired_tests import (
    REPO_ROOT,
    assert_paths_absent,
    assert_symbols_unreferenced,
)


def test_state_withdraw_web_retired():
    assert_paths_absent(["tests/state/lxp_test_state_diff.c"])
    assert_symbols_unreferenced(
        [
            "lxp_checkpoint_certificate",
            "lxp_guarantor_ctx",
            "lxp_guarantor_attestation",
            "lxp_guarantor_key_record",
            "lxp_guarantor_cert",
            "lxp_guarantor_attest",
            "lxp_guarantor_cert_assemble",
            "lxp_checkpoint_certificate_hash",
        ],
        ["tests/test_bridge_withdraw.c"],
    )
    assert_symbols_unreferenced(
        [
            "lx_web_observation_encode",
            "lx_batch_header",
            "lx_web_root",
            "lx_batch_header_set_web_root",
        ],
        ["tests/test_web_program_path.c"],
    )
    result = subprocess.run(
        ["make", "-C", REPO_ROOT, "build/tests/test_web_program_path"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"make -C {REPO_ROOT} build/tests/test_web_program_path exited with code "
        f"{result.returncode}:\n{result.stdout}\n{result.stderr}"
    )
